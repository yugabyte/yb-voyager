/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.sql.JDBCType;
import java.util.HashSet;
import java.util.Properties;
import java.util.Set;

import org.apache.kafka.connect.data.SchemaBuilder;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import io.debezium.spi.converter.CustomConverter;
import io.debezium.spi.converter.RelationalColumn;

public class PostgresToYbValueConverter implements CustomConverter<SchemaBuilder, RelationalColumn> {

    private static final Logger LOGGER = LoggerFactory.getLogger(PostgresToYbValueConverter.class);

    // Comma-separated name:oid pairs that voyager looks up in the exported database, since an
    // extension type like hstore has a different OID in every database.
    static final String STRINGIFY_TYPE_OIDS_PROPERTY = "stringify.type.oids";

    private final Set<Integer> stringifyRootTypeOids = new HashSet<>();

    @Override
    public void configure(Properties props) {
        String typeOids = props.getProperty(STRINGIFY_TYPE_OIDS_PROPERTY);
        if (typeOids == null || typeOids.isBlank()) {
            throw new IllegalArgumentException("missing converter property " + STRINGIFY_TYPE_OIDS_PROPERTY);
        }
        for (String entry : typeOids.split(",")) {
            String[] nameAndOid = entry.trim().split(":");
            if (nameAndOid.length != 2) {
                throw new IllegalArgumentException("invalid " + STRINGIFY_TYPE_OIDS_PROPERTY + " entry '" + entry + "', expected name:oid");
            }
            // Debezium reads pg_type.oid with (int) getLong, so an OID above Integer.MAX_VALUE wraps
            // the same way here and still matches nativeType().
            stringifyRootTypeOids.add((int) Long.parseLong(nameAndOid[1].trim()));
            LOGGER.info("Stringifying columns whose root type is {} (OID {})", nameAndOid[0].trim(), nameAndOid[1].trim());
        }
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

    private boolean shouldStringify(RelationalColumn column) {
        switch (JDBCType.valueOf(column.jdbcType())) {
            case BIT:
            case STRUCT:
            case ARRAY:
                return true;
            default:
                break;
        }
        // Matched by nativeType(), not typeName(): every connector sets nativeType() to the OID at the
        // root of the domain chain, so it covers a plain column and a domain over the type alike, while
        // typeName() of a domain is the domain's own name. A missed tsvector or tsquery domain is emitted
        // as raw bytes that land on the target as a \x hex literal these types accept as valid input.
        return stringifyRootTypeOids.contains(column.nativeType());
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
