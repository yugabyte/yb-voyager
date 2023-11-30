/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.io.File;
import java.io.IOException;
import java.io.SyncFailedException;
import java.nio.file.DirectoryStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.sql.SQLException;
import java.util.ArrayList;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import static java.lang.Math.max;

/**
 * This takes care of writing to the cdc queue file.
 * Using a single queue.ndjson to represent the entire cdc is not desirable
 * as the file size may become too large. Therefore, we break it into smaller QueueSegments
 * and rotate them as soon as we reach a size threshold.
 * If shutdown abruptly, this class is capable of resuming by retrieving the latest queue
 * segment that was written to, and continues writing from that segment.
 */
public class EventQueue implements RecordWriter {
    private static final Logger LOGGER = LoggerFactory.getLogger(EventQueue.class);
    private static final String QUEUE_SEGMENT_FILE_NAME = "segment";
    private static final String QUEUE_SEGMENT_FILE_EXTENSION = "ndjson";
    private static final String QUEUE_FILE_DIR = "queue";

    private long queueSegmentMaxBytes = 1024 * 1024 * 1024; // default 1 GB
    private String dataDir;
    private QueueSegment currentQueueSegment;
    private long currentQueueSegmentIndex = 0;
    private SequenceNumberGenerator sng;


    public EventQueue(String datadirStr, Long queueSegmentMaxBytes) {
        dataDir = datadirStr;
        if (queueSegmentMaxBytes != null){
            this.queueSegmentMaxBytes = queueSegmentMaxBytes;
        }

        // mkdir cdc
        File queueDir = new File(String.format("%s/%s", dataDir, QUEUE_FILE_DIR));
        if (!queueDir.exists()){
            boolean dirCreated = queueDir.mkdir();
            if (!dirCreated){
                throw new RuntimeException("failed to create dir for cdc");
            }
        }
        sng = new SequenceNumberGenerator(1);
        recoverStateFromDisk();
        if (currentQueueSegment == null){
            currentQueueSegment = new QueueSegment(datadirStr, currentQueueSegmentIndex, getFilePathWithIndex(currentQueueSegmentIndex));
        }
    }

    private void recoverStateFromDisk(){
        recoverLatestQueueSegment();
        // recover sequence numberof last written record and resume
        if (currentQueueSegment != null){
            long nextSequenceNumber;
            try {
                long lastRecordSequenceNumber = currentQueueSegment.getSequenceNumberOfLastRecord();
                if (lastRecordSequenceNumber == -1){
                    // current queue segment is empty, we need to look at the second last segment
                    if (currentQueueSegmentIndex == 0){
                        // there is no second last segment, start from 1
                        nextSequenceNumber = 1;
                    }
                    else {
                        QueueSegment secondLastQueueSegment = new QueueSegment(dataDir, currentQueueSegmentIndex-1, getFilePathWithIndex(currentQueueSegmentIndex-1));
                        nextSequenceNumber = secondLastQueueSegment.getSequenceNumberOfLastRecord() + 1;
                        secondLastQueueSegment.close();
                    }
                }
                else {
                    nextSequenceNumber = lastRecordSequenceNumber + 1;
                }
            } catch (IOException | SQLException e) {
                throw new RuntimeException(e);
            }
            LOGGER.info("advancing sequence number to {}", nextSequenceNumber);
            sng.advanceTo(nextSequenceNumber);

            LOGGER.info("checking for if current queue segment is closed");
            if (currentQueueSegment.isClosed()){
                LOGGER.info("current queue segment is closed.rotating");
                rotateQueueSegment();
            }
        }
    }

    /**
     * This function reads the /queue dir for all the queue segment files.
     * Then it retrieves the index number from the paths, and then finds
     * the max index - which is the latest queue segment that was written to.
     * If no files are found, we just return.
     * TODO handle edge case: even if the  latest queue segment was already closed (i.e. EOF marker was written),
     * we would still consider it to be the latest and open it. when the next write arrives, we will rotate the
     * queue segment, which would again write an EOF marker, leading to two EOF markers sequentially.
     * Voyager import data will proceed to next segment upon seeing the first EOF so it's not a concern.
     */
    private void recoverLatestQueueSegment(){
        // read dir to find all queue files
        Path queueDirPath = Path.of(dataDir, QUEUE_FILE_DIR);
        String searchGlob = String.format("%s.[0-9]*.%s", QUEUE_SEGMENT_FILE_NAME, QUEUE_SEGMENT_FILE_EXTENSION);
        ArrayList<Path> filePaths = new ArrayList<>();
        try {
            DirectoryStream<Path> stream = Files.newDirectoryStream(queueDirPath, searchGlob);
            for (Path entry: stream) {
                filePaths.add(entry);
            }
            if (filePaths.size() == 0){
                // no files found. nothing to recover.
                LOGGER.info("No files found matching {}. Nothing to recover", searchGlob);
                return;
            }
            // extract max index of all files
            long maxIndex = 0;
            Path maxIndexPath = null;
            for(Path p: filePaths){
                String filename = p.getFileName().toString();
                String indexStr = filename.substring(QUEUE_SEGMENT_FILE_NAME.length() + 1, filename.length() - (QUEUE_SEGMENT_FILE_EXTENSION.length() + 1));
                long index = Long.parseLong(indexStr);
                if (index >= maxIndex){
                    maxIndex = index;
                    maxIndexPath = p;
                }
            }
            // create queue segment for last file segment
            currentQueueSegmentIndex = maxIndex;
            currentQueueSegment = new QueueSegment(dataDir, currentQueueSegmentIndex, maxIndexPath.toString());

            LOGGER.info("Recovered from queue segment-{} with byte count={}", maxIndexPath, currentQueueSegment.getByteCount());
        }
        catch (IOException x) {
            throw new RuntimeException(x);
        }
    }

    /**
     * each queue segment's file name is of the format segment.<N>.ndjson
     * where N is the segment number.
    */
    private String getFilePathWithIndex(long index){
        String queueSegmentFileName = String.format("%s.%d.%s", QUEUE_SEGMENT_FILE_NAME, index, QUEUE_SEGMENT_FILE_EXTENSION);
        return Path.of(dataDir, QUEUE_FILE_DIR, queueSegmentFileName).toString();
    }

    private boolean shouldRotateQueueSegment(){
        return (currentQueueSegment.getByteCount() >= queueSegmentMaxBytes);
    }

    private void rotateQueueSegment(){
        // close old file.
        try {
            currentQueueSegment.close();
        } catch (IOException | SQLException e) {
            throw new RuntimeException(e);
        }
        currentQueueSegmentIndex++;
        LOGGER.info("rotating queue segment to #{}", currentQueueSegmentIndex);
        currentQueueSegment = new QueueSegment(dataDir, currentQueueSegmentIndex, getFilePathWithIndex(currentQueueSegmentIndex));
    }

    @Override
    public void writeRecord(Record r) {
        if (r.op.equals("u") && r.valueValues.size() == 0) {
            LOGGER.debug("Skipping record {} as there are no values to update.", r);
            return;
        }
        if (shouldRotateQueueSegment()) rotateQueueSegment();
        augmentRecordWithSequenceNo(r);
        currentQueueSegment.write(r);
    }

    private void augmentRecordWithSequenceNo(Record r){
        r.vsn = sng.getNextValue();
    }

    @Override
    public void flush() {
        try {
            currentQueueSegment.flush();
        }
        catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public void close() {
        try {
            currentQueueSegment.close();
        }
        catch (IOException | SQLException e) {
            throw new RuntimeException(e);
        }
    }

    @Override
    public void sync() {
        try {
            currentQueueSegment.sync();
        }
        catch (IOException | SQLException e) {
            throw new RuntimeException(e);
        }
    }
}

class SequenceNumberGenerator{
    private long nextValue;
    public SequenceNumberGenerator(long start){
        nextValue = start;
    }

    public long getNextValue(){
        return nextValue++;
    }

    public long peekNextValue(){
        return nextValue;
    }

    public void advanceTo(long val){
        nextValue = val;
    }
}
