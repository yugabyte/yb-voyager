package io.debezium.server.ybexporter;

import com.fasterxml.jackson.databind.DeserializationFeature;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.util.List;

public class MigrationStatusRecord {
    public String MigrationUUID;
    public String SourceDBType;
    public String ExportType;
    public boolean FallForwarDBExists;
    public List<String> TableListExportedFromSource;
    public boolean CutoverToTargetRequested;;
    public boolean CutoverProcessedBySourceExporter;
    public boolean CutoverProcessedByTargetImporter;
    public boolean ExportFromTargetFallForwardStarted;
    public boolean CutoverToSourceReplicaRequested;
    public boolean CutoverToSourceReplicaProcessedByTargetExporter;
    public boolean CutoverToSourceReplicaProcessedBySRImporter;
    public boolean CutoverToSourceRequested;
    public boolean CutoverToSourceProcessedByTargetExporter;
    public boolean CutoverToSourceProcessedBySourceImporter;
    public boolean EndMigrationRequested;
    public boolean ExportSchemaDone;
    public boolean ExportDataDone;

    public static MigrationStatusRecord fromJsonString(String jsonString) {
        ObjectMapper objectMapper = new ObjectMapper();
        objectMapper.configure(DeserializationFeature.FAIL_ON_UNKNOWN_PROPERTIES, false);
        try {
            return objectMapper.readValue(jsonString, MigrationStatusRecord.class);
        }
        catch (Exception e) {
            throw new RuntimeException(e);
        }
    }
}
