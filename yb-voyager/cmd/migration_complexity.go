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
package cmd

import (
	"encoding/csv"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const NOT_AVAILABLE = "NOT AVAILABLE"

var (
	UNSUPPORTED_DATATYPE_XML_ISSUE  = fmt.Sprintf("%s - xml", UNSUPPORTED_DATATYPE)
	UNSUPPORTED_DATATYPE_XID_ISSUE  = fmt.Sprintf("%s - xid", UNSUPPORTED_DATATYPE)
	APP_CHANGES_HIGH_THRESHOLD      = 5
	APP_CHANGES_MEDIUM_THRESHOLD    = 1
	SCHEMA_CHANGES_HIGH_THRESHOLD   = math.MaxInt32
	SCHEMA_CHANGES_MEDIUM_THRESHOLD = 20
)

var appChanges = []string{
	INHERITANCE_ISSUE_REASON,
	CONVERSION_ISSUE_REASON,
	DEFERRABLE_CONSTRAINT_ISSUE,
	UNSUPPORTED_DATATYPE_XML_ISSUE,
	UNSUPPORTED_DATATYPE_XID_ISSUE,
	UNSUPPORTED_EXTENSION_ISSUE, // will confirm this
}

func readEnvForAppOrSchemaCounts() {
	APP_CHANGES_HIGH_THRESHOLD = utils.GetEnvAsInt("APP_CHANGES_HIGH_THRESHOLD", APP_CHANGES_HIGH_THRESHOLD)
	APP_CHANGES_MEDIUM_THRESHOLD = utils.GetEnvAsInt("APP_CHANGES_MEDIUM_THRESHOLD", APP_CHANGES_MEDIUM_THRESHOLD)
	SCHEMA_CHANGES_HIGH_THRESHOLD = utils.GetEnvAsInt("SCHEMA_CHANGES_HIGH_THRESHOLD", SCHEMA_CHANGES_HIGH_THRESHOLD)
	SCHEMA_CHANGES_MEDIUM_THRESHOLD = utils.GetEnvAsInt("SCHEMA_CHANGES_MEDIUM_THRESHOLD", SCHEMA_CHANGES_MEDIUM_THRESHOLD)
}

// Migration complexity calculation from the conversion issues
func calculateMigrationComplexity(sourceDBType string, schemaDirectory string, schemaAnalysisReport utils.SchemaReport) string {
	if sourceDBType != ORACLE && sourceDBType != POSTGRESQL {
		return NOT_AVAILABLE
	} else if schemaAnalysisReport.MigrationComplexity != "" {
		return schemaAnalysisReport.MigrationComplexity
	}

	if sourceDBType == ORACLE {
		mc, err := calculateMigrationComplexityForOracle(schemaDirectory)
		if err != nil {
			log.Errorf("failed to get migration complexity for oracle: %v", err)
			return NOT_AVAILABLE
		}
		return mc
	} else if sourceDBType == POSTGRESQL {
		log.Infof("Calculating migration complexity..")
		return calculateMigrationComplexityForPG(schemaAnalysisReport)
	}
	return NOT_AVAILABLE
}

func calculateMigrationComplexityForPG(schemaAnalysisReport utils.SchemaReport) string {
	readEnvForAppOrSchemaCounts()
	appChangesCount := 0
	for _, issue := range schemaAnalysisReport.Issues {
		for _, appChange := range appChanges {
			if strings.Contains(issue.Reason, appChange) {
				appChangesCount++
			}
		}
	}
	schemaChangesCount := len(schemaAnalysisReport.Issues) - appChangesCount

	if appChangesCount > APP_CHANGES_HIGH_THRESHOLD || schemaChangesCount > SCHEMA_CHANGES_HIGH_THRESHOLD {
		return HIGH
	} else if appChangesCount > APP_CHANGES_MEDIUM_THRESHOLD || schemaChangesCount > SCHEMA_CHANGES_MEDIUM_THRESHOLD {
		return MEDIUM
	}
	//LOW in case appChanges == 0 or schemaChanges [0-20]
	return LOW
}

// This is a temporary logic to get migration complexity for oracle based on the migration level from ora2pg report.
// Ideally, we should ALSO be considering the schema analysis report to get the migration complexity.
func calculateMigrationComplexityForOracle(schemaDirectory string) (string, error) {
	ora2pgReportPath := filepath.Join(schemaDirectory, "ora2pg_report.csv")
	if !utils.FileOrFolderExists(ora2pgReportPath) {
		return "", fmt.Errorf("ora2pg report file not found at %s", ora2pgReportPath)
	}
	file, err := os.Open(ora2pgReportPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file %s: %w", ora2pgReportPath, err)
	}
	defer func() {
		if err := file.Close(); err != nil {
			log.Errorf("Error while closing file %s: %v", ora2pgReportPath, err)
		}
	}()
	// Sample file contents

	// "dbi:Oracle:(DESCRIPTION = (ADDRESS = (PROTOCOL = TCP)(HOST = xyz)(PORT = 1521))(CONNECT_DATA = (SERVICE_NAME = DMS)))";
	// "Oracle Database 19c Enterprise Edition Release 19.0.0.0.0";"ASSESS_MIGRATION";"261.62 MB";"1 person-day(s)";"A-2";
	// "0/0/0.00";"0/0/0";"0/0/0";"25/0/6.50";"0/0/0.00";"0/0/0";"0/0/0";"0/0/0";"0/0/0";"3/0/1.00";"3/0/1.00";
	// "44/0/4.90";"27/0/2.70";"9/0/1.80";"4/0/16.00";"5/0/3.00";"2/0/2.00";"125/0/58.90"
	//
	// X/Y/Z - total/invalid/cost for each type of objects(table,function,etc). Last data element is the sum total.
	// total cost = 58.90 units (1 unit = 5 minutes). Therefore total cost is approx 1 person-days.
	// column 6 is Migration level.
	//  Migration levels:
	//     A - Migration that might be run automatically
	//     B - Migration with code rewrite and a human-days cost up to 5 days
	//     C - Migration with code rewrite and a human-days cost above 5 days
	// 	Technical levels:
	//     1 = trivial: no stored functions and no triggers
	//     2 = easy: no stored functions but with triggers, no manual rewriting
	//     3 = simple: stored functions and/or triggers, no manual rewriting
	//     4 = manual: no stored functions but with triggers or views with code rewriting
	//     5 = difficult: stored functions and/or triggers with code rewriting
	reader := csv.NewReader(file)
	reader.Comma = ';'
	rows, err := reader.ReadAll()
	if err != nil {
		log.Errorf("error reading csv file %s: %v", ora2pgReportPath, err)
		return "", fmt.Errorf("error reading csv file %s: %w", ora2pgReportPath, err)
	}
	if len(rows) > 1 {
		return "", fmt.Errorf("invalid ora2pg report file format. Expected 1 row, found %d. contents = %v", len(rows), rows)
	}
	reportData := rows[0]
	migrationLevel := strings.Split(reportData[5], "-")[0]

	switch migrationLevel {
	case "A":
		return LOW, nil
	case "B":
		return MEDIUM, nil
	case "C":
		return HIGH, nil
	default:
		return "", fmt.Errorf("invalid migration level [%s] found in ora2pg report %v", migrationLevel, reportData)
	}
}
