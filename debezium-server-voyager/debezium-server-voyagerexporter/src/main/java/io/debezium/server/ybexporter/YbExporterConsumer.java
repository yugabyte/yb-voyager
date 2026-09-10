/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.net.URISyntaxException;
import java.sql.SQLException;
import java.util.HashMap;
import java.util.List;
import java.util.Locale;
import java.util.Map;
import java.util.concurrent.ConcurrentHashMap;

import java.io.File;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.StandardOpenOption;

import org.eclipse.microprofile.config.Config;
import org.eclipse.microprofile.config.ConfigProvider;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import io.debezium.engine.ChangeEvent;
import io.debezium.engine.DebeziumEngine;
import io.debezium.server.BaseChangeConsumer;

import com.yugabyte.ybvoyager.BytemanMarkers;

/**
 * Implementation of the consumer that exports the messages to file in a
 * Yugabyte-compatible form.
 */

public class YbExporterConsumer extends BaseChangeConsumer {
    private static final Logger LOGGER = LoggerFactory.getLogger(YbExporterConsumer.class);
    private static final String PROP_PREFIX = "debezium.sink.ybexporter.";
    private static final String SOURCE_DB_EXPORTER_ROLE = "source_db_exporter";
    private static final String TARGET_DB_EXPORTER_FF_ROLE = "target_db_exporter_ff";
    private static final String TARGET_DB_EXPORTER_FB_ROLE = "target_db_exporter_fb";
    private static final int OFFSET_COMMIT_MAX_ATTEMPTS = 5;
    private static final long OFFSET_COMMIT_INITIAL_RETRY_DELAY_MS = 250;
    final Config config = ConfigProvider.getConfig();
    boolean ybGRPCConnectorEnabled;
    String snapshotMode;
    String dataDir;
    String exportDir;
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
    Object flushingSnapshotFilesLock = new Object();
    private static final Integer ObjectMapperMaxStringLength = 500_000_000;

    // Lock file
    private File lockFile;

    public YbExporterConsumer(String dataDir) {
        this.dataDir = dataDir;
    }

    void connect() throws URISyntaxException {
        BytemanMarkers.checkpoint("before-connect");
        LOGGER.info("connect() called: dataDir = {}", dataDir);

        final Config config = ConfigProvider.getConfig();

        snapshotMode = config.getOptionalValue("debezium.source.snapshot.mode", String.class).orElse("");
        retrieveSourceType(config);
        exporterRole = config.getValue("debezium.sink.ybexporter.exporter.role", String.class);
        exportDir = config.getValue("debezium.sink.ybexporter.exportDir", String.class);
        if (sourceType.equals("yb")) {
            ybGRPCConnectorEnabled = config.getValue("debezium.source.grpc.connector.enabled", Boolean.class);
        }
        lockFile = new File(exportDir, String.format(".debezium_%s.lck", exporterRole));
        
        // Acquire lock file at startup
        acquireLockFile();

        // Register shutdown hook to release lock on exit
        Runtime.getRuntime().addShutdownHook(new Thread(() -> {
            releaseLockFile();
        }));

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
        String sequenceMaxMapString = config
                .getOptionalValue(PROP_PREFIX + SequenceObjectUpdater.initSequenceMaxpropertyName, String.class)
                .orElse(null);
        sequenceObjectUpdater = new SequenceObjectUpdater(dataDir, sourceType, columnSequenceMapString,
                sequenceMaxMapString, exportStatus.getSequenceMaxMap());
        recordTransformer = new DebeziumRecordTransformer();

        flusherThread = new Thread(this::flush);
        flusherThread.setDaemon(true);
        flusherThread.start();
        
        BytemanMarkers.checkpoint("after-connect");
    }

    /**
     * Reads the lock file and checks if the PID is still running.
     * If the PID is not running, it deletes the lock file.
     * If the PID is running, it throws an IllegalStateException.
     */
    private void checkExistingLockFile() {
        if (!lockFile.exists()) {
            return; // No lock file exists, nothing to check
        }
        try {
            String lockContent = Files.readString(lockFile.toPath());
            String[] lines = lockContent.split("\n");
            if (lines == null || lines.length == 0) {
                // If the lock file is empty, error out
                String msg = String.format("Lock file {} is empty.", lockFile.getAbsolutePath());
                throw new IllegalStateException(msg);
            }
            String pid = lines[0].trim();
            // Check if the PID is a valid number
            LOGGER.info("Lock file {} exists with PID: {}", lockFile.getAbsolutePath(), pid);
            //Parse PID and check if the process is running
            try {
                long pidLong = Long.parseLong(pid);
                // Check if the process with this PID is running
                if (ProcessHandle.of(pidLong).isPresent()) {
                    // Process is running, error out
                    String msg = String.format("Lock file %s already exists and process with PID %s is running. Another process may be running for this dataDir. Terminate the process on the pid %s and re-run the command", lockFile.getAbsolutePath(), pid, pid);
                    LOGGER.error(msg);
                    throw new IllegalStateException(msg);
                } else {
                    // Process is not running, we can safely delete the lock file
                    LOGGER.warn("Lock file {} exists but process with PID {} is not running. Deleting it.", lockFile.getAbsolutePath(), pid);
                    releaseLockFile();
                }
            } catch (NumberFormatException e) {
                // If PID is not a valid number, throw an error
                String msg = String.format("Invalid PID in lock file {}: {}.", lockFile.getAbsolutePath(), pid);
                throw new IllegalStateException(msg, e);
            }
            
        } catch (IOException e) {
            String msg = String.format("Error reading lock file %s: %s", lockFile.getAbsolutePath(), e.getMessage());
            LOGGER.error(msg, e);
            throw new IllegalStateException(msg, e);
        }
    }
    /**
     * Acquires a lock file in the dataDir with <exporter-role>.lck and stores the pid of the process 
     * Fails if already locked.
     */
    private void acquireLockFile() {
        checkExistingLockFile();
        try {
            // Create the lock file
            boolean created = lockFile.createNewFile();
            if (!created) {
                String msg = String.format("Failed to create lock file %s. Another process may be running.", lockFile.getAbsolutePath());
                LOGGER.error(msg);
                throw new IllegalStateException(msg);
            }
            //write PID to check later if the PID is still running
            String pid = String.valueOf(ProcessHandle.current().pid());
            Files.writeString(lockFile.toPath(), pid + "\n", StandardOpenOption.WRITE);
            LOGGER.info("Acquired lock file: {} for PID: %s", lockFile.getAbsolutePath(), pid);
        } catch (IOException e) {
            String msg = String.format("Error creating lock file %s: %s", lockFile.getAbsolutePath(), e.getMessage());
            LOGGER.error(msg, e);
            throw new IllegalStateException(msg, e);
        }
    }

    /**
     * Releases (deletes) the lock file if it exists.
     */
    private void releaseLockFile() {
        if (lockFile != null && lockFile.exists()) {
            try {
                Files.delete(lockFile.toPath());
                LOGGER.info("Released lock file: {}", lockFile.getAbsolutePath());
            } catch (IOException e) {
                LOGGER.warn("Failed to delete lock file {}: {}", lockFile.getAbsolutePath(), e.getMessage());
            }
        }
    }

    private ExportMode getExportModeToStartWith(String snapshotMode) {
        if (snapshotMode.equals("never")) {
            return ExportMode.STREAMING;
        } else {
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
            case "io.debezium.connector.yugabytedb.YugabyteDBgRPCConnector":
                sourceType = "yb";
                break;
            case "io.debezium.connector.postgresql.YugabyteDBConnector":
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
            switchOperation = "cutover.target";
        } else if (exporterRole.equals(TARGET_DB_EXPORTER_FF_ROLE)) {
            switchOperation = "cutover.source_replica";
        } else if (exporterRole.equals(TARGET_DB_EXPORTER_FB_ROLE)) {
            switchOperation = "cutover.source";
        } else {
            throw new RuntimeException(String.format("invalid exportRole %s", exporterRole));
        }

        while (true) {
            synchronized (flushingSnapshotFilesLock) {
                for (RecordWriter writer : snapshotWriters.values()) {
                    writer.flush();
                    writer.sync();
                }
            }
            // TODO: doing more than flushing files to disk. maybe move this call to another
            // thread?
            if (exportStatus != null) {
                exportStatus.flushToDisk();
            }

            checkForSwitchOperationAndHandle(switchOperation);
            checkForEndMigrationAndHandle();
            try {
                Thread.sleep(2000);
            } catch (InterruptedException e) {
                // Noop.
            }
        }
    }

    private void checkForSwitchOperationAndHandle(String operation) {
        try {
            if (!exportStatus.checkIfSwitchOperationRequested(operation)) {
                return;
            }
        } catch (SQLException e) {
            throw new RuntimeException(e);
        }

        LOGGER.info("Observed {} trigger present in metadb. Cutting over...", operation);
        Record switchOperationRecord = new Record();
        switchOperationRecord.op = operation;
        switchOperationRecord.t = new Table(null, null, null); // just to satisfy being a proper Record object.
        synchronized (eventQueue) { // need to synchronize with handleBatch
            eventQueue.writeRecord(switchOperationRecord);
            eventQueue.close();
            LOGGER.info("Wrote {} record to event queue", operation);
            exportStatus.flushToDisk();
            LOGGER.info("{} processing complete. Exiting...", operation);
            shutDown = true; // to ensure that no event gets written after switch operation.
        }
        System.exit(0);
    }

    private void checkForEndMigrationAndHandle() {
        try {
            if (!exportStatus.checkifEndMigrationRequested()) {
                return;
            }
        } catch (SQLException e) {
            throw new RuntimeException(e);
        }

        LOGGER.info("Observed request for end migration in metadb. Shutting down gracefully.");
        synchronized (eventQueue) { // need to synchronize with handleBatch
            eventQueue.close();

            exportStatus.flushToDisk();
            LOGGER.info("End migration processing complete. Exiting...");
            shutDown = true; // to ensure that no event gets written after switch operation.
        }
        System.exit(0);
    }

    public void handleBatch(List<ChangeEvent<Object, Object>> changeEvents,
            DebeziumEngine.RecordCommitter<ChangeEvent<Object, Object>> committer)
            throws InterruptedException {
        BytemanMarkers.cdc("before-batch");
        if (exportStatus.getMode().equals(ExportMode.STREAMING)) {
            BytemanMarkers.cdc("before-batch-streaming");
        }
        LOGGER.info("Processing batch with {} records", changeEvents.size());
        checkIfHelperThreadAlive();

        for (ChangeEvent<Object, Object> event : changeEvents) {
            BytemanMarkers.cdc("before-process-record");
            Object objKey = event.key();
            Object objVal = event.value();

            LOGGER.debug("Processing record {} => {}", objKey, objVal);

            // PARSE
            var r = parser.parseRecord(objKey, objVal);
            if (!checkIfEventNeedsToBeWritten(r)) {
                continue;
            }

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
                    BytemanMarkers.cdc("before-write-record");
                    writer.writeRecord(r);
                }
            } else {
                writer.writeRecord(r);
            }
            // Handle snapshot->cdc transition
            checkIfSnapshotComplete(r);
            BytemanMarkers.cdc("after-process-record");
        }
        BytemanMarkers.cdc("before-handle-batch-complete");
        handleBatchComplete();
        LOGGER.debug("Fsynced batch with {} records", changeEvents.size());
        // committer.markProcessed(event) updates offsets in memory,
        // committer.MarkBatchFinished flushes those
        // offsets to disk. Offsets are also flushed to disk when debezium-server is
        // gracefully shutdown. (which can
        // happen multiple times during a migration).
        // To the scenario where events were marked as processed (and flushed to disk by
        // graceful shutdown),
        // but not fsynced and updated in metadb, it is important to mark the events as
        // processed only AFTER we fsync/
        // update metaDB.
        // TODO: optimize by only marking the last event as processed.
        BytemanMarkers.cdc("before-offset-commit");
        commitBatchOffsets(changeEvents, committer);
        handleSnapshotOnlyComplete();
        BytemanMarkers.cdc("after-batch");
    }

    /**
     * Marks every event in the batch processed and flushes the offsets, retrying while a
     * tablet split still has the replication stream closed.
     *
     * <p><b>How the debezium side fits together.</b> Two threads matter. The <i>producer</i>
     * thread (named {@code ...change-event-source-coordinator}) reads WAL from the source
     * through a single {@code ReplicationStream}. The <i>engine</i> thread (named
     * {@code pool-N-thread-M}) loops {@code EmbeddedEngine.run() -> pollRecords() ->
     * handleBatch()}, and so is the thread running this method. Both use the same stream
     * object, held in one {@code AtomicReference} on {@code PostgresStreamingChangeEventSource}:
     * the producer reads from it, and the offset commit below writes a flushed LSN back to it.
     *
     * <p>By the time we get here, {@code handleBatch()} has written every event of this batch
     * to the export queue and {@code handleBatchComplete()} has fsynced it, so the exported
     * data is durable no matter what the offset commit does. The commit is two distinct steps
     * inside {@code markBatchFinished()}:
     * <ol>
     * <li>{@code offsetWriter.doFlush()} persists the resume position to
     * {@code data/offsets.<exporter-role>.dat}. This is what debezium reads on restart.</li>
     * <li>{@code task.commit() -> commitOffset() -> stream.flushLsn()} tells the source that
     * WAL up to this LSN may be released. This is the only step that touches the stream, and
     * therefore the only one that can fail here.</li>
     * </ol>
     * {@code markProcessed()} itself only records positions in memory; it never does I/O.
     *
     * <p><b>What a tablet split does.</b> It closes that shared stream, which surfaces twice.
     * The producer's read fails with "Could not find the two split children" - debezium treats
     * that as retriable, {@code ErrorHandler} stores it, and the engine thread restarts the
     * connector on its next {@code poll()}. Independently, step 2 above throws
     * "This replication stream has been closed" (DB-20886). Letting that second exception
     * propagate exits the embedded engine <i>before</i> the restart runs, which is what turns a
     * recoverable split into a fatal exporter crash. That is the bug this method prevents.
     *
     * <p><b>What a retry sees.</b> Depending on how far the producer's teardown has progressed,
     * the retried commit hits one of two states: the stream is still present but closed, so
     * {@code flushLsn()} throws again; or the reference has been cleared, and debezium's
     * {@code commitOffset()} logs "Streaming has already stopped, ignoring commit callback..."
     * and returns without flushing. Either way the batch is safe, and the next successful flush
     * after the restart carries a newer LSN, so at worst one WAL-release hint is lost.
     *
     * <p><b>Why retry instead of skipping.</b> Skipping would only be safe because step 1 runs
     * before step 2, leaving the resume position already durable when the throw arrives. If a
     * future debezium release reorders those steps, a skip would leave the position unpersisted
     * and the batch would be re-delivered after the restart - and dedup does not cover this
     * path, because {@code parseEventId()} only populates {@code eventId} for postgresql and
     * oracle while the fall-back connector reports {@code sourceType} "yb". Retrying re-runs the
     * whole commit and so carries no assumption about that ordering.
     *
     * <p><b>Why the retries are bounded.</b> The restart runs on this same engine thread and
     * cannot begin until we return, so an unbounded loop would block the very recovery it is
     * waiting for. Backoff is 250/500/1000/2000 ms, i.e. at most ~3.75s before the original
     * exception is rethrown, which stays far inside the source's CDC retention barrier.
     */
    private void commitBatchOffsets(List<ChangeEvent<Object, Object>> changeEvents,
            DebeziumEngine.RecordCommitter<ChangeEvent<Object, Object>> committer)
            throws InterruptedException {
        for (int attempt = 1; attempt <= OFFSET_COMMIT_MAX_ATTEMPTS; attempt++) {
            try {
                for (ChangeEvent<Object, Object> event : changeEvents) {
                    committer.markProcessed(event);
                }
                committer.markBatchFinished();
                LOGGER.debug("Committed batch complete with {} records", changeEvents.size());
                return;
            }
            catch (RuntimeException e) {
                // Only target-side exporters stream from YB, so only they can see a split;
                // anywhere else, and for any other exception, this stays fatal.
                if (!isTargetDbExporter() || !isReplicationStreamClosed(e)) {
                    throw e;
                }
                if (attempt == OFFSET_COMMIT_MAX_ATTEMPTS) {
                    LOGGER.error("Offset commit failed on all {} attempts; the replication stream "
                            + "is still closed. The batch is durably written to the export queue, "
                            + "but the offsets could not be committed.", OFFSET_COMMIT_MAX_ATTEMPTS, e);
                    throw e;
                }
                long delayMs = OFFSET_COMMIT_INITIAL_RETRY_DELAY_MS * (1L << (attempt - 1));
                LOGGER.warn("Offset commit failed (attempt {}/{}): the replication stream is closed "
                        + "(typically a YB tablet split). Retrying in {} ms.",
                        attempt, OFFSET_COMMIT_MAX_ATTEMPTS, delayMs, e);
                Thread.sleep(delayMs);
            }
        }
    }

    /**
     * True for the fall-back and fall-forward exporters, the only roles that stream from
     * YugabyteDB and can therefore hit a tablet split.
     */
    private boolean isTargetDbExporter() {
        return exporterRole.equals(TARGET_DB_EXPORTER_FB_ROLE)
                || exporterRole.equals(TARGET_DB_EXPORTER_FF_ROLE);
    }

    /**
     * True if any cause in the chain reports a closed replication stream, which debezium
     * raises from V3PGReplicationStream.checkClose() during an offset flush. Matches on
     * "replication stream" plus "closed" rather than debezium's exact sentence, so that
     * rewording it in a future bump cannot silently re-break DB-20886.
     */
    private boolean isReplicationStreamClosed(Throwable e) {
        for (Throwable t = e; t != null; t = t.getCause()) {
            String msg = t.getMessage();
            if (msg == null) {
                continue;
            }
            String lower = msg.toLowerCase(Locale.ROOT);
            if (lower.contains("replication stream") && lower.contains("closed")) {
                return true;
            }
        }
        return false;
    }

    private boolean checkIfEventNeedsToBeWritten(Record r) {
        if (r.isUnsupported()) {
            LOGGER.debug("Skipping unsupported record {}", r);
            return false;
        }
        return true;
    }

    private RecordWriter getWriterForRecord(Record r) {
        if (exportStatus.getMode() == ExportMode.SNAPSHOT) {
            BytemanMarkers.snapshot("get-writer");
            RecordWriter writer = snapshotWriters.get(r.t);
            if (writer == null) {
                writer = new TableSnapshotWriterCSV(dataDir, r.t, sourceType);
                snapshotWriters.put(r.t, writer);
            }
            return writer;
        } else {
            BytemanMarkers.cdc("get-writer");
            return eventQueue;
        }
    }

    /**
     * The last record we recieve will have the snapshot field='last'.
     * We interpret this to mean that snapshot phase is complete, and move on to
     * streaming phase
     */
    private void checkIfSnapshotComplete(Record r) {
        if ((r.snapshot != null) && (r.snapshot.equals("last"))) {
            BytemanMarkers.snapshot("detected-complete");
            handleSnapshotComplete();
        }
    }

    /**
     * In an edge case where the last table scanned by debezium in the snapshot
     * phase
     * has 0 rows, we do not get snapshot=last in the last record of the snapshot
     * phase.
     * This is because debezium expected there to be more records in the subsequent
     * table(s),
     * but the last table scanned ended up having 0 rows.
     *
     * To work around this, we check if we're still in snapshot phase, and if we get
     * a record with snapshot=null/false
     * (which is indicative of streaming phase), we transition to streaming phase.
     * Note that this method would have to be called before the record is written.
     * 
     * @param r
     */
    private void checkIfSnapshotAlreadyComplete(Record r) {
        if ((exportStatus.getMode() == ExportMode.SNAPSHOT) && (r.snapshot == null || r.snapshot.equals("false"))) {
            LOGGER.debug("Interpreting snapshot as complete since snapshot field of record is null");
            handleSnapshotComplete();
        }
    }

    private void handleSnapshotComplete() {
        BytemanMarkers.snapshot("before-complete");
        synchronized (flushingSnapshotFilesLock) {
            closeSnapshotWriters();
        }
        exportStatus.updateMode(ExportMode.STREAMING);
        exportStatus.flushToDisk();
        openCDCWriter();
        BytemanMarkers.snapshot("after-complete");
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
        // Flush exportStatus (sequence max values) before Debezium commits the batch and the offsets.
        // Otherwise on a hard stop (ctrl-c), queue events and Debezium offsets can be durable while
        // exportStatus.json lags behind the periodic 2s flusher, leaving stale sequence max on resumption
        // and breaking sequence restoration on the target after cutover.
        exportStatus.flushToDisk();
    }

    /**
     * At the end of batch, we sync streaming data to storage.
     * This is inline with debezium behavior -
     * https://debezium.io/documentation/reference/stable/development/engine.html#_handling_failures
     * In case machine powers off before data is synced to storage, those events
     * will be received again upon restart
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
        Long queueSegmentMaxBytes = config.getOptionalValue(PROP_PREFIX + "queueSegmentMaxBytes", Long.class)
                .orElse(null);
        eventQueue = new EventQueue(dataDir, queueSegmentMaxBytes, ybGRPCConnectorEnabled, exporterRole, ObjectMapperMaxStringLength);
    }

    private void checkIfHelperThreadAlive() {
        if (!flusherThread.isAlive()) {
            // if the flusher thread dies, export status will stop being updated,
            // so interrupting main thread as well.
            throw new RuntimeException("Flusher Thread exited unexpectedly.");
        }
    }
}
