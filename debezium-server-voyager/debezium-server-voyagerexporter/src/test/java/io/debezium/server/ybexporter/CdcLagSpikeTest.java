/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import static org.junit.jupiter.api.Assertions.assertDoesNotThrow;
import static org.junit.jupiter.api.Assertions.assertFalse;

import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.SchemaBuilder;
import org.apache.kafka.connect.data.Struct;
import org.junit.jupiter.api.Test;

/**
 * Guards the two properties the diagnostic must have: it is off unless explicitly
 * enabled, and it can never throw into the export path whatever shape of record it
 * is handed. Delete alongside CdcLagSpike.
 */
class CdcLagSpikeTest {

    private static Struct structWithoutTsMs() {
        Schema s = SchemaBuilder.struct().field("snapshot", Schema.OPTIONAL_STRING_SCHEMA).build();
        return new Struct(s).put("snapshot", "false");
    }

    private static Struct structWithTsMs(long tsMs) {
        Schema s = SchemaBuilder.struct()
                .field("ts_ms", Schema.OPTIONAL_INT64_SCHEMA)
                .field("snapshot", Schema.OPTIONAL_STRING_SCHEMA)
                .build();
        return new Struct(s).put("ts_ms", tsMs).put("snapshot", "false");
    }

    private static Struct structWithNullTsMs() {
        Schema s = SchemaBuilder.struct().field("ts_ms", Schema.OPTIONAL_INT64_SCHEMA).build();
        return new Struct(s);
    }

    @Test
    void isDisabledUnlessEnvVarSet() {
        // The test JVM does not set YB_CDC_LAG_SPIKE, so the diagnostic must be inert.
        assertFalse(CdcLagSpike.enabled(), "spike must default to off");
    }

    @Test
    void observeNeverThrowsOnAnyRecordShape() {
        long now = System.currentTimeMillis();
        assertDoesNotThrow(() -> {
            // fields entirely absent from the schema
            CdcLagSpike.observe(structWithoutTsMs(), structWithoutTsMs(), "c", "false");
            // present but null
            CdcLagSpike.observe(structWithNullTsMs(), structWithNullTsMs(), "c", "false");
            // well-formed streaming event
            CdcLagSpike.observe(structWithTsMs(now), structWithTsMs(now - 250), "u", "false");
            // snapshot event, which must be counted and not measured
            CdcLagSpike.observe(structWithTsMs(now), structWithTsMs(now), "r", "true");
            // nulls everywhere
            CdcLagSpike.observe(null, null, null, null);
        });
    }
}
