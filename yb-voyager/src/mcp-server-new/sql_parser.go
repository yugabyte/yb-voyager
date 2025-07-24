package mcpservernew

import (
	"fmt"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

// SQLParser handles SQL statement parsing and validation
type SQLParser struct{}

// NewSQLParser creates a new SQL parser
func NewSQLParser() *SQLParser {
	return &SQLParser{}
}

// ParseSQL validates a SQL statement using pg_query_go
func (sp *SQLParser) ParseSQL(sqlStatement string) (map[string]interface{}, error) {
	if sqlStatement == "" {
		return nil, fmt.Errorf("SQL statement is required")
	}

	// Try to parse the SQL statement
	parseResult, err := pg_query.Parse(sqlStatement)
	if err != nil {
		// Return error details for invalid SQL
		return map[string]interface{}{
			"valid":   false,
			"error":   err.Error(),
			"sql":     sqlStatement,
			"message": "SQL statement is not valid PostgreSQL syntax",
		}, nil
	}

	// If parsing succeeds, return success details
	return map[string]interface{}{
		"valid":      true,
		"sql":        sqlStatement,
		"message":    "SQL statement is valid PostgreSQL syntax",
		"stmt_len":   len(sqlStatement),
		"parse_tree": parseResult.String(),
	}, nil
}

// GetSQLInfo returns detailed information about a SQL statement
func (sp *SQLParser) GetSQLInfo(sqlStatement string) (map[string]interface{}, error) {
	if sqlStatement == "" {
		return nil, fmt.Errorf("SQL statement is required")
	}

	// Try to parse the SQL statement
	parseResult, err := pg_query.Parse(sqlStatement)
	if err != nil {
		return map[string]interface{}{
			"valid":    false,
			"error":    err.Error(),
			"sql":      sqlStatement,
			"message":  "SQL statement is not valid PostgreSQL syntax",
			"stmt_len": len(sqlStatement),
		}, nil
	}

	// If parsing succeeds, return detailed information
	return map[string]interface{}{
		"valid":      true,
		"sql":        sqlStatement,
		"message":    "SQL statement is valid PostgreSQL syntax",
		"stmt_len":   len(sqlStatement),
		"parse_tree": parseResult.String(),
		"has_stmts":  len(parseResult.GetStmts()) > 0,
		"stmt_count": len(parseResult.GetStmts()),
	}, nil
}
