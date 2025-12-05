/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package com.yugabyte.ybvoyager;

/**
 * Marker methods for Byteman fault injection testing.
 * 
 * These methods are intentionally no-ops and serve as stable injection points
 * for Byteman rules during testing. They have zero runtime overhead as the JIT
 * compiler inlines empty methods.
 * 
 * Usage in production code:
 *   BytemanMarkers.checkpoint("before-critical-operation");
 * 
 * Usage in Go tests:
 *   NewRule("test").AtMarker(MarkerCheckpoint, "before-critical-operation").ThrowException(...)
 * 
 * @see https://byteman.jboss.org/ for Byteman documentation
 */
public class BytemanMarkers {
    
    /**
     * Generic checkpoint marker for any operation.
     * Use this for general-purpose injection points.
     * 
     * @param name Descriptive checkpoint name (e.g., "before-wal-read", "after-commit")
     */
    public static void checkpoint(String name) {
        // Intentionally empty - used for Byteman injection
    }
    
    /**
     * Database operation marker.
     * Use this for database-related operations.
     * 
     * @param operation Operation type (e.g., "connect", "query", "commit", "rollback")
     */
    public static void db(String operation) {
        // Intentionally empty - used for Byteman injection
    }
    
    /**
     * CDC/Streaming operation marker.
     * Use this for change data capture and streaming operations.
     * 
     * @param event Event type (e.g., "before-poll", "after-poll", "process-change", "commit-offset")
     */
    public static void cdc(String event) {
        // Intentionally empty - used for Byteman injection
    }
    
    /**
     * Snapshot export operation marker.
     * Use this for snapshot phase operations.
     * 
     * @param phase Phase name (e.g., "start", "before-table", "during-read", "after-table", "complete")
     */
    public static void snapshot(String phase) {
        // Intentionally empty - used for Byteman injection
    }
}


