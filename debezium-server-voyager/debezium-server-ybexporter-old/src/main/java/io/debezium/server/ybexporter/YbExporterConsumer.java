/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.io.File;
import java.net.URISyntaxException;
import java.nio.file.Path;
import java.util.HashMap;
import java.util.List;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;
import jakarta.enterprise.context.Dependent;
import jakarta.inject.Named;
import jakarta.annotation.PostConstruct;
import org.eclipse.microprofile.config.Config;
import org.eclipse.microprofile.config.ConfigProvider;
import org.eclipse.microprofile.config.inject.ConfigProperty;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import io.debezium.engine.ChangeEvent;
import io.debezium.engine.DebeziumEngine;
import io.debezium.server.BaseChangeConsumer;

/**
 * Implementation of the consumer that exports the messages to file in a Yugabyte-compatible form.
 */
@Named("ybexporter")
@Dependent
public class YbExporterConsumer extends BaseChangeConsumer implements DebeziumEngine.ChangeConsumer<ChangeEvent<Object, Object>> {
    private static final Logger LOGGER = LoggerFactory.getLogger(YbExporterConsumer.class);
    private static final String PROP_PREFIX = "debezium.sink.ybexporter.";
    private static final String SOURCE_DB_EXPORTER_ROLE = "source_db_exporter";
    private static final String TARGET_DB_EXPORTER_ROLE = "target_db_exporter_fb";
    String snapshotMode;
    @ConfigProperty(name = PROP_PREFIX + "dataDir")
    String dataDir;
    String triggersDir;
    String sourceType;
    String exporterRole;
    private Map<String, Table> tableMap = new HashMap<>();
    private RecordParser parser;
    private Map<Table, RecordWriter> snapshotWriters = new ConcurrentHashMap<>();
    private RecordWriter eventQueue;
    private ExportStatus exportStatus;
    private SequenceObjectUpdater sequenceObjectUpdater;
    private RecordTransformer recordTransformer;
    Thread flusherThread;
    boolean shutDown = false;

    @PostConstruct
    void connect() throws URISyntaxException {
        LOGGER.info("connect() called: dataDir = {}", dataDir);
        final Config config = ConfigProvider.getConfig();

        snapshotMode = config.getOptionalValue("debezium.source.snapshot.mode", String.class).orElse("");
        retrieveSourceType(config);
        triggersDir = config.getValue(PROP_PREFIX + "triggers.dir", String.class);
        exporterRole = config.getValue("debezium.sink.ybexporter.exporter.role", String.class);

        exportStatus = ExportStatus.getInstance(dataDir);
        exportStatus.setSourceType(sourceType);
        if (exportStatus.getMode() == null) {
            exportStatus.updateMode(getExportModeToStartWith(snapshotMode));
        }
        if (exportStatus.getMode().equals(ExportMode.STREAMING)) {
            handleSnapshotComplete();
        }
        parser = new KafkaConnectRecordParser(dataDir, sourceType, tableMap);
        String propertyVal = PROP_PREFIX + SequenceObjectUpdater.propertyName;
        String columnSequenceMapString = config.getOptionalValue(propertyVal, String.class).orElse(null);
        sequenceObjectUpdater = new SequenceObjectUpdater(dataDir, sourceType, columnSequenceMapString, exportStatus.getSequenceMaxMap());
        recordTransformer = new DebeziumRecordTransformer();

        flusherThread = new Thread(this::flush);
        flusherThread.setDaemon(true);
        flusherThread.start();
    }

    private ExportMode getExportModeToStartWith(String snapshotMode) {
        if (snapshotMode.equals("never")) {
            return ExportMode.STREAMING;
        }
        else {
            return ExportMode.SNAPSHOT;
        }
    }

    void retrieveSourceType(Config config) {
        String sourceConnector = config.getValue("debezium.source.connector.class", String.class);
        switch (sourceConnector) {
            case "io.debezium.connector.postgresql.PostgresConnector":
                sourceType = "postgresql";
                break;
            case "io.debezium.connector.oracle.OracleConnector":
                sourceType = "oracle";
                break;
            case "io.debezium.connector.mysql.MySqlConnector":
                sourceType = "mysql";
                break;
            case "io.debezium.connector.yugabytedb.YugabyteDBConnector":
                sourceType = "yb";
                break;
            default:
                throw new RuntimeException("Invalid source type");
        }
    }

    void flush() {
        LOGGER.info("XXX Started flush thread.");
        String switchOperation;
        if (exporterRole.equals(SOURCE_DB_EXPORTER_ROLE)) {
            switchOperation = "cutover";
        }
        else if (exporterRole.equals(TARGET_DB_EXPORTER_ROLE)) {
            switchOperation = "fallforward";
        }
        else {
            throw new RuntimeException(String.format("invalid exportRole %s", exporterRole));
        }

        while (true) {
            for (RecordWriter writer : snapshotWriters.values()) {
                writer.flush();
                writer.sync();
            }
            // TODO: doing more than flushing files to disk. maybe move this call to another thread?
            if (exportStatus != null) {
                exportStatus.flushToDisk();
            }

            checkForSwitchOperationAndHandle(switchOperation);
            try {
                Thread.sleep(2000);
            }
            catch (InterruptedException e) {
                // Noop.
            }
        }
    }

    private void checkForSwitchOperationAndHandle(String operation) {
        File switchTriggerFile = new File(Path.of(triggersDir, operation).toString());
        if (!switchTriggerFile.exists()) {
            return;
        }
        LOGGER.info("Observed {} trigger file. Cutting over...", operation);
        Record switchOperationRecord = new Record();
        switchOperationRecord.op = operation;
        switchOperationRecord.t = new Table(null, null, null); // just to satisfy being a proper Record object.
        synchronized (eventQueue) { // need to synchronize with handleBatch
            eventQueue.writeRecord(switchOperationRecord);
            eventQueue.flush();
            eventQueue.sync();
            LOGGER.info("Wrote {} record to event queue", operation);

            exportStatus.flushToDisk();
            LOGGER.info("{} processing complete. Exiting...", operation);
            shutDown = true; // to ensure that no event gets written after switch operation.
        }
        System.exit(0);
    }

    @Override
    public void handleBatch(List<ChangeEvent<Object, Object>> changeEvents, DebeziumEngine.RecordCommitter<ChangeEvent<Object, Object>> committer)
            throws InterruptedException {
        LOGGER.info("Processing batch with {} records", changeEvents.size());
        checkIfHelperThreadAlive();
        for (ChangeEvent<Object, Object> event : changeEvents) {
            Object objKey = event.key();
            Object objVal = event.value();

            // PARSE
            var r = parser.parseRecord(objKey, objVal);
            if (r.isUnsupported()) {
                committer.markProcessed(event);
                continue;
            }
            // LOGGER.info("Processing record {} => {}", r.getTableIdentifier(), r.getValueFieldValues());
            checkIfSnapshotAlreadyComplete(r);
            recordTransformer.transformRecord(r);
            sequenceObjectUpdater.processRecord(r);

            // WRITE
            RecordWriter writer = getWriterForRecord(r);
            if (exportStatus.getMode().equals(ExportMode.STREAMING)) {
                // need to synchronize access with cutover/fall-forward thread
                synchronized (writer) {
                    if (shutDown) {
                        return;
                    }
                    writer.writeRecord(r);
                }
            }
            else {
                writer.writeRecord(r);
            }
            // Handle snapshot->cdc transition
            checkIfSnapshotComplete(r);

            committer.markProcessed(event);
        }
        handleBatchComplete();
        committer.markBatchFinished();
        handleSnapshotOnlyComplete();
    }

    private RecordWriter getWriterForRecord(Record r) {
        if (exportStatus.getMode() == ExportMode.SNAPSHOT) {
            RecordWriter writer = snapshotWriters.get(r.t);
            if (writer == null) {
                writer = new TableSnapshotWriterCSV(dataDir, r.t, sourceType);
                snapshotWriters.put(r.t, writer);
            }
            return writer;
        }
        else {
            return eventQueue;
        }
    }

    /**
     * The last record we recieve will have the snapshot field='last'.
     * We interpret this to mean that snapshot phase is complete, and move on to streaming phase
     */
    private void checkIfSnapshotComplete(Record r) {
        if ((r.snapshot != null) && (r.snapshot.equals("last"))) {
            handleSnapshotComplete();
        }
    }

    /**
     * In an edge case where the last table scanned by debezium in the snapshot phase
     * has 0 rows, we do not get snapshot=last in the last record of the snapshot phase.
     * This is because debezium expected there to be more records in the subsequent table(s),
     * but the last table scanned ended up having 0 rows.
     *
     * To work around this, we check if we're still in snapshot phase, and if we get a record with snapshot=null/false
     * (which is indicative of streaming phase), we transition to streaming phase.
     * Note that this method would have to be called before the record is written.
     * @param r
     */
    private void checkIfSnapshotAlreadyComplete(Record r) {
        if ((exportStatus.getMode() == ExportMode.SNAPSHOT) && (r.snapshot == null || r.snapshot.equals("false"))) {
            LOGGER.debug("Interpreting snapshot as complete since snapshot field of record is null");
            handleSnapshotComplete();
        }
    }

    private void handleSnapshotComplete() {
        closeSnapshotWriters();
        exportStatus.updateMode(ExportMode.STREAMING);
        exportStatus.flushToDisk();
        openCDCWriter();
    }

    private void handleSnapshotOnlyComplete() {
        if ((exportStatus.getMode() == ExportMode.STREAMING) && (snapshotMode.equals("initial_only"))) {
            LOGGER.info("Snapshot complete. Interrupting thread as snapshot mode = initial_only");
            exportStatus.flushToDisk();
            Thread.currentThread().interrupt();
        }
    }

    private void closeSnapshotWriters() {
        for (RecordWriter writer : snapshotWriters.values()) {
            writer.close();
        }
        snapshotWriters.clear();
    }

    private void handleBatchComplete() {
        flushSyncStreamingData();
    }

    /**
     * At the end of batch, we sync streaming data to storage.
     * This is inline with debezium behavior - https://debezium.io/documentation/reference/stable/development/engine.html#_handling_failures
     * In case machine powers off before data is synced to storage, those events will be received again upon restart
     * because debezium flushes its offsets information at the end of every batch.
     */
    private void flushSyncStreamingData() {
        if (exportStatus.getMode().equals(ExportMode.STREAMING)) {
            if (eventQueue != null) {
                eventQueue.flush();
                eventQueue.sync();
            }
        }
    }

    private void openCDCWriter() {
        final Config config = ConfigProvider.getConfig();
        Long queueSegmentMaxBytes = config.getOptionalValue(PROP_PREFIX + "queueSegmentMaxBytes", Long.class).orElse(null);
        eventQueue = new EventQueue(dataDir, queueSegmentMaxBytes);
    }

    private void checkIfHelperThreadAlive() {
        if (!flusherThread.isAlive()) {
            // if the flusher thread dies, export status will stop being updated,
            // so interrupting main thread as well.
            throw new RuntimeException("Flusher Thread exited unexpectedly.");
        }
    }
}
