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
	"encoding/csv"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var AssessmentMetadataDir string

type Record map[string]any

var SizingReport *SizingAssessmentReport

type SizingAssessmentReport struct {
	ColocatedTables                 []string
	ColocatedReasoning              string
	ShardedTables                   []string
	NumNodes                        float64
	VCPUsPerInstance                float64
	MemoryPerInstance               float64
	OptimalSelectConnectionsPerNode int64
	OptimalInsertConnectionsPerNode int64
	MigrationTimeTakenInMin         float64
	ParallelVoyagerThreadsSharded   int64
	ParallelVoyagerThreadsColocated int64
	FailureReasoning                string
}

func LoadCSVDataFile[T any](filePath string) ([]*T, error) {
	result := make([]*T, 0)
	records, err := loadCSVDataFileGeneric(filePath)
	if err != nil {
		log.Errorf("error loading csv data file %s: %v", filePath, err)
		return nil, fmt.Errorf("error loading csv data file %s: %w", filePath, err)
	}

	for _, record := range records {
		var tmplRec T
		camelCaseRecord := make(Record)
		for key, value := range record {
			camelCaseRecord[convertSnakeCaseToCamelCase(key)] = value
		}

		bs, err := json.Marshal(camelCaseRecord)
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
		log.Warnf("error opening file %s: %v", filePath, err)
		return nil, nil
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

// converts snake_case column names to camelCase to match JSON tags
func convertSnakeCaseToCamelCase(str string) string {
	parts := strings.Split(str, "_")
	for i := 1; i < len(parts); i++ {
		parts[i] = cases.Title(language.English, cases.NoLower).String(parts[i])
	}
	return strings.Join(parts, "")
}

func checkInternetAccess() (ok bool) {
	_, err := http.Get("http://clients3.google.com/generate_204")
	return err == nil
}
