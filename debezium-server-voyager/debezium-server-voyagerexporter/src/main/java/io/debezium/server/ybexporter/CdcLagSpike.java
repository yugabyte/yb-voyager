/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import org.apache.kafka.connect.data.Struct;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * THROWAWAY DIAGNOSTIC. Measures client-side CDC lag from event timestamps, to
 * settle one question before any real metric is built:
 *
 * <p><b>Does {@code source.ts_ms} carry the transaction COMMIT time, or merely the
 * time the walsender emitted the record?</b>
 *
 * <p>Two intervals are recorded per event:
 * <ul>
 * <li>{@code src_to_dbz} = envelope {@code ts_ms} - source {@code ts_ms} — how long
 * YugabyteDB took to get the change to the connector.
 * <li>{@code dbz_to_now} = now - envelope {@code ts_ms} — how long it then spent
 * inside this exporter before being parsed.
 * </ul>
 *
 * <p><b>How to read the result.</b> If {@code source.ts_ms} is the commit time, then
 * under a known backlog {@code src_to_dbz} should grow into the seconds/minutes and
 * track the server-side {@code cdcsdk_sent_lag_micros}. If it is instead the emit
 * time, {@code src_to_dbz} will sit near zero no matter how far behind the pipeline
 * is — which would make it useless as a lag signal, and that is exactly what this
 * spike exists to find out before anything is built on top of it.
 *
 * <p><b>Clock skew warning.</b> Source timestamps are minted by the database host
 * and compared here against this JVM's clock, so any skew between the two lands
 * directly in {@code dbz_to_now} (and in the total). {@code src_to_dbz} compares two
 * timestamps that Debezium itself derives, so it is the more trustworthy of the two.
 * Check NTP on both hosts before believing an absolute number.
 *
 * <p>Inert unless {@code YB_CDC_LAG_SPIKE} is set to 1/true/yes. Enabling it costs
 * two long subtractions and a counter per event; detail lines are sampled and an
 * aggregate is emitted on a fixed interval, so it will not flood the log at a few
 * thousand events/second.
 *
 * <p>Delete this class and its single call site in KafkaConnectRecordParser once the
 * question above is answered.
 */
final class CdcLagSpike {
    private static final Logger LOGGER = LoggerFactory.getLogger(CdcLagSpike.class);

    private static final boolean ENABLED;
    private static final long SAMPLE_EVERY;
    private static final long SUMMARY_MILLIS;

    static {
        ENABLED = envFlag("YB_CDC_LAG_SPIKE");
        SAMPLE_EVERY = envLong("YB_CDC_LAG_SPIKE_SAMPLE", 200L);
        SUMMARY_MILLIS = envLong("YB_CDC_LAG_SPIKE_SUMMARY_SECS", 30L) * 1000L;
        if (ENABLED) {
            LOGGER.warn("CdcLagSpike ENABLED (diagnostic): sampling 1 in {} events, summary every {} ms. "
                    + "This is a throwaway measurement, not a supported metric.", SAMPLE_EVERY, SUMMARY_MILLIS);
        }
    }

    private CdcLagSpike() {
    }

    // Aggregates for the current summary window. Guarded by the class monitor;
    // contention is irrelevant at a few thousand events/second.
    private static long windowStart = 0L;
    private static long seen = 0L;
    private static long missingSourceTs = 0L;
    private static long missingEnvelopeTs = 0L;
    private static long snapshotEvents = 0L;

    private static long srcToDbzMin = Long.MAX_VALUE;
    private static long srcToDbzMax = Long.MIN_VALUE;
    private static long srcToDbzSum = 0L;
    private static long dbzToNowMin = Long.MAX_VALUE;
    private static long dbzToNowMax = Long.MIN_VALUE;
    private static long dbzToNowSum = 0L;
    private static long measured = 0L;

    static boolean enabled() {
        return ENABLED;
    }

    /**
     * Record one event. Never throws: a diagnostic must not be able to break the
     * export path, so any unexpected schema shape is counted and ignored.
     */
    static synchronized void observe(Struct value, Struct source, String op, String snapshot) {
        if (!ENABLED) {
            return;
        }
        try {
            long now = System.currentTimeMillis();
            if (windowStart == 0L) {
                windowStart = now;
            }
            seen++;

            // A snapshot event's source ts_ms is the snapshot time, not a commit
            // time, so it says nothing about streaming lag. Counted, not measured.
            boolean isSnapshot = snapshot != null && !"false".equalsIgnoreCase(snapshot);
            if (isSnapshot) {
                snapshotEvents++;
            }

            Long srcTs = readLong(source, "ts_ms");
            Long envTs = readLong(value, "ts_ms");
            if (srcTs == null) {
                missingSourceTs++;
            }
            if (envTs == null) {
                missingEnvelopeTs++;
            }

            if (!isSnapshot && srcTs != null && envTs != null) {
                long srcToDbz = envTs - srcTs;
                long dbzToNow = now - envTs;
                measured++;
                srcToDbzSum += srcToDbz;
                dbzToNowSum += dbzToNow;
                if (srcToDbz < srcToDbzMin) {
                    srcToDbzMin = srcToDbz;
                }
                if (srcToDbz > srcToDbzMax) {
                    srcToDbzMax = srcToDbz;
                }
                if (dbzToNow < dbzToNowMin) {
                    dbzToNowMin = dbzToNow;
                }
                if (dbzToNow > dbzToNowMax) {
                    dbzToNowMax = dbzToNow;
                }

                if (SAMPLE_EVERY > 0 && measured % SAMPLE_EVERY == 0) {
                    LOGGER.info("cdc-lag-spike sample op={} src_ts={} dbz_ts={} now={} "
                            + "src_to_dbz_ms={} dbz_to_now_ms={} total_ms={}",
                            op, srcTs, envTs, now, srcToDbz, dbzToNow, now - srcTs);
                }
            }

            if (now - windowStart >= SUMMARY_MILLIS) {
                emitSummary(now);
            }
        }
        catch (Exception e) {
            // Deliberately swallowed: see method contract.
            LOGGER.debug("cdc-lag-spike observe failed (ignored)", e);
        }
    }

    private static void emitSummary(long now) {
        long windowMs = Math.max(1L, now - windowStart);
        if (measured > 0) {
            LOGGER.info("cdc-lag-spike SUMMARY window_ms={} events={} measured={} snapshot={} "
                    + "missing_src_ts={} missing_env_ts={} | src_to_dbz_ms min={} avg={} max={} "
                    + "| dbz_to_now_ms min={} avg={} max={}",
                    windowMs, seen, measured, snapshotEvents, missingSourceTs, missingEnvelopeTs,
                    srcToDbzMin, srcToDbzSum / measured, srcToDbzMax,
                    dbzToNowMin, dbzToNowSum / measured, dbzToNowMax);
        }
        else {
            LOGGER.info("cdc-lag-spike SUMMARY window_ms={} events={} measured=0 snapshot={} "
                    + "missing_src_ts={} missing_env_ts={} (nothing measurable this window)",
                    windowMs, seen, snapshotEvents, missingSourceTs, missingEnvelopeTs);
        }
        windowStart = now;
        seen = 0L;
        measured = 0L;
        snapshotEvents = 0L;
        missingSourceTs = 0L;
        missingEnvelopeTs = 0L;
        srcToDbzMin = Long.MAX_VALUE;
        srcToDbzMax = Long.MIN_VALUE;
        srcToDbzSum = 0L;
        dbzToNowMin = Long.MAX_VALUE;
        dbzToNowMax = Long.MIN_VALUE;
        dbzToNowSum = 0L;
    }

    /** Read an int64 field, tolerating an absent field or a null value. */
    private static Long readLong(Struct s, String field) {
        if (s == null || s.schema() == null || s.schema().field(field) == null) {
            return null;
        }
        Object v = s.get(field);
        if (v instanceof Number) {
            return ((Number) v).longValue();
        }
        return null;
    }

    private static boolean envFlag(String name) {
        String v = System.getenv(name);
        if (v == null) {
            return false;
        }
        v = v.trim();
        return v.equals("1") || v.equalsIgnoreCase("true") || v.equalsIgnoreCase("yes");
    }

    private static long envLong(String name, long dflt) {
        String v = System.getenv(name);
        if (v == null || v.trim().isEmpty()) {
            return dflt;
        }
        try {
            return Long.parseLong(v.trim());
        }
        catch (NumberFormatException e) {
            return dflt;
        }
    }
}
