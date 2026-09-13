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

	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"
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
// embedding) — including the namereg.YBDBInterface methods, which now
// live on TargetPostgreSQL — and layer on only the AMP-specific identity:
// a guardrail that confirms the endpoint really is yb-amp.
type TargetYugabyteDBAmp struct {
	*TargetPostgreSQL
}

func newTargetYugabyteDBAmp(tconf *TargetConf) *TargetYugabyteDBAmp {
	return &TargetYugabyteDBAmp{TargetPostgreSQL: newTargetPostgreSQL(tconf)}
}

func (amp *TargetYugabyteDBAmp) Init() error {
	if err := amp.TargetPostgreSQL.Init(); err != nil {
		return err
	}
	return nil
}

// validateAmpTarget confirms the connected compute is a yb-amp endpoint
// rather than a vanilla PostgreSQL or YugabyteDB server, so a user who
// mistypes --target-db-type gets a clear, actionable error instead of a
// subtly wrong migration.
func (amp *TargetYugabyteDBAmp) validateAmpTarget() error {
	hasAmp, err := endpointHasAmpGUCs(amp.db)
	if err != nil {
		return fmt.Errorf("validate target is YugabyteDB AMP (yb-amp): %w", err)
	}
	if hasAmp {
		log.Infof("validated target as YugabyteDB AMP (yb-amp): found '%s*' settings; compute version=%s",
			AMP_MARKER_SETTING_PREFIX, amp.GetVersion())
		return nil
	}
	// Not yb-amp — name the right target type to use depending on whether
	// this is a real YugabyteDB cluster or plain PostgreSQL.
	if isYB, _ := endpointIsRealYugabyteDB(amp.db); isYB {
		return goerrors.Errorf("the target at %s:%d is a standard YugabyteDB cluster, not YugabyteDB AMP (yb-amp). "+
			"Use --target-db-type %s instead of %s",
			amp.tconf.Host, amp.tconf.Port, YUGABYTEDB, YUGABYTEDB_AMP)
	}
	return goerrors.Errorf("the target at %s:%d does not look like a YugabyteDB AMP (yb-amp) endpoint: "+
		"no '%s*' settings were found (it looks like plain PostgreSQL). "+
		"If you are migrating to a standard YugabyteDB server, use --target-db-type %s instead of %s",
		amp.tconf.Host, amp.tconf.Port, AMP_MARKER_SETTING_PREFIX, YUGABYTEDB, YUGABYTEDB_AMP)
}
