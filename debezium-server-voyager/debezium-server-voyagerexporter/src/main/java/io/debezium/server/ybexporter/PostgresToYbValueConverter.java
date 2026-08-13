/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.sql.JDBCType;
import java.util.Properties;

import org.apache.kafka.connect.data.SchemaBuilder;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import io.debezium.spi.converter.CustomConverter;
import io.debezium.spi.converter.RelationalColumn;

public class PostgresToYbValueConverter implements CustomConverter<SchemaBuilder, RelationalColumn> {

    private static final Logger LOGGER = LoggerFactory.getLogger(PostgresToYbValueConverter.class);

    @Override
    public void configure(Properties props) {
        return;
    }

    @Override
    public void converterFor(RelationalColumn column,
            ConverterRegistration<SchemaBuilder> registration) {

        LOGGER.debug("Processing converter for column: {}, type: {}, JDBC type: {}", column.name(), column.typeName(), column.jdbcType());
        JDBCType jdbcType = JDBCType.valueOf(column.jdbcType());
        switch (jdbcType) {
            case BIT:
            case STRUCT:
            case ARRAY:
                LOGGER.info("Configuring stringify converter for column: {}, type: {}, JDBC type: {}", column.name(), column.typeName(), column.jdbcType());
                registration.register(SchemaBuilder.string(), this::stringify);
                break;

        }
        switch (column.typeName()) {
            case "varbit":
            case "tsvector":
            case "tsquery":
            /*
             * hstore: pass through postgres' own text representation (e.g. "key"=>"value", "k2"=>NULL)
             * rather than letting debezium decode it into a Map (hstore.handling.mode=map) which we
             * would then have to re-serialize by hand. Postgres has already done the quoting and
             * escaping correctly, and it is the only representation that can express a SQL NULL entry
             * value distinctly from an empty string. Re-serializing a decoded Map re-does that escaping
             * and previously threw an NPE on null entry values.
             */
            case "hstore":
                LOGGER.info("Configuring stringify converter for column: {}, type: {}, JDBC type: {}", column.name(), column.typeName(), column.jdbcType());
                registration.register(SchemaBuilder.string(), this::stringify);
                break;
        }
    }

    private Object stringify(Object x) {
        if (x == null) {
            return null;
        } else {
            LOGGER.debug("stringify: input: {}, class: {}", x, x.getClass().getName());
            return x.toString();
        }
    }

}
