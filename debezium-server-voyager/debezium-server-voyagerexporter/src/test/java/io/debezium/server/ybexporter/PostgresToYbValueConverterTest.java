/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import static org.assertj.core.api.Assertions.assertThat;

import java.sql.Types;
import java.util.OptionalInt;
import java.util.Properties;

import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

import io.debezium.spi.converter.CustomConverter;
import io.debezium.spi.converter.RelationalColumn;

/**
 * Tests that {@link PostgresToYbValueConverter} passes hstore through as postgres' own text
 * instead of letting debezium decode it into a Kafka Connect MAP.
 */
public class PostgresToYbValueConverterTest {

    private PostgresToYbValueConverter converter;

    @BeforeEach
    public void setUp() {
        converter = new PostgresToYbValueConverter();
        converter.configure(new Properties());
    }

    /** Captures whatever the converter registers for a column, if anything. */
    private static class CapturingRegistration implements CustomConverter.ConverterRegistration<SchemaBuilder> {
        SchemaBuilder schema;
        CustomConverter.Converter converter;

        @Override
        public void register(SchemaBuilder schema, CustomConverter.Converter converter) {
            this.schema = schema;
            this.converter = converter;
        }

        boolean registered() {
            return converter != null;
        }
    }

    private static RelationalColumn column(String typeName, int jdbcType) {
        return new RelationalColumn() {
            @Override
            public String name() {
                return "metadata";
            }

            @Override
            public String dataCollection() {
                return "public.payments";
            }

            @Override
            public int jdbcType() {
                return jdbcType;
            }

            @Override
            public int nativeType() {
                return 0;
            }

            @Override
            public String typeName() {
                return typeName;
            }

            @Override
            public String typeExpression() {
                return typeName;
            }

            @Override
            public OptionalInt length() {
                return OptionalInt.empty();
            }

            @Override
            public OptionalInt scale() {
                return OptionalInt.empty();
            }

            @Override
            public boolean isOptional() {
                return true;
            }

            @Override
            public Object defaultValue() {
                return null;
            }

            @Override
            public boolean hasDefaultValue() {
                return false;
            }
        };
    }

    private CapturingRegistration convertFor(String typeName, int jdbcType) {
        CapturingRegistration reg = new CapturingRegistration();
        converter.converterFor(column(typeName, jdbcType), reg);
        return reg;
    }

    // ---------------------------------------------------------------------
    // hstore is routed to the string pass-through.
    // ---------------------------------------------------------------------

    /** A STRING schema is what keeps the value out of the MAP branch. */
    @Test
    public void hstoreColumnRegistersStringSchema() {
        CapturingRegistration reg = convertFor("hstore", Types.OTHER);

        assertThat(reg.registered()).isTrue();
        assertThat(reg.schema.build().type()).isEqualTo(Schema.Type.STRING);
    }

    /** A SQL NULL entry value must survive untouched as an unquoted NULL. */
    @Test
    public void hstorePassThroughPreservesNullEntryValue() {
        CapturingRegistration reg = convertFor("hstore", Types.OTHER);

        assertThat(reg.converter.convert("\"source\"=>\"import\", \"MY_CUSTOM\"=>NULL"))
                .isEqualTo("\"source\"=>\"import\", \"MY_CUSTOM\"=>NULL");
    }

    /** NULL, empty string and the literal string "NULL" are three distinct values. */
    @Test
    public void hstorePassThroughKeepsNullEmptyAndLiteralNullDistinct() {
        CapturingRegistration reg = convertFor("hstore", Types.OTHER);

        assertThat(reg.converter.convert("\"k\"=>NULL")).isEqualTo("\"k\"=>NULL");
        assertThat(reg.converter.convert("\"k\"=>\"\"")).isEqualTo("\"k\"=>\"\"");
        assertThat(reg.converter.convert("\"k\"=>\"NULL\"")).isEqualTo("\"k\"=>\"NULL\"");
    }

    /** Escaping is postgres' job; the pass-through must not touch it. */
    @Test
    public void hstorePassThroughDoesNotReEscapeQuotesOrBackslashes() {
        CapturingRegistration reg = convertFor("hstore", Types.OTHER);

        assertThat(reg.converter.convert("\"a\\\"b\"=>\"c\\\"d\"")).isEqualTo("\"a\\\"b\"=>\"c\\\"d\"");
        assertThat(reg.converter.convert("\"back\\\\slash\"=>\"val\\\\ue\"")).isEqualTo("\"back\\\\slash\"=>\"val\\\\ue\"");
    }

    /** An empty hstore is an empty string, not null. */
    @Test
    public void hstorePassThroughPreservesEmptyHstore() {
        CapturingRegistration reg = convertFor("hstore", Types.OTHER);

        assertThat(reg.converter.convert("")).isEqualTo("");
    }

    /** A wholly NULL hstore column stays null. */
    @Test
    public void nullHstoreColumnValueStaysNull() {
        CapturingRegistration reg = convertFor("hstore", Types.OTHER);

        assertThat(reg.converter.convert(null)).isNull();
    }

    // ---------------------------------------------------------------------
    // Existing routing must not regress, and must not broaden.
    // ---------------------------------------------------------------------

    @Test
    public void varbitTsvectorAndTsqueryStillRegisterStringSchema() {
        assertThat(convertFor("varbit", Types.OTHER).registered()).isTrue();
        assertThat(convertFor("tsvector", Types.OTHER).registered()).isTrue();
        assertThat(convertFor("tsquery", Types.OTHER).registered()).isTrue();
    }

    /** Guards against the type match broadening to unrelated columns. */
    @Test
    public void ordinaryTextColumnIsNotRegistered() {
        assertThat(convertFor("text", Types.VARCHAR).registered()).isFalse();
    }
}
