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
 * Tests for {@link DebeziumRecordTransformer}'s handling of Postgres hstore columns,
 * which Debezium delivers as a Kafka Connect MAP when hstore.handling.mode=map
 * (the mode yb-voyager configures - see yb-voyager/src/dbzm/config.go).
 *
 * An hstore value distinguishes three states that must NOT be conflated:
 *   'k=>"v"'   -> key present, ordinary value
 *   'k=>""'    -> key present, value is the empty string
 *   'k=>NULL'  -> key present, value is SQL NULL
 * Debezium models the third as a null entry inside the map, which is why the map's
 * value schema is OPTIONAL_STRING_SCHEMA.
 */
public class DebeziumRecordTransformerTest {

    private DebeziumRecordTransformer transformer;

    @BeforeEach
    public void setUp() {
        transformer = new DebeziumRecordTransformer();
    }

    /**
     * Builds the schema Debezium emits for an hstore column in map mode. The value
     * schema is optional, i.e. Debezium explicitly permits null values in this map.
     */
    private static Table tableWithHstoreColumn() {
        Table t = new Table("testdb", "public", "payments");
        Schema hstoreSchema = SchemaBuilder
                .map(Schema.STRING_SCHEMA, Schema.OPTIONAL_STRING_SCHEMA)
                .optional()
                .build();
        LinkedHashMap<String, Field> fieldSchemas = new LinkedHashMap<>();
        fieldSchemas.put("metadata", new Field("metadata", 0, hstoreSchema));
        t.fieldSchemas = fieldSchemas;
        return t;
    }

    /**
     * Runs a single hstore column value through the transformer and returns what
     * would be written out for it.
     */
    private String transformHstore(HashMap<String, String> hstoreValue) {
        Record r = new Record();
        r.t = tableWithHstoreColumn();
        r.op = "u";
        r.addAfterValueField("metadata", hstoreValue);
        transformer.transformRecord(r);
        return (String) r.afterValueValues.get(0);
    }

    // ---------------------------------------------------------------------
    // The bug: an hstore entry whose value is SQL NULL.
    // ---------------------------------------------------------------------

    /**
     * Before the fix this threw NullPointerException from makeFieldValueSerializable,
     * permanently stalling CDC streaming: the connector crashes before committing the
     * offset, so every restart replays the same WAL record and crashes identically.
     */
    @Test
    public void hstoreEntryWithNullValueIsSerializedAsUnquotedNull() {
        HashMap<String, String> hstore = new HashMap<>();
        hstore.put("MY_CUSTOM", null);

        assertThat(transformHstore(hstore)).isEqualTo("\"MY_CUSTOM\" => NULL");
    }

    /**
     * The null entry must not take down the entries around it, and must not disturb
     * how they are quoted.
     */
    @Test
    public void hstoreWithNullValueAmongOtherEntriesSerializesAllEntries() {
        HashMap<String, String> hstore = new LinkedHashMap<>();
        hstore.put("source_system", "legacy");
        hstore.put("MY_CUSTOM", null);
        hstore.put("region", "eu-west");

        assertThat(transformHstore(hstore))
                .isEqualTo("\"source_system\" => \"legacy\",\"MY_CUSTOM\" => NULL,\"region\" => \"eu-west\"");
    }

    /**
     * An hstore whose every value is NULL still has to produce valid hstore text.
     */
    @Test
    public void hstoreWithOnlyNullValuesSerializesAllEntries() {
        HashMap<String, String> hstore = new LinkedHashMap<>();
        hstore.put("a", null);
        hstore.put("b", null);

        assertThat(transformHstore(hstore)).isEqualTo("\"a\" => NULL,\"b\" => NULL");
    }

    // ---------------------------------------------------------------------
    // Guards against "fixing" the NPE by coercing null to something else.
    // Postgres treats all three of these as different values, so collapsing
    // them would silently corrupt migrated data instead of crashing loudly.
    // ---------------------------------------------------------------------

    /**
     * 'k=>""' must stay an empty string and must NOT become NULL.
     */
    @Test
    public void hstoreEntryWithEmptyStringValueStaysQuotedEmptyString() {
        HashMap<String, String> hstore = new HashMap<>();
        hstore.put("MY_CUSTOM", "");

        assertThat(transformHstore(hstore)).isEqualTo("\"MY_CUSTOM\" => \"\"");
    }

    /**
     * A value that is literally the four characters N-U-L-L must stay quoted,
     * otherwise it would be read back as SQL NULL.
     */
    @Test
    public void hstoreEntryWithLiteralNullStringStaysQuoted() {
        HashMap<String, String> hstore = new HashMap<>();
        hstore.put("MY_CUSTOM", "NULL");

        assertThat(transformHstore(hstore)).isEqualTo("\"MY_CUSTOM\" => \"NULL\"");
    }

    // ---------------------------------------------------------------------
    // Existing behaviour that must not regress.
    // ---------------------------------------------------------------------

    @Test
    public void hstoreEntryWithOrdinaryValueIsQuoted() {
        HashMap<String, String> hstore = new HashMap<>();
        hstore.put("MY_CUSTOM", "some_value");

        assertThat(transformHstore(hstore)).isEqualTo("\"MY_CUSTOM\" => \"some_value\"");
    }

    @Test
    public void hstoreEntryWithQuotesAndBackslashesIsEscaped() {
        HashMap<String, String> hstore = new HashMap<>();
        hstore.put("a\"b", "c\\d");

        assertThat(transformHstore(hstore)).isEqualTo("\"a\\\"b\" => \"c\\\\d\"");
    }

    @Test
    public void emptyHstoreSerializesToEmptyString() {
        assertThat(transformHstore(new HashMap<>())).isEqualTo("");
    }

    /**
     * A wholly NULL hstore column is a different case from an hstore containing a
     * null value, and was already handled correctly.
     */
    @Test
    public void nullHstoreColumnStaysNull() {
        assertThat(transformHstore(null)).isNull();
    }
}
