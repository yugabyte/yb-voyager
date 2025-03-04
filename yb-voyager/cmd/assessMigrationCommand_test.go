//go:build integration_voyager_command

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
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestMain(m *testing.M) {
	// set logging level to WARN
	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()
	os.Exit(exitCode)
}

func Test_AssessMigration(t *testing.T) {
	// create temp export dir and setting global exportDir variable
	exportDir = testutils.CreateTempExportDir()
	defer testutils.RemoveTempExportDir(exportDir)

	// setting up source test container and source params for assessment
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(context.Background())
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}
	defer postgresContainer.Terminate(context.Background())

	// create table and initial data in it
	postgresContainer.ExecuteSqls(`
	CREATE TABLE test_data (
		id SERIAL PRIMARY KEY,
		value TEXT
	);`,
		`INSERT INTO test_data (value)
	SELECT md5(random()::text) FROM generate_series(1, 100000);`)
	if err != nil {
		t.Errorf("Failed to create test table: %v", err)
	}

	doDuringCmd := func() {
		log.Infof("Performing sequential reads on the table")
		// sequential reads on the table
		postgresContainer.ExecuteSqls(`
DO $$ 
DECLARE i INT;
BEGIN
	FOR i IN 1..100 LOOP
		PERFORM COUNT(*) FROM test_data;
	END LOOP;
END $$;`)
		if err != nil {
			t.Errorf("Failed to perform sequential reads on the table: %v", err)
		}
	}

	// running the command
	err = testutils.RunVoyagerCommmand(postgresContainer, "assess-migration", []string{
		"--iops-capture-interval", "10",
		"--export-dir", exportDir,
		"--yes",
	}, doDuringCmd)
	if err != nil {
		t.Errorf("Failed to run assess-migration command: %v", err)
	}

	assessmentReportPath := filepath.Join(exportDir, "assessment", "reports", fmt.Sprintf("%s.json", ASSESSMENT_FILE_NAME))
	report, err := ParseJSONToAssessmentReport(assessmentReportPath)
	if err != nil {
		t.Errorf("failed to parse json report file %q: %v", assessmentReportPath, err)
	}

	colocatedTables, err := report.GetColocatedTablesRecommendation()
	if err != nil {
		t.Errorf("failed to get colocated tables recommendation: %v", err)
	}

	fmt.Printf("Colocated tables: %v\n", colocatedTables)

	fmt.Printf("Sizing recommendations: %+v\n", report.Sizing)
	fmt.Printf("TableIndexStats: %s\n", spew.Sdump(report.TableIndexStats))
}
