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
package tgtdbsuite

import (
	"encoding/base64"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/schemareg"
)

var OraValueConverterSuite = map[string]ConverterFn{
	"DATE": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		// from oracle for DATE type debezium gives epoch milliseconds with type `io.debezium.time.Timestamp`
		epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch milliseconds: %v", err)
		}
		epochSecs := epochMilliSecs / 1000
		parsedTime := time.Unix(epochSecs, 0).UTC()
		oracleDateFormat := "02-01-2006" //format: DD-MON-YYYY
		formattedDate := parsedTime.Format(oracleDateFormat)
		if err != nil {
			return "", fmt.Errorf("parsing date: %v", err)
		}
		if formatIfRequired {
			formattedDate = fmt.Sprintf("TO_DATE('%s', 'DD-MM-YYYY')", formattedDate)
		}
		return formattedDate, nil
	},
	"io.debezium.time.Date": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		epochDays, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch seconds: %v", err)
		}
		epochSecs := epochDays * 24 * 60 * 60
		parsedTime := time.Unix(int64(epochSecs), 0).UTC()
		oracleDateFormat := "02-01-2006" //format: DD-MON-YYYY
		formattedDate := parsedTime.Format(oracleDateFormat)
		if formatIfRequired {
			formattedDate = fmt.Sprintf("TO_DATE('%s', 'DD-MM-YYYY')", formattedDate)
		}
		return formattedDate, nil
	},
	"io.debezium.time.Timestamp": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		epochMilliSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch milliseconds: %v", err)
		}
		epochSecs := epochMilliSecs / 1000
		timestamp := time.Unix(epochSecs, 0).UTC()
		oracleTimestampFormat := "02-01-2006 03.04.05.000 PM" //format: DD-MM-YY HH.MI.SS.FFF PM
		formattedTimestamp := timestamp.Format(oracleTimestampFormat)
		if formatIfRequired {
			formattedTimestamp = fmt.Sprintf("TO_TIMESTAMP('%s','DD-MM-RR HH:MI:SS.FF9 AM')", formattedTimestamp)
		}
		return formattedTimestamp, nil
	},
	"io.debezium.time.MicroTimestamp": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		epochMicroSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return columnValue, fmt.Errorf("parsing epoch microseconds: %v", err)
		}
		epochSeconds := epochMicroSecs / 1000000
		epochNanos := (epochMicroSecs % 1000000) * 1000
		microTimeStamp := time.Unix(epochSeconds, epochNanos).UTC()
		oracleTimestampFormat := "02-01-2006 03.04.05.000000 PM" //format: DD-MON-YYYY HH.MI.SS.FFFFFF PM
		formattedTimestamp := microTimeStamp.Format(oracleTimestampFormat)
		if formatIfRequired {
			formattedTimestamp = fmt.Sprintf("TO_TIMESTAMP('%s','DD-MM-RR HH:MI:SS.FF9 AM')", formattedTimestamp)
		}
		return formattedTimestamp, nil
	},
	"io.debezium.time.NanoTimestamp": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		epochNanoSecs, err := strconv.ParseInt(columnValue, 10, 64)
		if err != nil {
			return "", fmt.Errorf("parsing epoch nanoseconds: %v", err)
		}
		epochSeconds := epochNanoSecs / 1000000000
		epochNanos := epochNanoSecs % 1000000000
		nanoTimestamp := time.Unix(epochSeconds, epochNanos).UTC()
		oracleTimestampFormat := "02-01-2006 03.04.05.000000000 PM" //format: DD-MON-YYYY HH.MI.SS.FFFFFFFFF PM
		formattedTimestamp := nanoTimestamp.Format(oracleTimestampFormat)
		if formatIfRequired {
			formattedTimestamp = fmt.Sprintf("TO_TIMESTAMP('%s','DD-MM-RR HH:MI:SS.FF9 AM')", formattedTimestamp)
		}
		return formattedTimestamp, nil
	},
	"io.debezium.time.ZonedTimestamp": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		debeziumFormat := "2006-01-02T15:04:05Z07:00"

		parsedTime, err := time.Parse(debeziumFormat, columnValue)
		if err != nil {
			return "", fmt.Errorf("parsing timestamp: %v", err)
		}

		oracleFormat := "06-01-02 3:04:05.000000000 PM -07:00"
		format := "RR-MM-DD HH:MI:SS.FF9 AM TZR"
		formattedTimestamp := parsedTime.Format(oracleFormat)
		if formatIfRequired {
			formattedTimestamp = fmt.Sprintf("TO_TIMESTAMP_TZ('%s','%s')", formattedTimestamp, format)
		}
		return formattedTimestamp, nil
	},
	"BYTES": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		//decode base64 string to bytes
		decodedBytes, err := base64.StdEncoding.DecodeString(columnValue) //e.g.`////wv==` -> `[]byte{0x00, 0x00, 0x00, 0x00}`
		if err != nil {
			return columnValue, fmt.Errorf("decoding base64 string: %v", err)
		}
		//convert bytes to hex string e.g. `[]byte{0x00, 0x00, 0x00, 0x00}` -> `0000000`
		hexString := ""
		for _, b := range decodedBytes {
			hexString += fmt.Sprintf("%02x", b)
		}
		hexValue := hexString
		if formatIfRequired {
			hexValue = fmt.Sprintf("'%s'", hexString) // in insert statement no need of escaping the backslash and add quotes
		}
		return string(hexValue), nil
	},
	"STRING": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		if formatIfRequired {
			formattedColumnValue := strings.Replace(columnValue, "'", "''", -1)
			return fmt.Sprintf("'%s'", formattedColumnValue), nil
		} else {
			return columnValue, nil
		}
	},
	"INTERVAL YEAR TO MONTH": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		// for INTERVAL types of oracle with default precision
		// columnValue format: P-1Y-5M0DT0H0M0S
		splits := strings.Split(strings.TrimPrefix(columnValue, "P"), "M") // ["-1Y-5", "0DT0H0M0S"]
		yearsMonths := strings.Split(splits[0], "Y")                       // ["-1", "-5"]
		years, err := strconv.ParseInt(yearsMonths[0], 10, 64)             // -1
		if err != nil {
			return "", fmt.Errorf("parsing years: %v", err)
		}
		months, err := strconv.ParseInt(yearsMonths[1], 10, 64) // -5
		if err != nil {
			return "", fmt.Errorf("parsing months: %v", err)
		}
		if years < 0 || months < 0 {
			years = int64(math.Abs(float64(years)))
			months = int64(math.Abs(float64(months)))
			columnValue = fmt.Sprintf("-%d-%d", years, months) // -1-5
		} else {
			columnValue = fmt.Sprintf("%d-%d", years, months) // 1-5
		}
		if formatIfRequired {
			columnValue = fmt.Sprintf("INTERVAL'%s'YEAR TO MONTH", columnValue)
		}
		return columnValue, nil
	},
	"INTERVAL DAY TO SECOND": func(columnValue string, formatIfRequired bool, dbzmSchema *schemareg.ColumnSchema) (string, error) {
		//columnValue format: P0Y0M24DT23H34M5.878667S //TODO check regex will be better or not
		splits := strings.Split(strings.TrimPrefix(columnValue, "P"), "M") // ["0Y0M", "24DT23H34, 5.878667S"]
		daysTime := strings.Split(splits[1], "DT")                         // ["24", "23H34M5.878667S"]
		days, err := strconv.ParseInt(daysTime[0], 10, 64)                 // 24
		if err != nil {
			return "", fmt.Errorf("parsing days: %v", err)
		}
		time := strings.Split(daysTime[1], "H")         // ["23", "34"]
		hours, err := strconv.ParseInt(time[0], 10, 64) // 23
		if err != nil {
			return "", fmt.Errorf("parsing hours: %v", err)
		}
		mins, err := strconv.ParseInt(time[1], 10, 64) // 34
		if err != nil {
			return "", fmt.Errorf("parsing minutes: %v", err)
		}
		seconds, err := strconv.ParseFloat(strings.TrimSuffix(splits[2], "S"), 64) // 5.878667
		if err != nil {
			return "", fmt.Errorf("parsing seconds: %v", err)
		}
		if days < 0 || hours < 0 || mins < 0 || seconds < 0 {
			days = int64(math.Abs(float64(days)))
			hours = int64(math.Abs(float64(hours)))
			mins = int64(math.Abs(float64(mins)))
			seconds = math.Abs(seconds)
			columnValue = fmt.Sprintf("-%d %d:%d:%.9f", days, hours, mins, seconds) // -24 23:34:5.878667
		} else {
			columnValue = fmt.Sprintf("%d %d:%d:%.9f", days, hours, mins, seconds) // 24 23:34:5.878667
		}
		if formatIfRequired {
			columnValue = fmt.Sprintf("INTERVAL'%s'DAY TO SECOND", columnValue)
		}
		return columnValue, nil
	},
}
