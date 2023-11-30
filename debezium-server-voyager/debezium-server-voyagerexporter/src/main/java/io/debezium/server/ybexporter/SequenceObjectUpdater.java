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
    String dataDirStr;
    String sourceType;
    Map<String, Map<String, Map<String, String>>> columnSequenceMap; // Schema:table:column -> sequence
    ConcurrentMap<String, Long> sequenceMax;
    ExportStatus es;
    public SequenceObjectUpdater(String dataDirStr, String sourceType, String columnSequenceMapString, ConcurrentMap<String, Long> sequenceMax){
        this.dataDirStr = dataDirStr;
        this.sourceType = sourceType;
        this.columnSequenceMap = new HashMap<>();

        es = ExportStatus.getInstance(dataDirStr);
        if (sequenceMax == null){
           this.sequenceMax = new ConcurrentHashMap<>();
        }
        else{
            this.sequenceMax = sequenceMax;
        }
        initColumnSequenceMap(columnSequenceMapString);
        es.setSequenceMaxMap(this.sequenceMax);
    }

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
        for(int i = 0; i < r.valueColumns.size(); i++){
            String schemaOrDbname;
            if (sourceType.equals("mysql")){
                schemaOrDbname = r.t.dbName;
            }
            else {
                schemaOrDbname = r.t.schemaName;
            }
            String seqName = getFromColumnSequenceMap(schemaOrDbname, r.t.tableName, r.valueColumns.get(i));
            if (seqName != null){
                Long columnValue = Long.valueOf(r.valueValues.get(i).toString());
                sequenceMax.put(seqName, Math.max(sequenceMax.get(seqName), columnValue));
            }
        }

    }

}
