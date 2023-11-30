/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import java.math.BigDecimal;
import java.nio.ByteBuffer;
import java.time.Instant;
import java.time.LocalDate;
import java.time.LocalDateTime;
import java.time.LocalTime;
import java.time.ZoneOffset;
import java.time.temporal.ChronoUnit;
import java.util.BitSet;
import java.util.HashMap;
import java.util.Map;

import io.debezium.data.VariableScaleDecimal;
import org.apache.kafka.connect.data.Field;
import org.apache.kafka.connect.data.Schema.Type;
import org.apache.kafka.connect.data.Struct;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

import io.debezium.data.Bits;

public class YugabyteDialectConverter {
    private static final Logger LOGGER = LoggerFactory.getLogger(YugabyteDialectConverter.class);

    /**
     * TODO: Try using a Custom Converter - https://debezium.io/documentation/reference/stable/development/converters.html
     * instead to handle these conversions.
     * Converts objects from those present in kafka connect SourceRecord object to that interpretable by Yugabyte's dialect.
     * @param field of type kafka.connect.data.Field having schema that of
     *              https://kafka.apache.org/20/javadoc/org/apache/kafka/connect/data/Schema.Type.html
     * @param fieldValue Value of field as present in kafka.connect.data.SourceRecord. For example, as java int/long/bool
     *                   or kafka.connect Struct
     */
    public static Object fromConnect(Field field, Object fieldValue) {
        // LOGGER.info("field={}", field);
        if (fieldValue == null) {
            return fieldValue;
        }
        // The below types are not supported by debezium's postgres connector.
        // Therefore, we interpret the actual source column type (set by config datatype.propagate.source.type)
        // and then handle them.
        var params = field.schema().parameters();
        if (params != null) {
            String columnType = params.get("__debezium.source.column.type");
            if (columnType != null) {
                switch (columnType) {
                    case "BOX":
                    case "LINE":
                    case "LSEG":
                    case "PATH":
                    case "POLYGON":
                    case "CIRCLE":
                        byte[] byteArr = ((ByteBuffer) fieldValue).array();
                        return new String(byteArr);
                }
            }
        }

        String logicalType = field.schema().name();
        if (logicalType != null) {
            switch (logicalType) {
                case "io.debezium.time.Date":
                    LocalDate date = LocalDate.ofEpochDay(Long.valueOf((Integer) fieldValue));
                    return date.toString(); // default yyyy-MM-dd
                case "io.debezium.time.Time":
                    long millisecondsSinceMidnight = Long.valueOf((Integer) fieldValue);
                    LocalTime t = LocalTime.ofSecondOfDay(0).plus(millisecondsSinceMidnight, ChronoUnit.MILLIS);
                    return t.toString();
                case "io.debezium.time.MicroTime":
                    long microsecondsSinceMidnight = (Long) fieldValue;
                    LocalTime mt = LocalTime.ofSecondOfDay(0).plus(microsecondsSinceMidnight, ChronoUnit.MICROS);
                    return mt.toString();
                case "io.debezium.time.Timestamp":
                    long tsEpochMilliSeconds = (Long) fieldValue;
                    LocalDateTime tsDt = LocalDateTime.ofInstant(Instant.ofEpochMilli(tsEpochMilliSeconds), ZoneOffset.UTC);
                    return tsDt.toString();
                case "io.debezium.time.MicroTimestamp":
                    long epochMicroSeconds = (Long) fieldValue;
                    long epochSeconds = epochMicroSeconds / 1000000;
                    long nanoOffset = (epochMicroSeconds % 1000000) * 1000;
                    LocalDateTime dt = LocalDateTime.ofInstant(Instant.ofEpochSecond(epochSeconds, nanoOffset), ZoneOffset.UTC);
                    return dt.toString();
                case "io.debezium.time.NanoTimestamp":
                    long nanoEpochNanoSeconds = (Long) fieldValue;
                    long nanoEpochSeconds = nanoEpochNanoSeconds / 1_000_000_000;
                    long nanoEpochNanoOffset = nanoEpochNanoSeconds % 1_000_000_000;
                    LocalDateTime nanoDt = LocalDateTime.ofInstant(Instant.ofEpochSecond(nanoEpochSeconds, nanoEpochNanoOffset), ZoneOffset.UTC);
                    return nanoDt.toString();
                case "io.debezium.data.Bits":
                    BitSet bs = Bits.toBitSet(null, (byte[]) fieldValue);
                    StringBuilder s = new StringBuilder();
                    for (int i = bs.length() - 1; i >= 0; i--) {
                        s.append(bs.get(i) ? "1" : "0");
                    }
                    return s.toString();
                case "io.debezium.data.geometry.Point":
                    // TODO: figure out if we want to represent it as a postgres native point or postgis geometry point.
                    // Struct ptStruct = (Struct) fieldValue;
                    // double[] point = Point.parseWKBPoint(ptStruct.getBytes("wkb"));
                    // return String.format("(%f,%f)", point[0], point[1]);
                case "io.debezium.data.geometry.Geometry":
                case "io.debezium.data.geometry.Geography":
                    Struct geometryStruct = (Struct) fieldValue;
                    byte[] wkb = (byte[]) geometryStruct.get("wkb");
                    return bytesToHex(wkb);
                case "org.apache.kafka.connect.data.Decimal":
                    return ((BigDecimal) fieldValue).toString();
                case "io.debezium.data.VariableScaleDecimal":
                    return VariableScaleDecimal.toLogical((Struct)fieldValue).toString();
            }
        }
        Type type = field.schema().type();
        switch (type) {
            case BYTES:
                String hexPrefix = "\\x";
                byte[] byteArr = ((ByteBuffer) fieldValue).array();
                return hexPrefix + bytesToHex(byteArr);
            case MAP:
                StringBuilder mapString = new StringBuilder();
                for (Map.Entry<String, String> entry : ((HashMap<String, String>) fieldValue).entrySet()) {
                    String key = entry.getKey();
                    String val = entry.getValue();
                    mapString.append("\"");
                    mapString.append(key);
                    mapString.append("\"");
                    mapString.append(" => ");
                    mapString.append("\"");
                    mapString.append(val);
                    mapString.append("\"");
                    mapString.append(",");
                }
                return mapString.toString().substring(0, mapString.length() - 1);
        }

        return fieldValue;
    }

    /**
     * Converts objects to a string representation that can be used in a DML SQL query.
     * For example, strings are single quoted.
     */
    public static String makeSqlStatementCompatible(Object value) {
        if (value == null) {
            return "null";
        }
        if (value instanceof String) {
            // escape single quotes
            String formattedVal = value.toString().replace("'", "''");
            // single quote strings.
            formattedVal = "'" + formattedVal + "'";
            return formattedVal;
        }
        return value.toString();
    }

    /**
     * Credits : https://stackoverflow.com/a/9855338
     */
    private static final char[] HEX_ARRAY = "0123456789ABCDEF".toCharArray();

    private static String bytesToHex(byte[] bytes) {
        char[] hexChars = new char[bytes.length * 2];
        for (int j = 0; j < bytes.length; j++) {
            int v = bytes[j] & 0xFF;
            hexChars[j * 2] = HEX_ARRAY[v >>> 4];
            hexChars[j * 2 + 1] = HEX_ARRAY[v & 0x0F];
        }
        return new String(hexChars);
    }
}
