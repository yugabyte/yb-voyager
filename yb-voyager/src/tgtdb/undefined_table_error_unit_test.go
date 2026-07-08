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
package tgtdb

import (
	"fmt"
	"testing"

	pgconn5 "github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func TestIsUndefinedTableError(t *testing.T) {
	require.True(t, IsUndefinedTableError(&pgconn5.PgError{Code: "42P01"}))

	wrapped := fmt.Errorf("query failed: %w", &pgconn5.PgError{Code: "42P01"})
	require.True(t, IsUndefinedTableError(wrapped))

	require.False(t, IsUndefinedTableError(&pgconn5.PgError{Code: "42501"}))
	require.False(t, IsUndefinedTableError(fmt.Errorf("boom")))
	require.False(t, IsUndefinedTableError(nil))
}
