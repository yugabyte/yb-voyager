/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.sql.JDBCType;
import java.text.DecimalFormat;
import java.text.DecimalFormatSymbols;
import java.util.Locale;
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

        JDBCType jdbcType = JDBCType.valueOf(column.jdbcType());
        switch (jdbcType) {
            case STRUCT:
            case ARRAY:
                registration.register(SchemaBuilder.string(), this::stringify);
                break;

        }
        switch (column.typeName()) {
            case "tsvector":
            case "tsquery":
                registration.register(SchemaBuilder.string(), this::stringify);
                break;
        }
    }

    private Object stringify(Object x){
        if (x == null) {
            return null;
        }
        else {
            return x.toString();
        }
    }

}
