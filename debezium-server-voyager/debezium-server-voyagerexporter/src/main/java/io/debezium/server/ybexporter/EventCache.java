/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.io.IOException;
import java.nio.file.DirectoryStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.Set;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class EventCache {
    private static final Logger LOGGER = LoggerFactory.getLogger(EventQueue.class);
    private static final String EOF_MARKER = "\\.";
    String dataDir;
    private Set<Integer> hashCache = new HashSet<>();
    private long currentQueueSegmentIndex = 0;
    private static final String QUEUE_SEGMENT_FILE_NAME = "segment";
    private static final String QUEUE_SEGMENT_FILE_EXTENSION = "ndjson";
    private static final String QUEUE_FILE_DIR = "queue";
    private long maxCacheSize = 1000000;
    private QueueSegment currentQueueSegment;

    public EventCache(String dataDir) {
        this.dataDir = dataDir;
        recoverCacheFromDisk();
    }

    public void recoverCacheFromDisk() {
        hashCache = new HashSet<>();
        recoverLatestQueueSegment();
        if (currentQueueSegment == null) {
            LOGGER.info("No queue segment found. Nothing to recover into event cache");
            return;
        }
        while (true) {
            String events = currentQueueSegment.readAllEvents();
            if (events == null) {
                // Segment is empty
                break;
            }
            String[] eventArray = events.split("\n");
            LOGGER.info("Read {} events from queue segment-{}", eventArray.length, currentQueueSegmentIndex);
            LOGGER.info(events);
            for (int i = eventArray.length - 1; i >= 0; i--) {
                if (eventArray[i].equals(EOF_MARKER) || eventArray[i].isEmpty() || eventArray[i].isBlank()) {
                    continue;
                }
                hashCache.add(eventArray[i].hashCode());
                if (hashCache.size() >= maxCacheSize) {
                    break;
                }
            }

            if (hashCache.size() >= maxCacheSize) {
                break;
            }

            // Segment is exhausted. Move to previous segment
            currentQueueSegmentIndex--;
            if (currentQueueSegmentIndex < 0) {
                // No more segments to read
                break;
            }
            currentQueueSegment = new QueueSegment(dataDir, currentQueueSegmentIndex,
                    getFilePathWithIndex(currentQueueSegmentIndex));
        }
        LOGGER.info("Recovered {} events into event cache", hashCache.size());
    }

    private String getFilePathWithIndex(long index) {
        String queueSegmentFileName = String.format("%s.%d.%s", QUEUE_SEGMENT_FILE_NAME, index,
                QUEUE_SEGMENT_FILE_EXTENSION);
        return Path.of(dataDir, QUEUE_FILE_DIR, queueSegmentFileName).toString();
    }

    // Duplicate in EventQueue.java
    private void recoverLatestQueueSegment() {
        // read dir to find all queue files
        Path queueDirPath = Path.of(dataDir, QUEUE_FILE_DIR);
        String searchGlob = String.format("%s.[0-9]*.%s", QUEUE_SEGMENT_FILE_NAME, QUEUE_SEGMENT_FILE_EXTENSION);
        ArrayList<Path> filePaths = new ArrayList<>();
        try {
            DirectoryStream<Path> stream = Files.newDirectoryStream(queueDirPath, searchGlob);
            for (Path entry : stream) {
                filePaths.add(entry);
            }
            if (filePaths.size() == 0) {
                // no files found. nothing to recover.
                LOGGER.info("No files found matching {}. Nothing to recover", searchGlob);
                return;
            }
            // extract max index of all files
            long maxIndex = 0;
            Path maxIndexPath = null;
            for (Path p : filePaths) {
                String filename = p.getFileName().toString();
                String indexStr = filename.substring(QUEUE_SEGMENT_FILE_NAME.length() + 1,
                        filename.length() - (QUEUE_SEGMENT_FILE_EXTENSION.length() + 1));
                long index = Long.parseLong(indexStr);
                if (index >= maxIndex) {
                    maxIndex = index;
                    maxIndexPath = p;
                }
            }
            // create queue segment for last file segment
            currentQueueSegmentIndex = maxIndex;
            currentQueueSegment = new QueueSegment(dataDir, currentQueueSegmentIndex, maxIndexPath.toString());

            LOGGER.info("Starting to recover event cache from queue segment-{} with byte count={}", maxIndexPath,
                    currentQueueSegment.getByteCount());
        } catch (IOException x) {
            throw new RuntimeException(x);
        }
    }

}
