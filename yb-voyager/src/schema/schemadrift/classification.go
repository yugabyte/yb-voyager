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

func getDriftInfo(t schemadiff.DiffType, s situation) (DriftInfo, error) {
	if s.Timing == TimingAfterCutover {
		return afterCutoverInfo, nil
	}
	info, ok := infoByDiffType[t][s.Timing]
	if !ok && s.Timing == TimingChangesOnlyAfterExportStart {
		info, ok = infoByDiffType[t][TimingLiveAfterExportStart]
	}
	if !ok {
		return DriftInfo{}, goerrors.Errorf("no Impact and Action for change type %q at timing %q", t, s.Timing)
	}
	if risk, ok := fallBackRiskByDiffType[t]; ok && s.FallBackPossible {
		info.Impact += " " + risk
	}
	if action, ok := fallForwardActionByDiffType[t]; ok && s.FallForwardPossible {
		info.Action += " " + action
	}
	return info, nil
}

// ─── Impact and Action per change type and timing ──────────────────────────────

// Severity says what the MIGRATION does about a change, not how alarming the DDL
// sounds, so it reads backwards in places: an ADDED column is Recoverable because
// import data can fail on it, while a DROPPED one is only Potential impact.
var infoByDiffType = map[schemadiff.DiffType]map[Timing]DriftInfo{
	schemadiff.TableDropped: {
		TimingBeforeExportData: {Severity: SeverityAdvisory,
			Impact: "The table was dropped before `export data` started, so it is not part of the migration.",
			Action: "Drop the table on the target if you don't need it."},
		TimingOfflineAfterExportStart: {Severity: SeverityAdvisory,
			Impact: "Rows already exported are not affected.",
			Action: "Drop the table on the target if you don't need it."},
		TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "`export data` keeps running, but if it is restarted, the resume fails because the migration still expects this table.",
			Action: "If `export data` has to restart, restart the data migration: `export data --start-clean true`, then `import data --start-clean true --truncate-tables true`."},
		TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "`export data` keeps running, but if it is restarted, the resume fails because the migration still expects this table.",
			Action: "If `export data` has to restart, start a new migration in a new export directory."},
	},
	schemadiff.TableNameChanged: {
		TimingBeforeExportData:            tableRenamedOrMovedBeforeExportInfo,
		TimingOfflineAfterExportStart:     tableRenamedOrMovedOfflineInfo,
		TimingLiveAfterExportStart:        tableRenamedOrMovedLiveInfo,
		TimingChangesOnlyAfterExportStart: tableRenamedOrMovedChangesOnlyInfo,
	},
	schemadiff.TableSchemaChanged: {
		TimingBeforeExportData:            tableRenamedOrMovedBeforeExportInfo,
		TimingOfflineAfterExportStart:     tableRenamedOrMovedOfflineInfo,
		TimingLiveAfterExportStart:        tableRenamedOrMovedLiveInfo,
		TimingChangesOnlyAfterExportStart: tableRenamedOrMovedChangesOnlyInfo,
	},
	schemadiff.TableAdded: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: "If the table is in the export's scope, it is exported, so `import data` fails unless the target already has a table with that name.",
			Action: "Create the table on the target before running `import data`. If `import data` already failed on it, run it again."},
		TimingOfflineAfterExportStart: {Severity: SeverityAdvisory,
			Impact: "The table was created after `export data` started, so its rows are not migrated.",
			Action: "If you need this table, create it on the target and migrate its rows separately: a new export directory with only this table in `--table-list`."},
		TimingLiveAfterExportStart: {Severity: SeverityAdvisory,
			Impact: "The table was created after `export data` started, so its rows are not migrated, and a resume of `export data` doesn't pick it up. If you cut over with fall-back, changes to this table on the target are not sent back to the source.",
			Action: "If you need this table, create it on the target, then either migrate its rows separately (a new export directory with only this table in `--table-list`) or restart the data migration: `export data --start-clean true`, then `import data --start-clean true --truncate-tables true`."},
		TimingChangesOnlyAfterExportStart: {Severity: SeverityAdvisory,
			Impact: "The table was created after `export data` started, so its rows are not migrated, and a resume of `export data` doesn't pick it up. If you cut over with fall-back, changes to this table on the target are not sent back to the source.",
			Action: "If you need this table, create it on the target, then either migrate its rows separately (a new export directory with only this table in `--table-list`) or start a new migration in a new export directory."},
	},
	schemadiff.ColumnAdded: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: "The export includes the new column, which the target doesn't have, so `import data` fails.",
			Action: "Add the column on the target before running `import data`. If `import data` already failed on it, run it again."},
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: "Rows already exported are not affected, but the target doesn't have the column.",
			Action: "Add the column on the target before the application moves to the target."},
		TimingLiveAfterExportStart: {Severity: SeverityBreaksRecoverable,
			Impact: "`export data` stops at the first change that sets the new column, and `import data` can't apply it until the target has the column.",
			Action: "Add the column on the target, then restart `export data` and `import data`."},
	},
	schemadiff.ColumnDropped: {
		TimingBeforeExportData: {Severity: SeverityPotentialImpact,
			Impact: "The export doesn't include the column, which the target still has. `import data` fails only if that column is NOT NULL without a default on the target.",
			Action: "Drop the column on the target, or give it a default, before running `import data`. If `import data` already failed on it, run it again."},
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: "Rows already exported are not affected, but the target still has the column.",
			Action: "Drop the column on the target before the application moves to the target."},
		TimingLiveAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: "The target keeps the dropped column. `import data` is not affected unless that column is NOT NULL without a default on the target.",
			Action: "Drop the column on the target before cutover."},
	},
	schemadiff.ColumnNameChanged: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: "The export uses the new name, which the target doesn't have, so `import data` fails.",
			Action: "Rename the column on the target before running `import data`. If `import data` already failed on it, run it again."},
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: "Rows already exported are not affected, but the target still has the old name.",
			Action: "Rename the column on the target before the application moves to the target."},
		TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "`export data` stops at the first change written with the new name, and changes already queued under the old name can't be applied once the names differ.",
			Action: "Rename the column on the target, then restart the data migration: `export data --start-clean true`, then `import data --start-clean true --truncate-tables true`."},
		TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "`export data` stops at the first change written with the new name, and changes already queued under the old name can't be applied once the names differ.",
			Action: "Rename the column on the target, then start a new migration in a new export directory."},
	},
	schemadiff.ColumnTypeChanged: {
		TimingBeforeExportData: columnTypeChangedInfo,
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: "Rows already exported are not affected, but the target column still has the old type.",
			Action: "Change the column type on the target before the application moves to the target."},
		TimingLiveAfterExportStart: columnTypeChangedInfo,
	},
	schemadiff.ColumnNotNullDropped: {
		TimingBeforeExportData: notNullDroppedInfo,
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: "Rows already exported are not affected, but the target column still rejects NULLs.",
			Action: "Drop NOT NULL on the target before the application moves to the target."},
		TimingLiveAfterExportStart: notNullDroppedInfo,
	},
	schemadiff.ColumnNotNullSet: {
		TimingBeforeExportData:        notNullSetInfo,
		TimingOfflineAfterExportStart: notNullSetInfo,
		TimingLiveAfterExportStart:    notNullSetInfo,
	},
	schemadiff.ColumnDefaultChanged: {
		TimingBeforeExportData:        columnDefaultChangedInfo,
		TimingOfflineAfterExportStart: columnDefaultChangedInfo,
		TimingLiveAfterExportStart:    columnDefaultChangedInfo,
	},
	schemadiff.TableKindChanged: {
		TimingBeforeExportData:        tableKindChangedInfo,
		TimingOfflineAfterExportStart: tableKindChangedInfo,
		TimingLiveAfterExportStart:    tableKindChangedInfo,
	},
	schemadiff.TablePartitionAttached: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: "The export treats the table as a partition, which the target doesn't have, so `import data` fails.",
			Action: "Attach the partition on the target before running `import data`. If `import data` already failed on it, run it again."},
		TimingOfflineAfterExportStart: partitionAttachedAfterExportStartInfo,
		TimingLiveAfterExportStart:    partitionAttachedAfterExportStartInfo,
	},
	schemadiff.TablePartitionDetached: {
		TimingBeforeExportData:        partitionDetachedInfo,
		TimingOfflineAfterExportStart: partitionDetachedInfo,
		TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "Rows written to the detached table are still sent to the partitioned table on the target: rows inside the old range land there, and rows outside it make `import data` fail.",
			Action: "Detach the partition on the target, then restart the data migration: `export data --start-clean true`, then `import data --start-clean true --truncate-tables true`."},
		TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "Rows written to the detached table are still sent to the partitioned table on the target: rows inside the old range land there, and rows outside it make `import data` fail.",
			Action: "Detach the partition on the target, then start a new migration in a new export directory."},
	},
	schemadiff.TablePartitionChildrenChanged: {
		TimingBeforeExportData:        partitionChildrenChangedInfo,
		TimingOfflineAfterExportStart: partitionChildrenChangedInfo,
		TimingLiveAfterExportStart:    partitionChildrenChangedInfo,
	},
	schemadiff.TableInheritAdded: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: "`export data` does not support tables that use inheritance, so it does not start.",
			Action: "Exclude the table and its parent with `--exclude-table-list`, and migrate them separately."},
		TimingOfflineAfterExportStart: inheritanceAddedOfflineInfo,
		TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "`export data` keeps running, but a restart fails because the table now uses inheritance.",
			Action: "If `export data` has to restart, restart the data migration: `export data --start-clean true`, then `import data --start-clean true --truncate-tables true`, with the table and its parent in `--exclude-table-list`. Both then leave the migration, so migrate them separately."},
		TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "`export data` keeps running, but a restart fails because the table now uses inheritance.",
			Action: "If `export data` has to restart, start a new migration in a new export directory, with the table and its parent in `--exclude-table-list`. Both then leave the migration, so migrate them separately."},
	},
	schemadiff.TableInheritRemoved: {
		TimingBeforeExportData:        inheritanceRemovedInfo,
		TimingOfflineAfterExportStart: inheritanceRemovedInfo,
		TimingLiveAfterExportStart:    inheritanceRemovedInfo,
	},
	schemadiff.TableInheritedByAdded: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: "The table now has a child table, and `export data` does not support tables that use inheritance, so it does not start.",
			Action: "Exclude the table and its child tables with `--exclude-table-list`, and migrate them separately."},
		TimingOfflineAfterExportStart: inheritanceAddedOfflineInfo,
		TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "`export data` keeps running, but a restart fails because the table now has a child table.",
			Action: "If `export data` has to restart, restart the data migration: `export data --start-clean true`, then `import data --start-clean true --truncate-tables true`, with the table and its child tables in `--exclude-table-list`. They then leave the migration, so migrate them separately."},
		TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: "`export data` keeps running, but a restart fails because the table now has a child table.",
			Action: "If `export data` has to restart, start a new migration in a new export directory, with the table and its child tables in `--exclude-table-list`. They then leave the migration, so migrate them separately."},
	},
	schemadiff.TableInheritedByRemoved: {
		TimingBeforeExportData:        inheritanceRemovedInfo,
		TimingOfflineAfterExportStart: inheritanceRemovedInfo,
		TimingLiveAfterExportStart:    inheritanceRemovedInfo,
	},
	schemadiff.PrimaryKeyAdded: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: "In a live migration, `import data` refuses the table until the target has a primary key. An offline migration is not affected.",
			Action: "Add the same primary key on the target before running `import data`. If `import data` already failed on it, run it again."},
		TimingOfflineAfterExportStart: {Severity: SeverityAdvisory,
			Impact: "`import data` is not affected.",
			Action: "Add the same primary key on the target to keep the schemas the same."},
		TimingLiveAfterExportStart: {Severity: SeverityAdvisory,
			Impact: "The live migration is not affected. Either the table had no primary key, so it was not part of the live migration, or it is a partitioned table whose partitions already had keys, which now belong to this one. If `PRIMARY_KEY_DROPPED` is also reported for the table in the same interval, the key was replaced, and that entry's Action applies.",
			Action: "Add the same primary key on the target."},
	},
	schemadiff.PrimaryKeyDropped: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: "In a live migration, `export data` refuses a table without a primary key. In an offline migration, the target still has the key, so `import data` fails on the first duplicate key value.",
			Action: "In a live migration, add the key back on the source before starting `export data`. In an offline migration, drop it on the target too, and run `import data` again if it already failed."},
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: "Rows exported after the drop can repeat a key value. The target's key rejects them, so `import data` fails.",
			Action: "Drop the primary key on the target, then run `import data` again."},
		TimingLiveAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: primaryKeyDroppedLiveImpact,
			Action: "Add the key back on the source, then restart the data migration: `export data --start-clean true`, then `import data --start-clean true --truncate-tables true`."},
		TimingChangesOnlyAfterExportStart: {Severity: SeverityBreaksUnrecoverable,
			Impact: primaryKeyDroppedLiveImpact,
			Action: "Add the key back on the source, then start a new migration in a new export directory."},
	},
	schemadiff.PrimaryKeyNameChanged: {
		TimingBeforeExportData:        keyNameChangedInfo,
		TimingOfflineAfterExportStart: keyNameChangedInfo,
		TimingLiveAfterExportStart:    keyNameChangedInfo,
	},
	schemadiff.UniqueAdded: {
		TimingBeforeExportData:        uniqueAddedInfo,
		TimingOfflineAfterExportStart: uniqueAddedInfo,
		TimingLiveAfterExportStart:    uniqueAddedInfo,
	},
	schemadiff.UniqueDropped: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: uniqueDroppedImpact,
			Action: "Drop it on the target too, and run `import data` again if it already failed."},
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: uniqueDroppedImpact,
			Action: "Drop it on the target too, and run `import data` again if it already failed."},
		TimingLiveAfterExportStart: uniqueDroppedLiveInfo,
	},
	schemadiff.UniqueNameChanged: {
		TimingBeforeExportData:        keyNameChangedInfo,
		TimingOfflineAfterExportStart: keyNameChangedInfo,
		TimingLiveAfterExportStart:    keyNameChangedInfo,
	},
	schemadiff.CheckAdded: {
		TimingBeforeExportData:        checkAddedInfo,
		TimingOfflineAfterExportStart: checkAddedInfo,
		TimingLiveAfterExportStart:    checkAddedInfo,
	},
	schemadiff.CheckDropped: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: checkDroppedImpact,
			Action: "Drop it on the target too, and run `import data` again if it already failed."},
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: checkDroppedImpact,
			Action: "Drop it on the target too, and run `import data` again if it already failed."},
		TimingLiveAfterExportStart: {Severity: SeverityBreaksRecoverable,
			Impact: "The source now allows rows the target's check rejects, so `import data` stops on the first one.",
			Action: "Drop it on the target, then run `import data` again."},
	},
	schemadiff.CheckNameChanged: {
		TimingBeforeExportData:        keyNameChangedInfo,
		TimingOfflineAfterExportStart: keyNameChangedInfo,
		TimingLiveAfterExportStart:    keyNameChangedInfo,
	},
	schemadiff.ForeignKeyAdded: {
		TimingBeforeExportData:        foreignKeyAddedInfo,
		TimingOfflineAfterExportStart: foreignKeyAddedInfo,
		TimingLiveAfterExportStart:    foreignKeyAddedInfo,
	},
	schemadiff.ForeignKeyDropped: {
		TimingBeforeExportData:        foreignKeyDroppedInfo,
		TimingOfflineAfterExportStart: foreignKeyDroppedInfo,
		TimingLiveAfterExportStart:    foreignKeyDroppedInfo,
	},
	schemadiff.ForeignKeyNameChanged: {
		TimingBeforeExportData:        keyNameChangedInfo,
		TimingOfflineAfterExportStart: keyNameChangedInfo,
		TimingLiveAfterExportStart:    keyNameChangedInfo,
	},
	schemadiff.UniqueIndexAdded: {
		TimingBeforeExportData:        uniqueAddedInfo,
		TimingOfflineAfterExportStart: uniqueAddedInfo,
		TimingLiveAfterExportStart:    uniqueIndexAddedLiveInfo,
	},
	schemadiff.UniqueIndexDropped: {
		TimingBeforeExportData: {Severity: SeverityBreaksRecoverable,
			Impact: uniqueDroppedImpact,
			Action: "Drop it on the target too, and run `import data` again if it already failed."},
		TimingOfflineAfterExportStart: {Severity: SeverityPotentialImpact,
			Impact: uniqueDroppedImpact,
			Action: "Drop it on the target too, and run `import data` again if it already failed."},
		TimingLiveAfterExportStart: uniqueDroppedLiveInfo,
	},
	schemadiff.UniqueIndexNameChanged: {
		TimingBeforeExportData:        keyNameChangedInfo,
		TimingOfflineAfterExportStart: keyNameChangedInfo,
		TimingLiveAfterExportStart:    keyNameChangedInfo,
	},
	schemadiff.IndexAdded: {
		TimingBeforeExportData:        indexChangedInfo,
		TimingOfflineAfterExportStart: indexChangedInfo,
		TimingLiveAfterExportStart:    indexChangedInfo,
	},
	schemadiff.IndexDropped: {
		TimingBeforeExportData:        indexChangedInfo,
		TimingOfflineAfterExportStart: indexChangedInfo,
		TimingLiveAfterExportStart:    indexChangedInfo,
	},
	schemadiff.IndexNameChanged: {
		TimingBeforeExportData:        keyNameChangedInfo,
		TimingOfflineAfterExportStart: keyNameChangedInfo,
		TimingLiveAfterExportStart:    keyNameChangedInfo,
	},
}

// Appended to Impact while a cutover with fall-back is still possible.
var fallBackRiskByDiffType = map[schemadiff.DiffType]string{
	schemadiff.ColumnDropped:    "If you cut over with fall-back, `import data to source` stops at the first insert on this table while the target still has this column, and later changes don't reach the source.",
	schemadiff.UniqueAdded:      uniqueAddedFallBackRisk,
	schemadiff.UniqueIndexAdded: uniqueAddedFallBackRisk,
	schemadiff.CheckAdded:       "If you cut over with fall-back while the target doesn't have this check, `import data to source` stops at the first row from the target that breaks it.",
	schemadiff.ForeignKeyAdded:  "If you cut over with fall-back while the target doesn't have this key, rows from the target that break it still load into the source: `import data to source` runs with foreign-key checks off, or refuses to start until they are turned off. The source's key then no longer holds for those rows.",
}

const fallForwardAction = "Fall-forward is in use: make on the source-replica any change this asks you to make on the target, and when it restarts `import data`, restart `import data to source-replica` the same way."

// Appended to Action while fall-forward is in use and cutover has not happened.
// TABLE_PARTITION_CHILDREN_CHANGED has none: its Action points at each partition's own
// entry, which carries the sentence.
var fallForwardActionByDiffType = fallForwardActions()

func fallForwardActions() map[schemadiff.DiffType]string {
	actions := make(map[schemadiff.DiffType]string)
	for _, t := range schemadiff.AllDiffTypes() {
		if t != schemadiff.TablePartitionChildrenChanged {
			actions[t] = fallForwardAction
		}
	}
	return actions
}

// Shared by several timings or change types.
var (
	tableRenamedOrMovedBeforeExportInfo = DriftInfo{Severity: SeverityBreaksRecoverable,
		Impact: "If the table is in the export's scope, it is exported under its new name, which the target doesn't have, so `import data` fails.",
		Action: "Rename (or move) the table on the target before running `import data`. If `import data` already failed on it, run it again."}

	tableRenamedOrMovedOfflineInfo = DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "Rows already exported are not affected, but the target still has the old name.",
		Action: "Rename (or move) the table on the target before the application moves to the target."}

	tableRenamedOrMovedLiveInfo = DriftInfo{Severity: SeverityBreaksUnrecoverable,
		Impact: "Rows written under the new name are not migrated, and a restart of `export data` fails.",
		Action: "Rename (or move) the table on the target, then restart the data migration: `export data --start-clean true`, then `import data --start-clean true --truncate-tables true`."}

	tableRenamedOrMovedChangesOnlyInfo = DriftInfo{Severity: SeverityBreaksUnrecoverable,
		Impact: "Rows written under the new name are not migrated, and a restart of `export data` fails.",
		Action: "Rename (or move) the table on the target, then start a new migration in a new export directory."}

	columnTypeChangedInfo = DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "`import data` fails on the first value the target's column type can't hold.",
		Action: "Change the column type on the target. If `import data` already failed, run it again."}

	notNullDroppedInfo = DriftInfo{Severity: SeverityBreaksRecoverable,
		Impact: "The source now allows NULLs, so `import data` fails on the first NULL in this column.",
		Action: "Drop NOT NULL on the target. If `import data` already failed, run it again."}

	notNullSetInfo = DriftInfo{Severity: SeverityAdvisory,
		Impact: "`import data` is not affected.",
		Action: "Set NOT NULL on the target to keep the schemas the same."}

	columnDefaultChangedInfo = DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "`import data` is not affected, but once the application uses the target, its inserts get the old default.",
		Action: "Change the default on the target before the application moves to the target."}

	tableKindChangedInfo = DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "The migration is not affected mid-flight, but the target still has the table in its original form (ordinary vs partitioned vs foreign), so the schemas diverge and a restart may not reproduce the source layout.",
		Action: "Recreate the table on the target with the matching kind before cutover."}

	partitionAttachedAfterExportStartInfo = DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "The target's partition layout no longer matches the source.",
		Action: "Attach the partition on the target before the application moves to the target."}

	partitionDetachedInfo = DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "The target's partition layout no longer matches the source.",
		Action: "Detach the partition on the target before the application moves to the target."}

	partitionChildrenChangedInfo = DriftInfo{Severity: SeverityAdvisory,
		Impact: "The partitions of this table changed.",
		Action: "See the entry for each partition."}

	inheritanceAddedOfflineInfo = DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "Rows already exported are not affected, but the target's inheritance no longer matches.",
		Action: "Apply the same inheritance on the target before the application moves to the target."}

	inheritanceRemovedInfo = DriftInfo{Severity: SeverityAdvisory,
		Impact: "`export data` and `import data` are not affected.",
		Action: "Remove the same inheritance on the target before the application moves to the target."}

	primaryKeyDroppedLiveImpact = "Every later change to this table reaches `import data` with an empty key, so it stops, and it fails again on every restart. A restart of `export data` refuses the table. This is also reported, with `PRIMARY_KEY_ADDED`, when the key was changed or re-created: if it was changed in one transaction, changing it on the target and running `import data` again recovers."

	uniqueAddedInfo = DriftInfo{Severity: SeverityAdvisory,
		Impact: "`import data` is not affected.",
		Action: "Add it on the target to keep the schemas the same. If `import data` is running, restart it afterwards: it reads the target's unique keys only when it starts, so with the new one missing, the first write that frees and reuses a value stops it."}

	// An expression key has no columns for import data's conflict check, and the tables
	// that need in-order apply for one are fixed on import data's first run.
	uniqueIndexAddedLiveInfo = DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "`import data` is not affected while the target doesn't have the index. If the index is on an expression, such as `lower(email)`, adding it on the target before cutover can stop `import data` on a unique violation: `import data` can't keep changes to such an index in order, and restarting it does not prevent another stop.",
		Action: "Add it on the target to keep the schemas the same. If it is on plain columns, add it now, and if `import data` is running, restart it afterwards: it reads the target's unique keys only when it starts, so with the new one missing, the first write that frees and reuses a value stops it. If it is on an expression, add it only after cutover, once `import data` has applied every change and before the application moves to the target. If you already added it and `import data` stopped, drop it on the target, run `import data` again, and add it back after cutover."}

	uniqueDroppedImpact = "The source now allows duplicate values, but the target still enforces uniqueness, so `import data` fails on the first duplicate. This is also reported, with an added unique key, unique index or primary key on the same columns, when it was rebuilt (`REINDEX CONCURRENTLY`, a column type change) or turned into a constraint (`ADD CONSTRAINT ... USING INDEX`). The source then still enforces uniqueness, so keep it on the target."

	uniqueDroppedLiveInfo = DriftInfo{Severity: SeverityBreaksRecoverable,
		Impact: "The source now allows duplicate values, but the target still enforces uniqueness, so `import data` stops at the first duplicate value. This is also reported, with an added unique key, unique index or primary key on the same columns, when it was rebuilt (`REINDEX CONCURRENTLY`, a column type change) or turned into a constraint (`ADD CONSTRAINT ... USING INDEX`). The source then still enforces uniqueness, so keep it on the target.",
		Action: "Drop it on the target, then run `import data` again."}

	uniqueAddedFallBackRisk = "If you cut over with fall-back while the target doesn't have it, `import data to source` stops at the first row from the target that breaks it."

	checkAddedInfo = DriftInfo{Severity: SeverityAdvisory,
		Impact: "`import data` is not affected.",
		Action: "Add it on the target to keep the schemas the same."}

	checkDroppedImpact = "The source now allows rows the target's check rejects, so `import data` fails on the first one."

	indexChangedInfo = DriftInfo{Severity: SeverityAdvisory,
		Impact: "Only query speed is affected. `export data` and `import data` are not.",
		Action: "Make the same change on the target."}

	foreignKeyAddedInfo = DriftInfo{Severity: SeverityAdvisory,
		Impact: "`import data` is not affected, because the target does not have the key.",
		Action: "Add it on the target to keep the schemas the same."}

	foreignKeyDroppedInfo = DriftInfo{Severity: SeverityPotentialImpact,
		Impact: "Rows the source now allows load anyway, so the target's foreign key no longer holds for them. If the target user can't turn foreign-key checks off, the target still enforces the key: `import data` stops on the first row that breaks it, and an `ON DELETE` or `ON UPDATE` action such as `CASCADE` removes or changes rows the source keeps.",
		Action: "Drop it on the target (or change it to match), then run `import data` again if it stopped. Restore from the source any rows the key's action removed or changed on the target, because running `import data` again does not bring them back."}

	keyNameChangedInfo = DriftInfo{Severity: SeverityAdvisory,
		Impact: "`export data` and `import data` are not affected.",
		Action: "Rename it on the target to keep the names the same."}
)

var afterCutoverInfo = DriftInfo{
	Severity: SeverityAdvisory,
	Impact:   "This change was made after cutover to target. Data already migrated to the target is not affected.",
}
