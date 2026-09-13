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

package schemadiff_test

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

const driftSchema = "diff_it"

// TestDiff_EndToEnd starts a real PostgreSQL container, captures two schema
// snapshots with DDL mutations in between, and asserts that schemadiff.Diff
// produces the expected findings including rename detection, column changes,
// and partition children changes.
//
// Run with:
//
//	go test -tags integration -run TestDiff ./src/schemadiff/... -v
func TestDiff_EndToEnd(t *testing.T) {
	ctx := context.Background()

	pg := testcontainers.NewTestContainer(testcontainers.POSTGRESQL, nil)
	require.NoError(t, pg.Start(ctx), "start postgres container")
	defer pg.Terminate(ctx)

	// ── Baseline DDL — snapshot A ──────────────────────────────────────────────
	pg.ExecuteSqls(
		`CREATE SCHEMA `+driftSchema,
		`CREATE TABLE `+driftSchema+`.orders (
			id       integer NOT NULL,
			customer text,
			amount   numeric DEFAULT 0
		)`,
		`CREATE TABLE `+driftSchema+`.legacy (a int, b text)`,
		`CREATE TABLE `+driftSchema+`.events (
			id bigint NOT NULL
		) PARTITION BY RANGE (id)`,
		`CREATE TABLE `+driftSchema+`.events_2026 PARTITION OF `+driftSchema+`.events FOR VALUES FROM (1) TO (1000000)`,
	)

	db, err := pg.GetConnection()
	require.NoError(t, err, "get connection for snapshot A")

	host, port, err := pg.GetHostPort()
	require.NoError(t, err, "get host/port")
	pgCfg := pg.GetConfig()

	params := schemasnapshot.CaptureParams{
		DatabaseType: constants.POSTGRESQL,
		Side:         schemasnapshot.SideSource,
		DBMetadata: schemasnapshot.DBMetadata{
			Host:     host,
			Port:     port,
			Database: pgCfg.DBName,
			User:     pgCfg.User,
		},
		Schemas: []string{driftSchema},
		Label:   schemasnapshot.LabelExportSchema,
	}

	snapAWrapper, err := schemasnapshot.Capture(ctx, db, params)
	require.NoError(t, err, "Capture snapshot A")
	snapA := snapAWrapper.Content
	db.Close()

	// ── Mutation DDL — snapshot B ──────────────────────────────────────────────
	pg.ExecuteSqls(
		`ALTER TABLE `+driftSchema+`.orders RENAME TO purchases`,
		`ALTER TABLE `+driftSchema+`.purchases ADD COLUMN discount numeric`,
		`ALTER TABLE `+driftSchema+`.purchases ALTER COLUMN customer TYPE varchar(255)`,
		`ALTER TABLE `+driftSchema+`.purchases ALTER COLUMN id DROP NOT NULL`,
		`ALTER TABLE `+driftSchema+`.purchases DROP COLUMN amount`,
		`DROP TABLE `+driftSchema+`.legacy`,
		`CREATE TABLE `+driftSchema+`.newbie (x int)`,
		`CREATE TABLE `+driftSchema+`.events_2027 PARTITION OF `+driftSchema+`.events FOR VALUES FROM (1000000) TO (2000000)`,
	)

	db2, err := pg.GetConnection()
	require.NoError(t, err, "get connection for snapshot B")
	defer db2.Close()

	snapBWrapper, err := schemasnapshot.Capture(ctx, db2, params)
	require.NoError(t, err, "Capture snapshot B")
	snapB := snapBWrapper.Content

	// ── Run the diff ───────────────────────────────────────────────────────────
	diffs := schemadiff.Diff(snapA, snapB)
	require.NotEmpty(t, diffs, "Diff must return at least some findings")

	t.Logf("Total findings: %d", len(diffs))
	for i, d := range diffs {
		t.Logf("  [%d] Type=%-30s Object=%-30s SideAValue=%v SideBValue=%v",
			i, d.Type, identDisplay(d, constants.POSTGRESQL), d.SideAValue, d.SideBValue)
	}

	// ── Helper ─────────────────────────────────────────────────────────────────
	findDiffs := func(pred func(schemadiff.Difference) bool) []schemadiff.Difference {
		var out []schemadiff.Difference
		for _, d := range diffs {
			if pred(d) {
				out = append(out, d)
			}
		}
		return out
	}

	// ── 1. Exactly one TableNameChanged for orders → purchases ─────────────────
	renames := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableNameChanged && identKey(d, constants.POSTGRESQL) == driftSchema+".orders"
	})
	require.Len(t, renames, 1, "expected exactly one TableNameChanged anchored to diff_it.orders")
	assert.Equal(t, "purchases", renames[0].SideBValue, "NewValue must be the new table name 'purchases'")

	// ── 2. No TableAdded/TableDropped for orders or purchases ──────────────────
	spurious := findDiffs(func(d schemadiff.Difference) bool {
		if d.Type != schemadiff.TableAdded && d.Type != schemadiff.TableDropped {
			return false
		}
		return identKey(d, constants.POSTGRESQL) == driftSchema+".orders" || identKey(d, constants.POSTGRESQL) == driftSchema+".purchases"
	})
	assert.Empty(t, spurious,
		"rename must NOT appear as TableAdded+TableDropped for 'orders' or 'purchases'; got: %v", spurious)

	// ── 3. TableDropped for diff_it.legacy ─────────────────────────────────────
	legacyDropped := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableDropped && identKey(d, constants.POSTGRESQL) == driftSchema+".legacy"
	})
	require.Len(t, legacyDropped, 1, "expected exactly one TableDropped for diff_it.legacy")

	// TableDropped for a wholly-dropped table carries its columns as OldValue,
	// and those columns must NOT also appear as standalone ColumnDropped findings.
	legacyTbl, ok := legacyDropped[0].SideAValue.(schemasnapshot.Table)
	require.True(t, ok, "TableDropped.SideAValue must be schemasnapshot.Table, got %T", legacyDropped[0].SideAValue)
	var legacyColNames []string
	for _, c := range legacyTbl.Columns {
		legacyColNames = append(legacyColNames, c.Name)
	}
	assert.ElementsMatch(t, []string{"a", "b"}, legacyColNames,
		"TableDropped(legacy).SideAValue must carry legacy's columns")
	legacyStandaloneColumnDrops := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnDropped && strings.HasPrefix(identKey(d, constants.POSTGRESQL), driftSchema+".legacy.")
	})
	assert.Empty(t, legacyStandaloneColumnDrops,
		"legacy's columns must remain suppressed as standalone ColumnDropped findings")

	// ── 4. TableAdded for diff_it.newbie ───────────────────────────────────────
	newbieAdded := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableAdded && identKey(d, constants.POSTGRESQL) == driftSchema+".newbie"
	})
	require.Len(t, newbieAdded, 1, "expected exactly one TableAdded for diff_it.newbie")

	// TableAdded for a wholly-added table carries its columns as NewValue, and
	// those columns must NOT also appear as standalone ColumnAdded findings.
	newbieTbl, ok := newbieAdded[0].SideBValue.(schemasnapshot.Table)
	require.True(t, ok, "TableAdded.SideBValue must be schemasnapshot.Table, got %T", newbieAdded[0].SideBValue)
	var newbieColNames []string
	for _, c := range newbieTbl.Columns {
		newbieColNames = append(newbieColNames, c.Name)
	}
	assert.ElementsMatch(t, []string{"x"}, newbieColNames,
		"TableAdded(newbie).SideBValue must carry newbie's columns")
	newbieStandaloneColumnAdds := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnAdded && strings.HasPrefix(identKey(d, constants.POSTGRESQL), driftSchema+".newbie.")
	})
	assert.Empty(t, newbieStandaloneColumnAdds,
		"newbie's columns must remain suppressed as standalone ColumnAdded findings")

	// ── 5. TableAdded for diff_it.events_2027 ──────────────────────────────────
	events2027Added := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableAdded && identKey(d, constants.POSTGRESQL) == driftSchema+".events_2027"
	})
	require.Len(t, events2027Added, 1, "expected exactly one TableAdded for diff_it.events_2027")

	// ── 6. ColumnAdded for discount (on the renamed table, side-B name "purchases") ──
	discountAdded := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnAdded && identKey(d, constants.POSTGRESQL) == driftSchema+".purchases.discount"
	})
	require.Len(t, discountAdded, 1, "expected exactly one ColumnAdded with Object.Key='diff_it.purchases.discount'")

	// ── 7. ColumnDropped for amount (on the pre-rename table, side-A name "orders") ──
	amountDropped := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnDropped && identKey(d, constants.POSTGRESQL) == driftSchema+".orders.amount"
	})
	require.Len(t, amountDropped, 1, "expected exactly one ColumnDropped with Object.Key='diff_it.orders.amount'")

	// ── 8. ColumnTypeChanged for customer ──────────────────────────────────────
	customerTypeChanged := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnTypeChanged && identKey(d, constants.POSTGRESQL) == driftSchema+".orders.customer"
	})
	require.Len(t, customerTypeChanged, 1, "expected exactly one ColumnTypeChanged with Object.Key='diff_it.orders.customer'")

	// ── 9. ColumnNullabilityChanged for id (NotNull true → false) ──────────────
	idNullabilityChanged := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnNullabilityChanged && identKey(d, constants.POSTGRESQL) == driftSchema+".orders.id"
	})
	require.Len(t, idNullabilityChanged, 1, "expected exactly one ColumnNullabilityChanged for 'id'")
	assert.Equal(t, true, idNullabilityChanged[0].SideAValue,
		"OldValue (was NotNull) must be true")
	assert.Equal(t, false, idNullabilityChanged[0].SideBValue,
		"NewValue (now nullable) must be false")

	// ── 10. TablePartitionChildrenChanged anchored to diff_it.events ────────────────
	partChanged := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TablePartitionChildrenChanged && identKey(d, constants.POSTGRESQL) == driftSchema+".events"
	})
	require.Len(t, partChanged, 1, "expected exactly one TablePartitionChildrenChanged for diff_it.events")

	// ── 11. Scope filtering: Tables=["diff_it.purchases"] (new name) ───────────
	// Rename/move alias handling is TEMPORARILY DISABLED in FilterByScope (see
	// filter.go, pending the cross-window alias decision). With the alias off,
	// scoping by a table name retains only findings whose OWN derived anchor is
	// that table — the old↔new bridge is gone. So scoping by the new name
	// "purchases" keeps the column finding anchored to purchases, but NOT the
	// rename finding or the amount-drop finding, both of which anchor to the old
	// name "orders".
	//
	// When the alias is re-enabled, the two assert.Empty checks below flip back to
	// requiring exactly one retained finding (mirrors the skipped alias unit tests
	// in filter_test.go / differ_test.go).
	scopedByNew := schemadiff.FilterByScope(diffs, schemadiff.Scope{
		Tables: []schemasnapshot.ObjectRef{{Schema: driftSchema, Name: "purchases"}},
	})
	t.Logf("Scoped by new name 'purchases': %d findings", len(scopedByNew))

	// Direct match: the added column lives on the new table, so it is retained.
	scopedDiscountByNew := filterLocal(scopedByNew, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnAdded && identKey(d, constants.POSTGRESQL) == driftSchema+".purchases.discount"
	})
	assert.Len(t, scopedDiscountByNew, 1,
		"ColumnAdded(discount) anchored to 'purchases' must be retained under scope Tables=['diff_it.purchases']")

	// Alias OFF: rename finding anchors to old 'orders' and is NOT bridged to the new name.
	scopedRenameByNew := filterLocal(scopedByNew, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableNameChanged && identKey(d, constants.POSTGRESQL) == driftSchema+".orders"
	})
	assert.Empty(t, scopedRenameByNew,
		"alias disabled: TableNameChanged anchored to old name 'orders' is NOT retained when scoping by new name 'purchases'")

	// Alias OFF: the amount-drop finding also anchors to old 'orders' and is dropped.
	scopedAmountByNew := filterLocal(scopedByNew, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnDropped && identKey(d, constants.POSTGRESQL) == driftSchema+".orders.amount"
	})
	assert.Empty(t, scopedAmountByNew,
		"alias disabled: ColumnDropped(amount) anchored to old name 'orders' is NOT retained when scoping by new name 'purchases'")

	// ── 12. Scope filtering: Tables=["diff_it.orders"] (old name) ──────────────
	scopedByOld := schemadiff.FilterByScope(diffs, schemadiff.Scope{
		Tables: []schemasnapshot.ObjectRef{{Schema: driftSchema, Name: "orders"}},
	})
	t.Logf("Scoped by old name 'orders': %d findings", len(scopedByOld))

	// Direct match: the rename finding anchors to the old name, so it is retained
	// without any alias.
	scopedRenameByOld := filterLocal(scopedByOld, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableNameChanged && identKey(d, constants.POSTGRESQL) == driftSchema+".orders"
	})
	assert.Len(t, scopedRenameByOld, 1,
		"TableNameChanged anchored to 'orders' must be retained when scoping by old name 'orders'")

	// Alias OFF: the added column lives on the new table 'purchases', so scoping by
	// the old name 'orders' does NOT retain it.
	scopedDiscountByOld := filterLocal(scopedByOld, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnAdded && identKey(d, constants.POSTGRESQL) == driftSchema+".purchases.discount"
	})
	assert.Empty(t, scopedDiscountByOld,
		"alias disabled: ColumnAdded(discount) anchored to 'purchases' is NOT retained when scoping by old name 'orders'")
}

// filterLocal is a package-local helper mirroring findDiffs but operates on an
// already-filtered slice to keep scope-assertion code readable.
func filterLocal(diffs []schemadiff.Difference, pred func(schemadiff.Difference) bool) []schemadiff.Difference {
	var out []schemadiff.Difference
	for _, d := range diffs {
		if pred(d) {
			out = append(out, d)
		}
	}
	return out
}

// identKey renders a finding's own identity key: side-A's key, falling back to
// side-B for *_ADDED (where ObjectA is nil), in dbType's dialect. This is the
// direct replacement for the old pre-rendered Difference.Object.Key.
func identKey(d schemadiff.Difference, dbType string) string {
	if d.ObjectA != nil {
		return d.ObjectA.ForKey(dbType)
	}
	return d.ObjectB.ForKey(dbType)
}

// identDisplay is identKey's ForDisplay counterpart, replacing the old
// Difference.Object.Display.
func identDisplay(d schemadiff.Difference, dbType string) string {
	if d.ObjectA != nil {
		return d.ObjectA.ForDisplay(dbType)
	}
	return d.ObjectB.ForDisplay(dbType)
}
