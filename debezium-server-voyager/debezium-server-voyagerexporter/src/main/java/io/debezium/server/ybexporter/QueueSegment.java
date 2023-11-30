/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;


import com.fasterxml.jackson.core.JsonFactory;
import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;
import com.fasterxml.jackson.databind.ObjectWriter;
import org.eclipse.microprofile.config.Config;
import org.eclipse.microprofile.config.ConfigProvider;
import org.graalvm.collections.Pair;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;


import java.io.BufferedReader;
import java.io.BufferedWriter;
import java.io.FileDescriptor;
import java.io.FileOutputStream;
import java.io.FileReader;
import java.io.FileWriter;
import java.io.IOException;
import java.io.RandomAccessFile;
import java.io.Writer;
import java.nio.file.Files;
import java.nio.file.Path;
import java.sql.SQLException;
import java.util.HashMap;
import java.util.Map;

/**
 * A QueueSegment represents a segment of the cdc queue.
 */
public class QueueSegment {
    private static final Logger LOGGER = LoggerFactory.getLogger(QueueSegment.class);

    private static final String EOF_MARKER = "\\.";
    private String filePath;
    private long segmentNo;
    private FileOutputStream fos;
    private FileDescriptor fd;
    private Writer writer;
    private long byteCount;
    private ObjectWriter ow;
    private ExportStatus es;
    private String exporterRole;
    // (schemaName, tableName) -> (operation -> count)
    private Map<Pair<String, String>, Map<String, Long>> eventCountDeltaPerTable;

    public QueueSegment(String datadirStr, long segmentNo, String filePath){
        this.segmentNo = segmentNo;
        this.filePath = filePath;
        es = ExportStatus.getInstance(datadirStr);
        ow = new ObjectMapper().writer();
        try {
            openFile();
        } catch (IOException e) {
            throw new RuntimeException(e);
        }

        es.queueSegmentCreated(segmentNo, filePath);
        long committedSize = es.getQueueSegmentCommittedSize(segmentNo);
        LOGGER.info("Opened queue segment {}; byteCount={}, committedSize={}", filePath, byteCount, committedSize);
        if (committedSize < byteCount){
            truncateFileAfterOffset(committedSize);
        }
        eventCountDeltaPerTable = new HashMap<>();
        final Config config = ConfigProvider.getConfig();
        exporterRole = config.getValue("debezium.sink.ybexporter.exporter.role", String.class);
    }

    private void openFile() throws IOException {
        fos = new FileOutputStream(filePath, true);
        fd = fos.getFD();
        FileWriter fw = new FileWriter(fd);
        writer = new BufferedWriter(fw);
        byteCount = Files.size(Path.of(filePath));
    }

    public long getByteCount() {
        return byteCount;
    }

    public void write(Record r){
        try {
            String cdcJson = ow.writeValueAsString(generateCdcMessageForRecord(r)) + "\n";
            writer.write(cdcJson);
            byteCount += cdcJson.length();
            updateStats(r);
        }
        catch (IOException e) {
            throw new RuntimeException(e);
        }
    }
    private void updateStats(Record r){
        Pair<String, String> fullyQualifiedTableName =  Pair.create(r.t.schemaName, r.t.tableName);

        Map<String, Long> tableMap = eventCountDeltaPerTable.computeIfAbsent(fullyQualifiedTableName, k -> new HashMap<>());
        tableMap.put(r.op, tableMap.getOrDefault(r.op, 0L) + 1);
    }

    private HashMap<String, Object> generateCdcMessageForRecord(Record r) {
        // TODO: optimize, don't create objects every time.
        HashMap<String, Object> key = new HashMap<>();
        HashMap<String, Object> fields = new HashMap<>();

        for (int i = 0; i < r.keyValues.size(); i++) {
            Object formattedVal = r.keyValues.get(i);
            key.put(r.keyColumns.get(i), formattedVal);
        }

        for (int i = 0; i < r.valueValues.size(); i++) {
            Object formattedVal = r.valueValues.get(i);
            fields.put(r.valueColumns.get(i), formattedVal);
        }

        HashMap<String, Object> cdcInfo = new HashMap<>();
        cdcInfo.put("op", r.op);
        cdcInfo.put("vsn", r.vsn);
        cdcInfo.put("schema_name", r.t.schemaName);
        cdcInfo.put("table_name", r.t.tableName);
        cdcInfo.put("key", key);
        cdcInfo.put("fields", fields);
        cdcInfo.put("exporter_role", exporterRole);
        return cdcInfo;
    }

    public void flush() throws IOException {
        writer.flush();
    }

    public void close() throws IOException, SQLException {
        LOGGER.info("Closing queue file {}", filePath);
        writer.write(EOF_MARKER);
        writer.write("\n");
        writer.write("\n");
        writer.flush();
        sync();
        writer.close();
    }

    public void sync() throws IOException, SQLException {
        fd.sync();
        es.updateQueueSegmentMetaInfo(segmentNo, Files.size(Path.of(filePath)), eventCountDeltaPerTable);
        // TODO: optimize to reset counters to 0 instead of clearing the map.
        eventCountDeltaPerTable.clear();
    }

    public long getSequenceNumberOfLastRecord(){
        ObjectMapper mapper = new ObjectMapper(new JsonFactory());
        long vsn = -1;
        String last = null, line;
        BufferedReader input;
        try {
            input = new BufferedReader(new FileReader(filePath));
            while ((line = input.readLine()) != null) {
                if (line.equals(EOF_MARKER)){
                    break;
                }
                last = line;
            }
            if (last != null){
                JsonNode lastRecordJson = mapper.readTree(last);
                vsn = lastRecordJson.get("vsn").asLong();
            }
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
        return vsn;
    }

    public boolean isClosed() {
        String last = null, line;
        BufferedReader input;
        try {
            input = new BufferedReader(new FileReader(filePath));
            while ((line = input.readLine()) != null) {
                last = line;
                if (last.equals(EOF_MARKER)){
                    return true;
                }
            }
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
        return false;
    }

    private void truncateFileAfterOffset(long offset){
        try {
            writer.close();
            LOGGER.info("Truncating queue segment {} at path {} to size {}", segmentNo, filePath, offset);
            RandomAccessFile f = new RandomAccessFile(filePath, "rw");
            f.setLength(offset);
            f.close();
            openFile();
        } catch (IOException e) {
            throw new RuntimeException(e);
        }

    }
}
