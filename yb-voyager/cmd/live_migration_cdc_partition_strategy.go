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
auto/expr-UK rules), and persists TableToCDCPartitioningStrategyMap.

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
	if importDataStatus.TableToCDCPartitioningStrategyMap != nil {
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
func getCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, string], error) {
	tableToPartitioningStrategyMap := utils.NewStructMap[sqlname.NameTuple, string]()

	if importerRole != TARGET_DB_IMPORTER_ROLE {
		//For PG/ORacle source/source-replica, using partitioning by table since there won't be any huge difference in
		// performance between the two strategies for single node databases like PG/Oracle
		//and Parititon by table is better from data correctness perspective
		for _, t := range tableNames {
			tableToPartitioningStrategyMap.Put(t, PARTITION_BY_TABLE)
		}
		return tableToPartitioningStrategyMap, nil
	}

	if sourceDBType != POSTGRESQL {
		//Oracle sources do not support unique-key conflict detection during live migration
		//(we do not fetch unique indexes for Oracle). Force PARTITION_BY_TABLE so that all
		//events of a table run sequentially on a single channel, which makes unique-key
		//conflicts impossible and hence conflict detection unnecessary.
		//anything other than PG is not supported for conflict and hence we force table partitioning
		for _, t := range tableNames {
			tableToPartitioningStrategyMap.Put(t, PARTITION_BY_TABLE)
		}
		return tableToPartitioningStrategyMap, nil
	}

	importDataStatus, err := metaDB.GetImportDataStatusRecord()
	if err != nil {
		return nil, fmt.Errorf("error getting cdc partitioning strategy: %w", err)
	}
	if importDataStatus == nil {
		return nil, goerrors.Errorf("import data status record not found")
	}
	if importDataStatus.TableToCDCPartitioningStrategyMap != nil {
		log.Infof("cdc partitioning strategy found in metadb: %v, strategy: %v", metadb.IMPORT_DATA_STATUS_KEY, importDataStatus.TableToCDCPartitioningStrategyMap)
		for tableName, strategy := range importDataStatus.TableToCDCPartitioningStrategyMap {
			tuple, err := namereg.NameReg.LookupTableName(tableName)
			if err != nil {
				return nil, fmt.Errorf("error looking up table name: %w", err)
			}
			tableToPartitioningStrategyMap.Put(tuple, strategy)
		}
		for _, t := range tableNames {
			if _, ok := tableToPartitioningStrategyMap.Get(t); !ok {
				return nil, goerrors.Errorf("cdc partitioning strategy not found for table: %s", t.ForKey())
			}
		}
		return tableToPartitioningStrategyMap, nil
	}
	return nil, goerrors.Errorf("cdc partitioning strategy per table not found in metadb")
}

// resolveEffectiveCdcPartitionKeys applies global strategy, then per-table overlays,
// then rejects effective pk for expression-UK tables. Pure helper for unit tests.
func resolveEffectiveCdcPartitionKeys(
	tableNames []sqlname.NameTuple,
	globalKey string,
	overrides *utils.StructMap[sqlname.NameTuple, string],
	exprUKSet *utils.StructMap[sqlname.NameTuple, bool],
	targetDBType string,
) (*utils.StructMap[sqlname.NameTuple, string], error) {
	result := utils.NewStructMap[sqlname.NameTuple, string]()
	if overrides == nil {
		overrides = utils.NewStructMap[sqlname.NameTuple, string]()
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
				result.Put(t, PARTITION_BY_TABLE)
			}
		} else {
			for _, t := range tableNames {
				if _, ok := exprUKSet.Get(t); ok {
					result.Put(t, PARTITION_BY_TABLE)
				} else {
					result.Put(t, PARTITION_BY_PK)
				}
			}
		}
	default:
		for _, t := range tableNames {
			result.Put(t, globalKey)
		}
	}

	err := overrides.IterKV(func(t sqlname.NameTuple, strategy string) (bool, error) {
		result.Put(t, strategy)
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error applying cdc-partition-key-overrides: %w", err)
	}

	for _, t := range tableNames {
		strategy, _ := result.Get(t)
		if strategy == PARTITION_BY_PK {
			if _, isExprUK := exprUKSet.Get(t); isExprUK {
				return nil, goerrors.Errorf("cdc-partition-key pk is not allowed for table %s because it has an expression-based unique index; use table (via --cdc-partition-key or --cdc-partition-key-overrides)", t.ForOutput())
			}
		}
	}
	return result, nil
}

// resolveCdcPartitionKeyOverrides looks up override table names in namereg and
// validates each is present in importTableList. Returns a map keyed by NameTuple.
func resolveCdcPartitionKeyOverrides(rawOverrides map[string]string, importTableList []sqlname.NameTuple) (*utils.StructMap[sqlname.NameTuple, string], error) {
	resolved := utils.NewStructMap[sqlname.NameTuple, string]()
	if len(rawOverrides) == 0 {
		return resolved, nil
	}

	importTableSet := utils.NewStructMap[sqlname.NameTuple, bool]()
	for _, t := range importTableList {
		importTableSet.Put(t, true)
	}

	for tableSpec, strategy := range rawOverrides {
		tuple, err := namereg.NameReg.LookupTableName(tableSpec)
		if err != nil {
			return nil, goerrors.Errorf("cdc-partition-key-overrides: table %q not found in name registry: %w", tableSpec, err)
		}
		if _, ok := importTableSet.Get(tuple); !ok {
			return nil, goerrors.Errorf("cdc-partition-key-overrides: table %q is not in the import table list", tableSpec)
		}
		// Detect duplicates on the resolved NameTuple so different spellings of the
		// same table (casing/quoting/schema-qualification) don't silently overwrite.
		if existing, ok := resolved.Get(tuple); ok && !strings.EqualFold(existing, strategy) {
			return nil, goerrors.Errorf("cdc-partition-key-overrides: table %q (resolved to %s) specified multiple times with conflicting strategies %q and %q",
				tableSpec, tuple.ForOutput(), existing, strategy)
		}
		resolved.Put(tuple, strategy)
	}
	return resolved, nil
}

func checkIfNeedsExprUKCheck(cdcPartitionKey string, overrides *utils.StructMap[sqlname.NameTuple, string]) (bool, error) {
	// Collect expression-UK tables whenever we need them for auto resolution or pk-on-expr-UK validation.
	needsExprUKCheck := cdcPartitionKey == "auto" || cdcPartitionKey == PARTITION_BY_PK
	if !needsExprUKCheck {
		err := overrides.IterKV(func(_ sqlname.NameTuple, strategy string) (bool, error) {
			if strategy == PARTITION_BY_PK {
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
// rules and writes TableToCDCPartitioningStrategyMap to metaDB.
func computeAndPersistCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple, importDataStatus *metadb.ImportDataStatusRecord) (*utils.StructMap[sqlname.NameTuple, string], error) {
	tableToPartitioningStrategyMap, exprUKKeysForStorage, err := computeCdcPartitioningStrategyPerTable(tableNames, true, importDataStatus)
	if err != nil {
		return nil, fmt.Errorf("error computing cdc partitioning strategy per table: %w", err)
	}

	metadbMap := make(map[string]string)
	err = tableToPartitioningStrategyMap.IterKV(func(key sqlname.NameTuple, value string) (bool, error) {
		metadbMap[key.ForKey()] = value
		return true, nil
	})
	if err != nil {
		return nil, fmt.Errorf("error building cdc partitioning strategy map: %w", err)
	}
	err = metaDB.UpdateImportDataStatusRecord(func(obj *metadb.ImportDataStatusRecord) {
		obj.TableToCDCPartitioningStrategyMap = metadbMap
		obj.CdcExpressionUniqueIndexTables = exprUKKeysForStorage
	})
	if err != nil {
		return nil, fmt.Errorf("error updating cdc partitioning strategy in metadb: %w", err)
	}
	log.Infof("updated cdc partitioning strategy in metadb: %v with values: %v", metadb.IMPORT_DATA_STATUS_KEY, metadbMap)
	return tableToPartitioningStrategyMap, nil
}

func computeCdcPartitioningStrategyPerTable(tableNames []sqlname.NameTuple, isFirstRun bool, importDataStatus *metadb.ImportDataStatusRecord) (*utils.StructMap[sqlname.NameTuple, string], []string, error) {
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
	tableToPartitioningStrategyMap, err := resolveEffectiveCdcPartitionKeys(tableNames, cdcPartitionKey, overrides, exprUKSet, tconf.TargetDBType)

	if err != nil {
		return nil, nil, err
	}
	exprUKKeysForStorage := make([]string, 0, len(exprUKSet.Keys()))
	for _, t := range exprUKSet.Keys() {
		exprUKKeysForStorage = append(exprUKKeysForStorage, t)
	}
	return tableToPartitioningStrategyMap, exprUKKeysForStorage, nil
}

func getExpressionUniqueIndexTablesIfRequired(tableNames []sqlname.NameTuple, overrides *utils.StructMap[sqlname.NameTuple, string], cdcPartitionKey string, isFirstRun bool, importDataStatus *metadb.ImportDataStatusRecord) (*utils.StructMap[sqlname.NameTuple, bool], error) {
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

	resolvedTableToPartitioningStrategyMap, _, err := computeCdcPartitioningStrategyPerTable(tableNames, false, importDataStatus)
	if err != nil {
		return fmt.Errorf("error computing cdc partitioning strategy per table: %w", err)
	}
	return diffCdcPartitioningStrategy(resolvedTableToPartitioningStrategyMap, importDataStatus.TableToCDCPartitioningStrategyMap)
}

// diffCdcPartitioningStrategy compares a freshly-resolved per-table strategy map against
// the one persisted on the first run and returns an actionable error listing the tables
// whose effective strategy changed.
func diffCdcPartitioningStrategy(resolved *utils.StructMap[sqlname.NameTuple, string], stored map[string]string) error {
	var changed []string
	var missingInStored []string
	err := resolved.IterKV(func(t sqlname.NameTuple, strategy string) (bool, error) {
		storedStrategy, ok := stored[t.ForKey()]
		if !ok {
			missingInStored = append(missingInStored, t.ForKey())
		} else if storedStrategy != strategy {
			changed = append(changed, fmt.Sprintf("%s (persisted: %q, new: %q)", t.ForOutput(), storedStrategy, strategy))
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
