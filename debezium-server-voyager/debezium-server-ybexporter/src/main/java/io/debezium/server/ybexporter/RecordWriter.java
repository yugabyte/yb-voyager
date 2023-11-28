/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

public interface RecordWriter {
    void writeRecord(Record r);

    void flush();

    void close();

    void sync();
}
