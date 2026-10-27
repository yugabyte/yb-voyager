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
		`CREATE TABLE `+driftSchema+`.flags (a int)`,
		`CREATE TABLE `+driftSchema+`.metrics_a (id bigint NOT NULL) PARTITION BY RANGE (id)`,
		`CREATE TABLE `+driftSchema+`.metrics_b (id bigint NOT NULL) PARTITION BY RANGE (id)`,
		`CREATE TABLE `+driftSchema+`.metrics_p PARTITION OF `+driftSchema+`.metrics_a FOR VALUES FROM (1) TO (100)`,
		`CREATE TABLE `+driftSchema+`.base_x (a int)`,
		`CREATE TABLE `+driftSchema+`.base_y (a int)`,
		`CREATE TABLE `+driftSchema+`.base_z (a int)`,
		`CREATE TABLE `+driftSchema+`.kid (a int) INHERITS (`+driftSchema+`.base_x, `+driftSchema+`.base_y)`,
		`CREATE TABLE `+driftSchema+`."Pk_Added" ("Id" int)`,
		`CREATE TABLE `+driftSchema+`.pk_dropped (id int PRIMARY KEY)`,
		`CREATE TABLE `+driftSchema+`.pk_renamed (id int CONSTRAINT old_pk PRIMARY KEY)`,
		`CREATE TABLE `+driftSchema+`."Uq_Added" (a int)`,
		`CREATE TABLE `+driftSchema+`.uq_dropped (a int CONSTRAINT uq_dropped_key UNIQUE)`,
		`CREATE TABLE `+driftSchema+`.uq_renamed (a int CONSTRAINT old_uq UNIQUE)`,
		`CREATE TABLE `+driftSchema+`."Ck_Added" (a int)`,
		`CREATE TABLE `+driftSchema+`.ck_dropped (a int CONSTRAINT ck_dropped_check CHECK (a > 0))`,
		`CREATE TABLE `+driftSchema+`.ck_renamed (a int CONSTRAINT old_ck CHECK (a > 0))`,
		`CREATE TABLE `+driftSchema+`.pk_part (id int) PARTITION BY RANGE (id)`,
		`CREATE TABLE `+driftSchema+`.pk_part_a PARTITION OF `+driftSchema+`.pk_part FOR VALUES FROM (1) TO (10) PARTITION BY RANGE (id)`,
		`CREATE TABLE `+driftSchema+`.pk_part_a1 PARTITION OF `+driftSchema+`.pk_part_a FOR VALUES FROM (1) TO (5)`,
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
		`ALTER TABLE `+driftSchema+`.flags ALTER COLUMN a SET NOT NULL`,
		`ALTER TABLE `+driftSchema+`.metrics_a DETACH PARTITION `+driftSchema+`.metrics_p`,
		`ALTER TABLE `+driftSchema+`.metrics_b ATTACH PARTITION `+driftSchema+`.metrics_p FOR VALUES FROM (1) TO (100)`,
		`ALTER TABLE `+driftSchema+`.kid NO INHERIT `+driftSchema+`.base_x`,
		`ALTER TABLE `+driftSchema+`.kid INHERIT `+driftSchema+`.base_z`,
		`ALTER TABLE `+driftSchema+`."Pk_Added" ADD CONSTRAINT "Pk_Added_Pkey" PRIMARY KEY ("Id")`,
		`ALTER TABLE `+driftSchema+`.pk_dropped DROP CONSTRAINT pk_dropped_pkey`,
		`ALTER TABLE `+driftSchema+`.pk_renamed RENAME CONSTRAINT old_pk TO "New_Pk"`,
		`ALTER TABLE `+driftSchema+`."Uq_Added" ADD CONSTRAINT "Uq_Added_Key" UNIQUE (a)`,
		`ALTER TABLE `+driftSchema+`.uq_dropped DROP CONSTRAINT uq_dropped_key`,
		`ALTER TABLE `+driftSchema+`.uq_renamed RENAME CONSTRAINT old_uq TO "New_Uq"`,
		`ALTER TABLE `+driftSchema+`."Ck_Added" ADD CONSTRAINT "Ck_Added_Check" CHECK (a > 0)`,
		`ALTER TABLE `+driftSchema+`.ck_dropped DROP CONSTRAINT ck_dropped_check`,
		`ALTER TABLE `+driftSchema+`.ck_renamed RENAME CONSTRAINT old_ck TO "New_Ck"`,
		`ALTER TABLE `+driftSchema+`.pk_part ADD PRIMARY KEY (id)`,
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

	// ── 9. ColumnNotNullDropped for id, ColumnNotNullSet for flags.a ────────────
	idNotNullDropped := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnNotNullDropped && identKey(d, constants.POSTGRESQL) == driftSchema+".orders.id"
	})
	require.Len(t, idNotNullDropped, 1, "expected exactly one ColumnNotNullDropped for 'id'")
	assert.Equal(t, true, idNotNullDropped[0].SideAValue, "OldValue (was NotNull) must be true")
	assert.Equal(t, false, idNotNullDropped[0].SideBValue, "NewValue (now nullable) must be false")

	flagsNotNullSet := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.ColumnNotNullSet && identKey(d, constants.POSTGRESQL) == driftSchema+".flags.a"
	})
	require.Len(t, flagsNotNullSet, 1, "expected exactly one ColumnNotNullSet for 'flags.a'")
	assert.Equal(t, false, flagsNotNullSet[0].SideAValue)
	assert.Equal(t, true, flagsNotNullSet[0].SideBValue)

	// ── 9b. A partition moved between parents: one detach, one attach ─────────
	metricsPartition := findDiffs(func(d schemadiff.Difference) bool {
		return d.Attribute == schemadiff.AttrPartitionParent && identKey(d, constants.POSTGRESQL) == driftSchema+".metrics_p"
	})
	require.Len(t, metricsPartition, 2, "expected a detach and an attach for diff_it.metrics_p")
	assert.Equal(t, schemadiff.TablePartitionAttached, metricsPartition[0].Type)
	assert.Nil(t, metricsPartition[0].SideAValue)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: driftSchema, Name: "metrics_b"}, metricsPartition[0].SideBValue)
	assert.Equal(t, schemadiff.TablePartitionDetached, metricsPartition[1].Type)
	assert.Equal(t, schemasnapshot.ObjectRef{Schema: driftSchema, Name: "metrics_a"}, metricsPartition[1].SideAValue)
	assert.Nil(t, metricsPartition[1].SideBValue)

	// ── 9c. Inheritance: one finding per parent or child added or removed ──────
	inheritanceFindings := findDiffs(func(d schemadiff.Difference) bool {
		return d.Attribute == schemadiff.AttrInherits || d.Attribute == schemadiff.AttrInheritedBy
	})
	type inheritanceFinding struct {
		Type   schemadiff.DiffType
		Object string
		Old    any
		New    any
	}
	gotInheritance := make([]inheritanceFinding, len(inheritanceFindings))
	for i, d := range inheritanceFindings {
		gotInheritance[i] = inheritanceFinding{Type: d.Type, Object: identKey(d, constants.POSTGRESQL), Old: d.SideAValue, New: d.SideBValue}
	}
	kid := schemasnapshot.ObjectRef{Schema: driftSchema, Name: "kid"}
	assert.ElementsMatch(t, []inheritanceFinding{
		{Type: schemadiff.TableInheritedByRemoved, Object: driftSchema + ".base_x", Old: []schemasnapshot.ObjectRef{kid}},
		{Type: schemadiff.TableInheritedByAdded, Object: driftSchema + ".base_z", New: []schemasnapshot.ObjectRef{kid}},
		{Type: schemadiff.TableInheritAdded, Object: driftSchema + ".kid", New: []schemasnapshot.ObjectRef{{Schema: driftSchema, Name: "base_z"}}},
		{Type: schemadiff.TableInheritRemoved, Object: driftSchema + ".kid", Old: []schemasnapshot.ObjectRef{{Schema: driftSchema, Name: "base_x"}}},
	}, gotInheritance)

	// ── 10. TablePartitionChildrenChanged anchored to diff_it.events ────────────────
	partChanged := findDiffs(func(d schemadiff.Difference) bool {
		return d.Type == schemadiff.TablePartitionChildrenChanged && identKey(d, constants.POSTGRESQL) == driftSchema+".events"
	})
	require.Len(t, partChanged, 1, "expected exactly one TablePartitionChildrenChanged for diff_it.events")

	// ── 10b. Constraint findings ────────────────────────────────────────────────
	constraintFindings := findDiffs(func(d schemadiff.Difference) bool { return d.ObjectType == schemadiff.ObjectTypeConstraint })
	type constraintFinding struct {
		Type   schemadiff.DiffType
		Object string
		Old    any
		New    any
	}
	gotConstraints := make([]constraintFinding, len(constraintFindings))
	for i, d := range constraintFindings {
		gotConstraints[i] = constraintFinding{Type: d.Type, Object: identKey(d, constants.POSTGRESQL), Old: d.SideAValue, New: d.SideBValue}
	}
	assert.ElementsMatch(t, []constraintFinding{
		{Type: schemadiff.PrimaryKeyAdded, Object: driftSchema + `.Pk_Added.Pk_Added_Pkey`, New: `PRIMARY KEY ("Id")`},
		{Type: schemadiff.PrimaryKeyDropped, Object: driftSchema + `.pk_dropped.pk_dropped_pkey`, Old: `PRIMARY KEY (id)`},
		{Type: schemadiff.PrimaryKeyNameChanged, Object: driftSchema + `.pk_renamed.old_pk`, Old: "old_pk", New: "New_Pk"},
		{Type: schemadiff.UniqueAdded, Object: driftSchema + `.Uq_Added.Uq_Added_Key`, New: `UNIQUE (a)`},
		{Type: schemadiff.UniqueDropped, Object: driftSchema + `.uq_dropped.uq_dropped_key`, Old: `UNIQUE (a)`},
		{Type: schemadiff.UniqueNameChanged, Object: driftSchema + `.uq_renamed.old_uq`, Old: "old_uq", New: "New_Uq"},
		{Type: schemadiff.CheckAdded, Object: driftSchema + `.Ck_Added.Ck_Added_Check`, New: `CHECK (a > 0)`},
		{Type: schemadiff.CheckDropped, Object: driftSchema + `.ck_dropped.ck_dropped_check`, Old: `CHECK (a > 0)`},
		{Type: schemadiff.CheckNameChanged, Object: driftSchema + `.ck_renamed.old_ck`, Old: "old_ck", New: "New_Ck"},
		// The key's copies on pk_part_a and pk_part_a1 report nothing.
		{Type: schemadiff.PrimaryKeyAdded, Object: driftSchema + `.pk_part.pk_part_pkey`, New: `PRIMARY KEY (id)`},
	}, gotConstraints)

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
	allObjectTypes := []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn}
	scopedByNew := schemadiff.FilterByScope(diffs, schemadiff.Scope{
		Schemas:     []string{driftSchema},
		Tables:      []schemasnapshot.ObjectRef{{Schema: driftSchema, Name: "purchases"}},
		ObjectTypes: allObjectTypes,
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
		Schemas:     []string{driftSchema},
		Tables:      []schemasnapshot.ObjectRef{{Schema: driftSchema, Name: "orders"}},
		ObjectTypes: allObjectTypes,
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
// side-B for OpAdded (where ObjectA is nil), in dbType's dialect. This is the
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
