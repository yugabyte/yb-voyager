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
	"database/sql"
	"fmt"
)

// Common helpers for YugabyteDB

// AMP_MARKER_SETTING_PREFIX matches the family of GUCs that yb-amp's
// patched compute exposes (yb_amp.tenant_id, yb_amp.pageserver_connstring,
// yb_amp.timeline_id, ...). Neither stock PostgreSQL nor YugabyteDB YSQL
// has any setting under this namespace, so its presence is a reliable
// "this is yb-amp" signal.
const AMP_MARKER_SETTING_PREFIX = "yb_amp."

// endpointHasAmpGUCs reports whether the endpoint exposes any yb_amp.*
// settings — the yb-amp fingerprint.
func endpointHasAmpGUCs(db *sql.DB) (bool, error) {
	var count int
	query := fmt.Sprintf("SELECT count(*) FROM pg_settings WHERE name LIKE '%s%%'", AMP_MARKER_SETTING_PREFIX)
	if err := db.QueryRow(query).Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}

// endpointIsRealYugabyteDB reports whether the endpoint is a genuine
// YugabyteDB cluster. yb_servers() is a YugabyteDB built-in that is absent
// on vanilla PostgreSQL and on yb-amp's PG17 compute, so its presence in
// pg_proc is a reliable "real YB" signal.
func endpointIsRealYugabyteDB(db *sql.DB) (bool, error) {
	var count int
	if err := db.QueryRow("SELECT count(*) FROM pg_proc WHERE proname = 'yb_servers'").Scan(&count); err != nil {
		return false, err
	}
	return count > 0, nil
}
