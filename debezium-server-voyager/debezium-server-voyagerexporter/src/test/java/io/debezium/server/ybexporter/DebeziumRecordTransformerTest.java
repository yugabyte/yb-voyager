/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import static org.assertj.core.api.Assertions.assertThat;

import java.util.HashMap;
import java.util.LinkedHashMap;

import org.apache.kafka.connect.data.Field;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.junit.jupiter.api.BeforeEach;
import org.junit.jupiter.api.Test;

/**
 * Regression tests for {@link DebeziumRecordTransformer}'s MAP branch.
 *
 * NOTE: hstore columns no longer reach this branch in normal operation.
 * {@link PostgresToYbValueConverter} registers a string pass-through for hstore, so the
 * column's schema is STRING and postgres' own text representation is forwarded untouched
 * - see PostgresToYbValueConverterTest.
 *
 * This branch is nonetheless kept null-safe as a backstop: if that registration ever does
 * not happen (e.g. the column's typeName does not match), a null map value should produce
 * correct hstore text rather than an NPE that permanently stalls streaming.
 */
public class DebeziumRecordTransformerTest {

    private DebeziumRecordTransformer transformer;

    @BeforeEach
    public void setUp() {
        transformer = new DebeziumRecordTransformer();
    }

    private static Table tableWithMapColumn() {
        Table t = new Table("testdb", "public", "payments");
        Schema mapSchema = SchemaBuilder
                .map(Schema.STRING_SCHEMA, Schema.OPTIONAL_STRING_SCHEMA)
                .optional()
                .build();
        LinkedHashMap<String, Field> fieldSchemas = new LinkedHashMap<>();
        fieldSchemas.put("metadata", new Field("metadata", 0, mapSchema));
        t.fieldSchemas = fieldSchemas;
        return t;
    }

    /**
     * Runs a single MAP column value through the transformer and returns what
     * would be written out for it.
     */
    private String transformMap(HashMap<String, String> mapValue) {
        Record r = new Record();
        r.t = tableWithMapColumn();
        r.op = "u";
        r.addAfterValueField("metadata", mapValue);
        transformer.transformRecord(r);
        return (String) r.afterValueValues.get(0);
    }

    // ---------------------------------------------------------------------
    // Backstop: a null map value must serialize, not throw.
    // ---------------------------------------------------------------------

    @Test
    public void mapEntryWithNullValueIsSerializedAsUnquotedNull() {
        HashMap<String, String> map = new HashMap<>();
        map.put("MY_CUSTOM", null);

        assertThat(transformMap(map)).isEqualTo("\"MY_CUSTOM\" => NULL");
    }

    @Test
    public void mapWithNullValueAmongOtherEntriesSerializesAllEntries() {
        HashMap<String, String> map = new LinkedHashMap<>();
        map.put("source_system", "legacy");
        map.put("MY_CUSTOM", null);
        map.put("region", "eu-west");

        assertThat(transformMap(map))
                .isEqualTo("\"source_system\" => \"legacy\",\"MY_CUSTOM\" => NULL,\"region\" => \"eu-west\"");
    }

    @Test
    public void mapWithOnlyNullValuesSerializesAllEntries() {
        HashMap<String, String> map = new LinkedHashMap<>();
        map.put("a", null);
        map.put("b", null);

        assertThat(transformMap(map)).isEqualTo("\"a\" => NULL,\"b\" => NULL");
    }

    // ---------------------------------------------------------------------
    // Existing behaviour, and guards against collapsing the three states.
    // ---------------------------------------------------------------------

    @Test
    public void mapEntryWithOrdinaryValueIsQuoted() {
        HashMap<String, String> map = new HashMap<>();
        map.put("MY_CUSTOM", "some_value");

        assertThat(transformMap(map)).isEqualTo("\"MY_CUSTOM\" => \"some_value\"");
    }

    @Test
    public void mapEntryWithQuotesAndBackslashesIsEscaped() {
        HashMap<String, String> map = new HashMap<>();
        map.put("a\"b", "c\\d");

        assertThat(transformMap(map)).isEqualTo("\"a\\\"b\" => \"c\\\\d\"");
    }

    @Test
    public void mapEntryWithEmptyStringValueStaysQuotedEmptyString() {
        HashMap<String, String> map = new HashMap<>();
        map.put("MY_CUSTOM", "");

        assertThat(transformMap(map)).isEqualTo("\"MY_CUSTOM\" => \"\"");
    }

    @Test
    public void mapEntryWithLiteralNullStringStaysQuoted() {
        HashMap<String, String> map = new HashMap<>();
        map.put("MY_CUSTOM", "NULL");

        assertThat(transformMap(map)).isEqualTo("\"MY_CUSTOM\" => \"NULL\"");
    }

    @Test
    public void emptyMapSerializesToEmptyString() {
        assertThat(transformMap(new HashMap<>())).isEqualTo("");
    }

    @Test
    public void nullMapColumnStaysNull() {
        assertThat(transformMap(null)).isNull();
    }
}
