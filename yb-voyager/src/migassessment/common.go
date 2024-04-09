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
package migassessment

import (
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/pelletier/go-toml/v2"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var AssessmentDataDir string

type Record map[string]any

// type QueryResult []Record
var FinalReport Report

type Report struct {
	*ShardingReport `json:"sharding"`
	*SizingReport   `json:"sizing"`
}

var assessmentParams = &AssessmentParams{}

type AssessmentParams struct {
	TargetYBVersion string `toml:"target_yb_version"`
	ShardingParams  `toml:"sharding_params"`
	SizingParams    `toml:"sizing_params"`
}

func loadCSVDataFile[T any](filePath string) ([]*T, error) {
	result := make([]*T, 0)
	records, err := loadCSVDataFileGeneric(filePath)
	if err != nil {
		log.Errorf("error loading csv data file %s: %v", filePath, err)
		return nil, fmt.Errorf("error loading csv data file %s: %w", filePath, err)
	}

	for _, record := range records {
		var tmplRec T
		bs, err := json.Marshal(record)
		if err != nil {
			log.Errorf("error marshalling record: %v", err)
			return nil, fmt.Errorf("error marshalling record: %w", err)
		}
		err = json.Unmarshal(bs, &tmplRec)
		if err != nil {
			log.Errorf("error unmarshalling record: %v", err)
			return nil, fmt.Errorf("error unmarshalling record: %w", err)
		}

		result = append(result, &tmplRec)
	}

	return result, nil
}

func loadCSVDataFileGeneric(filePath string) ([]Record, error) {
	file, err := os.Open(filePath)
	if err != nil {
		log.Errorf("error opening file %s: %v", filePath, err)
		return nil, fmt.Errorf("error opening file %s: %w", filePath, err)
	}

	defer func() {
		err := file.Close()
		if err != nil {
			log.Errorf("error closing file %s: %v", filePath, err)
		}
	}()

	csvReader := csv.NewReader(file)
	csvReader.ReuseRecord = true

	rows, err := csvReader.ReadAll()
	if err != nil {
		log.Errorf("error reading csv file %s: %v", filePath, err)
		return nil, fmt.Errorf("error reading csv file %s: %w", filePath, err)
	}

	if len(rows) == 0 {
		log.Warnf("file '%s' is empty, no records", filePath)
		return nil, nil
	}

	columnNames := rows[0]
	result := make([]Record, len(rows)-1)
	for rowNum := 1; rowNum < len(rows); rowNum++ {
		record := make(Record)
		row := rows[rowNum]
		for columnIdx, columnName := range columnNames {
			record[columnName] = row[columnIdx]
		}
		result[rowNum-1] = record
	}
	return result, nil
}

func LoadAssessmentParams(userInputFpath string) error {
	if userInputFpath == "" {
		log.Infof("user input file path is empty, skipping loading assessment parameters")
		return nil
	}
	log.Infof("loading assessment parameters from file %s", userInputFpath)
	tomlData, err := os.ReadFile(userInputFpath)
	if err != nil {
		log.Errorf("error reading toml file %s: %v", userInputFpath, err)
		return fmt.Errorf("error reading toml file %s: %w", userInputFpath, err)
	}

	err = toml.Unmarshal(tomlData, &assessmentParams)
	if err != nil {
		log.Errorf("error unmarshalling toml file's data: %v", err)
		return fmt.Errorf("error unmarshalling toml file's data: %w", err)
	}

	utils.PrintAndLog("assessment params: %+v", assessmentParams)
	return nil
}

func convertToMap(rows *sql.Rows) []map[string]interface{} {
	columns, _ := rows.Columns()

	// for each database row / record, a map with the column names and row values is added to the allMaps slice
	var allMaps []map[string]interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		pointers := make([]interface{}, len(columns))
		for i, _ := range values {
			pointers[i] = &values[i]
		}
		err := rows.Scan(pointers...)
		checkErr(err)
		resultMap := make(map[string]interface{})
		for i, val := range values {
			//fmt.Printf("Adding key=%s val=%v\n", columns[i], val)
			resultMap[columns[i]] = val
		}
		allMaps = append(allMaps, resultMap)
	}
	return allMaps
}

func convertToMapOfStringString(input []map[string]interface{}) []map[string]string {
	output := make([]map[string]string, len(input))
	for i, m := range input {
		output[i] = make(map[string]string)
		for k, v := range m {
			switch value := v.(type) {
			case string:
				output[i][k] = value
			case int:
				output[i][k] = strconv.Itoa(value)
			// Add more cases if you expect other types
			default:
				output[i][k] = fmt.Sprintf("%v", value)
			}
		}
	}
	return output
}

func printMap(allMaps []map[string]interface{}) {
	for _, v := range allMaps {
		for k1, v1 := range v {
			fmt.Printf("%v : %v\n", k1, v1)
		}
		fmt.Println()
	}
}

func checkErr(err error) {
	if err != nil {
		log.Fatal(err)
	}
}

func checkInternetAccess() (ok bool) {
	_, err := http.Get("http://clients3.google.com/generate_204")
	if err != nil {
		return false
	}
	return true
}

func checkLocalFileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !errors.Is(err, os.ErrNotExist)
}
