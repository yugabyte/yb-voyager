//go:build unit

// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package all_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	// Import all to trigger provider registrations.
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	_ "github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot/databases/all"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
)

// TestAllRegistersPostgres verifies that importing databases/all registers
// the "postgresql" provider.
func TestAllRegistersPostgres(t *testing.T) {
	p, err := schemasnapshot.NewSnapshotProvider(constants.POSTGRESQL)
	require.NoError(t, err, "importing databases/all should register postgresql")
	assert.Equal(t, "postgresql", p.DatabaseType())
	assert.True(t, p.HasStableIdentity())
}
