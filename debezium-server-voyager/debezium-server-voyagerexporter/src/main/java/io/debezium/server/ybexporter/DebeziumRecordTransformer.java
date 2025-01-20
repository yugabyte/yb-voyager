/*
 * Copyright Debezium Authors.
 *
 * Licensed under the Apache Software License version 2.0, available at http://www.apache.org/licenses/LICENSE-2.0
 */
package io.debezium.server.ybexporter;

import io.debezium.data.VariableScaleDecimal;
import org.apache.kafka.connect.data.Field;
import org.apache.kafka.connect.data.Schema;
import org.apache.kafka.connect.data.Struct;
import org.apache.kafka.connect.json.JsonConverter;
import org.apache.kafka.connect.json.JsonConverterConfig;

import java.math.BigDecimal;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Map;
import java.util.HashMap;
import org.slf4j.Logger;
import org.slf4j.LoggerFactory;

/**
 * This class ensures of doing any transformation of the record received from debezium
 * before actually writing that record.
 */
public class DebeziumRecordTransformer implements RecordTransformer {
	private static final Logger LOGGER = LoggerFactory.getLogger(DebeziumRecordTransformer.class);

    private JsonConverter jsonConverter;
    public DebeziumRecordTransformer(){
        jsonConverter = new JsonConverter();
        Map<String, String> jsonConfig = Collections.singletonMap(JsonConverterConfig.SCHEMAS_ENABLE_CONFIG, "false");
        jsonConverter.configure(jsonConfig, false);
    }

    @Override
    public void transformRecord(Record r) {
        transformColumnValues(r.keyColumns, r.keyValues, r.t.fieldSchemas);
        transformColumnValues(r.afterValueColumns, r.afterValueValues, r.t.fieldSchemas);

        transformColumnValues(r.beforeValueColumns, r.beforeValueValues, r.t.fieldSchemas);
    }

    private void transformColumnValues(ArrayList<String> columnNames, ArrayList<Object> values, Map<String, Field> fieldSchemas){
        for (int i = 0; i < values.size(); i++) {
            Object val = values.get(i);
            String columnName = columnNames.get(i);
            Object formattedVal = makeFieldValueSerializable(val, fieldSchemas.get(columnName));
            values.set(i, formattedVal);
        }
    }

    /**
     * For certain data-types like decimals/bytes/structs, we convert them
     * to certain formats that is serializable by downstream snapshot/streaming
     * writers. For the rest, we just stringify them.
     */
    private String makeFieldValueSerializable(Object fieldValue, Field field){
        if (fieldValue == null) {
            return null;
        }
        String logicalType = field.schema().name();
        if (logicalType != null) {
            switch (logicalType) {
                case "org.apache.kafka.connect.data.Decimal":
                    return ((BigDecimal) fieldValue).toString();
                case "io.debezium.data.VariableScaleDecimal":
                    return VariableScaleDecimal.toLogical((Struct)fieldValue).toString();
            }
        }
        Schema.Type type = field.schema().type();
        switch (type){
            case BYTES:
            case STRUCT:
                return toKafkaConnectJsonConverted(fieldValue, field);
            case MAP:
	            StringBuilder mapString = new StringBuilder();
                for (Map.Entry<String, String> entry : ((HashMap<String, String>) fieldValue).entrySet()) {
                    String key = entry.getKey();
                    String val = entry.getValue();
                    LOGGER.debug("[MAP] before transforming key - {}", key);
                    LOGGER.debug("[MAP] before transforming value - {}", val);                    
                    /*
                     Escaping the key and value here for the  double quote (")" and backslash char (\) with a backslash character as mentioned here 
                     https://www.postgresql.org/docs/9/hstore.html#:~:text=To%20include%20a%20double%20quote%20or%20a%20backslash%20in%20a%20key%20or%20value%2C%20escape%20it%20with%20a%20backslash.
                     
                     Following the order of escaping the backslash first and then the double quote becasue first escape the backslashes in the string and adding the backslash for escaping to handle case like
                     e.g. key - "a\"b" -> (first escaping) -> "a\\"b" -> (second escaping) -> "a\\\"b"
                     */
                    key = key.replace("\\", "\\\\"); // escaping backslash \ -> \\ ( "a\b" -> "a\\b" ) "
                    val = val.replace("\\", "\\\\");
                    key = key.replace("\"", "\\\""); // escaping double quotes " -> \" ( "a"b" -> "a\"b" ) "
                    val = val.replace("\"", "\\\"");

		            LOGGER.debug("[MAP] after transforming key - {}", key);
                    LOGGER.debug("[MAP] after transforming value - {}", val);
                    
                    mapString.append("\"");
                    mapString.append(key);
                    mapString.append("\"");
                    mapString.append(" => ");
                    mapString.append("\"");
                    mapString.append(val);
                    mapString.append("\"");
                    mapString.append(",");
                }
		        if(mapString.length() == 0) {
                    return "";
                } 
                return mapString.toString().substring(0, mapString.length() - 1);
            
        }
        return fieldValue.toString();
    }

    /**
     * Use the kafka connect json converter to convert it to a json friendly string
     */
    private String toKafkaConnectJsonConverted(Object fieldValue, Field f){
        String jsonFriendlyString = new String(jsonConverter.fromConnectData("test", f.schema(), fieldValue));
        if (jsonFriendlyString.length() > 0){
            if ((jsonFriendlyString.charAt(0) == '"') && (jsonFriendlyString.charAt(jsonFriendlyString.length()-1) == '"')){
                jsonFriendlyString = jsonFriendlyString.substring(1, jsonFriendlyString.length() - 1);
            }
        }
        return jsonFriendlyString;
    }
}
