/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.util.ArrayList;
import java.util.LinkedHashMap;

import org.apache.kafka.connect.data.Field;

public class Table {
    public String dbName, schemaName, tableName;
    public LinkedHashMap<String, Field> fieldSchemas = new LinkedHashMap<>();
    private String asString = "";

    public Table(String _dbName, String _schemaName, String _tableName) {
        dbName = _dbName;
        schemaName = _schemaName;
        tableName = _tableName;
        asString = dbName + "-" + schemaName + "-" + tableName;
    }

    @Override
    public String toString() {
        return asString;
        // return String.format("%s-%s-%s", dbName, schemaName, tableName);
    }

    public ArrayList<String> getColumns() {
        return new ArrayList<>(fieldSchemas.keySet());
    }

    @Override
    public int hashCode() {
        return toString().hashCode();
    }

    @Override
    public boolean equals(Object o) {
        if (this == o) {
            return true;
        }
        if (!(o instanceof Table)) {
            return false;
        }
        Table t = (Table) o;
        return dbName.equals(t.dbName)
                && schemaName.equals(t.schemaName)
                && tableName.equals(t.tableName);
    }
}
