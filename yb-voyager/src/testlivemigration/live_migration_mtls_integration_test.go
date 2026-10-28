//go:build integration_live_migration

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
package testlivemigration

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

const (
	mtlsSourceUser = "ybvoyager"
	mtlsTargetUser = "yugabyte"

	invalidAuthorizationSQLState = "28000"
)

// mtlsCertFiles are the client-side files voyager is pointed at via --source-ssl-* and
// --target-ssl-*.
type mtlsCertFiles struct {
	RootCert   string
	SourceCert string
	SourceKey  string
	TargetCert string
	TargetKey  string
}

func (f mtlsCertFiles) sourceSSLArgs() map[string]string {
	return map[string]string{
		"--source-ssl-mode":      "verify-full",
		"--source-ssl-root-cert": f.RootCert,
		"--source-ssl-cert":      f.SourceCert,
		"--source-ssl-key":       f.SourceKey,
	}
}

func (f mtlsCertFiles) targetSSLArgs() map[string]string {
	return map[string]string{
		"--target-ssl-mode":      "verify-full",
		"--target-ssl-root-cert": f.RootCert,
		"--target-ssl-cert":      f.TargetCert,
		"--target-ssl-key":       f.TargetKey,
	}
}

type mtlsClientPEMs struct {
	ca, sourceCert, sourceKey, targetCert, targetKey []byte
}

func (p mtlsClientPEMs) writeTo(t *testing.T, dir string) mtlsCertFiles {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o700))
	files := mtlsCertFiles{
		RootCert:   filepath.Join(dir, "ca.crt"),
		SourceCert: filepath.Join(dir, "source-client.crt"),
		SourceKey:  filepath.Join(dir, "source-client.key"),
		TargetCert: filepath.Join(dir, "target-client.crt"),
		TargetKey:  filepath.Join(dir, "target-client.key"),
	}
	// libpq refuses a client key that is group- or world-readable.
	for path, content := range map[string][]byte{
		files.RootCert:   p.ca,
		files.SourceCert: p.sourceCert,
		files.SourceKey:  p.sourceKey,
		files.TargetCert: p.targetCert,
		files.TargetKey:  p.targetKey,
	} {
		require.NoError(t, os.WriteFile(path, content, 0o600), "write %s", path)
	}
	return files
}

type hbaRule struct {
	Type       string
	Database   string
	User       string
	Address    string
	AuthMethod string
}

// TestLiveMigrationWithMTLSOnlySourceAndTarget pins that a snapshot-and-changes live migration
// (export schema via pg_dump, import schema, Debezium snapshot + CDC, import data, cutover to
// target, finalize-schema-post-data-import, end migration), and one with fall-back, work when the
// source and the target accept only TLS connections authenticated by a client certificate, using
// sslmode=verify-full; and that voyager refuses to connect when the TLS setup is wrong.
//
// The cert/key paths reach several consumers that each parse them differently: pgx and libpq
// (pg_dump) read them from a connection URI's query string, pg_dump additionally runs inside
// `bash -c '<uri>'`, and Debezium reads them from the generated application.properties. The
// path-variant subtests put the files under directories whose names are special in at least
// one of those grammars; a leg that does not escape (or over-escapes) a path fails to open the
// file or opens a different one.
func TestLiveMigrationWithMTLSOnlySourceAndTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	pki, err := testutils.NewTestPKI()
	require.NoError(t, err)
	// verify-full checks these against the host the tests connect to (testcontainers' Host()).
	serverHostnames := []string{"localhost"}
	serverIPs := []net.IP{net.ParseIP("127.0.0.1"), net.ParseIP("::1")}
	sourceServerCert, sourceServerKey, err := pki.IssueServerCert("source-postgres", serverHostnames, serverIPs)
	require.NoError(t, err)
	targetServerCert, targetServerKey, err := pki.IssueServerCert("target-yugabytedb", serverHostnames, serverIPs)
	require.NoError(t, err)

	var pems mtlsClientPEMs
	pems.ca = pki.CACertPEM
	pems.sourceCert, pems.sourceKey, err = pki.IssueClientCert(mtlsSourceUser)
	require.NoError(t, err)
	pems.targetCert, pems.targetKey, err = pki.IssueClientCert(mtlsTargetUser)
	require.NoError(t, err)

	certsBaseDir, err := os.MkdirTemp("", "yb-voyager-mtls-")
	require.NoError(t, err)
	t.Cleanup(func() { _ = os.RemoveAll(certsBaseDir) })
	plainCerts := pems.writeTo(t, filepath.Join(certsBaseDir, "plain"))

	sourceMTLS := &testcontainers.MTLSConfig{
		CACertPEM:      pki.CACertPEM,
		ServerCertPEM:  sourceServerCert,
		ServerKeyPEM:   sourceServerKey,
		RootCertPath:   plainCerts.RootCert,
		ClientCertPath: plainCerts.SourceCert,
		ClientKeyPath:  plainCerts.SourceKey,
	}
	targetMTLS := &testcontainers.MTLSConfig{
		CACertPEM:      pki.CACertPEM,
		ServerCertPEM:  targetServerCert,
		ServerKeyPEM:   targetServerKey,
		RootCertPath:   plainCerts.RootCert,
		ClientCertPath: plainCerts.TargetCert,
		ClientKeyPath:  plainCerts.TargetKey,
	}

	// Same ContainerConfig as LiveMigrationTest.SetupContainers builds, so the subtests get these
	// containers from the registry instead of starting new ones.
	source := testcontainers.NewTestContainer(testcontainers.POSTGRESQL, &testcontainers.ContainerConfig{ForLive: true, MTLS: sourceMTLS})
	target := testcontainers.NewTestContainer(testcontainers.YUGABYTEDB, &testcontainers.ContainerConfig{MTLS: targetMTLS})
	// No other test uses these containers, and the package has no TestMain teardown; this frees
	// them once every subtest here is done instead of when the test binary exits.
	t.Cleanup(func() {
		source.Terminate(ctx)
		target.Terminate(ctx)
	})
	require.NoError(t, source.Start(ctx), "start mTLS source")
	require.NoError(t, target.Start(ctx), "start mTLS target")

	requireOnlyClientCertTLSAccepted(t, source, "postgres", plainCerts.RootCert, plainCerts.SourceCert, plainCerts.SourceKey,
		[]hbaRule{
			{Type: "hostssl", Database: "all", User: "all", Address: "all", AuthMethod: "cert"},
			{Type: "hostssl", Database: "replication", User: "all", Address: "all", AuthMethod: "cert"},
		})
	requireOnlyClientCertTLSAccepted(t, target, "yugabyte", plainCerts.RootCert, plainCerts.TargetCert, plainCerts.TargetKey,
		[]hbaRule{
			{Type: "hostssl", Database: "all", User: "all", Address: "all", AuthMethod: "cert"},
		})

	t.Run("plain cert paths", func(t *testing.T) {
		t.Parallel()
		runMTLSLiveMigration(t, "mtls_plain", sourceMTLS, targetMTLS, plainCerts, false)
	})

	t.Run("fall-back with cert path with space and plus", func(t *testing.T) {
		t.Parallel()
		certs := pems.writeTo(t, filepath.Join(certsBaseDir, "fall-back", "certs dir+fb"))
		runMTLSLiveMigration(t, "mtls_fall_back", sourceMTLS, targetMTLS, certs, true)
	})

	t.Run("rejected TLS setups", func(t *testing.T) {
		t.Parallel()
		runMTLSRejectionCases(t, sourceMTLS, targetMTLS, plainCerts, pems, filepath.Join(certsBaseDir, "rejections"))
	})

	pathCases := []struct {
		name   string
		dbName string
		dir    string
	}{
		// URI query values: pgx >= v5.11 and libpq reject an unescaped space.
		{name: "space", dbName: "mtls_space", dir: "certs dir"},
		// URI query values: "+" is literal for pgx >= v5.11 and libpq, but a space to form decoders.
		{name: "plus", dbName: "mtls_plus", dir: "certs+dir"},
		// URI query values: "%41" decodes to "A" if left unescaped; "%zz" is a malformed escape.
		{name: "percent", dbName: "mtls_percent", dir: "certs%41dir%zz"},
		// URI query: unescaped, "&sslmode=disable" would become a separate parameter.
		{name: "ampersand and equals", dbName: "mtls_amp_eq", dir: "certs&sslmode=disable"},
		// URI fragment delimiter; comment marker at the start of a properties line.
		{name: "hash", dbName: "mtls_hash", dir: "certs#dir"},
		// Ends the single-quoted URI that pg_dump is run with under bash -c.
		{name: "single quote", dbName: "mtls_quote", dir: "certs'dir"},
		// Multi-byte UTF-8: percent-encoded in URIs; written raw into application.properties.
		{name: "non-ASCII", dbName: "mtls_non_ascii", dir: "certs-é-证书"},
	}
	for _, tc := range pathCases {
		t.Run("cert path with "+tc.name, func(t *testing.T) {
			t.Parallel()
			// A parent per case: siblings "certs dir" and "certs+dir" would otherwise hold valid
			// certs at each other's mis-decoded path, so a '+'/space mix-up would still pass.
			certs := pems.writeTo(t, filepath.Join(certsBaseDir, "variants", tc.dbName, tc.dir))
			runMTLSLiveMigration(t, tc.dbName, sourceMTLS, targetMTLS, certs, false)
		})
	}
}

// requireOnlyClientCertTLSAccepted proves the server is locked down: a plaintext connection and a
// TLS connection without a client certificate are both refused at authentication, a verify-full
// connection with the client certificate succeeds over TLS as the certificate's CN, and the hba
// rules for network connections start with wantNetworkHBA, whose catch-all rules make any later
// rule unreachable.
func requireOnlyClientCertTLSAccepted(t *testing.T, container testcontainers.TestContainer, dbName string,
	rootCert, clientCert, clientKey string, wantNetworkHBA []hbaRule) {
	t.Helper()
	ctx := context.Background()
	config := container.GetConfig()
	host, port, err := container.GetHostPort()
	require.NoError(t, err)
	uri := url.URL{
		Scheme: "postgresql",
		User:   url.UserPassword(config.User, config.Password),
		Host:   fmt.Sprintf("%s:%d", host, port),
		Path:   dbName,
	}

	connectWith := func(query string) (*pgx.Conn, error) {
		uri.RawQuery = query
		return pgx.Connect(ctx, uri.String())
	}
	requireRejected := func(query string, wantMessage string) {
		conn, err := connectWith(query)
		if err == nil {
			_ = conn.Close(ctx)
		}
		var pgErr *pgconn.PgError
		require.ErrorAs(t, err, &pgErr, "%s: connecting with %q must be refused by the server", config.DBType, query)
		assert.Equal(t, invalidAuthorizationSQLState, pgErr.Code, "%s: %q", config.DBType, query)
		assert.Contains(t, pgErr.Message, wantMessage, "%s: %q", config.DBType, query)
	}

	requireRejected("sslmode=disable", "no pg_hba.conf entry")
	requireRejected("sslmode=verify-full&sslrootcert="+utils.EscapeConnUriQueryValue(rootCert),
		"connection requires a valid client certificate")

	conn, err := connectWith(fmt.Sprintf("sslmode=verify-full&sslrootcert=%s&sslcert=%s&sslkey=%s",
		utils.EscapeConnUriQueryValue(rootCert), utils.EscapeConnUriQueryValue(clientCert), utils.EscapeConnUriQueryValue(clientKey)))
	require.NoError(t, err, "%s: verify-full with the client certificate must be accepted", config.DBType)
	defer conn.Close(ctx)

	var sslInUse bool
	var clientDN string
	err = conn.QueryRow(ctx, "SELECT ssl, client_dn FROM pg_stat_ssl WHERE pid = pg_backend_pid()").Scan(&sslInUse, &clientDN)
	require.NoError(t, err)
	assert.True(t, sslInUse, "%s: connection must be using TLS", config.DBType)
	assert.Equal(t, "/CN="+config.User, clientDN, "%s: connection must be authenticated by the client certificate", config.DBType)

	rows, err := conn.Query(ctx, `SELECT type,
		array_to_string(database, ','),
		array_to_string(user_name, ','),
		coalesce(address, ''),
		auth_method
	FROM pg_hba_file_rules
	ORDER BY line_number`)
	require.NoError(t, err)
	allHBA, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (hbaRule, error) {
		var r hbaRule
		err := row.Scan(&r.Type, &r.Database, &r.User, &r.Address, &r.AuthMethod)
		return r, err
	})
	require.NoError(t, err)
	// `local` rules govern Unix-socket connections, which are not reachable from outside the container.
	networkHBA := lo.Filter(allHBA, func(r hbaRule, _ int) bool { return r.Type != "local" })
	// YugabyteDB appends its default "hostssl all all all trust" after ysql_hba_conf_csv; hba is
	// first-match, so the catch-all cert rule ahead of it shadows it.
	require.GreaterOrEqual(t, len(networkHBA), len(wantNetworkHBA), "%s: network hba rules in effect (all rules: %v)", config.DBType, allHBA)
	require.Equal(t, wantNetworkHBA, networkHBA[:len(wantNetworkHBA)], "%s: network hba rules in effect (all rules: %v)", config.DBType, allHBA)
}

// runMTLSLiveMigration runs one basic live migration with voyager pointed at certs. With
// fallBack, cutover prepares for fall-back, and target changes stream back to the source
// before cutover to source.
func runMTLSLiveMigration(t *testing.T, dbName string, sourceMTLS, targetMTLS *testcontainers.MTLSConfig, certs mtlsCertFiles, fallBack bool) {
	const (
		accountsTable = `"mtls_schema"."accounts"`
		eventsTable   = `"mtls_schema"."events"`
	)
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB: ContainerConfig{
			Type:         "postgresql",
			ForLive:      true,
			DatabaseName: dbName,
			MTLS:         sourceMTLS,
		},
		TargetDB: ContainerConfig{
			Type:         "yugabytedb",
			DatabaseName: dbName,
			MTLS:         targetMTLS,
		},
		SchemaNames: []string{"mtls_schema"},
		// Source only: import schema creates the target schema.
		SourceSetupSchemaSQL: []string{
			`CREATE SCHEMA mtls_schema;`,
			`CREATE TABLE mtls_schema.accounts (id INT PRIMARY KEY, name TEXT NOT NULL, balance NUMERIC(12,2));`,
			`CREATE TABLE mtls_schema.events (id BIGINT PRIMARY KEY, account_id INT, payload TEXT);`,
			`ALTER TABLE mtls_schema.accounts REPLICA IDENTITY FULL;`,
			`ALTER TABLE mtls_schema.events REPLICA IDENTITY FULL;`,
		},
		InitialDataSQL: []string{
			`INSERT INTO mtls_schema.accounts SELECT i, 'account_' || i, i * 10 FROM generate_series(1, 20) i;`,
			`INSERT INTO mtls_schema.events SELECT i, (i % 20) + 1, md5(i::text) FROM generate_series(1, 30) i;`,
		},
		SourceDeltaSQL: []string{
			`INSERT INTO mtls_schema.accounts SELECT i, 'account_' || i, i * 10 FROM generate_series(21, 25) i;`,
			`UPDATE mtls_schema.accounts SET balance = balance + 1 WHERE id <= 3;`,
			`DELETE FROM mtls_schema.events WHERE id > 25;`,
			`INSERT INTO mtls_schema.events SELECT i, 1, md5(i::text) FROM generate_series(31, 32) i;`,
		},
		TargetDeltaSQL: []string{
			`INSERT INTO mtls_schema.accounts SELECT i, 'account_' || i, i * 10 FROM generate_series(26, 28) i;`,
			`UPDATE mtls_schema.accounts SET balance = balance + 1 WHERE id IN (4, 5);`,
			`DELETE FROM mtls_schema.events WHERE id <= 2;`,
		},
		CleanupSQL: []string{
			`DROP SCHEMA IF EXISTS mtls_schema CASCADE;`,
		},
	})
	defer lm.Cleanup()

	require.NoError(t, lm.SetupContainers(context.Background()), "setup containers")
	require.NoError(t, lm.SetupSchema(), "setup source schema")

	require.NoError(t, lm.RunAssessMigration(certs.sourceSSLArgs()), "assess migration")
	require.NoError(t, lm.RunExportSchema(certs.sourceSSLArgs()), "export schema")
	require.NoError(t, lm.RunImportSchema(certs.targetSSLArgs()), "import schema")

	require.NoError(t, lm.StartExportData(true, certs.sourceSSLArgs()), "start export data")
	require.NoError(t, lm.StartImportData(true, certs.targetSSLArgs()), "start import data")

	require.NoError(t, lm.WaitForSnapshotComplete(map[string]int64{
		accountsTable: 20,
		eventsTable:   30,
	}, 180), "snapshot")
	require.NoError(t, lm.ValidateDataConsistency([]string{accountsTable, eventsTable}, "id"), "snapshot data")

	// voyager rewrites the source key to DER under the export dir for pgjdbc; the other files
	// must reach Debezium as the exact paths the user passed.
	assert.Equal(t, map[string]string{
		"debezium.source.database.sslmode":     "verify-full",
		"debezium.source.database.sslcert":     certs.SourceCert,
		"debezium.source.database.sslkey":      filepath.Join(lm.exportDir, "metainfo", "ssl", ".key.der"),
		"debezium.source.database.sslrootcert": certs.RootCert,
	}, readDebeziumSSLProperties(t, filepath.Join(lm.exportDir, "metainfo", "conf", "application.properties")))

	require.NoError(t, lm.ExecuteSourceDelta(), "source delta")
	require.NoError(t, lm.WaitForForwardStreamingComplete(map[string]ChangesCount{
		accountsTable: {Inserts: 5, Updates: 3, Deletes: 0},
		eventsTable:   {Inserts: 2, Updates: 0, Deletes: 5},
	}, 180, 2), "streaming")
	require.NoError(t, lm.ValidateDataConsistency([]string{accountsTable, eventsTable}, "id"), "streamed data")

	require.NoError(t, lm.InitiateCutoverToTarget(fallBack, nil), "initiate cutover to target")
	require.NoError(t, lm.WaitForCutoverComplete(0, 180), "cutover to target")

	if fallBack {
		// Export data from target and import data to source are started by voyager itself and
		// get the SSL settings only from the migration status record, never from the CLI.
		require.NoError(t, lm.ExecuteTargetDelta(), "target delta")
		require.NoError(t, lm.WaitForFallbackStreamingComplete(map[string]ChangesCount{
			accountsTable: {Inserts: 3, Updates: 2, Deletes: 0},
			eventsTable:   {Inserts: 0, Updates: 0, Deletes: 2},
		}, 180, 2), "fall-back streaming")
		require.NoError(t, lm.ValidateDataConsistency([]string{accountsTable, eventsTable}, "id"), "fall-back data")

		require.NoError(t, lm.InitiateCutoverToSource(nil), "initiate cutover to source")
		require.NoError(t, lm.WaitForCutoverSourceComplete(0, 180), "cutover to source")
	} else {
		require.NoError(t, lm.RunFinalizeSchemaPostDataImport(certs.targetSSLArgs()), "finalize schema post data import")
	}
	require.NoError(t, lm.ValidateDataConsistency([]string{accountsTable, eventsTable}, "id"), "data after cutover")

	require.NoError(t, lm.EndMigration(nil, false), "end migration")
}

// runMTLSRejectionCases runs voyager against the mTLS-only source (export schema, export data) and
// target (import schema, import data) with TLS settings that libpq would also refuse, and checks that voyager fails
// with the server's or the TLS stack's reason instead of connecting some other way. For a missing
// file, the error must name the exact path the user passed, which shows the path was decoded back
// unchanged even when it is full of URI syntax.
func runMTLSRejectionCases(t *testing.T, sourceMTLS, targetMTLS *testcontainers.MTLSConfig, goodCerts mtlsCertFiles,
	pems mtlsClientPEMs, certsDir string) {
	const dbName = "mtls_rejections"
	lm := NewLiveMigrationTest(t, &TestConfig{
		SourceDB:    ContainerConfig{Type: "postgresql", ForLive: true, DatabaseName: dbName, MTLS: sourceMTLS},
		TargetDB:    ContainerConfig{Type: "yugabytedb", DatabaseName: dbName, MTLS: targetMTLS},
		SchemaNames: []string{"mtls_schema"},
		SourceSetupSchemaSQL: []string{
			`CREATE SCHEMA mtls_schema;`,
			`CREATE TABLE mtls_schema.accounts (id INT PRIMARY KEY);`,
		},
		CleanupSQL: []string{`DROP SCHEMA IF EXISTS mtls_schema CASCADE;`},
	})
	defer lm.Cleanup()
	require.NoError(t, lm.SetupContainers(context.Background()), "setup containers")
	require.NoError(t, lm.SetupSchema(), "setup source schema")
	// import schema and import data need exported schema and data before they connect to the target.
	// Offline export, so import data does not wait for a live snapshot.
	require.NoError(t, lm.RunExportSchema(goodCerts.sourceSSLArgs()), "export schema")
	exportDataArgs := func(exportDir string) []string {
		return []string{
			"--export-dir", exportDir,
			"--source-db-schema", "mtls_schema",
			"--source-db-name", dbName,
			"--export-type", cmd.SNAPSHOT_ONLY,
			"--disable-pb", "true",
			"--yes",
		}
	}
	require.NoError(t, testutils.NewVoyagerCommandRunner(lm.sourceContainer, "export data",
		withFlags(exportDataArgs(lm.exportDir), goodCerts.sourceSSLArgs()), nil, false).WithT(t).Run(), "export data")

	untrustedPKI, err := testutils.NewTestPKI()
	require.NoError(t, err)
	untrusted := mtlsClientPEMs{ca: untrustedPKI.CACertPEM}
	untrusted.sourceCert, untrusted.sourceKey, err = untrustedPKI.IssueClientCert(mtlsSourceUser)
	require.NoError(t, err)
	untrusted.targetCert, untrusted.targetKey, err = untrustedPKI.IssueClientCert(mtlsTargetUser)
	require.NoError(t, err)
	untrustedCerts := untrusted.writeTo(t, filepath.Join(certsDir, "untrusted"))

	// Signed by the trusted CA, but for the other side's role, so the CN does not match.
	swappedRoleCerts := goodCerts
	swappedRoleCerts.SourceCert, swappedRoleCerts.SourceKey = goodCerts.TargetCert, goodCerts.TargetKey
	swappedRoleCerts.TargetCert, swappedRoleCerts.TargetKey = goodCerts.SourceCert, goodCerts.SourceKey

	// Never created. Unescaped, this path would truncate at '&' or '#', or decode "%41" or '+'.
	missingRootCert := filepath.Join(certsDir, "missing", "no such+dir%41&sslmode=disable#x", "ca.crt")
	missingRootCertCerts := goodCerts
	missingRootCertCerts.RootCert = missingRootCert

	withoutClientCert := func(args map[string]string) map[string]string {
		return lo.OmitByKeys(args, []string{"--source-ssl-cert", "--source-ssl-key", "--target-ssl-cert", "--target-ssl-key"})
	}

	cases := []struct {
		name       string
		sourceArgs map[string]string
		targetArgs map[string]string
		wantOutput string
	}{
		{
			name:       "TLS disabled",
			sourceArgs: map[string]string{"--source-ssl-mode": "disable"},
			targetArgs: map[string]string{"--target-ssl-mode": "disable"},
			wantOutput: "no pg_hba.conf entry",
		},
		{
			name:       "no client certificate",
			sourceArgs: withoutClientCert(goodCerts.sourceSSLArgs()),
			targetArgs: withoutClientCert(goodCerts.targetSSLArgs()),
			wantOutput: "connection requires a valid client certificate",
		},
		{
			name:       "client certificate for another role",
			sourceArgs: swappedRoleCerts.sourceSSLArgs(),
			targetArgs: swappedRoleCerts.targetSSLArgs(),
			wantOutput: "certificate authentication failed for user",
		},
		{
			name:       "server certificate from an untrusted CA",
			sourceArgs: untrustedCerts.sourceSSLArgs(),
			targetArgs: untrustedCerts.targetSSLArgs(),
			wantOutput: "certificate signed by unknown authority",
		},
		{
			name:       "root certificate path that does not exist",
			sourceArgs: missingRootCertCerts.sourceSSLArgs(),
			targetArgs: missingRootCertCerts.targetSSLArgs(),
			wantOutput: missingRootCert,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/export schema", func(t *testing.T) {
			args := []string{
				"--export-dir", testutils.CreateTempExportDir(),
				"--source-db-schema", "mtls_schema",
				"--source-db-name", dbName,
				"--yes",
			}
			requireVoyagerRejected(t, lm.sourceContainer, "export schema", args, tc.sourceArgs, tc.wantOutput)
		})
		t.Run(tc.name+"/export data", func(t *testing.T) {
			requireVoyagerRejected(t, lm.sourceContainer, "export data", exportDataArgs(testutils.CreateTempExportDir()),
				tc.sourceArgs, tc.wantOutput)
		})
		t.Run(tc.name+"/import schema", func(t *testing.T) {
			args := []string{
				"--export-dir", lm.exportDir,
				"--target-db-name", dbName,
				"--yes",
			}
			requireVoyagerRejected(t, lm.targetContainer, "import schema", args, tc.targetArgs, tc.wantOutput)
		})
		t.Run(tc.name+"/import data", func(t *testing.T) {
			args := []string{
				"--export-dir", lm.exportDir,
				"--target-db-name", dbName,
				"--disable-pb", "true",
				"--yes",
			}
			requireVoyagerRejected(t, lm.targetContainer, "import data", args, tc.targetArgs, tc.wantOutput)
		})
	}
}

func requireVoyagerRejected(t *testing.T, container testcontainers.TestContainer, cmdName string, args []string,
	sslArgs map[string]string, wantOutput string) {
	t.Helper()
	runner := testutils.NewVoyagerCommandRunner(container, cmdName, withFlags(args, sslArgs), nil, false).WithT(t)
	err := runner.Run()
	output := runner.Stdout() + runner.Stderr()
	require.Error(t, err, "%s must fail; output:\n%s", cmdName, output)
	assert.Contains(t, output, wantOutput, "%s failed for a different reason", cmdName)
}

func withFlags(args []string, flags map[string]string) []string {
	for key, value := range flags {
		args = append(args, key, value)
	}
	return args
}

func readDebeziumSSLProperties(t *testing.T, path string) map[string]string {
	t.Helper()
	file, err := os.Open(path)
	require.NoError(t, err)
	defer file.Close()

	sslProps := map[string]string{}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, found := strings.Cut(scanner.Text(), "=")
		if found && strings.HasPrefix(key, "debezium.source.database.ssl") && key != "debezium.source.database.sslpassword" {
			sslProps[key] = value
		}
	}
	require.NoError(t, scanner.Err())
	return sslProps
}
