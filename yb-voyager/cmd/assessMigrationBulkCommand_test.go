//go:build unit

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
    2. Missing each of the mandatory args from [source-db-type, source-db-user, source-db-schema, one of [source-db-name, oracle-db-sid, oracle-tns-alias]]
    3. Mis-spell some of the header fields
2. Records testing
    1. Invalid length header [length more than the total] or less than the minimum
    2. Missing each of the mandatory args from [source-db-type, source-db-user, source-db-schema, one of [source-db-name, oracle-db-sid, oracle-tns-alias]]
    3. No line after header
	4. Invalid source db type
3. Multi line config [generate below case, differently for each line]
    1. Invalid length header [length more than the total] or less than the minimum
    2. Missing each of the mandatory args from [source-db-type, source-db-user, source-db-schema, one of [source-db-name, oracle-db-sid, oracle-tns-alias]]
	3. Invalid source db type
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
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name,extra_field1,extra_field2
	oracle,user1,schema1,db1`,
			expectedErrSuffix: "invalid fields found in the fleet config file's header: ['extra_field1', 'extra_field2']",
		},
		{
			name: "Missing Mandatory Field in Header (source-db-type)",
			fileContent: `source-db-user,source-db-schema,source-db-name
	user1,schema1,db1`,
			expectedErrSuffix: "mandatory fields missing in the header: 'source-db-type'",
		},
		{
			name: "Missing Mandatory Field in Header (source-db-user)",
			fileContent: `source-db-type,source-db-schema,source-db-name
	oracle,schema1,db1`,
			expectedErrSuffix: "mandatory fields missing in the header: 'source-db-user'",
		},
		{
			name: "Missing Mandatory Field in Header (source-db-schema)",
			fileContent: `source-db-type,source-db-user,source-db-name
	oracle,user1,db1`,
			expectedErrSuffix: "mandatory fields missing in the header: 'source-db-schema'",
		},
		{
			name: "Missing Required DB Identifier in Header",
			fileContent: `source-db-type,source-db-user,source-db-schema
	oracle,user1,schema1`,
			expectedErrSuffix: "mandatory fields missing in the header: 'one of [source-db-name, oracle-db-sid, oracle-tns-alias]'",
		},
		{
			name: "Misspelled Header Fields",
			fileContent: `source-db-type,source-db-uesr,source-db-shcema,source-db-name
		oracle,user1,schema1,db1`,
			expectedErrSuffix: "invalid fields found in the fleet config file's header: ['source-db-uesr', 'source-db-shcema']",
		},
		// Single line Record Tests
		{
			name: "Invalid Record Length - Too Many Fields",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
oracle,user1,schema1,db1,extra_field`,
			expectedErrSuffix: "line 2 does not match header length: expected 4 fields, got 5",
		},
		{
			name: "Invalid Record Length - Too Few Fields",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
oracle,user1`,
			expectedErrSuffix: "line 2 does not match header length: expected 4 fields, got 2",
		},
		{
			name:              "No Line After Header",
			fileContent:       `source-db-type,source-db-user,source-db-schema,source-db-name`,
			expectedErrSuffix: "fleet config file contains only a header with no data lines",
		},
		// Multi line Records Tests
		{
			name: "Multi-Line Config - Invalid Length (Too Many Fields) on Second Line",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
oracle,user1,schema1,db1
oracle,user2,schema2,db2,extra_field`,
			expectedErrSuffix: "line 3 does not match header length: expected 4 fields, got 5",
		},
		// Passing Tests
		{
			name: "Valid Config with Minimum Header",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
oracle,user1,schema1,db1`,
			expectedErrSuffix: "",
		},
		{
			name: "Valid Config with Single Line Entry and all fields",
			fileContent: `source-db-type,source-db-host,source-db-port,source-db-user,source-db-schema,source-db-name,oracle-db-sid,oracle-tns-alias,source-db-password
oracle,localhost,1521,user1,schema1,db1,,,password_easy`,
			expectedErrSuffix: "",
		},
		{
			name: "Valid Config with Multiple Entries",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
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
			name: "Missing Mandatory Field in Record (source-db-user, source-db-schema)",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
oracle,,,dms`,
			expectedErrSuffix: "failed to create config for line 2 in fleet config file: mandatory fields missing in the record: 'source-db-user', 'source-db-schema'",
		},
		{
			name: "Missing Mandatory Field in Record (source-db-user, source-db-schema)",
			fileContent: `source-db-type,source-db-user,schema,source-db-name
oracle,,,`,
			expectedErrSuffix: "failed to create config for line 2 in fleet config file: mandatory fields missing in the record: 'source-db-user', 'source-db-schema', 'one of [source-db-name, oracle-db-sid, oracle-tns-alias]'",
		},
		{
			name: "Invalid source-db-type in Single Record",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
mysql,user1,schema1,db1`,
			expectedErrSuffix: "failed to create config for line 2 in fleet config file: unsupported/invalid source-db-type: 'mysql'. Only 'oracle' is supported",
		},
		// Multiple line Record Tests
		{
			name: "Multi-Line Config - Missing Mandatory Field on Third Line",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
oracle,user1,schema1,db1
oracle,user2,,db2
oracle,user3,,db3`,
			expectedErrSuffix: "failed to create config for line 3 in fleet config file: mandatory fields missing in the record: 'source-db-schema'",
		},
		{
			name: "Multi-Line Config - Missing DB Identifier on Fourth Line",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
oracle,user1,schema1,db1
oracle,user2,schema2,db2
oracle,user3,schema3,`,
			expectedErrSuffix: "failed to create config for line 4 in fleet config file: mandatory fields missing in the record: 'one of [source-db-name, oracle-db-sid, oracle-tns-alias]'",
		},
		{
			name: "Multi-Line Config - Invalid source-db-type on Third Line",
			fileContent: `source-db-type,source-db-user,source-db-schema,source-db-name
oracle,user1,schema1,db1
postgresql,user2,schema2,db2
oracle,user3,schema3,db3`,
			expectedErrSuffix: "failed to create config for line 3 in fleet config file: unsupported/invalid source-db-type: 'postgresql'. Only 'oracle' is supported",
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
				assert.Error(t, err)
				assert.True(t, strings.HasSuffix(err.Error(), tt.expectedErrSuffix), "expected error suffix: %v, got: %v", tt.expectedErrSuffix, err.Error())
			}
		})
	}
}
