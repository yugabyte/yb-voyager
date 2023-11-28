/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.util.Collections;
import java.util.HashMap;
import java.util.Map;
import java.util.Objects;

import org.apache.kafka.connect.data.Field;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.json.JsonConverterConfig;
import org.apache.kafka.connect.source.SourceRecord;
import org.eclipse.microprofile.config.Config;
import org.eclipse.microprofile.config.ConfigProvider;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

class KafkaConnectRecordParser implements RecordParser {
    private static final Logger LOGGER = LoggerFactory.getLogger(KafkaConnectRecordParser.class);
    private final ExportStatus es;
    String dataDirStr;
    String sourceType;
    private Map<String, Table> tableMap;
    private JsonConverter jsonConverter;
    private Map<String, String> renameTables;
    Record r = new Record();

    public KafkaConnectRecordParser(String dataDirStr, String sourceType, Map<String, Table> tblMap) {
        this.dataDirStr = dataDirStr;
        this.sourceType = sourceType;
        es = ExportStatus.getInstance(dataDirStr);
        tableMap = tblMap;
        jsonConverter = new JsonConverter();
        Map<String, String> jsonConfig = Collections.singletonMap(JsonConverterConfig.SCHEMAS_ENABLE_CONFIG, "false");
        jsonConverter.configure(jsonConfig, false);
        renameTables = new HashMap<>();
        retrieveRenameTablesFromConfig();
    }

    private void retrieveRenameTablesFromConfig(){
        final Config config = ConfigProvider.getConfig();
        String renameTablesConfig = config.getOptionalValue("debezium.sink.ybexporter.tables.rename", String.class).orElse("");
        if (!renameTablesConfig.isEmpty()){
            for (String renameTableConfig: renameTablesConfig.split(",")){
                String[] beforeAndAfter = renameTableConfig.split(":");
                if (beforeAndAfter.length != 2){
                    throw  new RuntimeException(String.format("Incorrect format for specifying table rename config %s. Provide it as <oldname>:<newname>", renameTableConfig));
                }
                String before = beforeAndAfter[0];
                String after = beforeAndAfter[1];
                if ((before.split("\\.").length != 2) && (!sourceType.equals("mysql"))){
                    throw  new RuntimeException(String.format("Incorrect format for specifying table rename config %s. Provide it as <schema>.<tableName>", before));
                }
                if ((after.split("\\.").length != 2) && (!sourceType.equals("mysql"))){
                    throw  new RuntimeException(String.format("Incorrect format for specifying table rename config %s. Provide it as <schema>.<tableName>", after));
                }
                renameTables.put(before, after);
            }
        }
    }

    /**
     * This parser parses a kafka connect SourceRecord object to a Record object
     * that contains the relevant field schemas and values.
     */
    @Override
    public Record parseRecord(Object keyObj, Object valueObj) {
        try {
            r.clear();

            // Deserialize to Connect object
            Struct value = (Struct) ((SourceRecord) valueObj).value();
            Struct key = (Struct) ((SourceRecord) valueObj).key();

            if (value == null) {
                // Ideally, we should have config tombstones.on.delete=false. In case that is not set correctly,
                // we will get those events where value field = null. Skipping those events.
                LOGGER.warn("Empty value field in event. Assuming tombstone event. Skipping - {}", valueObj);
                r.op = "unsupported";
                return r;
            }
            Struct source = value.getStruct("source");
            r.op = value.getString("op");
            r.snapshot = source.getString("snapshot");

            // Parse table/schema the first time to be able to format specific field values
            parseTable(value, source, r);

            // Parse key and values
            if (key != null) {
                parseKeyFields(key, r);
            }
            parseValueFields(value, r);

            return r;
        }
        catch (Exception ex) {
            LOGGER.error("Failed to parse msg: {}", ex);
            throw new RuntimeException(ex);
        }
    }

    protected void parseTable(Struct value, Struct sourceNode, Record r) {
        String dbName = sourceNode.getString("db");
        String schemaName = "";
        if (sourceNode.schema().field("schema") != null) {
            schemaName = sourceNode.getString("schema");
        }
        String tableName = sourceNode.getString("table");
        // rename table name
        String qualifiedTableName = tableName;
        if (!schemaName.equals("")){
            qualifiedTableName = schemaName + "." + tableName;
        }
        if (renameTables.containsKey(qualifiedTableName)){
            String[] renamedTableName = renameTables.get(qualifiedTableName).split("\\.");
            // TODO: support MySQL
            schemaName = renamedTableName[0];
            tableName = renamedTableName[1];
        }
        var tableIdentifier = dbName + "-" + schemaName + "-" + tableName;

        Table t = tableMap.get(tableIdentifier);
        if (t == null) {
            // create table
            t = new Table(dbName, schemaName, tableName);

            // parse fields
            Struct structWithAllFields = value.getStruct("after");
            if (structWithAllFields == null) {
                // in case of delete events the after field is empty, and the before field is populated.
                structWithAllFields = value.getStruct("before");
            }
            for (Field f : structWithAllFields.schema().fields()) {
                if (sourceType.equals("yb")){
                    // values in the debezium connector are as follows:
                    // "val1" : {
                    //  "value" : "value for val1 column",
                    //  "set" : true
                    //}
                    // Therefore, we need to get the schema of the inner value field, but name of the outer field
                    t.fieldSchemas.put(f.name(), new Field(f.name(), 0, f.schema().field("value").schema()));
                }
                else {
                    t.fieldSchemas.put(f.name(), f);
                }
            }

            tableMap.put(tableIdentifier, t);
            es.updateTableSchema(t);
        }
        r.t = t;
    }

    protected void parseKeyFields(Struct key, Record r) {
        for (Field f : key.schema().fields()) {
            Object fieldValue;
            if (sourceType.equals("yb")){
                // values in the debezium connector are as follows:
                // "val1" : {
                //  "value" : "value for val1 column",
                //  "set" : true
                //}
                Struct valueAndSet = key.getStruct(f.name());
                if (!valueAndSet.getBoolean("set")){
                    continue;
                }
                fieldValue = valueAndSet.get("value");
            }
            else{
                fieldValue = key.get(f);
            }
            r.addKeyField(f.name(), fieldValue);

        }
    }

    /**
     * Parses value fields from the msg.
     * In case of update operation, only stores the fields that have changed by comparing
     * the before and after structs.
     */
    protected void parseValueFields(Struct value, Record r) {
        Struct after = value.getStruct("after");
        // TODO: error handle before is NULL
        Struct before = value.getStruct("before");
        if (after == null) {
            return;
        }
        for (Field f : after.schema().fields()) {
            Object fieldValue;
            if (sourceType.equals("yb")){
                // TODO: write a proper transformer for this logic
                // values in the debezium connector are as follows:
                // "val1" : {
                //  "value" : "value for val1 column",
                //  "set" : true
                //}
                Struct valueAndSet = after.getStruct(f.name());
                if (r.op.equals("u")) {
                    // in the default configuration of the stream, for an update, the fields in the after struct
                    // are only the delta fields, therefore, it is possible for a field to not be there.
                    if (valueAndSet == null){
                        continue;
                    }
                }

                if (!valueAndSet.getBoolean("set")){
                    continue;
                }
                fieldValue = valueAndSet.get("value");
            }
            else{
                if (r.op.equals("u")) {
                    if (Objects.equals(after.get(f), before.get(f))) {
                        // no need to record this as field is unchanged
                        continue;
                    }
                }
                fieldValue = after.get(f);
            }

            r.addValueField(f.name(), fieldValue);
        }
    }



}
