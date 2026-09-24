//go:build integration

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

package export_test

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

// startCaptureTestDB starts a postgres container and returns a live connection, its
// display coordinates, and a cleanup func.
//
// Deliberately seeds nothing, so it serves any integration test in this package.
// schemasnapshot's equivalent fixture builds a canonical schema for testing catalog-read
// fidelity, which is that package's concern; the tests here create whatever schema they
// need, so this is container plumbing only.
func startCaptureTestDB(t *testing.T, cfg *testcontainers.ContainerConfig) (*sql.DB, schemasnapshot.DBMetadata, func()) {
	t.Helper()
	ctx := context.Background()

	pg := testcontainers.NewTestContainer(testcontainers.POSTGRESQL, cfg)
	require.NoError(t, pg.Start(ctx), "start postgres container")

	db, err := pg.GetConnection()
	require.NoError(t, err, "connect to postgres container")

	host, port, err := pg.GetHostPort()
	require.NoError(t, err, "resolve container host/port")
	pgCfg := pg.GetConfig()

	dbMeta := schemasnapshot.DBMetadata{
		Host:     host,
		Port:     port,
		Database: pgCfg.DBName,
		User:     pgCfg.User,
	}

	cleanup := func() {
		db.Close()
		pg.Terminate(ctx)
	}
	return db, dbMeta, cleanup
}

// newIntegrationTestMetaDB creates a MetaDB backed by a fresh SQLite file in a temp dir.
func newIntegrationTestMetaDB(t *testing.T) *metadb.MetaDB {
	t.Helper()
	dir := t.TempDir()
	metainfoDir := filepath.Join(dir, "metainfo")
	require.NoError(t, os.MkdirAll(metainfoDir, 0o755))
	f, err := os.Create(filepath.Join(metainfoDir, "meta.db"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	mdb, err := metadb.NewMetaDB(dir)
	require.NoError(t, err)
	return mdb
}
