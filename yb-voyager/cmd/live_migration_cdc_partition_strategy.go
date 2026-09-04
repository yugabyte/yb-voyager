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
package cmd

import (
	"fmt"
	"slices"
	"sort"
	"strings"

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

/*
prepareCdcPartitionKey validates overrides against the import table list, resolves
per-table strategies (global --cdc-partition-key + --cdc-partition-key-overrides +
auto/expr-UK/generated-stored-column rules), and persists TableToCDCPartitionKey.

It is intentionally called before snapshot import so bad configs fail fast.
On resume (map already in metaDB) this is a no-op.
*/
func prepareCdcPartitionKey(tableNames []sqlname.NameTuple, tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]) error {
	if importerRole != TARGET_DB_IMPORTER_ROLE || !changeStreamingIsEnabled(importType) || sourceDBType != POSTGRESQL {
		return nil
	}

	importDataStatus, err := metaDB.GetImportDataStatusRecord()
	if err != nil {
		return fmt.Errorf("error getting import data status record for cdc-partition-key: %w", err)
	}
	if importDataStatus == nil {
		return goerrors.Errorf("import data status record not found")
	}
	if importDataStatus.TableToCDCPartitionKey != nil {
		// On resume the per-table strategy map is already persisted. Rather than trusting
		// a raw string comparison of the flags (done for the global key in
		// validateCdcPartitionKeyFlags), re-resolve the effective per-table strategy from
		// the current flags — reusing the expression-UK and generated-stored-column tables
		// captured on the first run so no target-DB re-query is needed — and reject if it
		// differs from what was persisted. This catches semantically-different overrides
		// that a plain string compare would miss (ordering, spelling/quoting, whitespace).
		if err := validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus, tableToUniqueIndexes); err != nil {
			return err
		}
		log.Infof("cdc partition key already prepared in metadb and unchanged; skipping recompute")
		return nil
	}

	_, err = computeAndPersistCdcPartitioningStrategyPerTable(tableNames, importDataStatus, tableToUniqueIndexes)
	return err
}

/*
getCdcPartitioningStrategyPerTable loads the per-table CDC partition strategy for streaming.

For target PG→YB live import the map is prepared before snapshot via prepareCdcPartitionKey.
Non-target and Oracle source paths force PARTITION_BY_TABLE in-memory (not persisted).

TODO: handle upgrade scenario for PG/Oracle pk->table change
*/
func getCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], error) {
	tablePartitionKeyMap := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()

	importDataStatus, err := metaDB.GetImportDataStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("error getting cdc partitioning strategy: %w", err)
	}
	if importDataStatus == nil {
		return nil, goerrors.Errorf("import data status record not found")
	}
	if importDataStatus.TableToCDCPartitionKey == nil {
		return nil, goerrors.Errorf("cdc partitioning strategy per table not found in metadb")
	}

	log.Infof("cdc partition key found in metadb: %v, value: %v", metadb.IMPORT_DATA_STATUS_KEY, importDataStatus.TableToCDCPartitionKey)
	for tableName, partitionKey := range importDataStatus.TableToCDCPartitionKey {
		tuple, err := namereg.NameReg.LookupTableName(tableName)
		if err != nil {
			return nil, fmt.Errorf("error looking up table name: %w", err)
		}
		tablePartitionKeyMap.Put(tuple, cdcPartitionKeyOverride{Strategy: partitionKey.Strategy, Columns: partitionKey.Columns})
	}
	for _, t := range tableNames {
		partitionKey, ok := tablePartitionKeyMap.Get(t)
		if !ok {
			return nil, goerrors.Errorf("cdc partitioning strategy not found for table: %s", t.ForKey())
		}
		if partitionKey.Strategy == PARTITION_BY_CUSTOM && len(partitionKey.Columns) == 0 {
			return nil, goerrors.Errorf("cdc custom partition key columns not found for table: %s", t.ForKey())
		}
	}
	return tablePartitionKeyMap, nil
}

func shouldForceTablePartitioning(importerRole string, sourceDBType string, targetDBType string) bool {
	if importerRole != TARGET_DB_IMPORTER_ROLE {
		//For PG/ORacle source/source-replica, using partitioning by table since there won't be any huge difference in
		// performance between the two strategies for single node databases like PG/Oracle
		//and Parititon by table is better from data correctness perspective
		return true
	}
	if sourceDBType != POSTGRESQL {
		//For Oracle source and target db impoorter, using partitioning by table since we don't have complete support for the conflict detection
		//and Parititon by table is better from data correctness perspective
		return true
	}
	if targetDBType == YUGABYTEDB_AMP {
		// yb-amp is a single-node PostgreSQL-compatible compute (no YB
		// tablets / colocation), so — exactly like the PG/Oracle
		// source/source-replica case — PARTITION_BY_TABLE has no real
		// throughput downside and is safer for correctness. It also sidesteps
		// getExpressionUniqueIndexTables(), which is implemented only on the
		// TargetYugabyteDB driver.
		return true
	}
	return false
}

// resolveEffectiveCdcPartitionKeys applies global strategy, then per-table overlays,
func resolveEffectiveCdcPartitionKeys(
	tableNames []sqlname.NameTuple,
	globalKey string,
	overrides *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride],
	exprUKSet *utils.StructMap[sqlname.NameTuple, bool],
	generatedStoredCols *utils.StructMap[sqlname.NameTuple, []GeneratedStoredColumn],
	targetDBType string,
) (*utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], error) {
	result := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	if overrides == nil {
		overrides = utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	}
	if exprUKSet == nil {
		exprUKSet = utils.NewStructMap[sqlname.NameTuple, bool]()
	}
	if generatedStoredCols == nil {
		generatedStoredCols = utils.NewStructMap[sqlname.NameTuple, []GeneratedStoredColumn]()
	}

	switch globalKey {
	case "auto":
		if shouldForceTablePartitioning(importerRole, sourceDBType, targetDBType) {
			for _, t := range tableNames {
				result.Put(t, cdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
			}
		} else {
			for _, t := range tableNames {
				if ok, _, _ := shouldForceTablePartitioningForTable(t, exprUKSet, generatedStoredCols); ok {
					result.Put(t, cdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
				} else {
					result.Put(t, cdcPartitionKeyOverride{Strategy: PARTITION_BY_PK})
				}
			}
		}
	default:
		for _, t := range tableNames {
			result.Put(t, cdcPartitionKeyOverride{Strategy: globalKey})
		}
	}

	err := overrides.IterKV(func(t sqlname.NameTuple, override cdcPartitionKeyOverride) (bool, error) {
		result.Put(t, override)
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error applying cdc-partition-key-overrides: %w", err)
	}
	return result, nil
}

func tableHasUniqueIndexOnGenerated(generatedStoredCols *utils.StructMap[sqlname.NameTuple, []GeneratedStoredColumn], t sqlname.NameTuple) bool {
	cols, ok := generatedStoredCols.Get(t)
	if !ok {
		return false
	}
	for _, col := range cols {
		if col.InUniqueIndex {
			return true
		}
	}
	return false
}

// Generated cols and override Columns are Unquoted cased names with proper casing
func tableHasGeneratedColumn(generatedStoredCols *utils.StructMap[sqlname.NameTuple, []GeneratedStoredColumn], t sqlname.NameTuple, overrideColumns []string) ([]string, bool) {
	cols, ok := generatedStoredCols.Get(t)
	if !ok {
		return nil, false
	}
	generatedCustomCols := make([]string, 0)
	for _, col := range overrideColumns {
		if lo.ContainsBy(cols, func(c GeneratedStoredColumn) bool {
			return c.Name == col
		}) {
			generatedCustomCols = append(generatedCustomCols, col)
		}
	}
	return generatedCustomCols, len(generatedCustomCols) > 0
}

// resolveCdcPartitionKeyOverrides looks up override table names in namereg and
// validates each is present in importTableList. Returns a map keyed by NameTuple.
func resolveCdcPartitionKeyOverrides(rawOverrides map[string]cdcPartitionKeyOverride, importTableList []sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], error) {
	resolved := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	if len(rawOverrides) == 0 {
		return resolved, nil
	}

	importTableSet := utils.NewStructMap[sqlname.NameTuple, bool]()
	for _, t := range importTableList {
		importTableSet.Put(t, true)
	}

	for tableSpec, override := range rawOverrides {
		tuple, err := namereg.NameReg.LookupTableName(tableSpec)
		if err != nil {
			return nil, goerrors.Errorf("cdc-partition-key-overrides: table %q not found in name registry: %w", tableSpec, err)
		}
		if _, ok := importTableSet.Get(tuple); !ok {
			return nil, goerrors.Errorf("cdc-partition-key-overrides: table %q is not in the import table list", tableSpec)
		}
		// Detect duplicates on the resolved NameTuple so different spellings of the
		// same table (casing/quoting/schema-qualification) don't silently overwrite.
		if _, ok := resolved.Get(tuple); ok {
			return nil, goerrors.Errorf("cdc-partition-key-overrides: table %q (resolved to %s) specified multiple times",
				tableSpec, tuple.ForOutput())
		}
		resolved.Put(tuple, override)
	}
	return resolved, nil
}

func needsNonTablePartitionKeyChecks(cdcPartitionKey string, overrides *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) (bool, error) {
	// Collect expression-UK / generated-stored-column tables whenever we need them for
	// auto resolution or pk/custom validation.
	if cdcPartitionKey != PARTITION_BY_TABLE {
		return true, nil
	}
	var validationsNeeded bool
	err := overrides.IterKV(func(_ sqlname.NameTuple, override cdcPartitionKeyOverride) (bool, error) {
		if override.Strategy != PARTITION_BY_TABLE {
			validationsNeeded = true
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return false, err
	}
	return validationsNeeded, nil
}

func shouldForceTablePartitioningForTable(table sqlname.NameTuple, exprUKSet *utils.StructMap[sqlname.NameTuple, bool], generatedStoredCols *utils.StructMap[sqlname.NameTuple, []GeneratedStoredColumn]) (bool, bool, bool) {
	if _, ok := exprUKSet.Get(table); ok {
		return true, true, false
	}
	if tableHasUniqueIndexOnGenerated(generatedStoredCols, table) {
		return true, false, true
	}
	return false, false, false
}

// computeAndPersistCdcPartitioningStrategyPerTable resolves global + overrides +
// auto/expr-UK/generated-stored-column rules and writes TableToCDCPartitionKey to metaDB.
func computeAndPersistCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple, importDataStatus *metadb.ImportDataStatusRecord, tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]) (*utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], error) {
	tableToPartitionKeyOverrideMap, exprUKKeysForStorage, err := computeCdcPartitioningStrategyPerTable(tableNames, true, importDataStatus, tableToUniqueIndexes)
	if err != nil {
		return nil, fmt.Errorf("error computing cdc partitioning strategy per table: %w", err)
	}

	// Combine strategy + custom key columns into the single persisted per-table map.
	metadbMap := make(map[string]metadb.CDCPartitionKey)
	err = tableToPartitionKeyOverrideMap.IterKV(func(key sqlname.NameTuple, override cdcPartitionKeyOverride) (bool, error) {
		metadbMap[key.ForKey()] = metadb.CDCPartitionKey{Strategy: override.Strategy, Columns: override.Columns}
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error building cdc partition key map: %w", err)
	}

	err = metaDB.UpdateImportDataStatusRecord(func(obj *metadb.ImportDataStatusRecord) {
		obj.TableToCDCPartitionKey = metadbMap
		obj.CdcExpressionUniqueIndexTables = exprUKKeysForStorage
	})
	if err != nil {
		return nil, fmt.Errorf("error updating cdc partition key in metadb: %w", err)
	}
	log.Infof("updated cdc partition key in metadb: %v with values: %v", metadb.IMPORT_DATA_STATUS_KEY, metadbMap)
	return tableToPartitionKeyOverrideMap, nil
}

func computeCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple, isFirstRun bool, importDataStatus *metadb.ImportDataStatusRecord, tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]) (*utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], []string, error) {
	rawOverrides, err := parseCdcPartitionKeyOverrides(cdcPartitionKeyOverrides)
	if err != nil {
		return nil, nil, err
	}
	overrides, err := resolveCdcPartitionKeyOverrides(rawOverrides, tableNames)
	if err != nil {
		return nil, nil, err
	}

	exprUKSet, err := getExpressionUniqueIndexTablesIfRequired(tableNames, overrides, cdcPartitionKey, isFirstRun, importDataStatus)
	if err != nil {
		return nil, nil, err
	}
	// Generated stored columns are recomputed from the source-captured metaDB record on every
	// run (not persisted in the import status record), so this is independent of isFirstRun.
	generatedStoredCols, err := getGeneratedStoredColumnsIfRequired(tableNames, overrides, cdcPartitionKey, tableToUniqueIndexes)
	if err != nil {
		return nil, nil, err
	}
	tableToPartitionKeyOverrideMap, err := resolveEffectiveCdcPartitionKeys(tableNames, cdcPartitionKey, overrides, exprUKSet, generatedStoredCols, tconf.TargetDBType)

	if err != nil {
		return nil, nil, err
	}

	err = validateAndFinalizeCDCPartitionKeysPerTable(tableToPartitionKeyOverrideMap, tableNames, exprUKSet, generatedStoredCols)
	if err != nil {
		return nil, nil, err
	}

	exprUKKeysForStorage := make([]string, 0, len(exprUKSet.Keys()))
	exprUKKeysForStorage = append(exprUKKeysForStorage, exprUKSet.Keys()...)
	return tableToPartitionKeyOverrideMap, exprUKKeysForStorage, nil
}

func getExpressionUniqueIndexTablesIfRequired(tableNames []sqlname.NameTuple, overrides *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], cdcPartitionKey string, isFirstRun bool, importDataStatus *metadb.ImportDataStatusRecord) (*utils.StructMap[sqlname.NameTuple, bool], error) {
	// Always return a non-nil map: callers call exprUKSet.Keys()/pass it to
	// resolveEffectiveCdcPartitionKeys, and a nil *StructMap panics on Keys().
	exprUKSet := utils.NewStructMap[sqlname.NameTuple, bool]()

	needsExprUKCheck, err := needsNonTablePartitionKeyChecks(cdcPartitionKey, overrides)
	if err != nil {
		return nil, err
	}
	if !needsExprUKCheck {
		return exprUKSet, nil
	}

	var expressionUniqueIndexTables []sqlname.NameTuple
	if isFirstRun {
		// First run: query the target DB for the authoritative set of expression-UK tables.
		expressionUniqueIndexTables, err = getExpressionUniqueIndexTables(tableNames)
		if err != nil {
			return nil, fmt.Errorf("error getting expression unique index tables: %w", err)
		}
	} else {
		// Resume: reuse the set captured on the first run (persisted in metaDB) so we do not
		// re-query the target DB.
		if importDataStatus == nil {
			return nil, goerrors.Errorf("import data status record not found")
		}
		if importDataStatus.CdcExpressionUniqueIndexTables == nil {
			return nil, goerrors.Errorf("cdc expression unique index tables not found in import data status record")
		}
		for _, key := range importDataStatus.CdcExpressionUniqueIndexTables {
			tuple, err := namereg.NameReg.LookupTableName(key)
			if err != nil {
				return nil, fmt.Errorf("error looking up expression-unique-index table %q: %w", key, err)
			}
			expressionUniqueIndexTables = append(expressionUniqueIndexTables, tuple)
		}
	}

	for _, t := range expressionUniqueIndexTables {
		exprUKSet.Put(t, true)
	}
	return exprUKSet, nil
}

// getGeneratedStoredColumnsIfRequired resolves the per-table STORED generated columns needed
// for the CDC partitioning decision. It is intentionally NOT persisted in the import status
// record: it is recomputed from the source-captured metaDB record (+ target UK/PK) on every
// run (first run and resume alike) via getGeneratedStoredColumnsHybrid.
func getGeneratedStoredColumnsIfRequired(tableNames []sqlname.NameTuple, overrides *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], cdcPartitionKey string, tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]) (*utils.StructMap[sqlname.NameTuple, []GeneratedStoredColumn], error) {
	needsCheck, err := needsNonTablePartitionKeyChecks(cdcPartitionKey, overrides)
	if err != nil {
		return nil, err
	}
	if !needsCheck {
		return utils.NewStructMap[sqlname.NameTuple, []GeneratedStoredColumn](), nil
	}
	return getGeneratedStoredColumnsHybrid(tableNames, tableToUniqueIndexes)
}

// validateCdcPartitioningStrategyUnchanged re-resolves the effective per-table CDC
// partitioning strategy from the current flags and fails if it differs from the map
// persisted on the first run. It reuses the expression-UK and generated-stored-column
// tables captured on the first run (treating a missing generated-stored set as empty for
// older records) so resume never silently applies a changed cdc-partition-key /
// cdc-partition-key-overrides.
func validateCdcPartitioningStrategyUnchanged(tableNames []sqlname.NameTuple, importDataStatus *metadb.ImportDataStatusRecord, tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]) error {

	cdcPartitionKeyOverridesStored := importDataStatus.CdcPartitionKeyOverridesConfig
	if cdcPartitionKeyOverridesStored == cdcPartitionKeyOverrides {
		//No changes in the overrides so we can continue
		return nil
	}

	resolvedTableToPartitionKeyOverrideMap, _, err := computeCdcPartitioningStrategyPerTable(tableNames, false, importDataStatus, tableToUniqueIndexes)
	if err != nil {
		return fmt.Errorf("error computing cdc partitioning strategy per table: %w", err)
	}
	return diffCdcPartitioningStrategy(resolvedTableToPartitionKeyOverrideMap, importDataStatus.TableToCDCPartitionKey)
}

// diffCdcPartitioningStrategy compares a freshly-resolved per-table strategy map against
// the one persisted on the first run and returns an actionable error listing the tables
// whose effective strategy changed.
//
// For PARTITION_BY_CUSTOM tables the strategy string ("custom") stays the same even when
// the routing columns change, so the custom key column lists are compared explicitly.
// Column order is significant because hashEvent hashes the values in that order, so a
// reordering is treated as a change. resolvedCustomColumns are the resolved custom key
// columns; stored is the persisted per-table CDC partition key (strategy + columns).
func diffCdcPartitioningStrategy(
	resolved *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride],
	stored map[string]metadb.CDCPartitionKey,
) error {
	var changed []string
	var missingInStored []string
	err := resolved.IterKV(func(t sqlname.NameTuple, override cdcPartitionKeyOverride) (bool, error) {
		storedPartitionKey, ok := stored[t.ForKey()]
		if !ok {
			missingInStored = append(missingInStored, t.ForKey())
		} else if storedPartitionKey.Strategy != override.Strategy {
			changed = append(changed, fmt.Sprintf("%s (persisted: %q, new: %q)", t.ForOutput(), storedPartitionKey.Strategy, override.Strategy))
			return true, nil
		}
		// Strategy is unchanged. For a custom key the strategy string alone cannot capture
		// a change in the routing columns, so compare the column lists (order-sensitive).
		if override.Strategy == PARTITION_BY_CUSTOM {
			var newColumns []string
			if override.Columns != nil {
				newColumns = override.Columns
			}
			oldColumns := storedPartitionKey.Columns
			if !slices.Equal(oldColumns, newColumns) {
				changed = append(changed, fmt.Sprintf("%s (persisted custom key columns: %v, new: %v)", t.ForOutput(), oldColumns, newColumns))
			}
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	var missingInResolved []string
	storedTables := lo.Keys(stored)
	for _, storedTable := range storedTables {
		tuple, err := namereg.NameReg.LookupTableName(storedTable)
		if err != nil {
			return fmt.Errorf("error looking up table name: %w", err)
		}
		if _, ok := resolved.Get(tuple); !ok {
			missingInResolved = append(missingInResolved, storedTable)
		}
	}
	errorMsg := ""
	if len(missingInStored) > 0 {
		sort.Strings(missingInStored)
		errorMsg += fmt.Sprintf("cdc-partition-key-overrides: table %q is in the current import table list but was not part of the original import; use --start-clean to start a fresh import with the new configuration\n", strings.Join(missingInStored, ", "))
	}
	if len(missingInResolved) > 0 {
		sort.Strings(missingInResolved)
		errorMsg += fmt.Sprintf("cdc-partition-key-overrides: table %q was part of the original import but is missing from the current import table list; use --start-clean to start a fresh import with the new configuration\n", strings.Join(missingInResolved, ", "))
	}
	if len(changed) > 0 {
		sort.Strings(changed)
		errorMsg += fmt.Sprintf("changing cdc-partition-key / cdc-partition-key-overrides is not allowed after the import data has started; effective strategy changed for: %s.\nUse --start-clean to start a fresh import with the new configuration.", strings.Join(changed, "; "))
	}
	if errorMsg != "" {
		return goerrors.Errorf("change in cdc-partition-key-overrides in between runs detected: %s", errorMsg)
	}
	return nil
}

func getExpressionUniqueIndexTables(tableNames []sqlname.NameTuple) ([]sqlname.NameTuple, error) {
	if tconf.TargetDBType != YUGABYTEDB {
		return nil, nil
	}
	yb, ok := tdb.(*tgtdb.TargetYugabyteDB)
	if !ok {
		return nil, goerrors.Errorf("target db is not a YugabyteDB")
	}

	//returns a list of catalog table names, in case partitions it return catalog leaf partitions names and root table names
	expressionUniqueIndexTables, err := yb.GetTablesHavingExpressionUniqueIndexes(tableNames, true)
	if err != nil {
		return nil, fmt.Errorf("error getting tables having expression or normal unique indexes: %w", err)
	}

	return expressionUniqueIndexTables, nil
}

// getGeneratedStoredColumnsHybrid resolves per-table STORED generated columns for the CDC
// partitioning decision using the hybrid model:
//   - which columns are generated: authoritative SOURCE facts captured at export
//     (metaDB export_data_source_db_exporter_status; generated column values are absent from
//     the change events), and
//   - which of those columns participate in a unique index (so a collision would throw 23505
//     under pk/custom routing): the TARGET catalog (the same UK set conflict detection uses,
//     GetTableToUniqueIndexesMap).
//
// InUniqueIndex is set when a source-generated column is in that target unique-index set.
//
// NOTE: the primary key is intentionally NOT considered here. A primary key that includes a
// STORED generated column is a broader, unsupported case for live migration (the event's
// routing key itself is missing the generated value), which must be handled separately - not
// papered over by forcing PARTITION_BY_TABLE. See the caveat documented at the capture site
// (captureSourceGeneratedStoredColumns in exportData.go).
//
// If the source capture is absent (export written by an older voyager, or capture not yet
// persisted).
func getGeneratedStoredColumnsHybrid(tableNames []sqlname.NameTuple, tableToUniqueIndexes *utils.StructMap[sqlname.NameTuple, []tgtdb.UniqueIndex]) (*utils.StructMap[sqlname.NameTuple, []GeneratedStoredColumn], error) {
	result := utils.NewStructMap[sqlname.NameTuple, []GeneratedStoredColumn]()

	exportStatus, err := metaDB.GetExportDataSourceDBExporterStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("error getting export data source db exporter status record: %w", err)
	}
	if exportStatus == nil {
		// Export dir written by a voyager that predates source capture: we cannot know the
		// generated columns. Resolve to empty (no forced table) — or fail/force-table if
		// under-protection is unacceptable.
		log.Warnf("source generated stored columns not captured at export (older voyager export dir); generated-column guardrails are skipped")
		return result, nil
	}

	// Restrict to the tables that actually have source generated columns. When none do
	// (the common case), skip the target UK/PK queries entirely.
	tablesWithGeneratedCols := lo.Filter(tableNames, func(t sqlname.NameTuple, _ int) bool {
		return len(exportStatus.TableToGeneratedStoredColumns[t.ForKey()]) > 0
	})
	if len(tablesWithGeneratedCols) == 0 {
		return result, nil
	}

	for _, t := range tablesWithGeneratedCols {
		sourceGenCols := exportStatus.TableToGeneratedStoredColumns[t.ForKey()]

		// Columns whose collision matters for routing/conflict detection: the target's
		// unique-index columns (the primary key is excluded from this set and is handled
		// separately - see the function-level NOTE and captureSourceGeneratedStoredColumns).
		hasUniqueIndexOnTarget := make(map[string]bool)
		if uniqueIndexes, ok := tableToUniqueIndexes.Get(t); ok {
			for _, idx := range uniqueIndexes {
				for _, col := range idx.Columns {
					hasUniqueIndexOnTarget[col] = true
				}
			}
		}

		cols := buildGeneratedStoredColumns(sourceGenCols, hasUniqueIndexOnTarget)
		result.Put(t, cols)
		log.Infof("table %s: source generated columns %v resolved to %v (target unique-index protected set)", t.ForOutput(), sourceGenCols, cols)
	}
	return result, nil
}

// buildGeneratedStoredColumns marks each source-generated column with whether it is
// "protected" on the target (a member of hasUniqueIndexOnTarget). A protected generated
// column forces PARTITION_BY_TABLE; a non-protected one is still tracked so a custom key
// naming it can be rejected.
func buildGeneratedStoredColumns(sourceGenCols []string, hasUniqueIndexOnTarget map[string]bool) []GeneratedStoredColumn {
	cols := make([]GeneratedStoredColumn, 0, len(sourceGenCols))
	for _, name := range sourceGenCols {
		cols = append(cols, GeneratedStoredColumn{
			Name:          name,
			InUniqueIndex: hasUniqueIndexOnTarget[name],
		})
	}
	return cols
}

type GeneratedStoredColumn struct {
	Name          string
	InUniqueIndex bool
}

// validateAndFinalizeCDCPartitionKeysPerTable runs the import-start guardrails for tables routed by a
// custom partition key (see cdc_partition_key_followups.md, Follow-up 1):
//   - hard-fail if any custom key column does not exist on the table, so misconfiguration is
//     caught up front instead of erroring per-event in hashEvent.
//   - mutates the tableToPartitionKeyOverrideMap to include the finalized custom key column casing
//
// It queries the target DB, so callers should only invoke it on the first import (not resume).
// (Primary-key existence for custom-key tables is validated separately in
// validatePrimaryKeysForConflictDetection, which fetches the PK columns for the whole import
// table list once in importData.)
// rejects the PK/Custom strategy on the tables having expression-based unique index or a unique index on a stored generated column
// custom key columns can't be a stored generated column
func validateAndFinalizeCDCPartitionKeysPerTable(tableToPartitionKeyOverrideMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], tableNames []sqlname.NameTuple, exprUKSet *utils.StructMap[sqlname.NameTuple, bool], generatedStoredCols *utils.StructMap[sqlname.NameTuple, []GeneratedStoredColumn]) error {
	var customTables []sqlname.NameTuple
	err := tableToPartitionKeyOverrideMap.IterKV(func(t sqlname.NameTuple, override cdcPartitionKeyOverride) (bool, error) {
		if override.Strategy == PARTITION_BY_CUSTOM {
			customTables = append(customTables, t)
		}
		return true, nil
	})
	if err != nil {
		return err
	}
	// every custom key column must exist on the table.
	missingColumnsOnTables := make(map[string][]string)
	ambiguousColumnsOnTables := make(map[string][]string)
	tableToColumns := make(map[string][]string)
	for _, t := range customTables {
		override, _ := tableToPartitionKeyOverrideMap.Get(t)
		tableColumns, err := tdb.GetListOfTableAttributes(t)
		if err != nil {
			return fmt.Errorf("error getting columns of table %s for custom cdc-partition-key validation: %w", t.ForOutput(), err)
		}
		overrideColumns := make([]string, 0, len(override.Columns))
		var missing []string
		var ambiguous []string
		for _, c := range override.Columns {
			bestMatchingColumnName, err := tdb.FindBestMatchingTargetColumnName(c, tableColumns)
			if err != nil {
				if _, ok := err.(*tgtdb.ErrAmbiguousColumnName); ok {
					ambiguous = append(ambiguous, c)
					continue
				}
				if _, ok := err.(*tgtdb.ErrColumnNameNotFound); ok {
					missing = append(missing, c)
					continue
				}
				return err
			}
			overrideColumns = append(overrideColumns, bestMatchingColumnName)
		}
		sort.Strings(tableColumns)
		tableToColumns[t.ForOutput()] = tableColumns
		if len(missing) > 0 {
			sort.Strings(missing)
			missingColumnsOnTables[t.ForOutput()] = missing
		} else if len(ambiguous) > 0 {
			sort.Strings(ambiguous)
			ambiguousColumnsOnTables[t.ForOutput()] = ambiguous
		} else {
			//once we have finalized the custom key column casing, now we can put the override back into the map
			override.Columns = overrideColumns
			tableToPartitionKeyOverrideMap.Put(t, override)
		}
	}
	errMsg := ""
	if len(missingColumnsOnTables) > 0 {
		for table, columns := range missingColumnsOnTables {
			errMsg += fmt.Sprintf("custom key column(s) '%v' do not exist on table '%s' (available columns: %v)\n", columns, table, tableToColumns[table])
		}
	}
	if len(ambiguousColumnsOnTables) > 0 {
		for table, columns := range ambiguousColumnsOnTables {
			errMsg += fmt.Sprintf("custom key column(s) '%v' are ambiguous on table '%s' (available columns: %v)\n", columns, table, tableToColumns[table])
		}
	}
	if errMsg != "" {
		return goerrors.Errorf("cdc-partition-key-overrides: %s", errMsg)
	}

	//once we have finalized the custom key column casing, now we can do the validation where custom/partition keys can't be used
	for _, t := range tableNames {
		override, _ := tableToPartitionKeyOverrideMap.Get(t)
		if override.Strategy == PARTITION_BY_TABLE {
			continue
		}

		ok, isExprUK, isGeneratedStoredColumn := shouldForceTablePartitioningForTable(t, exprUKSet, generatedStoredCols)
		if ok {
			//For PK and custom, we need to check if the table has a unique index on a stored generated column
			//and if so, we need to reject the strategy
			if isExprUK {
				return goerrors.Errorf("cdc-partition-key %s is not allowed for table %s because it has an expression-based unique index; use table (via --cdc-partition-key or --cdc-partition-key-overrides)", override.Strategy, t.ForOutput())
			}
			if isGeneratedStoredColumn {
				return goerrors.Errorf("cdc-partition-key %s is not allowed for table %s because it has a unique index on a stored generated column; use table (via --cdc-partition-key or --cdc-partition-key-overrides)", override.Strategy, t.ForOutput())
			}
		}
		if override.Strategy == PARTITION_BY_CUSTOM {
			//For CUSTOM, we need to also check if the custom key column(s) are a stored generated column(s)
			//and if so, we need to reject the strategy
			cols, ok := tableHasGeneratedColumn(generatedStoredCols, t, override.Columns)
			if ok {
				return goerrors.Errorf("cdc-partition-key %s is not allowed for table %s because custom key column(s) - [%s] are a stored generated column(s); use table (via --cdc-partition-key or --cdc-partition-key-overrides)", override.Strategy, t.ForOutput(), strings.Join(cols, ", "))
			}
		}

	}
	return nil
}
