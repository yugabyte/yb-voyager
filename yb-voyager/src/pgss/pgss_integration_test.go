//go:build integration

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
package pgss

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

// TODO: Keep this in sync with yb-voyager's supported PostgreSQL versions
var supportedPgVersions = []string{"11", "12", "13", "14", "15", "16", "17"}

func TestPgStatStatementsRequiredColumns(t *testing.T) {
	for _, version := range supportedPgVersions {
		t.Run(fmt.Sprintf("PostgreSQL_%s", version), func(t *testing.T) {
			container := testcontainers.NewTestContainer(testcontainers.POSTGRESQL, &testcontainers.ContainerConfig{
				DBVersion: version,
				User:      "postgres",
				Password:  "password",
				DBName:    "postgres",
			})

			if err := container.Start(context.Background()); err != nil {
				t.Fatalf("Failed to start PostgreSQL %s: %v", version, err)
			}

			db, err := container.GetConnection()
			if err != nil {
				t.Fatalf("Failed to connect to PostgreSQL %s: %v", version, err)
			}
			defer db.Close()

			_, err = db.Exec("CREATE EXTENSION IF NOT EXISTS pg_stat_statements")
			if err != nil {
				t.Fatalf("Failed to create pg_stat_statements: %v", err)
			}

			availableColumns, err := getPgStatStatementsColumns(db)
			if err != nil {
				t.Fatalf("Failed to get columns: %v", err)
			}

			hasOldFormat := hasColumns(availableColumns, []string{"total_time", "mean_time", "min_time", "max_time", "stddev_time"})
			hasNewFormat := hasColumns(availableColumns, []string{"total_exec_time", "mean_exec_time", "min_exec_time", "max_exec_time", "stddev_exec_time"})
			hasCore := hasColumns(availableColumns, []string{"queryid", "query", "calls", "rows"})

			if !hasCore {
				t.Errorf("PostgreSQL %s missing core columns; available columns: %v", version, availableColumns)
				return
			}

			if !hasOldFormat && !hasNewFormat {
				t.Errorf("PostgreSQL %s missing timing columns; available columns: %v", version, availableColumns)
				return
			}

			t.Logf("PostgreSQL %s: available columns: %v", version, availableColumns)

			container.Stop(context.Background())
		})
	}
}

func getPgStatStatementsColumns(db *sql.DB) ([]string, error) {
	rows, err := db.Query(`
		SELECT column_name 
		FROM information_schema.columns 
		WHERE table_name = 'pg_stat_statements' 
		ORDER BY ordinal_position`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		columns = append(columns, col)
	}
	return columns, nil
}

func hasColumns(available []string, required []string) bool {
	colMap := make(map[string]bool)
	for _, col := range available {
		colMap[strings.ToLower(col)] = true
	}

	for _, req := range required {
		if !colMap[strings.ToLower(req)] {
			return false
		}
	}
	return true
}
