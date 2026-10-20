/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.sql.JDBCType;
import java.util.Properties;
import java.util.Set;

import org.apache.kafka.connect.data.SchemaBuilder;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import io.debezium.spi.converter.CustomConverter;
import io.debezium.spi.converter.RelationalColumn;

public class PostgresToYbValueConverter implements CustomConverter<SchemaBuilder, RelationalColumn> {

    private static final Logger LOGGER = LoggerFactory.getLogger(PostgresToYbValueConverter.class);

    private static final Set<String> STRINGIFY_TYPE_NAMES = Set.of("varbit", "tsvector", "tsquery", "hstore");

    // Fixed OIDs from PostgreSQL's pg_type catalog, shared by YugabyteDB. hstore is an extension
    // type with a per-database OID, but a domain over it already reaches the record transformer as
    // a MAP through hstore.handling.mode=map, so it needs no entry here.
    static final int VARBIT_OID = 1562;
    static final int TSVECTOR_OID = 3614;
    static final int TSQUERY_OID = 3615;
    private static final Set<Integer> STRINGIFY_ROOT_TYPE_OIDS = Set.of(VARBIT_OID, TSVECTOR_OID, TSQUERY_OID);

    @Override
    public void configure(Properties props) {
        return;
    }

    @Override
    public void converterFor(RelationalColumn column,
            ConverterRegistration<SchemaBuilder> registration) {

        LOGGER.debug("Processing converter for column: {}, type: {}, native type: {}, JDBC type: {}", column.name(), column.typeName(), column.nativeType(), column.jdbcType());
        if (shouldStringify(column)) {
            LOGGER.info("Configuring stringify converter for column: {}, type: {}, native type: {}, JDBC type: {}", column.name(), column.typeName(), column.nativeType(), column.jdbcType());
            registration.register(SchemaBuilder.string(), this::stringify);
        }
    }

    private static boolean shouldStringify(RelationalColumn column) {
        switch (JDBCType.valueOf(column.jdbcType())) {
            case BIT:
            case STRUCT:
            case ARRAY:
                return true;
            default:
                break;
        }
        if (STRINGIFY_TYPE_NAMES.contains(column.typeName())) {
            return true;
        }
        // typeName() of a domain column is the domain's own name, but the PostgreSQL connector sets
        // nativeType() to the OID at the root of the domain chain, so a domain over one of these
        // types is caught here. Without it the value is emitted as raw bytes and lands on the
        // target as a \x hex literal that these types accept as valid input.
        return STRINGIFY_ROOT_TYPE_OIDS.contains(column.nativeType());
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
