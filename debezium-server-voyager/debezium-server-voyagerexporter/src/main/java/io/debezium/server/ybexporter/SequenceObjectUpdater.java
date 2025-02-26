/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import org.eclipse.microprofile.config.Config;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import java.util.ArrayList;
import java.util.HashMap;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;

public class SequenceObjectUpdater {
    private static final Logger LOGGER = LoggerFactory.getLogger(SequenceObjectUpdater.class);
    public static String propertyName = "column_sequence.map";
    public static String initSequenceMaxpropertyName = "sequence.max.map";
    String dataDirStr;
    String sourceType;
    Map<String, Map<String, Map<String, String>>> columnSequenceMap; // Schema:table:column -> sequence
    ConcurrentMap<String, Long> sequenceMax;
    ExportStatus es;
    public SequenceObjectUpdater(String dataDirStr, String sourceType, String columnSequenceMapString, String initSequenceMapString, ConcurrentMap<String, Long> sequenceMax){
        this.dataDirStr = dataDirStr;
        this.sourceType = sourceType;
        this.columnSequenceMap = new HashMap<>();

        es = ExportStatus.getInstance(dataDirStr);

        initSequenceMax(initSequenceMapString, sequenceMax);
        initColumnSequenceMap(columnSequenceMapString);

        es.setSequenceMaxMap(this.sequenceMax);
    }

    // columnSequenceMapstring: public.t1.id:public."t1_id_seq",public.cst1.id:public."cSS1"
    public void initColumnSequenceMap(String columnSequenceMapString){
        if (columnSequenceMapString == null){
            return;
        }
        String[] columnSequenceItems = columnSequenceMapString.split(",");
        for (String columnSequenceStr: columnSequenceItems) {
            String[] colSequence = columnSequenceStr.split(":");
            if (colSequence.length != 2) {
                throw new RuntimeException("Incorrect config. Please provide comma separated list of " +
                        "'col:seq' with their fully qualified names.");
            }
            String fullyQualifiedColumnName = colSequence[0];
            String sequenceName = colSequence[1];
            String[] columnSplit = fullyQualifiedColumnName.split("\\.");
            if (columnSplit.length != 3){
                throw new RuntimeException("Incorrect format for column name in config param -" + columnSequenceStr +
                        ". Use <schema>.<table>.<column> (<database>.<table>.<name> for mysql) instead.");
            }
            insertIntoColumnSequenceMap(columnSplit[0], columnSplit[1], columnSplit[2], sequenceName);
            // add a base entry to sequenceMax. If already populated, do not update.
            this.sequenceMax.put(sequenceName, this.sequenceMax.getOrDefault(sequenceName, (long)0));
        }
    }

    // initSequenceMapString: public."cSS1":4,public."t1_id_seq":9
    public void initSequenceMax(String initSequenceMapString, ConcurrentMap<String, Long> sequenceMax){
        this.sequenceMax = new ConcurrentHashMap<>();
        if (initSequenceMapString != null){
            String[] sequenceMaxValsItems = initSequenceMapString.split(",");
            for (String sequenceMaxValsStr: sequenceMaxValsItems) {
                String[] sequenceMaxVals = sequenceMaxValsStr.split(":");
                if (sequenceMaxVals.length != 2) {
                    throw new RuntimeException("Incorrect config. Please provide comma separated list of " +
                            "'seq:value' with their fully qualified names.");
                }
                String sequenceName = sequenceMaxVals[0];
                Long sequenceMaxVal = Long.parseLong(sequenceMaxVals[1]);
                this.sequenceMax.put(sequenceName, sequenceMaxVal);
            }
        }
        if (sequenceMax != null){
            for (var entry : sequenceMax.entrySet()) {
                this.sequenceMax.put(entry.getKey(), entry.getValue());
            }
        }
    }

    private void insertIntoColumnSequenceMap(String schemaOrDb, String table, String column, String sequence){
        Map<String, Map<String, String>> schemaOrDbMap = this.columnSequenceMap.get(schemaOrDb);
        if (schemaOrDbMap == null){
            schemaOrDbMap = new HashMap<>();
            this.columnSequenceMap.put(schemaOrDb, schemaOrDbMap);
        }

        Map<String, String> schemaTableMap = schemaOrDbMap.get(table);
        if (schemaTableMap == null){
            schemaTableMap = new HashMap<>();
            schemaOrDbMap.put(table, schemaTableMap);
        }
        schemaTableMap.put(column, sequence);
    }

    private String getFromColumnSequenceMap(String schemaOrDb, String table, String column){
        Map<String, Map<String, String>> schemaMap = this.columnSequenceMap.get(schemaOrDb);
        if (schemaMap == null){
            return null;
        }

        Map<String, String> schemaTableMap = schemaMap.get(table);
        if (schemaTableMap == null){
            return null;
        }
        return schemaTableMap.get(column);
    }

    public void processRecord(Record r){
        if (r.op.equals("d")){
            // not processing delete events because the max would have already been updated in the
            // prior create/update events.
            return;
        }
        for(int i = 0; i < r.afterValueColumns.size(); i++){
            String schemaOrDbname;
            if (sourceType.equals("mysql")){
                schemaOrDbname = r.t.dbName;
            }
            else {
                schemaOrDbname = r.t.schemaName;
            }
            String seqName = getFromColumnSequenceMap(schemaOrDbname, r.t.tableName, r.afterValueColumns.get(i));
            if (seqName != null){
                Long columnValue = null;
                String value = r.afterValueValues.get(i).toString();
                try {
                    columnValue = Long.valueOf(value);
                    sequenceMax.put(seqName, Math.max(sequenceMax.get(seqName), columnValue));
                } catch (NumberFormatException e) {
                    /*
                    Skipping the sequences that are on non-Integer columns
                    the case where Table has a text column and its default is generating a string which include the sequence nextval
                    e.g. CREATE TABLE test(user_id text DEFAULT 'USR' || LPAD(nextval('user_code_seq')::TEXT, 4, '0'), user_name text);  
                    For the above table the row will have USR0001 column value for user_id column 
                    and while converting this USR0001 to Long will error out with NumberFormatException as its not a number
                    */
                    LOGGER.debug("Skipping unsupported sequence with non-interger value: '{}'", seqName);
                }
            }
        }

    }

}
