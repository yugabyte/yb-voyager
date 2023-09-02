/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
// package for tgtdb value converter suite
package tgtdbsuite

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

)


// value converter Function type
type ConverterFn func(v string, formatIfRequired bool) (string, error)

var YBValueConverterSuite = map[string]ConverterFn{
	"io.debezium.time.Date": func(columnValue string, formatIfRequired bool) (string, error) {
		epochDays, err := strconv.ParseUint(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch seconds: %v", err)
		}
		epochSecs := epochDays * 24 * 60 * 60
		date := time.Unix(int64(epochSecs), 0).UTC().Format(time.DateOnly)
		if formatIfRequired {
			date = fmt.Sprintf("'%s'", date)
		}
		return date, nil
	},
	"io.debezium.time.Timestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch milliseconds: %v", err)
		}
		epochSecs := epochMilliSecs / 1000
		timestamp := time.Unix(epochSecs, 0).UTC().Format(time.DateTime)
		if formatIfRequired {
			timestamp = fmt.Sprintf("'%s'", timestamp)
		}
		return timestamp, nil
	},
	"io.debezium.time.MicroTimestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMicroSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch microseconds: %v", err)
		}
		epochSeconds := epochMicroSecs / 1000000
		epochNanos := (epochMicroSecs % 1000000) * 1000
		timestamp := time.Unix(epochSeconds, epochNanos).UTC().Format("2006-01-02T15:04:05.999999")
		if formatIfRequired {
			timestamp = fmt.Sprintf("'%s'", timestamp)
		}
		return timestamp, nil
	},
	"io.debezium.time.NanoTimestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		epochNanoSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch nanoseconds: %v", err)
		}
		epochSeconds := epochNanoSecs / 1000000000
		epochNanos := epochNanoSecs % 1000000000
		timestamp := time.Unix(epochSeconds, epochNanos).UTC().Format("2006-01-02T15:04:05.999999999")
		if formatIfRequired {
			timestamp = fmt.Sprintf("'%s'", timestamp)
		}
		return timestamp, nil
	},
	"io.debezium.time.ZonedTimestamp": func(columnValue string, formatIfRequired bool) (string, error) {
		// no transformation as columnValue is formatted string from debezium by default
		if formatIfRequired {
			columnValue = fmt.Sprintf("'%s'", columnValue)
		}
		return columnValue, nil
	},
	"io.debezium.time.Time": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch milliseconds: %v", err)
		}
		epochSecs := epochMilliSecs / 1000
		timeValue := time.Unix(epochSecs, 0).Local().Format(time.TimeOnly)
		if formatIfRequired {
			timeValue = fmt.Sprintf("'%s'", timeValue)
		}
		return timeValue, nil
	},
	"io.debezium.time.MicroTime": func(columnValue string, formatIfRequired bool) (string, error) {
		epochMicroSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch microseconds: %v", err)
		}
		epochSeconds := epochMicroSecs / 1000000
		epochNanos := (epochMicroSecs % 1000000) * 1000
		MICRO_TIME_FORMAT := "15:04:05.000000"
		timeValue := time.Unix(epochSeconds, epochNanos).Local().Format(MICRO_TIME_FORMAT)
		if formatIfRequired {
			timeValue = fmt.Sprintf("'%s'", timeValue)
		}
		return timeValue, nil
	},
	"io.debezium.data.Bits": func(columnValue string, formatIfRequired bool) (string, error) {
		bytes, err := base64.StdEncoding.DecodeString(columnValue)
		if err != nil {
			return columnValue, fmt.Errorf("decoding variable scale decimal in base64: %v", err)
		}
		var data uint64
		if len(bytes) >= 8 {
			data = binary.LittleEndian.Uint64(bytes[:8])
		} else {
			for i, b := range bytes {
				data |= uint64(b) << (8 * i)
			}
		}
		if formatIfRequired {
			return fmt.Sprintf("'%b'", data), nil
		} else {
			return fmt.Sprintf("%b", data), nil
		}
	},
	"io.debezium.data.geometry.Point": func(columnValue string, formatIfRequired bool) (string, error) {
		// TODO: figure out if we want to represent it as a postgres native point or postgis point.
		return columnValue, nil
	},
	"io.debezium.data.geometry.Geometry": func(columnValue string, formatIfRequired bool) (string, error) {
		// TODO: figure out if we want to represent it as a postgres native point or postgis geometry point.
		return columnValue, nil
	},
	"io.debezium.data.geometry.Geography": func(columnValue string, formatIfRequired bool) (string, error) {
		//TODO: figure out if we want to represent it as a postgres native geography or postgis geometry geography.
		return columnValue, nil
	},
	"org.apache.kafka.connect.data.Decimal": func(columnValue string, formatIfRequired bool) (string, error) {
		return columnValue, nil //handled in exporter plugin
	},
	"io.debezium.data.VariableScaleDecimal": func(columnValue string, formatIfRequired bool) (string, error) {
		return columnValue, nil //handled in exporter plugin
	},
	"BYTES": func(columnValue string, formatIfRequired bool) (string, error) {
		//decode base64 string to bytes
		decodedBytes, err := base64.StdEncoding.DecodeString(columnValue) //e.g.`////wv==` -> `[]byte{0x00, 0x00, 0x00, 0x00}`
		if err != nil {
			return columnValue, fmt.Errorf("decoding base64 string: %v", err)
		}
		//convert bytes to hex string e.g. `[]byte{0x00, 0x00, 0x00, 0x00}` -> `\\x00000000`
		hexString := ""
		for _, b := range decodedBytes {
			hexString += fmt.Sprintf("%02x", b)
		}
		hexValue := ""
		if formatIfRequired {
			hexValue = fmt.Sprintf("'\\x%s'", hexString) // in insert statement no need of escaping the backslash and add quotes
		} else {
			hexValue = fmt.Sprintf("\\x%s", hexString) // in data file need to escape the backslash
		}
		return string(hexValue), nil
	},
	"MAP": func(columnValue string, _ bool) (string, error) {
		mapValue := make(map[string]interface{})
		err := json.Unmarshal([]byte(columnValue), &mapValue)
		if err != nil {
			return columnValue, fmt.Errorf("parsing map: %v", err)
		}
		var transformedMapValue string
		for key, value := range mapValue {
			transformedMapValue = transformedMapValue + fmt.Sprintf("\"%s\"=>\"%s\",", key, value)
		}
		return fmt.Sprintf("'%s'", transformedMapValue[:len(transformedMapValue)-1]), nil //remove last comma and add quotes
	},
	"STRING": func(columnValue string, formatIfRequired bool) (string, error) {
		if formatIfRequired {
			formattedColumnValue := strings.Replace(columnValue, "'", "''", -1)
			return fmt.Sprintf("'%s'", formattedColumnValue), nil
		} else {
			return columnValue, nil
		}
	},
	"io.debezium.time.Interval": func(columnValue string, formatIfRequired bool) (string, error) {
		if formatIfRequired {
			columnValue = fmt.Sprintf("'%s'", columnValue)
		}
		return columnValue, nil
	},
}