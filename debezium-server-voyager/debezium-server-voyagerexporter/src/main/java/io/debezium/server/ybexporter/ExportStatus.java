/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.sql.Connection;
import java.sql.DriverManager;
import java.sql.ResultSet;
import java.sql.SQLException;
import java.sql.Statement;
import java.time.LocalDateTime;
import java.time.ZoneOffset;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import java.util.concurrent.ConcurrentMap;

import com.fasterxml.jackson.annotation.JsonAutoDetect;
import com.fasterxml.jackson.annotation.PropertyAccessor;
import org.apache.kafka.connect.data.Field;
import org.eclipse.microprofile.config.Config;
import org.eclipse.microprofile.config.ConfigProvider;
import org.graalvm.collections.Pair;
import org.json.JSONObject;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.ObjectWriter;
import org.sqlite.SQLiteConfig;

import static java.nio.file.StandardCopyOption.REPLACE_EXISTING;

/**
 * Singleton class that is used to update the status of the export process.
 */

public class ExportStatus {
    private static final Logger LOGGER = LoggerFactory.getLogger(ExportStatus.class);
    private static final String EXPORT_STATUS_FILE_NAME = "export_status.json";
    private String MIGRATION_STATUS_KEY = "migration_status";
    private static ExportStatus instance;
    private static ObjectMapper mapper = new ObjectMapper(new JsonFactory());
    private String dataDir;
    private String sourceType;
    private Integer schemaCount;
    private ConcurrentMap<String, Long> sequenceMax;
    private ConcurrentMap<Table, TableExportStatus> tableExportStatusMap = new ConcurrentHashMap<>();
    private ExportMode mode;
    private ObjectWriter ow;
    private File f;
    private File tempf;
    private String metadataDBPath;
    private String runId;
    private String exporterRole;
    private Connection metadataDBConn;
    private static String QUEUE_SEGMENT_META_TABLE_NAME = "queue_segment_meta";
    private static String EVENT_STATS_TABLE_NAME = "exported_events_stats";
    private static String EVENT_STATS_PER_TABLE_TABLE_NAME = "exported_events_stats_per_table";
    private static String JSON_OBJECTS_TABLE_NAME = "json_objects";


    /**
     * Should only be called once in the lifetime of the process.
     * Creates and instance and assigns it to static instance property of the class.
     */
    private ExportStatus(String datadirStr) {
        //
        if (instance != null) {
            throw new RuntimeException("instance already exists.");
        }
        dataDir = datadirStr;
        ow = new ObjectMapper().writer().withDefaultPrettyPrinter();
        f = new File(getFilePath(datadirStr));

        // open connection to metadataDB
        // TODO: interpret config vars once and make them globally available to all classes
        final Config config = ConfigProvider.getConfig();
        runId = config.getValue("debezium.sink.ybexporter.run.id", String.class);
        exporterRole = config.getValue("debezium.sink.ybexporter.exporter.role", String.class);
        String schemas = config.getOptionalValue("debezium.source.schema.include.list", String.class).orElse("");
        schemaCount = schemas.split(",").length;

        metadataDBPath = config.getValue("debezium.sink.ybexporter.metadata.db.path", String.class);
        if (metadataDBPath == null){
            throw new RuntimeException("please provide value for debezium.sink.ybexporter.metadata.db.path.");
        }
        metadataDBConn = null;
        try {
            String url = "jdbc:sqlite:" + metadataDBPath;
            SQLiteConfig sqLiteConfig = new SQLiteConfig();
            sqLiteConfig.setTransactionMode(SQLiteConfig.TransactionMode.EXCLUSIVE);
            sqLiteConfig.setBusyTimeout(30000);
            metadataDBConn = DriverManager.getConnection(url, sqLiteConfig.toProperties());
            LOGGER.info("Connected to metadata db at {}", metadataDBPath);
        } catch (SQLException e) {
            throw new RuntimeException(String.format("Couldn't connect to metadata DB at %s", metadataDBPath), e);
        }

        // mkdir schemas
        File schemasRootDir = new File(String.format("%s/%s", dataDir, "schemas"));
        if (!schemasRootDir.exists()){
            boolean dirCreated = new File(String.format("%s/%s", dataDir, "schemas")).mkdir();
            if (!dirCreated){
                throw new RuntimeException("failed to create dir for schemas");
            }
        }
        File schemasDir = new File(String.format("%s/%s/%s", dataDir, "schemas", exporterRole));
        if (!schemasDir.exists()){
            boolean dirCreated = schemasDir.mkdir();
            if (!dirCreated){
                throw new RuntimeException("failed to create dir for schemas");
            }
        }

        instance = this;
    }

    /**
     * 1. Tries to check if an instance is already created
     * 2. If not, tries to load from disk.
     * 3. If not available on disk, creates a new instance.
     * @param datadirStr - data directory
     * @return instance
     */
    public static ExportStatus getInstance(String datadirStr) {
        if (instance != null) {
            return instance;
        }
        ExportStatus instanceFromDisk = loadFromDisk(datadirStr);
        if (instanceFromDisk != null) {
            instance = instanceFromDisk;
            return instance;
        }
        return new ExportStatus(datadirStr);
    }

    public ExportMode getMode() {
        return mode;
    }

    public void updateTableSchema(Table t){
        ObjectMapper schemaMapper = new ObjectMapper();
        schemaMapper.setVisibility(PropertyAccessor.FIELD, JsonAutoDetect.Visibility.ANY);
        ObjectWriter schemaWriter = schemaMapper.writer().withDefaultPrettyPrinter();

        HashMap<String, Object> tableSchema = new HashMap<>();
        ArrayList<Field> fields = new ArrayList<>(t.fieldSchemas.values());
        tableSchema.put("columns", fields);
        try {
            String fileName = t.tableName;
            if ((sourceType.equals("postgresql")) && (!t.schemaName.equals("public"))){
                fileName = t.schemaName + "." + fileName;
            }
            String schemaFilePath = String.format("%s/schemas/%s/%s_schema.json", dataDir, exporterRole, fileName);
            File schemaFile = new File(schemaFilePath);
            schemaWriter.writeValue(schemaFile, tableSchema);
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    public void updateTableSnapshotWriterCreated(Table t, String tblFilename) {
        TableExportStatus tableExportStatus = new TableExportStatus(tableExportStatusMap.size(), tblFilename);
        tableExportStatusMap.put(t, tableExportStatus);
    }

    public void updateTableSnapshotRecordWritten(Table t) {
        tableExportStatusMap.get(t).exportedRowCountSnapshot++;
    }

    public void updateMode(ExportMode modeEnum) {
        mode = modeEnum;
    }

    public void setSequenceMaxMap(ConcurrentMap<String, Long> sequenceMax){
        this.sequenceMax = sequenceMax;
    }

    public ConcurrentMap<String, Long> getSequenceMaxMap(){
        return this.sequenceMax;
    }

    public synchronized void flushToDisk() {
        // TODO: do not create fresh objects every time, just reuse.
        HashMap<String, Object> exportStatusMap = new HashMap<>();
        List<HashMap<String, Object>> tablesInfo = new ArrayList<>();
        for (Map.Entry<Table, TableExportStatus> pair : tableExportStatusMap.entrySet()) {
            Table t = pair.getKey();
            TableExportStatus tes = pair.getValue();
            HashMap<String, Object> tableInfo = new HashMap<>();
            tableInfo.put("database_name", t.dbName);
            tableInfo.put("schema_name", t.schemaName);
            tableInfo.put("table_name", t.tableName);
            tableInfo.put("file_name", tes.snapshotFilename);
            tableInfo.put("exported_row_count_snapshot", tes.exportedRowCountSnapshot);
            tableInfo.put("sno", tes.sno);
            tablesInfo.add(tableInfo);
        }

        exportStatusMap.put("source_type", sourceType);
        exportStatusMap.put("tables", tablesInfo);
        exportStatusMap.put("mode", mode);
        exportStatusMap.put("sequences", sequenceMax);

        try {
            // for atomic write, we write to a temp file, and then
            // rename to destination file path. This prevents readers from reading the file in a corrupted
            // state (for example, when the complete file has not been written)
            tempf = new File(getTempFilePath());
            ow.writeValue(tempf, exportStatusMap);
            Files.move(tempf.toPath(), f.toPath(), REPLACE_EXISTING);
        }
        catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    private static String getFilePath(String dataDirStr){
        return String.format("%s/%s", dataDirStr, EXPORT_STATUS_FILE_NAME);
    }

    private String getTempFilePath(){
        return getFilePath(dataDir) + ".tmp";
    }

    private static ExportStatus loadFromDisk(String datadirStr) {
        try {
            Path p = Paths.get(getFilePath(datadirStr));
            File f = new File(p.toUri());
            if (!f.exists()) {
                return null;
            }

            String fileContent = Files.readString(p);
            var exportStatusJson = mapper.readTree(fileContent);
            LOGGER.info("Loaded export status info from disk = {}", exportStatusJson);

            ExportStatus es = new ExportStatus(datadirStr);
            es.updateMode(ExportMode.valueOf(exportStatusJson.get("mode").asText()));
            es.setSourceType(exportStatusJson.get("source_type").asText());

            var tablesJson = exportStatusJson.get("tables");
            for (var tableJson : tablesJson) {
                // TODO: creating a duplicate table here. it will again be created when parsing a record of the table for the first time.
                Table t = new Table(tableJson.get("database_name").asText(), tableJson.get("schema_name").asText(), tableJson.get("table_name").asText());

                TableExportStatus tes = new TableExportStatus(tableJson.get("sno").asInt(), tableJson.get("file_name").asText());
                tes.exportedRowCountSnapshot = tableJson.get("exported_row_count_snapshot").asLong();
                es.tableExportStatusMap.put(t, tes);
            }
            var sequencesJson = exportStatusJson.get("sequences");
            var sequencesIterator = sequencesJson.fields();
            ConcurrentHashMap<String, Long> sequenceMaxMap = new ConcurrentHashMap<>();
            while (sequencesIterator.hasNext()){
                var entry = sequencesIterator.next();
                sequenceMaxMap.put(entry.getKey(), Long.valueOf(entry.getValue().asText()));
            }
            es.setSequenceMaxMap(sequenceMaxMap);

            return es;
        }
        catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    // TODO: refactor to retrieve config from a static class instead of having to set/pass it to each class.
    public void setSourceType(String sourceType) {
        this.sourceType = sourceType;
    }

    public void updateQueueSegmentMetaInfo(long segmentNo, long committedSize, Map<Pair<String, String>, Map<String, Long>> eventCountDeltaPerTable) throws SQLException {
        final boolean oldAutoCommit = metadataDBConn.getAutoCommit();
        metadataDBConn.setAutoCommit(false);
        int updatedRows;
        try {
            Statement queueMetaUpdateStmt = metadataDBConn.createStatement();
            updatedRows = queueMetaUpdateStmt.executeUpdate(String.format("UPDATE %s SET size_committed = %d WHERE segment_no=%d", QUEUE_SEGMENT_META_TABLE_NAME, committedSize, segmentNo));
            if (updatedRows != 1){
                throw new RuntimeException(String.format("Update of queue segment metadata failed with query-%s, rowsAffected -%d", queueMetaUpdateStmt, updatedRows));
            }
            queueMetaUpdateStmt.close();
            updateEventsStats(metadataDBConn, eventCountDeltaPerTable);

        } catch (SQLException e) {
            metadataDBConn.rollback();
            throw new RuntimeException(String.format("Failed to  update queue segment meta and stats " +
                    "- segmentNo: %d, committedSize:%d, eventCount:%s", segmentNo, committedSize, eventCountDeltaPerTable), e);
        } finally {
            metadataDBConn.commit();
            metadataDBConn.setAutoCommit(oldAutoCommit);
        }

    }

    public boolean checkIfSwitchOperationRequested(String operation) throws SQLException {
        Statement selectStmt = metadataDBConn.createStatement();
        String query = String.format("SELECT json_text from %s where key = '%s'",
            JSON_OBJECTS_TABLE_NAME, MIGRATION_STATUS_KEY);
        try {
            ResultSet rs = selectStmt.executeQuery(query);
            while (rs.next()) {
                MigrationStatusRecord msr = MigrationStatusRecord.fromJsonString(rs.getString("json_text"));
                switch(operation.toString()) {
                    case "cutover":
                        return msr.CutoverRequested;
                    case "fallforward":
                        return msr.FallForwardSwitchRequested;
                    case "fallback":
                        return msr.FallBackSwitchRequested;
                }
            }
        } catch (SQLException e) {
            throw e;
        } finally {
            selectStmt.close();
        }
        return false;
    }

    public boolean checkifEndMigrationRequested() throws SQLException {
        Statement selectStmt = metadataDBConn.createStatement();
        String query = String.format("SELECT json_text from %s where key = '%s'",
                JSON_OBJECTS_TABLE_NAME, MIGRATION_STATUS_KEY);
        try {
            ResultSet rs = selectStmt.executeQuery(query);
            while (rs.next()) {
                MigrationStatusRecord msr = MigrationStatusRecord.fromJsonString(rs.getString("json_text"));
                return msr.EndMigrationRequested;
            }
        } catch (SQLException e) {
            throw e;
        } finally {
            selectStmt.close();
        }
        return false;
    }

    private void updateEventsStats(Connection conn, Map<Pair<String, String>, Map<String, Long>> eventCountDeltaPerTable) throws SQLException {
        int updatedRows;

        long numTotalDelta = 0;
        long numInsertsDelta = 0;
        long numUpdatesDelta = 0;
        long numDeletesDelta = 0;

        // update stats per table
        for (var entry : eventCountDeltaPerTable.entrySet()) {
            Statement tableWiseStatsUpdateStmt = conn.createStatement();
            Pair<String, String> tableQualifiedName = entry.getKey();
            String schemaName = tableQualifiedName.getLeft();
            if (schemaCount <= 1){
                schemaName = ""; // in case there is only one schema in question, we need not fully qualify the table name.
            }
            Map<String, Long> eventCountDeltaTable = entry.getValue();
            Long numTotalDeltaTable = eventCountDeltaTable.getOrDefault("c", 0L) + eventCountDeltaTable.getOrDefault("u", 0L) + eventCountDeltaTable.getOrDefault("d", 0L);
            String updateQuery = String.format("UPDATE %s set num_total = num_total + %d," +
                            " num_inserts = num_inserts + %d," +
                            " num_updates = num_updates + %d," +
                            " num_deletes = num_deletes + %d" +
                            " WHERE schema_name = '%s' and table_name = '%s' AND exporter_role = '%s'", EVENT_STATS_PER_TABLE_TABLE_NAME,
                    numTotalDeltaTable,
                    eventCountDeltaTable.getOrDefault("c", 0L),
                    eventCountDeltaTable.getOrDefault("u", 0L),
                    eventCountDeltaTable.getOrDefault("d", 0L),
                    schemaName,
                    tableQualifiedName.getRight(),
                    exporterRole);
            updatedRows = tableWiseStatsUpdateStmt.executeUpdate(updateQuery);
            if (updatedRows == 0){
                // need to insert for the first time
                Statement insertStatment = conn.createStatement();
                String insertQuery = String.format("INSERT INTO %s (exporter_role, schema_name, table_name, num_total, num_inserts, num_updates, num_deletes) " +
                                "VALUES('%s', '%s', '%s', %d, %d, %d, %d)", EVENT_STATS_PER_TABLE_TABLE_NAME, exporterRole, schemaName, tableQualifiedName.getRight(),
                        numTotalDeltaTable,
                        eventCountDeltaTable.getOrDefault("c", 0L),
                        eventCountDeltaTable.getOrDefault("u", 0L),
                        eventCountDeltaTable.getOrDefault("d", 0L));
                insertStatment.executeUpdate(insertQuery);
            }
            else if (updatedRows != 1){
                throw new RuntimeException(String.format("Update of table wise stats failed with query-%s, rowsAffected -%d", updateQuery, updatedRows));
            }
            numTotalDelta += numTotalDeltaTable;
            numInsertsDelta += eventCountDeltaTable.getOrDefault("c", 0L);
            numUpdatesDelta += eventCountDeltaTable.getOrDefault("u", 0L);
            numDeletesDelta += eventCountDeltaTable.getOrDefault("d", 0L);
        }

        // update overall stats
        LocalDateTime now = LocalDateTime.now(ZoneOffset.UTC);
        LocalDateTime nowFlooredToNearest10s = now.minusSeconds(now.getSecond()%10);
        Long nowFlooredToNearest10sEpoch = nowFlooredToNearest10s.toEpochSecond(ZoneOffset.UTC);
        Statement updateStatement = conn.createStatement();
        String updateQuery = String.format("UPDATE %s set num_total = num_total + %d," +
                        " num_inserts = num_inserts + %d," +
                        " num_updates = num_updates + %d," +
                        " num_deletes = num_deletes + %d" +
                        " WHERE run_id = '%s' AND exporter_role='%s' AND timestamp = %d", EVENT_STATS_TABLE_NAME,
                numTotalDelta, numInsertsDelta, numUpdatesDelta, numDeletesDelta,
                runId, exporterRole, nowFlooredToNearest10sEpoch);
        updatedRows = updateStatement.executeUpdate(updateQuery);
        if (updatedRows == 0){
            // first insert for the minute.
            Statement insertStatment = conn.createStatement();
            String insertQuery = String.format("INSERT INTO %s (run_id, exporter_role, timestamp, num_total, num_inserts, num_updates, num_deletes) " +
                            "VALUES('%s', '%s', '%s', %d, %d, %d, %d)", EVENT_STATS_TABLE_NAME, runId, exporterRole, nowFlooredToNearest10sEpoch,
                    numTotalDelta, numInsertsDelta, numUpdatesDelta, numDeletesDelta);
            insertStatment.executeUpdate(insertQuery);
        }
        else if (updatedRows != 1){
            throw new RuntimeException(String.format("Update of stats failed with query-%s, rowsAffected -%d", updateQuery, updatedRows));
        }
    }

    public void queueSegmentCreated(long segmentNo, String segmentPath, String exporterRole){
        Statement insertStmt;
        try {
            insertStmt = metadataDBConn.createStatement();
            insertStmt.executeUpdate(String.format("INSERT OR IGNORE into %s (segment_no, file_path, size_committed, exporter_role) VALUES(%d, '%s', 0, '%s')",
                    QUEUE_SEGMENT_META_TABLE_NAME, segmentNo, segmentPath, exporterRole));
            insertStmt.close();
        } catch (SQLException e) {
            throw new RuntimeException(String.format("Failed to run update queue segment size " +
                    "- segmentNo: %d", segmentNo), e);
        }
    }

    public long getQueueSegmentCommittedSize(long segmentNo){
        Statement selectStmt;
        long sizeCommitted;
        try {
            selectStmt = metadataDBConn.createStatement();
            ResultSet rs = selectStmt.executeQuery(String.format("SELECT size_committed from %s where segment_no=%s", QUEUE_SEGMENT_META_TABLE_NAME, segmentNo));
            if (!rs.next()){
                throw new RuntimeException(String.format("Could not fetch committedSize for queue segment - %d", segmentNo));
            }
            sizeCommitted = rs.getLong("size_committed");
            selectStmt.close();
        } catch (SQLException e) {
            throw new RuntimeException(String.format("Failed to run update queue segment size " +
                    "- segmentNo: %d", segmentNo), e);
        }
        return sizeCommitted;
    }
}

class TableExportStatus {
    Integer sno;
    Long exportedRowCountSnapshot;
    String snapshotFilename;

    public TableExportStatus(Integer sno, String snapshotFilename){
        this.sno = sno;
        this.snapshotFilename = snapshotFilename;
        this.exportedRowCountSnapshot = 0L;
    }
}

enum ExportMode {
    SNAPSHOT,
    STREAMING,
}
