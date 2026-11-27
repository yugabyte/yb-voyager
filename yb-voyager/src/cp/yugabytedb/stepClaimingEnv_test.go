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
package yugabytedb

import (
	"errors"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestReadStepClaimingEnv(t *testing.T) {
	operationID := uuid.New()
	for _, tc := range []struct {
		name        string
		operationID string
		joinID      string
		executorRef string
		expected    stepClaimingEnv
		expectedErr string
	}{
		{name: "neither set is off"},
		{name: "operation id claims", operationID: operationID.String(),
			expected: stepClaimingEnv{mode: stepClaimingClaim, operationID: operationID}},
		{name: "operation id with executor ref", operationID: operationID.String(), executorRef: `{"a":1}`,
			expected: stepClaimingEnv{mode: stepClaimingClaim, operationID: operationID, executorRef: ptr(`{"a":1}`)}},
		{name: "step execution id joins", joinID: "42",
			expected: stepClaimingEnv{mode: stepClaimingJoin, joinID: 42}},
		{name: "both set", operationID: operationID.String(), joinID: "42", expectedErr: "must not both be set"},
		{name: "zero join id", joinID: "0", expectedErr: STEP_EXECUTION_ID_ENV_VAR},
		{name: "negative join id", joinID: "-3", expectedErr: STEP_EXECUTION_ID_ENV_VAR},
		{name: "garbage join id", joinID: "abc", expectedErr: STEP_EXECUTION_ID_ENV_VAR},
		{name: "bad operation id", operationID: "nope", expectedErr: OPERATION_ID_ENV_VAR},
		{name: "bad executor ref", operationID: operationID.String(), executorRef: "[1]", expectedErr: EXECUTOR_REF_ENV_VAR},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv(OPERATION_ID_ENV_VAR, tc.operationID)
			t.Setenv(STEP_EXECUTION_ID_ENV_VAR, tc.joinID)
			t.Setenv(EXECUTOR_REF_ENV_VAR, tc.executorRef)
			actual, err := readStepClaimingEnv()
			if tc.expectedErr != "" {
				assert.ErrorContains(t, err, tc.expectedErr)
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, tc.expected, actual)
		})
	}
}

func ptr[T any](v T) *T { return &v }

func TestChildProcessEnv(t *testing.T) {
	id := int64(7)
	for _, tc := range []struct {
		name     string
		environ  []string
		id       *int64
		expected []string
	}{
		{name: "strips both and adds the id",
			environ:  []string{"A=1", "VOYAGER_OPERATION_ID=x", "VOYAGER_STEP_EXECUTION_ID=3", "B=2"},
			id:       &id,
			expected: []string{"A=1", "B=2", "VOYAGER_STEP_EXECUTION_ID=7"}},
		{name: "strips both without an id",
			environ:  []string{"VOYAGER_OPERATION_ID=x", "A=1", "VOYAGER_STEP_EXECUTION_ID=3"},
			expected: []string{"A=1"}},
		{name: "leaves similarly named vars",
			environ:  []string{"VOYAGER_OPERATION_ID_X=1", "XVOYAGER_STEP_EXECUTION_ID=2"},
			expected: []string{"VOYAGER_OPERATION_ID_X=1", "XVOYAGER_STEP_EXECUTION_ID=2"}},
		{name: "empty environ", expected: []string{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, childProcessEnv(tc.environ, tc.id))
		})
	}
}

func TestClaimStepWithoutMigrationID(t *testing.T) {
	t.Run("claim mode fails", func(t *testing.T) {
		controlPlane := New("")
		controlPlane.claimingEnv = stepClaimingEnv{mode: stepClaimingClaim, operationID: uuid.New()}
		err := controlPlane.ClaimStep(func() (uuid.UUID, error) { return uuid.Nil, nil }, "export-schema", nil)
		assert.ErrorContains(t, err, "migration id unknown: YB_VOYAGER_MIGRATION_UUID is not set")
	})
	t.Run("claim mode reports a migration id that cannot be resolved", func(t *testing.T) {
		controlPlane := New("")
		controlPlane.claimingEnv = stepClaimingEnv{mode: stepClaimingClaim, operationID: uuid.New()}
		err := controlPlane.ClaimStep(func() (uuid.UUID, error) { return uuid.Nil, errors.New("bad env") }, "export-schema", nil)
		assert.ErrorContains(t, err, "resolve migration id: bad env")
	})
	t.Run("claiming off does nothing and never resolves the migration id", func(t *testing.T) {
		resolve := func() (uuid.UUID, error) {
			t.Fatal("migration id resolved with step claiming off")
			return uuid.Nil, nil
		}
		assert.NoError(t, New("").ClaimStep(resolve, "export-schema", nil))
	})
}

func TestChildProcessEnvMethod(t *testing.T) {
	environ := []string{"A=1", "VOYAGER_OPERATION_ID=x"}
	assert.Equal(t, []string{"A=1"}, New("").ChildProcessEnv(environ, true))
	assert.Equal(t, []string{"A=1"}, New("").ChildProcessEnv(environ, false))
}
