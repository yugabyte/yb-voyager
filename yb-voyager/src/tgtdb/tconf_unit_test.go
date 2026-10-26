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
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// pgconn.ParseConfig loads the root cert while parsing, so a missing file surfaces the exact path pgx decoded from the URI.
func TestGetConnectionUriSSLPathsRoundTripThroughPgx(t *testing.T) {
	tests := []struct {
		name        string
		sslRootCert string
	}{
		{name: "plain path", sslRootCert: "/nonexistent/certs/root.crt"},
		{name: "path with space", sslRootCert: "/nonexistent/my certs/root.crt"},
		{name: "path with plus", sslRootCert: "/nonexistent/c++/root.crt"},
		{name: "path with percent", sslRootCert: "/nonexistent/100%/root.crt"},
		{name: "path with ampersand and equals", sslRootCert: "/nonexistent/a&b=c/root.crt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tconf := &TargetConf{
				Host:        "localhost",
				Port:        5433,
				User:        "yugabyte",
				Password:    "p@ss word",
				DBName:      "yugabyte",
				SSLMode:     "verify-ca",
				SSLRootCert: tt.sslRootCert,
			}

			_, err := pgconn.ParseConfig(tconf.GetConnectionUri())

			require.Error(t, err)
			assert.Contains(t, err.Error(), "unable to read CA file: open "+tt.sslRootCert+": no such file or directory")
		})
	}
}
