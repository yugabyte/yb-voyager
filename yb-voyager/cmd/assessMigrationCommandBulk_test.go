package cmd

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func createTempFleetConfigFile(t *testing.T, content string) *os.File {
	t.Helper()
	file, err := os.CreateTemp("", "fleet_config_*.csv")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}

	_, err = file.WriteString(content)
	if err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}

	err = file.Close()
	if err != nil {
		t.Fatalf("failed to close temp file: %v", err)
	}

	return file
}

/*
Unit tests
1. Header testing
    1. Invalid length header [length more than the total]
    2. Missing each of the mandatory args from [dbtype, username, schema, one of [dbname,tns_alias,sid]]
    3. Mis-spell some of the header fields
2. Records testing
    1. Invalid length header [length more than the total] or less than the minimum
    2. Missing each of the mandatory args from [dbtype, username, schema, one of [dbname,tns_alias,sid]]
    3. No line after header
3. Multi line config [generate below case, differently for each line]
    1. Invalid length header [length more than the total] or less than the minimum
    2. Missing each of the mandatory args from [dbtype, username, schema, one of [dbname,tns_alias,sid]]
4. Passing tests
    1. Minimum header
    2. Single line entry after header
    3. Multiple lines entry after header
*/

func TestValidateFleetConfigFile(t *testing.T) {
	tests := []struct {
		name              string
		fileContent       string
		expectedErrSuffix string
	}{
		// Header Tests
		{
			name: "Invalid Header Length",
			fileContent: `dbtype,username,schema,dbname,extra_field1,extra_field2
	oracle,user1,schema1,db1`,
			expectedErrSuffix: "invalid fields found in the fleet config file's header: ['extra_field1', 'extra_field2']",
		},
		{
			name: "Missing Mandatory Field in Header (dbtype)",
			fileContent: `username,schema,dbname
	user1,schema1,db1`,
			expectedErrSuffix: "mandatory fields missing in the header: 'dbtype'",
		},
		{
			name: "Missing Mandatory Field in Header (username)",
			fileContent: `dbtype,schema,dbname
	oracle,schema1,db1`,
			expectedErrSuffix: "mandatory fields missing in the header: 'username'",
		},
		{
			name: "Missing Mandatory Field in Header (schema)",
			fileContent: `dbtype,username,dbname
	oracle,user1,db1`,
			expectedErrSuffix: "mandatory fields missing in the header: 'schema'",
		},
		{
			name: "Missing Required DB Identifier in Header",
			fileContent: `dbtype,username,schema
	oracle,user1,schema1`,
			expectedErrSuffix: "mandatory fields missing in the header: 'one of [dbname, sid, tns_alias]'",
		},
		{
			name: "Missing Required DB Identifier in Header",
			fileContent: `dbtype,username,schema
	oracle,user1,schema1`,
			expectedErrSuffix: "mandatory fields missing in the header: 'one of [dbname, sid, tns_alias]'",
		},
		{
			name: "Misspelled Header Fields",
			fileContent: `dbtype,uesrname,shcema,dbname
		oracle,user1,schema1,db1`,
			expectedErrSuffix: "invalid fields found in the fleet config file's header: ['uesrname', 'shcema']",
		},
		// Single line Record Tests
		{
			name: "Invalid Record Length - Too Many Fields",
			fileContent: `dbtype,username,schema,dbname
oracle,user1,schema1,db1,extra_field`,
			expectedErrSuffix: "line 2 does not match header length: expected 4 fields, got 5",
		},
		{
			name: "Invalid Record Length - Too Few Fields",
			fileContent: `dbtype,username,schema,dbname
oracle,user1`,
			expectedErrSuffix: "line 2 does not match header length: expected 4 fields, got 2",
		},
		{
			name:              "No Line After Header",
			fileContent:       `dbtype,username,schema,dbname`,
			expectedErrSuffix: "fleet config file contains only a header with no data lines",
		},
		// Multi line Records Tests
		{
			name: "Multi-Line Config - Invalid Length (Too Many Fields) on Second Line",
			fileContent: `dbtype,username,schema,dbname
oracle,user1,schema1,db1
oracle,user2,schema2,db2,extra_field`,
			expectedErrSuffix: "line 3 does not match header length: expected 4 fields, got 5",
		},
		// Passing Tests
		{
			name: "Valid Config with Minimum Header",
			fileContent: `dbtype,username,schema,dbname
oracle,user1,schema1,db1`,
			expectedErrSuffix: "",
		},
		{
			name: "Valid Config with Single Line Entry and all fields",
			fileContent: `dbtype,hostname,port,username,schema,dbname,sid,tns_alias,password
oracle,localhost,1521,user1,schema1,db1,,,password_easy`,
			expectedErrSuffix: "",
		},
		{
			name: "Valid Config with Multiple Entries",
			fileContent: `dbtype,username,schema,dbname
oracle,user1,schema1,db1
oracle,user2,schema2,db2
oracle,user3,schema3,db3`,
			expectedErrSuffix: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile := createTempFleetConfigFile(t, tt.fileContent)
			defer os.Remove(tempFile.Name())
			err := validateFleetConfigFile(tempFile.Name())
			if tt.expectedErrSuffix == "" {
				assert.NoError(t, err)
			} else {
				fmt.Printf("actual: %s\n", err.Error())
				fmt.Printf("expected: %s\n", tt.expectedErrSuffix)
				assert.Error(t, err)
				assert.True(t, strings.HasSuffix(err.Error(), tt.expectedErrSuffix), "expected error suffix: %v, got: %v", tt.expectedErrSuffix, err.Error())
			}
		})
	}
}

func TestParseFleetConfigFile(t *testing.T) {
	tests := []struct {
		name              string
		fileContent       string
		expectedErrSuffix string
	}{
		// Single line Record Tests
		{
			name: "Missing Mandatory Field in Record (username, schema)",
			fileContent: `dbtype,username,schema,dbname
oracle,,,dms`,
			expectedErrSuffix: "mandatory fields missing in the record: 'username', 'schema'",
		},
		{
			name: "Missing Mandatory Field in Record (username, schema)",
			fileContent: `dbtype,username,schema,dbname
oracle,,,`,
			expectedErrSuffix: "mandatory fields missing in the record: 'username', 'schema', 'one of [dbname, sid, tns_alias]'",
		},
		// Multiple line Record Tests
		{
			name: "Multi-Line Config - Missing Mandatory Field on Third Line",
			fileContent: `dbtype,username,schema,dbname
oracle,user1,schema1,db1
oracle,user2,,db2
oracle,user3,,db3`,
			expectedErrSuffix: "mandatory fields missing in the record: 'schema'",
		},
		{
			name: "Multi-Line Config - Missing DB Identifier on Fourth Line",
			fileContent: `dbtype,username,schema,dbname
oracle,user1,schema1,db1
oracle,user2,schema2,db2
oracle,user3,schema3,`,
			expectedErrSuffix: "mandatory fields missing in the record: 'one of [dbname, sid, tns_alias]'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tempFile := createTempFleetConfigFile(t, tt.fileContent)
			defer os.Remove(tempFile.Name())

			_, err := parseFleetConfigFile(tempFile.Name())
			if tt.expectedErrSuffix == "" {
				assert.NoError(t, err)
			} else {
				fmt.Printf("actual: %s\n", err.Error())
				fmt.Printf("expected: %s\n", tt.expectedErrSuffix)
				assert.Error(t, err)
				assert.True(t, strings.HasSuffix(err.Error(), tt.expectedErrSuffix), "expected error suffix: %v, got: %v", tt.expectedErrSuffix, err.Error())
			}
		})
	}
}
