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

package schemadrift

import (
	goerrors "github.com/go-errors/errors"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
)

// Severity is the severity level assigned to a DriftEntry.
type Severity string

const (
	SeverityAdvisory            Severity = "advisory"
	SeverityPotentialImpact     Severity = "potential_impact"
	SeverityBreaksRecoverable   Severity = "breaks_migration_recoverable"
	SeverityBreaksUnrecoverable Severity = "breaks_migration_unrecoverable"
)

// DriftInfo is what enriches a raw schemadiff.Difference into drift: how the
// migration is affected, and the "Impact & action" note explaining it. Severity and
// wording have to agree, so they are declared together rather than in parallel maps.
// DriftEntry embeds it; encoding/json flattens an anonymous embed, so the three
// fields stay top-level in the report.
//
// `backticks` become inline code in HTML (see codeSpans) and stay literal in JSON.
type DriftInfo struct {
	Severity Severity `json:"severity"`
	Impact   string   `json:"impact,omitempty"`
	Action   string   `json:"action,omitempty"`
}

// Timing is when a change happened relative to the migration, as situationFor
// places it.
type Timing string

const (
	TimingBeforeExportData        Timing = "before_export_data"
	TimingOfflineAfterExportStart Timing = "offline_after_export_start"
	TimingLiveAfterExportStart    Timing = "live_after_export_start"
	// Falls back to the TimingLiveAfterExportStart entry when a type has none of its own.
	TimingChangesOnlyAfterExportStart Timing = "changes_only_after_export_start"
	TimingAfterCutover                Timing = "after_cutover"
)

type driftGuidance struct {
	Default      DriftInfo
	ByTiming     map[Timing]DriftInfo
	FallBackRisk string // appended to Impact while a cutover with fall-back is still possible
}

const (
	restartDataMigration      = "restart the data migration: `export data --start-clean true`, then `import data --start-clean true --truncate-tables true`"
	newMigration              = "start a new migration in a new export directory"
	beforeApplicationMoves    = "before the application moves to the target"
	ifImportAlreadyFailedOnIt = "If `import data` already failed on it, run it again."
)

// Severity says what the MIGRATION does about a change, not how alarming the DDL
// sounds, so it reads backwards in places: an ADDED column is Recoverable because
// import data can fail on it, while a DROPPED one is only Potential impact.
var guidanceByDiffType = map[schemadiff.DiffType]driftGuidance{
	schemadiff.TableDropped: {
		Default: DriftInfo{Severity: SeverityAdvisory,
			Impact: "The table was dropped before `export data` started, so it is not part of the migration.",
			Action: "Drop the table on the target if you don't need it."},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: {Severity: SeverityAdvisory,
				Impact: "Rows already exported are not affected.",
				Action: "Drop the table on the target if you don't need it."},
			TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "`export data` keeps running, but if it is restarted, the resume fails because the migration still expects this table.",
				Action: "If `export data` has to restart, " + restartDataMigration + "."},
			TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "`export data` keeps running, but if it is restarted, the resume fails because the migration still expects this table.",
				Action: "If `export data` has to restart, " + newMigration + "."},
		},
	},
	schemadiff.TableNameChanged:   tableRenamedOrMovedGuidance,
	schemadiff.TableSchemaChanged: tableRenamedOrMovedGuidance,
	schemadiff.TableAdded: {
		Default: DriftInfo{Severity: SeverityBreaksRecoverable,
			Impact: "If the table is in the export's scope, it is exported, so `import data` fails unless the target already has a table with that name.",
			Action: "Create the table on the target before running `import data`. " + ifImportAlreadyFailedOnIt},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "The table was created after `export data` started, so its rows are not migrated.",
				Action: "Create the table on the target and migrate its rows separately: a new export directory with only this table in `--table-list`."},
			TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "The table was created after `export data` started, so its rows are not migrated, and a resume of `export data` doesn't pick it up. If you cut over with fall-back, changes to this table on the target are not sent back to the source.",
				Action: "Create the table on the target, then either migrate its rows separately (a new export directory with only this table in `--table-list`) or " + restartDataMigration + "."},
			TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "The table was created after `export data` started, so its rows are not migrated, and a resume of `export data` doesn't pick it up. If you cut over with fall-back, changes to this table on the target are not sent back to the source.",
				Action: "Create the table on the target, then either migrate its rows separately (a new export directory with only this table in `--table-list`) or " + newMigration + "."},
		},
	},
	schemadiff.ColumnAdded: {
		Default: DriftInfo{Severity: SeverityBreaksRecoverable,
			Impact: "The export includes the new column, which the target doesn't have, so `import data` fails.",
			Action: "Add the column on the target before running `import data`. " + ifImportAlreadyFailedOnIt},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
				Impact: "Rows already exported are not affected, but the target doesn't have the column.",
				Action: "Add the column on the target " + beforeApplicationMoves + "."},
			TimingLiveAfterExportStart: {Severity: SeverityBreaksRecoverable,
				Impact: "`export data` stops at the first change that sets the new column, and `import data` can't apply it until the target has the column.",
				Action: "Add the column on the target, then restart `export data` and `import data`."},
		},
	},
	schemadiff.ColumnDropped: {
		Default: DriftInfo{Severity: SeverityPotentialImpact,
			Impact: "The export doesn't include the column, which the target still has. `import data` fails only if that column is NOT NULL without a default on the target.",
			Action: "Drop the column on the target, or give it a default, before running `import data`. " + ifImportAlreadyFailedOnIt},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
				Impact: "Rows already exported are not affected, but the target still has the column.",
				Action: "Drop the column on the target " + beforeApplicationMoves + "."},
			TimingLiveAfterExportStart: {Severity: SeverityPotentialImpact,
				Impact: "The target keeps the dropped column. `import data` is not affected unless that column is NOT NULL without a default on the target.",
				Action: "Drop the column on the target before cutover."},
		},
		FallBackRisk: "If you cut over with fall-back, `import data to source` stops at the first insert on this table while the target still has this column, and later changes don't reach the source.",
	},
	schemadiff.ColumnNameChanged: {
		Default: DriftInfo{Severity: SeverityBreaksRecoverable,
			Impact: "The export uses the new name, which the target doesn't have, so `import data` fails.",
			Action: "Rename the column on the target before running `import data`. " + ifImportAlreadyFailedOnIt},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
				Impact: "Rows already exported are not affected, but the target still has the old name.",
				Action: "Rename the column on the target " + beforeApplicationMoves + "."},
			TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "`export data` stops at the first change written with the new name, and changes already queued under the old name can't be applied once the names differ.",
				Action: "Rename the column on the target, then " + restartDataMigration + "."},
			TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "`export data` stops at the first change written with the new name, and changes already queued under the old name can't be applied once the names differ.",
				Action: "Rename the column on the target, then " + newMigration + "."},
		},
	},
	schemadiff.ColumnTypeChanged: {
		Default: DriftInfo{Severity: SeverityPotentialImpact,
			Impact: "`import data` fails on the first value the target's column type can't hold.",
			Action: "Change the column type on the target. If `import data` already failed, run it again."},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
				Impact: "Rows already exported are not affected, but the target column still has the old type.",
				Action: "Change the column type on the target " + beforeApplicationMoves + "."},
		},
	},
	schemadiff.ColumnNotNullDropped: {
		Default: DriftInfo{Severity: SeverityBreaksRecoverable,
			Impact: "The source now allows NULLs, so `import data` fails on the first NULL in this column.",
			Action: "Drop NOT NULL on the target. If `import data` already failed, run it again."},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
				Impact: "Rows already exported are not affected, but the target column still rejects NULLs.",
				Action: "Drop NOT NULL on the target " + beforeApplicationMoves + "."},
		},
	},
	schemadiff.ColumnNotNullSet: {Default: DriftInfo{Severity: SeverityAdvisory,
		Impact: "`import data` is not affected.",
		Action: "Set NOT NULL on the target to keep the schemas the same."}},
	schemadiff.ColumnDefaultChanged: {Default: DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "`import data` is not affected, but once the application uses the target, its inserts get the old default.",
		Action: "Change the default on the target " + beforeApplicationMoves + "."}},
	schemadiff.TableKindChanged: {Default: DriftInfo{
		Severity: SeverityPotentialImpact,
		Impact:   "The migration is not affected mid-flight, but the target still has the table in its original form (ordinary vs partitioned vs foreign), so the schemas diverge and a restart may not reproduce the source layout.",
		Action:   "Recreate the table on the target with the matching kind before cutover.",
	}},
	schemadiff.TablePartitionAttached: {
		Default: DriftInfo{Severity: SeverityBreaksRecoverable,
			Impact: "The export treats the table as a partition, which the target doesn't have, so `import data` fails.",
			Action: "Attach the partition on the target before running `import data`. " + ifImportAlreadyFailedOnIt},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: partitionAttachedAfterExportStart,
			TimingLiveAfterExportStart:    partitionAttachedAfterExportStart,
		},
	},
	schemadiff.TablePartitionDetached: {
		Default: DriftInfo{Severity: SeverityPotentialImpact,
			Impact: "The target's partition layout no longer matches the source.",
			Action: "Detach the partition on the target " + beforeApplicationMoves + "."},
		ByTiming: map[Timing]DriftInfo{
			TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "Rows written to the detached table are still sent to the partitioned table on the target: rows inside the old range land there, and rows outside it make `import data` fail.",
				Action: "Detach the partition on the target, then " + restartDataMigration + "."},
			TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "Rows written to the detached table are still sent to the partitioned table on the target: rows inside the old range land there, and rows outside it make `import data` fail.",
				Action: "Detach the partition on the target, then " + newMigration + "."},
		},
	},
	schemadiff.TablePartitionChildrenChanged: {Default: DriftInfo{Severity: SeverityAdvisory,
		Impact: "The partitions of this table changed.",
		Action: "See the entry for each partition."}},
	schemadiff.TableInheritAdded: {
		Default: DriftInfo{Severity: SeverityBreaksRecoverable,
			Impact: "`export data` does not support tables that use inheritance, so it does not start.",
			Action: "Exclude the table and its parent with `--exclude-table-list`, and migrate them separately."},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
				Impact: "Rows already exported are not affected, but the target's inheritance no longer matches.",
				Action: "Apply the same inheritance on the target " + beforeApplicationMoves + "."},
			TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "`export data` keeps running, but a restart fails because the table now uses inheritance.",
				Action: "If `export data` has to restart, " + restartDataMigration + ", with the table and its parent in `--exclude-table-list`. Both then leave the migration, so migrate them separately."},
			TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "`export data` keeps running, but a restart fails because the table now uses inheritance.",
				Action: "If `export data` has to restart, " + newMigration + ", with the table and its parent in `--exclude-table-list`. Both then leave the migration, so migrate them separately."},
		},
	},
	schemadiff.TableInheritRemoved: {Default: DriftInfo{Severity: SeverityAdvisory,
		Impact: "`export data` and `import data` are not affected.",
		Action: "Remove the same inheritance on the target " + beforeApplicationMoves + "."}},
	schemadiff.TableInheritedByAdded: {
		Default: DriftInfo{Severity: SeverityBreaksRecoverable,
			Impact: "The table now has a child table, and `export data` does not support tables that use inheritance, so it does not start.",
			Action: "Exclude the table and its child tables with `--exclude-table-list`, and migrate them separately."},
		ByTiming: map[Timing]DriftInfo{
			TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
				Impact: "Rows already exported are not affected, but the target's inheritance no longer matches.",
				Action: "Apply the same inheritance on the target " + beforeApplicationMoves + "."},
			TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "`export data` keeps running, but a restart fails because the table now has a child table.",
				Action: "If `export data` has to restart, " + restartDataMigration + ", with the table and its child tables in `--exclude-table-list`. They then leave the migration, so migrate them separately."},
			TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
				Impact: "`export data` keeps running, but a restart fails because the table now has a child table.",
				Action: "If `export data` has to restart, " + newMigration + ", with the table and its child tables in `--exclude-table-list`. They then leave the migration, so migrate them separately."},
		},
	},
	schemadiff.TableInheritedByRemoved: {Default: DriftInfo{Severity: SeverityAdvisory,
		Impact: "`export data` and `import data` are not affected.",
		Action: "Remove the same inheritance on the target " + beforeApplicationMoves + "."}},
}

var tableRenamedOrMovedGuidance = driftGuidance{
	Default: DriftInfo{Severity: SeverityBreaksRecoverable,
		Impact: "If the table is in the export's scope, it is exported under its new name, which the target doesn't have, so `import data` fails.",
		Action: "Rename (or move) the table on the target before running `import data`. " + ifImportAlreadyFailedOnIt},
	ByTiming: map[Timing]DriftInfo{
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: "Rows already exported are not affected, but the target still has the old name.",
			Action: "Rename (or move) the table on the target " + beforeApplicationMoves + "."},
		TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "Rows written under the new name are not migrated, and a restart of `export data` fails.",
			Action: "Rename (or move) the table on the target, then " + restartDataMigration + "."},
		TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "Rows written under the new name are not migrated, and a restart of `export data` fails.",
			Action: "Rename (or move) the table on the target, then " + newMigration + "."},
	},
}

var partitionAttachedAfterExportStart = DriftInfo{Severity: SeverityPotentialImpact,
	Impact: "The target's partition layout no longer matches the source.",
	Action: "Attach the partition on the target " + beforeApplicationMoves + "."}

var afterCutoverInfo = DriftInfo{
	Severity: SeverityAdvisory,
	Impact:   "This change was made after cutover to target. Data already migrated to the target is not affected. Changes sent back with fall-back are not checked against it.",
	Action:   "None.",
}

func getDriftInfo(t schemadiff.DiffType, s situation) (DriftInfo, error) {
	if s.Timing == TimingAfterCutover {
		return afterCutoverInfo, nil
	}
	g, ok := guidanceByDiffType[t]
	if !ok {
		return DriftInfo{}, goerrors.Errorf("no Impact and Action for change type %q", t)
	}
	info, ok := g.ByTiming[s.Timing]
	if !ok && s.Timing == TimingChangesOnlyAfterExportStart {
		info, ok = g.ByTiming[TimingLiveAfterExportStart]
	}
	if !ok {
		info = g.Default
	}
	if s.FallBackPossible && g.FallBackRisk != "" {
		info.Impact += " " + g.FallBackRisk
	}
	return info, nil
}
