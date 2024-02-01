/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.util.ArrayList;

public class Record {
    public Table t;
    public String snapshot;
    public String op;
    public String eventId;
    public long vsn; // Voyager Sequence Number.

     // Value information for 'before' struct
    public ArrayList<String> beforeValueColumns = new ArrayList<>();
    public ArrayList<Object> beforeValueValues = new ArrayList<>();

    // Value information for 'after' struct
    public ArrayList<String> afterValueColumns = new ArrayList<>();
    public ArrayList<Object> afterValueValues = new ArrayList<>();

    public ArrayList<String> keyColumns = new ArrayList<>();
    public ArrayList<Object> keyValues = new ArrayList<>();

    public void clear() {
        t = null;
        snapshot = "";
        op = "";
        vsn = 0;
        keyColumns.clear();
        keyValues.clear();
        afterValueColumns.clear();
        afterValueValues.clear();
        
        beforeValueColumns.clear();
        beforeValueValues.clear();
    }

    public boolean isUnsupported() {
        return op.equals("unsupported");
    }

    public String getTableIdentifier() {
        return t.toString();
    }

    public ArrayList<Object> getAfterValueFieldValues() {
        return afterValueValues;
    }

    public void addAfterValueField(String key, Object value) {
        afterValueColumns.add(key);
        afterValueValues.add(value);
    }

    public void addBeforeValueField(String key, Object value) {
        beforeValueColumns.add(key);
        beforeValueValues.add(value);
    }

    public void addKeyField(String key, Object value) {
        keyColumns.add(key);
        keyValues.add(value);
    }

    @Override
    public String toString() {
        return "Record{" +
                "t=" + t +
                ", snapshot='" + snapshot + '\'' +
                ", op='" + op + '\'' +
                ", vsn=" + vsn +
                ", beforeValueColumns=" + beforeValueColumns +
                ", beforeValueValues=" + beforeValueValues +
                ", keyColumns=" + keyColumns +
                ", keyValues=" + keyValues +
                ", afterValueColumns=" + afterValueColumns +
                ", afterValueValues=" + afterValueValues +
                '}';
    }
}
