//go:build integration

package gather_assessment_metadata

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"testing"

	_ "github.com/jackc/pgx/v5/stdlib"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

func TestPgStatStatementsAcrossVersions(t *testing.T) {
	pgVersions := []string{"11", "12", "13", "14", "15", "16", "17"}
	requiredColumns := []string{"query", "mean_exec_time", "total_exec_time", "min_exec_time", "max_exec_time", "calls", "rows"}

	var summaries []string

	for _, version := range pgVersions {
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

			// Enable pg_stat_statements extension
			_, err = db.Exec("CREATE EXTENSION IF NOT EXISTS pg_stat_statements")
			if err != nil {
				t.Fatalf("Failed to create pg_stat_statements: %v", err)
			}

			// Get column names
			columnNames, err := getPgStatStatementsColumns(db)
			if err != nil {
				t.Fatalf("Failed to get columns: %v", err)
			}

			// Print columns in single line
			t.Logf("PostgreSQL %s columns: %s", version, strings.Join(columnNames, ", "))

			// Check missing columns
			missing := checkRequiredColumns(columnNames, requiredColumns)

			// Create summary for this version
			var summary string
			if len(missing) > 0 {
				summary = fmt.Sprintf("PG %s: %d cols, missing: %s", version, len(columnNames), strings.Join(missing, ", "))
				t.Logf("❌ PostgreSQL %s missing: %s", version, strings.Join(missing, ", "))
			} else {
				summary = fmt.Sprintf("PG %s: %d cols, ✅ all required", version, len(columnNames))
				t.Logf("✅ PostgreSQL %s has all required columns", version)
			}
			summaries = append(summaries, summary)

			// stop docker container
			if err := container.Stop(context.Background()); err != nil {
				t.Fatalf("Failed to stop PostgreSQL %s: %v", version, err)
			}
		})
	}

	// Print final summary
	t.Logf("\n=== SUMMARY ===")
	for _, summary := range summaries {
		t.Logf(summary)
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
		rows.Scan(&col)
		columns = append(columns, col)
	}
	return columns, nil
}

func checkRequiredColumns(actual, required []string) []string {
	colMap := make(map[string]bool)
	for _, col := range actual {
		colMap[strings.ToLower(col)] = true
	}

	var missing []string
	for _, req := range required {
		if !colMap[strings.ToLower(req)] {
			missing = append(missing, req)
		}
	}
	return missing
}
