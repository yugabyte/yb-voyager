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
package dbzm

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDebeziumStopTimeoutSeconds(t *testing.T) {
	t.Run("env unset defaults to 100", func(t *testing.T) {
		os.Unsetenv("DEBEZIUM_STOP_TIMEOUT_SECONDS")
		require.Equal(t, 100, debeziumStopTimeoutSeconds())
	})

	t.Run("valid env value is honored", func(t *testing.T) {
		t.Setenv("DEBEZIUM_STOP_TIMEOUT_SECONDS", "20")
		require.Equal(t, 20, debeziumStopTimeoutSeconds())
	})

	t.Run("non-positive value falls back to default", func(t *testing.T) {
		t.Setenv("DEBEZIUM_STOP_TIMEOUT_SECONDS", "0")
		require.Equal(t, 100, debeziumStopTimeoutSeconds())
	})

	t.Run("unparseable value falls back to default", func(t *testing.T) {
		t.Setenv("DEBEZIUM_STOP_TIMEOUT_SECONDS", "abc")
		require.Equal(t, 100, debeziumStopTimeoutSeconds())
	})
}
