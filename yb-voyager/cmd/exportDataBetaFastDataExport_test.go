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
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateBetaFastDataExportSupportedForSource(t *testing.T) {
	tests := []struct {
		name        string
		dbType      string
		exportType  string
		useDebezium bool
		expectErr   bool
	}{
		{
			name:        "postgres snapshot export with the flag set is rejected",
			dbType:      POSTGRESQL,
			exportType:  SNAPSHOT_ONLY,
			useDebezium: true,
			expectErr:   true,
		},
		{
			name:        "postgres snapshot export without the flag is allowed",
			dbType:      POSTGRESQL,
			exportType:  SNAPSHOT_ONLY,
			useDebezium: false,
			expectErr:   false,
		},
		{
			// live migration always uses debezium, so the flag changes nothing there
			name:        "postgres live migration is allowed even with the flag set",
			dbType:      POSTGRESQL,
			exportType:  SNAPSHOT_AND_CHANGES,
			useDebezium: true,
			expectErr:   false,
		},
		{
			name:        "postgres changes-only is allowed even with the flag set",
			dbType:      POSTGRESQL,
			exportType:  CHANGES_ONLY,
			useDebezium: true,
			expectErr:   false,
		},
		{
			name:        "oracle snapshot export with the flag set is allowed",
			dbType:      ORACLE,
			exportType:  SNAPSHOT_ONLY,
			useDebezium: true,
			expectErr:   false,
		},
		{
			name:        "mysql snapshot export with the flag set is allowed",
			dbType:      MYSQL,
			exportType:  SNAPSHOT_ONLY,
			useDebezium: true,
			expectErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateBetaFastDataExportSupportedForSource(tt.dbType, tt.exportType, tt.useDebezium)
			if !tt.expectErr {
				require.NoError(t, err)
				return
			}
			require.Error(t, err)
			assert.Contains(t, err.Error(), "BETA_FAST_DATA_EXPORT is not supported")
			assert.Contains(t, err.Error(), POSTGRESQL)
		})
	}
}
