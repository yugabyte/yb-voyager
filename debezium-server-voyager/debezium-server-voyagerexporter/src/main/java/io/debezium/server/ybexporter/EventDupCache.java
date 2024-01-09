/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import com.fasterxml.jackson.databind.JsonNode;
import com.fasterxml.jackson.databind.ObjectMapper;

import java.io.IOException;
import java.nio.file.DirectoryStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.LinkedList;
import java.util.Set;

import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

public class EventDupCache {
    private static final Logger LOGGER = LoggerFactory.getLogger(EventQueue.class);
    private static final String EOF_MARKER = "\\.";
    String dataDir;
    private LinkedList<String> mostRecentIdFirstList = new LinkedList<>();
    private Set<Integer> cache = new HashSet<>();
    private long currentQueueSegmentIndex = 0;
    private static final String QUEUE_SEGMENT_FILE_NAME = "segment";
    private static final String QUEUE_SEGMENT_FILE_EXTENSION = "ndjson";
    private static final String QUEUE_FILE_DIR = "queue";
    private long maxCacheSize = 1000000;
    private String currentQueueSegmentPath;

    public EventDupCache(String dataDir) {
        this.dataDir = dataDir;
    }

    public void warmUp() {
        cache = new HashSet<>();
        recoverLatestQueueSegment();
        if (currentQueueSegmentPath == null) {
            LOGGER.info("No queue segment found. Nothing to recover into event cache");
            return;
        }
        while (true) {
            try (ReverseFileReader reverseReader = new ReverseFileReader(currentQueueSegmentPath)) {
                while (reverseReader.hasNext()) {
                    String event = reverseReader.nextLine();
                    if (event == null) {
                        // Segment is empty
                        break;
                    }
                    if (event.equals(EOF_MARKER) || event.isEmpty() || event.isBlank()) {
                        continue;
                    }
                    String cacheMetadata = getCacheMetadata(event);
                    if (cacheMetadata == null) {
                        continue;
                    }
                    cache.add(cacheMetadata.hashCode());
                    mostRecentIdFirstList.addLast(cacheMetadata);
                    if (cache.size() >= maxCacheSize) {
                        break;
                    }
                }
            } catch (IOException e) {
                throw new RuntimeException(e);
            }
            if (cache.size() >= maxCacheSize) {
                break;
            }

            // Segment is exhausted. Move to previous segment
            currentQueueSegmentIndex--;
            if (currentQueueSegmentIndex < 0) {
                // No more segments to read
                break;
            }
            currentQueueSegmentPath = getFilePathWithIndex(currentQueueSegmentIndex);
        }
        LOGGER.info("Recovered {} events into event cache", cache.size());
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
                // Example filename: segment.0.ndjson
                String indexStr = filename.substring(QUEUE_SEGMENT_FILE_NAME.length() + 1,
                        filename.length() - (QUEUE_SEGMENT_FILE_EXTENSION.length() + 1));
                long index = Long.parseLong(indexStr);
                if (index >= maxIndex) {
                    maxIndex = index;
                    maxIndexPath = p;
                }
            }
            currentQueueSegmentPath = maxIndexPath.toString();
            currentQueueSegmentIndex = maxIndex;
        } catch (IOException x) {
            throw new RuntimeException(x);
        }
    }

    public boolean isEventInCache(String event) {
        return cache.contains(event.hashCode());
    }

    public void addEventToCache(String event) {
        // Check if cache is full. If full, remove oldest event from cache and add new
        // event
        if (cache.size() >= maxCacheSize) {
            String oldestEvent = mostRecentIdFirstList.removeLast();
            cache.remove(oldestEvent.hashCode());
        }
        cache.add(event.hashCode());
        mostRecentIdFirstList.addFirst(event);
    }

    public long getCacheSize() {
        return cache.size();
    }

    private String getCacheMetadata(String event) {
        ObjectMapper mapper = new ObjectMapper();
        try {
            JsonNode jsonNode = mapper.readTree(event);
            JsonNode cacheMetadataNode = jsonNode.get("event_id");
            if (cacheMetadataNode == null) {
                return null;
            }
            return cacheMetadataNode.asText();
        } catch (IOException e) {
            throw new RuntimeException(e);
        }
    }

}
