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
		t.Logf("  [%d] Type=%-30s Object=%-30s SubObject=%s OldValue=%v NewValue=%v",
			i, d.Type, d.Object.ForDisplay(constants.POSTGRESQL), d.SubObject, d.OldValue, d.NewValue)
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
		return d.Type == schemadiff.TableNameChanged &&
			d.Object.Schema == driftSchema && d.Object.Name == "orders"
	})
	require.Len(t, renames, 1, "expected exactly one TableNameChanged anchored to diff_it.orders")
	assert.Equal(t, "purchases", renames[0].NewValue, "NewValue must be the new table name 'purchases'")

	// ── 2. No TableAdded/TableDropped for orders or purchases ──────────────────
	spurious := findDiffs(func(d schemadiff.Difference) bool {
		if d.Type != schemadiff.TableAdded && d.Type != schemadiff.TableDropped {
			return false
		}
		n := d.Object.Name
		return n == "orders" || n == "purchases"
	})
	assert.Empty(t, spurious,
		"rename must NOT appear as TableAdded+TableDropped for 'orders' or 'purchases'; got: %v", spurious)

	// ── 3. TableDropped for diff_it.legacy ─────────────────────────────────────
	legacyDropped := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableDropped &&
			d.Object.Schema == driftSchema && d.Object.Name == "legacy"
	})
	require.Len(t, legacyDropped, 1, "expected exactly one TableDropped for diff_it.legacy")

	// TableDropped for a wholly-dropped table carries its columns as OldValue,
	// and those columns must NOT also appear as standalone ColumnDropped findings.
	legacyCols, ok := legacyDropped[0].OldValue.([]schemasnapshot.Column)
	require.True(t, ok, "TableDropped.OldValue must be []schemasnapshot.Column, got %T", legacyDropped[0].OldValue)
	var legacyColNames []string
	for _, c := range legacyCols {
		legacyColNames = append(legacyColNames, c.Name)
	}
	assert.ElementsMatch(t, []string{"a", "b"}, legacyColNames,
		"TableDropped(legacy).OldValue must carry legacy's columns")
	legacyStandaloneColumnDrops := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnDropped && d.Object.Name == "legacy"
	})
	assert.Empty(t, legacyStandaloneColumnDrops,
		"legacy's columns must remain suppressed as standalone ColumnDropped findings")

	// ── 4. TableAdded for diff_it.newbie ───────────────────────────────────────
	newbieAdded := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableAdded &&
			d.Object.Schema == driftSchema && d.Object.Name == "newbie"
	})
	require.Len(t, newbieAdded, 1, "expected exactly one TableAdded for diff_it.newbie")

	// TableAdded for a wholly-added table carries its columns as NewValue, and
	// those columns must NOT also appear as standalone ColumnAdded findings.
	newbieCols, ok := newbieAdded[0].NewValue.([]schemasnapshot.Column)
	require.True(t, ok, "TableAdded.NewValue must be []schemasnapshot.Column, got %T", newbieAdded[0].NewValue)
	var newbieColNames []string
	for _, c := range newbieCols {
		newbieColNames = append(newbieColNames, c.Name)
	}
	assert.ElementsMatch(t, []string{"x"}, newbieColNames,
		"TableAdded(newbie).NewValue must carry newbie's columns")
	newbieStandaloneColumnAdds := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnAdded && d.Object.Name == "newbie"
	})
	assert.Empty(t, newbieStandaloneColumnAdds,
		"newbie's columns must remain suppressed as standalone ColumnAdded findings")

	// ── 5. TableAdded for diff_it.events_2027 ──────────────────────────────────
	events2027Added := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableAdded &&
			d.Object.Schema == driftSchema && d.Object.Name == "events_2027"
	})
	require.Len(t, events2027Added, 1, "expected exactly one TableAdded for diff_it.events_2027")

	// ── 6. ColumnAdded for discount ────────────────────────────────────────────
	discountAdded := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnAdded && d.SubObject == "discount"
	})
	require.Len(t, discountAdded, 1, "expected exactly one ColumnAdded with SubObject='discount'")

	// ── 7. ColumnDropped for amount ────────────────────────────────────────────
	amountDropped := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnDropped && d.SubObject == "amount"
	})
	require.Len(t, amountDropped, 1, "expected exactly one ColumnDropped with SubObject='amount'")

	// ── 8. ColumnTypeChanged for customer ──────────────────────────────────────
	customerTypeChanged := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnTypeChanged && d.SubObject == "customer"
	})
	require.Len(t, customerTypeChanged, 1, "expected exactly one ColumnTypeChanged with SubObject='customer'")

	// ── 9. ColumnNullabilityChanged for id (NotNull true → false) ──────────────
	idNullabilityChanged := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnNullabilityChanged && d.SubObject == "id" &&
			d.Object.Schema == driftSchema
	})
	require.Len(t, idNullabilityChanged, 1, "expected exactly one ColumnNullabilityChanged for 'id'")
	assert.Equal(t, true, idNullabilityChanged[0].OldValue,
		"OldValue (was NotNull) must be true")
	assert.Equal(t, false, idNullabilityChanged[0].NewValue,
		"NewValue (now nullable) must be false")

	// ── 10. TablePartitionChildrenChanged anchored to diff_it.events ────────────────
	partChanged := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TablePartitionChildrenChanged &&
			d.Object.Schema == driftSchema && d.Object.Name == "events"
	})
	require.Len(t, partChanged, 1, "expected exactly one TablePartitionChildrenChanged for diff_it.events")

	// ── 11. Scope filtering: Tables=["diff_it.purchases"] ──────────────────────
	// The rename finding is anchored to old-name "diff_it.orders"; the alias map
	// must bridge purchases ↔ orders so both names retain the rename finding.
	scopedByNew := schemadiff.FilterByScope(diffs, schemadiff.Scope{
		Tables: []schemasnapshot.ObjectRef{{Schema: driftSchema, Name: "purchases"}},
	})
	t.Logf("Scoped by new name 'purchases': %d findings", len(scopedByNew))

	scopedRenameByNew := filterLocal(scopedByNew, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableNameChanged &&
			d.Object.Schema == driftSchema && d.Object.Name == "orders"
	})
	assert.Len(t, scopedRenameByNew, 1,
		"TableNameChanged anchored to old name 'orders' must be retained when scoping by new name 'purchases'")

	// Column findings on the renamed table must also survive scope by new name.
	scopedDiscountByNew := filterLocal(scopedByNew, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnAdded && d.SubObject == "discount"
	})
	assert.Len(t, scopedDiscountByNew, 1,
		"ColumnAdded(discount) must be retained under scope Tables=['diff_it.purchases']")

	scopedAmountByNew := filterLocal(scopedByNew, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnDropped && d.SubObject == "amount"
	})
	assert.Len(t, scopedAmountByNew, 1,
		"ColumnDropped(amount) must be retained under scope Tables=['diff_it.purchases']")

	// ── 12. Scope filtering: Tables=["diff_it.orders"] (old name) ──────────────
	scopedByOld := schemadiff.FilterByScope(diffs, schemadiff.Scope{
		Tables: []schemasnapshot.ObjectRef{{Schema: driftSchema, Name: "orders"}},
	})
	t.Logf("Scoped by old name 'orders': %d findings", len(scopedByOld))

	scopedRenameByOld := filterLocal(scopedByOld, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TableNameChanged &&
			d.Object.Schema == driftSchema && d.Object.Name == "orders"
	})
	assert.Len(t, scopedRenameByOld, 1,
		"TableNameChanged must be retained when scoping by old name 'orders'")

	scopedDiscountByOld := filterLocal(scopedByOld, func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnAdded && d.SubObject == "discount"
	})
	assert.Len(t, scopedDiscountByOld, 1,
		"ColumnAdded(discount) must be retained under scope Tables=['diff_it.orders']")
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
