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
package schemasnapshot

import (
	"context"
	"database/sql"
	"time"

	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
)

// SourceCapture is the source-side capture policy over CaptureAndSaveSnapshot: is
// capture live at all, bound every attempt by CaptureTimeout, fall back to a
// placeholder, and optionally tick on a schedule.
//
// Everything it needs is a field, so it carries no dependency on command state. The
// caller resolves those once -- see cmd's captureSourceSchemaSnapshot -- and owns the
// exporter-role gate, which is a command concern rather than a capture one.
type SourceCapture struct {
	// DB is the live source connection. A nil DB is treated as "gone during
	// teardown" rather than a programming error, since exit captures race the
	// connection being closed.
	DB     *sql.DB
	MetaDB *metadb.MetaDB

	DBType   string
	Metadata DBMetadata
	Schemas  []string

	// Disabled mirrors --disable-schema-snapshot-capture.
	Disabled bool
}

// Enabled reports whether capture is live at all: PostgreSQL source, not disabled. It
// also returns why it is not, so a caller that wants to say so logs the specific reason
// rather than a generic one.
func (c SourceCapture) Enabled() (bool, string) {
	if c.DBType != constants.POSTGRESQL {
		return false, "only PostgreSQL sources are supported"
	}
	if c.Disabled {
		return false, "disabled via --disable-schema-snapshot-capture"
	}
	return true, ""
}

// Capture captures the source schema and persists it as a snapshot for the given
// label/reason, returning why it could not.
//
// A skip is not an error: a non-PostgreSQL source or a disabled capture returns nil,
// since nothing went wrong.
//
// The caller decides what a failure means. Today every caller is an export hook that
// logs and carries on, because schema capture is off the data path and must never fail
// or stall an export.
func (c SourceCapture) Capture(ctx context.Context, label, reason string, placeholderOnFailure bool) error {
	if enabled, why := c.Enabled(); !enabled {
		log.Infof("schema-snapshot capture skipped for label %q: %s", label, why)
		return nil
	}

	// Bound the catalog read and the metaDB write together, so a wedged source can
	// never block the migration on best-effort work. A caller's tighter deadline
	// still wins. (The placeholder path below deliberately uses a fresh context.)
	ctx, cancel := context.WithTimeout(ctx, CaptureTimeout)
	defer cancel()

	if c.DB == nil {
		if placeholderOnFailure {
			// Still record the timeline marker so a lifecycle moment (e.g. an exit)
			// isn't lost just because the DB handle is gone during teardown.
			c.RecordPlaceholder(label, reason)
		}
		return goerrors.Errorf("no active database handle")
	}

	// Every capture is persisted unconditionally — no dedup. Periodic snapshots record the
	// source schema at each interval so the drift timeline shows exactly when a change
	// appeared, even when consecutive snapshots are identical.
	req := CaptureRequest{
		CaptureParams: CaptureParams{
			DatabaseType: c.DBType,
			DBMetadata:   c.Metadata,
			Schemas:      c.Schemas,
			Label:        label,
			Reason:       reason,
		},
		PlaceholderOnFailure: placeholderOnFailure,
	}
	name, err := CaptureAndSaveSnapshot(ctx, c.DB, c.MetaDB, req)
	if err != nil {
		return err
	}
	log.Infof("captured schema snapshot %q", name)
	return nil
}

// StartPeriodic tickers a Capture every `interval` for as long as ctx lives, covering
// both the snapshot and streaming phases. The goroutine stops when ctx is cancelled, so
// there is no separate stop function.
//
// Best-effort: a no-op when capture is not enabled or interval <= 0.
func (c SourceCapture) StartPeriodic(ctx context.Context, interval time.Duration) {
	if enabled, _ := c.Enabled(); !enabled {
		return
	}
	if interval <= 0 {
		return
	}
	// Logged once per started ticker, so a second start is visible in the log (and
	// asserted by the live E2E): two tickers would double every periodic snapshot.
	log.Infof("starting periodic schema-snapshot capture every %s", interval)
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if err := c.Capture(ctx, LabelExportDataFromSourcePeriodic, "", false); err != nil {
					log.Warnf("periodic schema-snapshot capture failed, migration unaffected: %v", err)
				}
			}
		}
	}()
}

// RecordPlaceholder records a metadata-only timeline marker for a moment that cannot be
// fully captured. Best-effort; honors the disable flag.
//
// It uses its OWN fresh, bounded context: the capture context may be exactly what died,
// and reusing it would drop the marker just when it is needed.
func (c SourceCapture) RecordPlaceholder(label, reason string) {
	if enabled, _ := c.Enabled(); !enabled {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), CaptureTimeout)
	defer cancel()
	h := SnapshotHeader{
		Label:         label,
		Reason:        reason,
		Side:          SideSource,
		CapturedAt:    time.Now().UTC(),
		Schemas:       c.Schemas,
		IsPlaceholder: true,
	}
	if _, err := SavePlaceholder(ctx, c.MetaDB, h); err != nil {
		log.Warnf("schema-snapshot placeholder for label %q failed: %v", label, err)
	}
}
