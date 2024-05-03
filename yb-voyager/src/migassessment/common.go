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
	log "github.com/sirupsen/logrus"
	"net/http"
	"os"
)

var AssessmentMetadataDir string

type Record map[string]any

var SizingReport = &SizingAssessmentReport{}

type SizingRecommendation struct {
	ColocatedTables                 []string
	ColocatedReasoning              string
	ShardedTables                   []string
	NumNodes                        float64
	VCPUsPerInstance                float64
	MemoryPerInstance               float64
	OptimalSelectConnectionsPerNode int64
	OptimalInsertConnectionsPerNode int64
	EstimatedTimeInMinForImport     float64
	ParallelVoyagerJobs             float64
}

type SizingAssessmentReport struct {
	SizingRecommendation SizingRecommendation
	FailureReasoning     string
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

func checkInternetAccess() (ok bool) {
	_, err := http.Get("http://clients3.google.com/generate_204")
	return err == nil
}
