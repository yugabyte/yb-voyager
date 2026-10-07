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
package srcdb

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestPostgreSQLConnectionUriConnectTimeout(t *testing.T) {
	base := Source{Host: "localhost", Port: 5432, User: "u", Password: "p", DBName: "db"}

	tests := []struct {
		name    string
		sslMode string
		timeout time.Duration
		want    string
	}{
		{name: "no timeout leaves the URI as before", sslMode: "prefer", want: "postgresql://u:p@localhost:5432/db?sslmode=prefer"},
		{name: "timeout is appended after the SSL mode", sslMode: "prefer", timeout: 10 * time.Second, want: "postgresql://u:p@localhost:5432/db?sslmode=prefer&connect_timeout=10"},
		{name: "timeout with a verifying SSL mode", sslMode: "require", timeout: 10 * time.Second, want: "postgresql://u:p@localhost:5432/db?sslmode=require&connect_timeout=10"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := base
			s.SSLMode = tt.sslMode
			s.ConnectTimeout = tt.timeout
			pg := &PostgreSQL{source: &s}
			assert.Equal(t, tt.want, pg.getConnectionUri())
		})
	}
}
