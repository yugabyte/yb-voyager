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

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
)

// TargetYugabyteDBAmp is the target driver for YugabyteDB AMP (yb-amp).
//
// yb-amp is "Agentic Multitenant Postgres": a stateless, patched
// PostgreSQL 17 compute whose durable storage lives in a YugabyteDB
// cluster. From a client's (and therefore Voyager's) perspective the
// compute is plain PostgreSQL 17 on the wire — standard heap DDL (no YB
// HASH/RANGE sharding clauses), standard PG COPY semantics, and standard
// PG session GUCs. It does NOT understand YugabyteDB-specific GUCs such
// as yb_enable_upsert_mode / yb_disable_transactional_writes, and there
// is no adaptive parallelism / colocation / tablet concept.
//
// We therefore reuse the PostgreSQL target driver wholesale (via
// embedding) and layer on only the AMP-specific identity: a guardrail
// that confirms the endpoint really is yb-amp.
type TargetYugabyteDBAmp struct {
	*TargetPostgreSQL
}

func newTargetYugabyteDBAmp(tconf *TargetConf) *TargetYugabyteDBAmp {
	return &TargetYugabyteDBAmp{TargetPostgreSQL: newTargetPostgreSQL(tconf)}
}

// AMP_MARKER_SETTING_PREFIX matches the family of GUCs that yb-amp's
// patched compute exposes (yb_amp.tenant_id, yb_amp.pageserver_connstring,
// yb_amp.timeline_id, ...). Neither stock PostgreSQL nor YugabyteDB YSQL
// has any setting under this namespace, so its presence is a reliable
// "this is yb-amp" signal.
const AMP_MARKER_SETTING_PREFIX = "yb_amp."

func (amp *TargetYugabyteDBAmp) Init() error {
	if err := amp.TargetPostgreSQL.Init(); err != nil {
		return err
	}
	return amp.validateAmpTarget()
}

// validateAmpTarget confirms the connected compute is a yb-amp endpoint
// rather than a vanilla PostgreSQL or YugabyteDB server, so a user who
// mistypes --target-db-type gets a clear error instead of a subtly wrong
// migration.
func (amp *TargetYugabyteDBAmp) validateAmpTarget() error {
	var count int
	query := fmt.Sprintf(
		"SELECT count(*) FROM pg_settings WHERE name LIKE '%s%%'",
		AMP_MARKER_SETTING_PREFIX)
	if err := amp.QueryRow(query).Scan(&count); err != nil {
		return fmt.Errorf("validate target is YugabyteDB AMP (yb-amp): %w", err)
	}
	if count == 0 {
		return fmt.Errorf("the target at %s:%d does not look like a YugabyteDB AMP (yb-amp) endpoint: "+
			"no '%s*' settings were found. If you are migrating to a standard YugabyteDB or PostgreSQL "+
			"server, use the matching --target-db-type (yugabytedb) instead of '%s'",
			amp.tconf.Host, amp.tconf.Port, AMP_MARKER_SETTING_PREFIX, YUGABYTEDB_AMP)
	}
	log.Infof("validated target as YugabyteDB AMP (yb-amp): found %d '%s*' settings; compute version=%s",
		count, AMP_MARKER_SETTING_PREFIX, amp.GetVersion())
	return nil
}

func (amp *TargetYugabyteDBAmp) GetCallhomeTargetDBInfo() *callhome.TargetDBDetails {
	// Reuse the PostgreSQL-shaped info (node count 1, cores, version). The
	// target-db-type recorded by callhome already distinguishes ybamp.
	return amp.TargetPostgreSQL.GetCallhomeTargetDBInfo()
}

// The three methods below satisfy namereg.YBDBInterface, which the name
// registry requires of every import-to-target driver. TargetPostgreSQL does
// not implement them (it is only used as a fall-forward/back target, where a
// different registry path is taken), so we provide them here using the same
// standard catalog queries the YugabyteDB driver uses — all valid on
// yb-amp's PostgreSQL 17 compute.

func (amp *TargetYugabyteDBAmp) GetAllSchemaNamesRaw() ([]string, error) {
	query := "SELECT schema_name FROM information_schema.schemata"
	rows, err := amp.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying yb-amp target for schema names: %w", err)
	}
	defer rows.Close()

	var schemaNames []string
	for rows.Next() {
		var schemaName string
		if err = rows.Scan(&schemaName); err != nil {
			return nil, fmt.Errorf("scanning schema name: %w", err)
		}
		schemaNames = append(schemaNames, schemaName)
	}
	return schemaNames, rows.Err()
}

func (amp *TargetYugabyteDBAmp) GetAllTableNamesRaw(schemaName string) ([]string, error) {
	query := fmt.Sprintf(`SELECT table_name
			  FROM information_schema.tables
			  WHERE table_type = 'BASE TABLE' AND
			        table_schema = '%s';`, schemaName)
	rows, err := amp.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying yb-amp target (%q) for table names: %w", query, err)
	}
	defer rows.Close()

	var tableNames []string
	for rows.Next() {
		var tableName string
		if err = rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("scanning table name: %w", err)
		}
		tableNames = append(tableNames, tableName)
	}
	return tableNames, rows.Err()
}

func (amp *TargetYugabyteDBAmp) GetAllSequencesRaw(schemaName string) ([]string, error) {
	query := fmt.Sprintf(`SELECT sequencename FROM pg_sequences WHERE schemaname = '%s';`, schemaName)
	rows, err := amp.Query(query)
	if err != nil {
		return nil, fmt.Errorf("querying yb-amp target (%q) for sequence names: %w", query, err)
	}
	defer rows.Close()

	var sequenceNames []string
	for rows.Next() {
		var sequenceName string
		if err = rows.Scan(&sequenceName); err != nil {
			return nil, fmt.Errorf("scanning sequence name: %w", err)
		}
		sequenceNames = append(sequenceNames, sequenceName)
	}
	return sequenceNames, rows.Err()
}
