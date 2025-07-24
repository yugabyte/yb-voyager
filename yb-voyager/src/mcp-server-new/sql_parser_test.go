package mcpservernew

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestSQLParser tests the SQL parsing functionality
func TestSQLParser(t *testing.T) {
	sp := NewSQLParser()

	// Test with empty SQL statement
	result, err := sp.ParseSQL("")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "SQL statement is required")
	fmt.Printf("Empty SQL error: %v\n", err)

	// Test with valid SQL
	result, err = sp.ParseSQL("SELECT * FROM users WHERE id = 1;")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result["valid"].(bool))
	assert.Equal(t, "SQL statement is valid PostgreSQL syntax", result["message"])
	fmt.Printf("Valid SQL result: %+v\n", result)

	// Test with invalid SQL
	result, err = sp.ParseSQL("SELECT * FROM users WHERE id = ;")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result["valid"].(bool))
	assert.Contains(t, result["error"].(string), "syntax error")
	fmt.Printf("Invalid SQL error: %+v\n", result)

	// Test with complex valid SQL
	result, err = sp.ParseSQL(`
		CREATE TABLE users (
			id SERIAL PRIMARY KEY,
			name VARCHAR(100) NOT NULL,
			email VARCHAR(255) UNIQUE,
			created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
		);
	`)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result["valid"].(bool))
	fmt.Printf("Complex valid SQL result: %+v\n", result)
}

// TestSQLParser_GetSQLInfo tests the GetSQLInfo functionality
func TestSQLParser_GetSQLInfo(t *testing.T) {
	sp := NewSQLParser()

	// Test with valid SQL
	result, err := sp.GetSQLInfo("SELECT * FROM users;")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result["valid"].(bool))
	assert.IsType(t, int(0), result["stmt_len"])
	assert.IsType(t, "", result["parse_tree"])
	assert.IsType(t, bool(false), result["has_stmts"])
	assert.IsType(t, int(0), result["stmt_count"])
	fmt.Printf("SQL info for valid SQL: %+v\n", result)

	// Test with invalid SQL
	result, err = sp.GetSQLInfo("SELECT * FROM;")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.False(t, result["valid"].(bool))
	assert.Contains(t, result["error"].(string), "syntax error")
	fmt.Printf("SQL info for invalid SQL: %+v\n", result)

	// Test with multiple statements
	result, err = sp.GetSQLInfo("SELECT * FROM users; SELECT * FROM orders;")
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.True(t, result["valid"].(bool))
	assert.Equal(t, 2, result["stmt_count"].(int))
	fmt.Printf("SQL info for multiple statements: %+v\n", result)
}

// TestSQLParser_EdgeCases tests edge cases for SQL parsing
func TestSQLParser_EdgeCases(t *testing.T) {
	sp := NewSQLParser()

	testCases := []struct {
		name        string
		sql         string
		expectValid bool
		description string
	}{
		{
			name:        "Empty string",
			sql:         "",
			expectValid: false,
			description: "Should fail with empty SQL",
		},
		{
			name:        "Whitespace only",
			sql:         "   \n\t  ",
			expectValid: true,
			description: "Should pass with whitespace only (pg_query_go treats this as valid)",
		},
		{
			name:        "Simple SELECT",
			sql:         "SELECT 1;",
			expectValid: true,
			description: "Should pass with simple SELECT",
		},
		{
			name:        "Missing semicolon",
			sql:         "SELECT * FROM users",
			expectValid: true,
			description: "Should pass without semicolon",
		},
		{
			name:        "Invalid table name",
			sql:         "SELECT * FROM 123table;",
			expectValid: false,
			description: "Should fail with invalid table name",
		},
		{
			name:        "Complex DDL",
			sql:         "CREATE INDEX idx_users_email ON users(email) WHERE email IS NOT NULL;",
			expectValid: true,
			description: "Should pass with complex DDL",
		},
		{
			name:        "Invalid syntax",
			sql:         "SELECT * FROM users WHERE id = 'unclosed quote;",
			expectValid: false,
			description: "Should fail with unclosed quote",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := sp.ParseSQL(tc.sql)
			if tc.expectValid {
				assert.NoError(t, err)
				assert.True(t, result["valid"].(bool), tc.description)
			} else {
				if err != nil {
					// This is expected for empty SQL
					assert.Contains(t, err.Error(), "SQL statement is required")
				} else {
					assert.False(t, result["valid"].(bool), tc.description)
				}
			}
			fmt.Printf("%s: %+v\n", tc.name, result)
		})
	}
}
