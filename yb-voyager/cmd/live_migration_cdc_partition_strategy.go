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
auto/expr-UK rules), and persists TableToCDCPartitionKey.

It is intentionally called before snapshot import so bad configs fail fast.
On resume (map already in metaDB) this is a no-op.
*/
func prepareCdcPartitionKey(tableNames []sqlname.NameTuple) error {
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
		// the current flags — reusing the expression-UK tables captured on the first run so
		// no target-DB re-query is needed — and reject if it differs from what was
		// persisted. This catches semantically-different overrides that a plain string
		// compare would miss (ordering, spelling/quoting, whitespace).
		if err := validateCdcPartitioningStrategyUnchanged(tableNames, importDataStatus); err != nil {
			return err
		}
		log.Infof("cdc partition key already prepared in metadb and unchanged; skipping recompute")
		return nil
	}

	_, err = computeAndPersistCdcPartitioningStrategyPerTable(tableNames, importDataStatus)
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

	if shouldForceTablePartitioning(importerRole, sourceDBType) {
		//For PG/ORacle source/source-replica, using partitioning by table since there won't be any huge difference in
		// performance between the two strategies for single node databases like PG/Oracle
		//and Parititon by table is better from data correctness perspective
		for _, t := range tableNames {
			tablePartitionKeyMap.Put(t, cdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
		}
		return tablePartitionKeyMap, nil
	}

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

func shouldForceTablePartitioning(importerRole string, sourceDBType string) bool {
	return importerRole != TARGET_DB_IMPORTER_ROLE || sourceDBType != POSTGRESQL
}

// resolveEffectiveCdcPartitionKeys applies global strategy, then per-table overlays,
// then rejects effective pk for expression-UK tables. Pure helper for unit tests.
func resolveEffectiveCdcPartitionKeys(
	tableNames []sqlname.NameTuple,
	globalKey string,
	overrides *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride],
	exprUKSet *utils.StructMap[sqlname.NameTuple, bool],
	targetDBType string,
) (*utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], error) {
	result := utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	if overrides == nil {
		overrides = utils.NewStructMap[sqlname.NameTuple, cdcPartitionKeyOverride]()
	}
	if exprUKSet == nil {
		exprUKSet = utils.NewStructMap[sqlname.NameTuple, bool]()
	}

	switch globalKey {
	case "auto":
		if targetDBType == YUGABYTEDB_AMP {
			// yb-amp is a single-node PostgreSQL-compatible compute (no YB
			// tablets / colocation), so — exactly like the PG/Oracle
			// source/source-replica case — PARTITION_BY_TABLE has no real
			// throughput downside and is safer for correctness. It also sidesteps
			// getExpressionUniqueIndexTables(), which is implemented only on the
			// TargetYugabyteDB driver.
			for _, t := range tableNames {
				result.Put(t, cdcPartitionKeyOverride{Strategy: PARTITION_BY_TABLE})
			}
		} else {
			for _, t := range tableNames {
				if _, ok := exprUKSet.Get(t); ok {
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

	for _, t := range tableNames {
		override, _ := result.Get(t)
		// Both pk and custom route by column *values*; an expression-based unique index
		// cannot be protected by value routing (its conflicting value is the expression
		// output, not a stored column), so neither strategy is safe on such tables.
		if override.Strategy != PARTITION_BY_TABLE {
			if _, isExprUK := exprUKSet.Get(t); isExprUK {
				return nil, goerrors.Errorf("cdc-partition-key %s is not allowed for table '%s' because it has an expression-based unique index; use table (via --cdc-partition-key or --cdc-partition-key-overrides)", override.Strategy, t.ForOutput())
			}
		}
	}
	return result, nil
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

func checkIfNeedsExprUKCheck(cdcPartitionKey string, overrides *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) (bool, error) {
	// Collect expression-UK tables whenever we need them for auto resolution or pk-on-expr-UK validation.
	needsExprUKCheck := cdcPartitionKey == "auto" || cdcPartitionKey == PARTITION_BY_PK
	if !needsExprUKCheck {
		err := overrides.IterKV(func(_ sqlname.NameTuple, override cdcPartitionKeyOverride) (bool, error) {
			if override.Strategy != PARTITION_BY_TABLE {
				needsExprUKCheck = true
				return false, nil
			}
			return true, nil
		})
		if err != nil {
			return false, err
		}
	}
	return needsExprUKCheck, nil
}

// computeAndPersistCdcPartitioningStrategyPerTable resolves global + overrides + auto/expr-UK
// rules and writes TableToCDCPartitionKey to metaDB.
func computeAndPersistCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple, importDataStatus *metadb.ImportDataStatusRecord) (*utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], error) {
	tableToPartitionKeyOverrideMap, exprUKKeysForStorage, err := computeCdcPartitioningStrategyPerTable(tableNames, true, importDataStatus)
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

func computeCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple, isFirstRun bool, importDataStatus *metadb.ImportDataStatusRecord) (*utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], []string, error) {
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
	tableToPartitionKeyOverrideMap, err := resolveEffectiveCdcPartitionKeys(tableNames, cdcPartitionKey, overrides, exprUKSet, tconf.TargetDBType)

	if err != nil {
		return nil, nil, err
	}

	// Import-start guardrails for custom-key tables (custom key column existence). These query
	// the target DB, so only run them on the first import; on resume the config is unchanged
	// (enforced by validateCdcPartitioningStrategyUnchanged) and re-querying is unnecessary.
	if isFirstRun {
		if err := validateCustomPartitionKeyTables(tableToPartitionKeyOverrideMap); err != nil {
			return nil, nil, err
		}
	}

	exprUKKeysForStorage := make([]string, 0, len(exprUKSet.Keys()))
	for _, t := range exprUKSet.Keys() {
		exprUKKeysForStorage = append(exprUKKeysForStorage, t)
	}
	return tableToPartitionKeyOverrideMap, exprUKKeysForStorage, nil
}

func getExpressionUniqueIndexTablesIfRequired(tableNames []sqlname.NameTuple, overrides *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride], cdcPartitionKey string, isFirstRun bool, importDataStatus *metadb.ImportDataStatusRecord) (*utils.StructMap[sqlname.NameTuple, bool], error) {
	// Always return a non-nil map: callers call exprUKSet.Keys()/pass it to
	// resolveEffectiveCdcPartitionKeys, and a nil *StructMap panics on Keys().
	exprUKSet := utils.NewStructMap[sqlname.NameTuple, bool]()

	needsExprUKCheck, err := checkIfNeedsExprUKCheck(cdcPartitionKey, overrides)
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

// validateCdcPartitioningStrategyUnchanged re-resolves the effective per-table CDC
// partitioning strategy from the current flags and fails if it differs from the map
// persisted on the first run. It reuses the expression-UK tables captured on the first
// run (falling back to a target-DB lookup only for older records that predate that
// capture) so resume never silently applies a changed cdc-partition-key /
// cdc-partition-key-overrides.
func validateCdcPartitioningStrategyUnchanged(tableNames []sqlname.NameTuple, importDataStatus *metadb.ImportDataStatusRecord) error {

	cdcPartitionKeyOverridesStored := importDataStatus.CdcPartitionKeyOverridesConfig
	if cdcPartitionKeyOverridesStored == cdcPartitionKeyOverrides {
		//No changes in the overrides so we can continue
		return nil
	}

	resolvedTableToPartitionKeyOverrideMap, _, err := computeCdcPartitioningStrategyPerTable(tableNames, false, importDataStatus)
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

// validateCustomPartitionKeyTables runs the import-start guardrails for tables routed by a
// custom partition key (see cdc_partition_key_followups.md, Follow-up 1):
//   - hard-fail if any custom key column does not exist on the table, so misconfiguration is
//     caught up front instead of erroring per-event in hashEvent.
//
// It queries the target DB, so callers should only invoke it on the first import (not resume).
// (Primary-key existence for custom-key tables is validated separately in
// validatePrimaryKeysForConflictDetection, which fetches the PK columns for the whole import
// table list once in importData.)
func validateCustomPartitionKeyTables(tableToPartitionKeyOverrideMap *utils.StructMap[sqlname.NameTuple, cdcPartitionKeyOverride]) error {
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
	if len(customTables) == 0 {
		return nil
	}

	// every custom key column must exist on the table.
	for _, t := range customTables {
		override, _ := tableToPartitionKeyOverrideMap.Get(t)
		tableColumns, err := tdb.GetListOfTableAttributes(t)
		if err != nil {
			return fmt.Errorf("error getting columns of table %s for custom cdc-partition-key validation: %w", t.ForOutput(), err)
		}
		columnSet := make(map[string]bool, len(tableColumns))
		for _, c := range tableColumns {
			columnSet[c] = true
		}
		var missing []string
		for _, c := range override.Columns {
			if !columnSet[c] {
				missing = append(missing, c)
			}
		}
		if len(missing) > 0 {
			sort.Strings(missing)
			sort.Strings(tableColumns)
			return goerrors.Errorf("cdc-partition-key-overrides: custom key column(s) '%v' do not exist on table '%s' (available columns: %v)", missing, t.ForOutput(), tableColumns)
		}
	}
	return nil
}
