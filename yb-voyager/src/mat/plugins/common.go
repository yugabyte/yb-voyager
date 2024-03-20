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
package mat

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

type Record map[string]any
type QueryResult []Record

type Report any

func LoadQueryResults(pluginName string, exportDir string) (map[string]QueryResult, error) {
	queryResults := make(map[string]QueryResult)
	assessmentDataDir := filepath.Join(exportDir, "assessment", "data")
	files, err := filepath.Glob(filepath.Join(assessmentDataDir, fmt.Sprintf("%s__*.csv", pluginName)))
	if err != nil {
		log.Errorf("error listing files in %s for plugin %s: %v", assessmentDataDir, pluginName, err)
		return nil, fmt.Errorf("error listing files in %s for plugin %s: %w", assessmentDataDir, pluginName, err)
	}

	for _, file := range files {
		baseFileName := filepath.Base(file)
		queryName := strings.TrimSuffix(strings.TrimPrefix(baseFileName, fmt.Sprintf("%s__", pluginName)), ".csv")
		// TODO: use csv reader
		bytes, err := os.ReadFile(file)
		if err != nil {
			log.Errorf("error opening file %s: %v", file, err)
			return nil, fmt.Errorf("error opening file %s: %w", file, err)
		}

		if len(bytes) == 0 {
			log.Warnf("file %s is empty", file)
			continue
		}

		rows := strings.Split(string(bytes), "\n")
		columnNames := strings.Split(rows[0], ",")
		queryResult := make(QueryResult, len(rows)-1)
		for i := 1; i < len(rows); i++ {
			if len(rows[i]) == 0 {
				continue
			}
			row := strings.Split(rows[i], ",")
			record := make(Record)
			for j, columnName := range columnNames {
				record[columnName] = row[j]
			}
			queryResult = append(queryResult, record)
		}
		queryResults[queryName] = queryResult
	}

	return queryResults, nil
}

func LoadUserInput(pluginName string, userInputFpath string) (map[string]any, error) {
	tomlData, err := os.ReadFile(userInputFpath)
	if err != nil {
		log.Errorf("error reading plugin params file %s: %v", userInputFpath, err)
		return nil, fmt.Errorf("error reading plugin params file %s: %w", userInputFpath, err)
	}

	tree, err := toml.LoadBytes(tomlData)
	if err != nil {
		log.Errorf("error parsing plugin params file %s: %v", userInputFpath, err)
		return nil, fmt.Errorf("error parsing plugin params file %s: %w", userInputFpath, err)
	}

	tableNames := []string{"common", pluginName}
	userInputParams := make(map[string]any)
	for _, tableName := range tableNames {
		if !tree.Has(tableName) {
			log.Warnf("table '%s' not found in plugin params file %s", tableName, userInputFpath)
			continue
		}
		table := tree.Get(tableName).(*toml.Tree)
		userInputParams = lo.Assign(userInputParams, table.ToMap())
	}

	log.Infof("loaded user input for plugin %s: %v", pluginName, userInputParams)
	return userInputParams, nil
}
