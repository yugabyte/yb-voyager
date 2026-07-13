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
	"strings"

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/importdata"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/types"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var targetDBPassword string
var sourceDBType string
var enableOrafce utils.BoolStr
var importType string
var prometheusMetricsPort int
var importUsePartitionRoot utils.BoolStr // default is true for backward compatibility
var disableNullConflicts utils.BoolStr   // default is false for backward compatibility

var supportedSSLModesOnTargetForImport = AllSSLModes // supported SSL modes for YugabyteDB is different for import VS export data from target(streaming phase)
var supportedSSLModesOnSourceOrSourceReplica = AllSSLModes

// tconf struct will be populated by CLI arguments parsing
var tconf tgtdb.TargetConf

var tdb tgtdb.TargetDB

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import schema and data from compatible source database to target database. ",
	Long:  `Import has various sub-commands i.e. import schema, import data to import into YugabyteDB from various compatible source databases(Oracle, MySQL, PostgreSQL). Also import data(snapshot + changes from target) into source-replica/source in case of live migration with fall-back/fall-forward worflows.`,
}

func init() {
	rootCmd.AddCommand(importCmd)
}

// If any changes are made to this function, verify if the change is also needed for importDataFileCommand.go
func validateImportFlags(cmd *cobra.Command, importerRole string) error {
	checkOrSetDefaultTargetSSLMode()
	validateTargetPortRange()
	validateAmpTargetSourceCompatibility()

	validateConflictsBetweenTableListFlags(tconf.TableList, tconf.ExcludeTableList)

	validateTableListFlag(tconf.TableList, "table-list")
	validateTableListFlag(tconf.ExcludeTableList, "exclude-table-list")

	var err error
	if tconf.TableList == "" {
		tconf.TableList, err = validateAndExtractTableNamesFromFile(tableListFilePath, "table-list-file-path")
		if err != nil {
			return err
		}
	}

	if tconf.ExcludeTableList == "" {
		tconf.ExcludeTableList, err = validateAndExtractTableNamesFromFile(excludeTableListFilePath, "exclude-table-list-file-path")
		if err != nil {
			return err
		}
	}

	if tconf.ImportObjects != "" && tconf.ExcludeImportObjects != "" {
		return goerrors.Errorf("only one of --object-type-list and --exclude-object-type-list are allowed")
	}
	validateImportObjectsFlag(tconf.ImportObjects, "object-type-list")
	validateImportObjectsFlag(tconf.ExcludeImportObjects, "exclude-object-type-list")
	validateTargetSchemaFlag()
	// For beta2.0 release (and onwards until further notice)
	if tconf.DisableTransactionalWrites {
		fmt.Println("WARNING: The --disable-transactional-writes feature is in the experimental phase, not for production use case.")
	}
	validateBatchSizeFlag(batchSizeInNumRows)
	switch importerRole {
	case TARGET_DB_IMPORTER_ROLE:
		getTargetPassword(cmd)
	case SOURCE_REPLICA_DB_IMPORTER_ROLE:
		getSourceReplicaDBPassword(cmd)
	case SOURCE_DB_IMPORTER_ROLE:
		getSourceDBPassword(cmd)
	}
	validateParallelismFlags(cmd)

	return nil
}

func validateImportDataFlags() error {
	err := validateOnPrimaryKeyConflictFlag()
	if err != nil {
		return fmt.Errorf("error validating --on-primary-key-conflict flag: %w", err)
	}

	err = validateTruncateTablesFlag()
	if err != nil {
		return err
	}

	return nil
}

// validateAmpTargetSourceCompatibility ensures yugabytedb-amp is only used with a
// PostgreSQL source. yb-amp is a PG-wire compute; the offline PG->amp path is the
// only flow that has been audited/supported (see ACTION_ITEMS.md). Other sources
// (oracle/mysql) bring schema/type transforms and YB-specific assumptions that have
// not been validated for amp. This runs from validateImportFlags, which is invoked
// by the import-schema, import-data / ...toTarget, and finalize PreRuns *after*
// sourceDBType is populated from the MSR, so it covers all import-side commands.
func validateAmpTargetSourceCompatibility() {
	if tconf.TargetDBType != YUGABYTEDB_AMP {
		return
	}
	if sourceDBType != POSTGRESQL {
		utils.ErrExit("--target-db-type %s is only supported with a PostgreSQL source (detected source: %q)", YUGABYTEDB_AMP, sourceDBType)
	}
}

// validateAmpUnsupportedFlags rejects, fail-fast, the import-data flags that have no
// meaning for a yugabytedb-amp target. yb-amp is a stateless PG17 compute with none of
// the YB cluster features these flags drive (no per-node fan-out, no upsert fast-path,
// no ON CONFLICT-aware COPY), so honoring them silently would be wrong:
//   - --target-endpoints / --use-public-ip: no multi-node cluster to distribute across.
//   - --enable-upsert: a silent no-op on the PG COPY path — never actually honored.
//   - --on-primary-key-conflict (non ERROR-POLICY, e.g. IGNORE): amp's snapshot path is
//     plain COPY and cannot honor ON CONFLICT, so IGNORE would degrade and then ABORT on
//     a duplicate key. (Validity of the value itself is checked separately in
//     validateOnPrimaryKeyConflictFlag.)
//
// Only relevant for the target-import role (these are import-data flags). Invoked from
// the import-data PreRun (shared by importDataCmd and importDataToTargetCmd).
func validateAmpUnsupportedFlags(cmd *cobra.Command) {
	if tconf.TargetDBType != YUGABYTEDB_AMP || importerRole != TARGET_DB_IMPORTER_ROLE {
		return
	}

	notApplicable := func(flag string) {
		utils.ErrExit("--%s is not applicable for --target-db-type %s", flag, YUGABYTEDB_AMP)
	}

	if tconf.TargetEndpoints != "" {
		notApplicable("target-endpoints")
	}
	if bool(tconf.UsePublicIP) {
		notApplicable("use-public-ip")
	}
	if bool(tconf.EnableUpsert) {
		notApplicable("enable-upsert")
	}
	// --on-primary-key-conflict has already been upper-cased by validateOnPrimaryKeyConflictFlag
	// when it runs (validateImportDataFlags -> validateOnPrimaryKeyConflictFlag); normalize here
	// too so we are order-independent. Anything other than ERROR-POLICY (i.e. IGNORE / future
	// UPDATE) cannot be honored by amp's plain-COPY snapshot path.
	if strings.ToUpper(tconf.OnPrimaryKeyConflictAction) != constants.PRIMARY_KEY_CONFLICT_ACTION_ERROR_POLICY {
		notApplicable("on-primary-key-conflict")
	}
	// Only the default `abort` error policy is validated for amp. `stash-and-continue`
	// (stashing errored snapshot rows and continuing) has not been tested on amp's
	// plain-COPY path, so reject it explicitly rather than silently allowing it.
	if errorPolicySnapshotFlag == importdata.StashAndContinueErrorPolicy {
		utils.ErrExit("--error-policy-snapshot %s is not supported for --target-db-type %s; only the default %s error policy is supported",
			importdata.StashAndContinueErrorPolicy, YUGABYTEDB_AMP, importdata.AbortErrorPolicy)
	}
}

func validateImportUsePartitionRootFlag() error {
	// --use-partition-root flag is only valid for live migration with a PostgreSQL source
	//and only for the CDC streaming phase and snapshot part isn't supported right now.
	if !importUsePartitionRoot {
		// Only validate when flag is explicitly set to false (non-default)
		// Read the export type from MSR since importType may not be set yet in PreRun
		msr, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			return goerrors.Errorf("failed to get migration status record: %w", err)
		}
		exportTypeFromSource := msr.ExportTypeFromSource
		if !changeStreamingIsEnabled(exportTypeFromSource) {
			return goerrors.Errorf("'--use-partition-root false' is only valid for live migration")
		}
		if importerRole == SOURCE_REPLICA_DB_IMPORTER_ROLE {
			return goerrors.Errorf("'--use-partition-root false' is not supported for source-replica")
		}
		// --use-partition-root controls how PostgreSQL declarative-partitioned tables are
		// streamed; it is meaningful only when the source database is PostgreSQL. The target
		// engine (yugabytedb / yugabytedb-amp / a PG fall-back target) is irrelevant here.
		if sourceDBType != POSTGRESQL {
			return goerrors.Errorf("'--use-partition-root false' is only valid when the source database is PostgreSQL")
		}
	}
	tconf.UsePartitionRoot = bool(importUsePartitionRoot)
	if importerRole != TARGET_DB_IMPORTER_ROLE {
		return nil
	}
	return metaDB.UpdateImportDataStatusRecord(func(record *metadb.ImportDataStatusRecord) {
		record.TargetUsePartitionRoot = bool(importUsePartitionRoot)
	})
}

var validCdcPartitioningStrategies = []string{"pk", "table", "auto"}

func validateCdcPartitioningStrategyFlag(cmd *cobra.Command) error {
	if importerRole != TARGET_DB_IMPORTER_ROLE {
		return nil
	}
	cdcPartitioningStrategyParameterPassed := cmd.Flags().Changed("cdc-partitioning-strategy")
	if !changeStreamingIsEnabled(importType) {
		if cdcPartitioningStrategyParameterPassed {
			utils.ErrExit("--cdc-partitioning-strategy is not supported for offline migration. Re-run the command without this flag.")
		}
		return nil
	}
	if sourceDBType != POSTGRESQL && cdcPartitioningStrategyParameterPassed {
		utils.ErrExit("--cdc-partitioning-strategy is only supported for PostgreSQL source")
	}

	if cdcPartitioningStrategy == "" {
		utils.ErrExit("cdc partitioning strategy is required")
	}

	if !lo.Contains(validCdcPartitioningStrategies, cdcPartitioningStrategy) {
		utils.ErrExit("invalid cdc partitioning strategy: %s. Supported values are: %s", cdcPartitioningStrategy, strings.Join(validCdcPartitioningStrategies, ", "))
	}

	importDataStatus, err := metaDB.GetImportDataStatusRecord()
	if err != nil {
		return fmt.Errorf("error getting import data status record: %w", err)
	}

	if importDataStatus == nil || !importDataStatus.ImportDataStarted || bool(startClean) {
		//if import data has not started or start-clean flag is used, allow the change in cdc partitioning strategy
		return nil
	}
	if importDataStatus.CdcPartitioningStrategyConfig == "" {
		//if not a first run and the cdc partitioning strategy is not set
		//this can be the case when the import data is resumed from an earlier version of yb-voyager
		//So we should use the cdc partitioning strategy as pk to be upgrade safe
		utils.ErrExit("Resuming from an earlier version of yb-voyager is not supported as cdc partition strategy was not set. Use --start-clean to start a fresh import with the new yb-voyager version.")
	}
	if cdcPartitioningStrategy != importDataStatus.CdcPartitioningStrategyConfig {
		utils.ErrExit("changing the cdc partitioning strategy is not allowed after the import data has started. Current strategy: %s, new strategy: %s\n Use --start-clean to start a fresh import with the new strategy.", importDataStatus.CdcPartitioningStrategyConfig, cdcPartitioningStrategy)
	}
	log.Infof("cdc partitioning strategy: %s", cdcPartitioningStrategy)
	return nil
}

func registerCommonImportFlags(cmd *cobra.Command) {
	BoolVar(cmd.Flags(), &tconf.ContinueOnError, "continue-on-error", false,
		"Ignore errors and continue with the import")

	BoolVar(cmd.Flags(), &tconf.RunGuardrailsChecks, "run-guardrails-checks", true, "Run guardrails checks during import. Setting this to false is unsafe: it skips critical pre-migration validations (such as source/target database permissions, binary dependencies, and version compatibility) and may lead to migration failures or data issues. Leave the default (true) unless you have a specific reason to disable checks.")
}

// registerTargetDBTypeFlag registers --target-db-type. It is intentionally
// NOT part of registerTargetDBConnFlags: the choice of target engine is only
// meaningful for the commands that import schema/data into the target
// (import schema, import data / ...toTarget, finalize-schema-post-data-import).
// Commands like import-data-file and compare-performance always target a real
// YugabyteDB, so they don't expose it.
func registerTargetDBTypeFlag(cmd *cobra.Command) {
	cmd.Flags().StringVar(&tconf.TargetDBType, "target-db-type", "",
		fmt.Sprintf("type of the target database to import into. Supported values: %s (default), %s (YugabyteDB AMP — a PostgreSQL-compatible compute over YugabyteDB storage)",
			YUGABYTEDB, YUGABYTEDB_AMP))
}

func registerTargetDBConnFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&tconf.Host, "target-db-host", "127.0.0.1",
		"host on which the YugabyteDB server is running")

	cmd.Flags().IntVar(&tconf.Port, "target-db-port", 0,
		"port on which the YugabyteDB YSQL API is running (Default: 5433)")

	cmd.Flags().StringVar(&tconf.User, "target-db-user", "",
		"username with which to connect to the target YugabyteDB server")
	cmd.MarkFlagRequired("target-db-user")

	cmd.Flags().StringVar(&tconf.Password, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server. Alternatively, you can also specify the password by setting the environment variable TARGET_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")

	cmd.Flags().StringVar(&tconf.DBName, "target-db-name", "",
		"name of the database on the target YugabyteDB server on which import needs to be done")

	cmd.Flags().StringVar(&tconf.SchemaConfig, "target-db-schema", "",
		"target schema name in YugabyteDB (Note: works only for source as Oracle and MySQL, in case of PostgreSQL you can ALTER schema name post import)")

	// TODO: SSL related more args might come. Need to explore SSL part completely.
	cmd.Flags().StringVar(&tconf.SSLCertPath, "target-ssl-cert", "",
		"Path of file containing target SSL Certificate")

	cmd.Flags().StringVar(&tconf.SSLMode, "target-ssl-mode", "prefer",
		fmt.Sprintf("specify the target SSL mode: [%s]",
			strings.Join(supportedSSLModesOnTargetForImport, ", ")))

	cmd.Flags().StringVar(&tconf.SSLKey, "target-ssl-key", "",
		"Path of file containing target SSL Key")

	cmd.Flags().StringVar(&tconf.SSLRootCert, "target-ssl-root-cert", "",
		"Path of file containing target SSL Root Certificate")

	cmd.Flags().StringVar(&tconf.SSLCRL, "target-ssl-crl", "",
		"Path of file containing target SSL Root Certificate Revocation List (CRL)")
}

func registerSourceDBAsTargetConnFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&tconf.Password, "source-db-password", "",
		"source password to connect as the specified user on the source DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")
}

func registerSourceReplicaDBAsTargetConnFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&tconf.Host, "source-replica-db-host", "127.0.0.1",
		"host on which the Source-Replica DB server is running")

	cmd.Flags().IntVar(&tconf.Port, "source-replica-db-port", 0,
		"port on which the Source-Replica DB server is running Default: ORACLE(1521), POSTGRESQL(5432)")

	cmd.Flags().StringVar(&tconf.User, "source-replica-db-user", "",
		"username with which to connect to the Source-Replica DB server")
	cmd.MarkFlagRequired("source-replica-db-user")

	cmd.Flags().StringVar(&tconf.Password, "source-replica-db-password", "",
		"password with which to connect to the Source-Replica DB server. Alternatively, you can also specify the password by setting the environment variable SOURCE_REPLICA_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")

	cmd.Flags().StringVar(&tconf.DBName, "source-replica-db-name", "",
		"name of the database on the Source-Replica DB server on which import needs to be done")

	cmd.Flags().StringVar(&tconf.DBSid, "source-replica-db-sid", "",
		"[For Oracle Only] Oracle System Identifier (SID) that you wish to use while importing data to Oracle instances")

	cmd.Flags().StringVar(&tconf.OracleHome, "oracle-home", "",
		"[For Oracle Only] Path to set $ORACLE_HOME environment variable. tnsnames.ora is found in $ORACLE_HOME/network/admin")

	cmd.Flags().StringVar(&tconf.TNSAlias, "oracle-tns-alias", "",
		"[For Oracle Only] Name of TNS Alias you wish to use to connect to Oracle instance. Refer to documentation to learn more about configuring tnsnames.ora and aliases")

	cmd.Flags().StringVar(&tconf.SchemaConfig, "source-replica-db-schema", "",
		"schema name in Source-Replica DB (Note: works only for source as Oracle, in case of PostgreSQL schemas remain same as of source)")

	// TODO: SSL related more args might come. Need to explore SSL part completely.
	cmd.Flags().StringVar(&tconf.SSLCertPath, "source-replica-ssl-cert", "",
		"Path of the file containing Source-Replica DB SSL Certificate Path")

	// Q: Do we need separate handling for Oracle vs PostgreSQL here?
	cmd.Flags().StringVar(&tconf.SSLMode, "source-replica-ssl-mode", "prefer",
		fmt.Sprintf("specify the Source-Replica DB SSL mode: [%s]",
			strings.Join(supportedSSLModesOnSourceOrSourceReplica, ", ")))

	cmd.Flags().StringVar(&tconf.SSLKey, "source-replica-ssl-key", "",
		"Path of the file containing Source-Replica DB SSL Key")

	cmd.Flags().StringVar(&tconf.SSLRootCert, "source-replica-ssl-root-cert", "",
		"Path of the file containing Source-Replica DB SSL Root Certificate")

	cmd.Flags().StringVar(&tconf.SSLCRL, "source-replica-ssl-crl", "",
		"Path of the file containing Source-Replica DB SSL Root Certificate Revocation List (CRL)")
}

func registerImportDataCommonFlags(cmd *cobra.Command) {
	BoolVar(cmd.Flags(), &disablePb, "disable-pb", false,
		"Disable progress bar/stats during data import (default false)")

	cmd.Flags().IntVar(&EVENT_BATCH_MAX_RETRY_COUNT, "max-retries-streaming", 15, "Maximum number of retries for failed event batch in live migration")

	cmd.Flags().StringVar(&tconf.ExcludeTableList, "exclude-table-list", "",
		"comma-separated list of the source db table names to exclude while import data.\n"+
			"Table names can include glob wildcard characters ? (matches one character) and * (matches zero or more characters) \n"+
			`In case the table names are case sensitive, double-quote them. For example --exclude-table-list 'orders,"Products",items'`)
	cmd.Flags().StringVar(&tconf.TableList, "table-list", "",
		"comma-separated list of the source db table names to include while importing data.\n"+
			"Table names can include glob wildcard characters ? (matches one character) and * (matches zero or more characters) \n"+
			`In case the table names are case sensitive, double-quote them. For example --table-list 'orders,"Products",items'`)

	cmd.Flags().StringVar(&excludeTableListFilePath, "exclude-table-list-file-path", "",
		"path of the file containing for list of the source db table names to exclude while importing data")
	cmd.Flags().StringVar(&tableListFilePath, "table-list-file-path", "",
		"path of the file containing the list of the source db table names to import data")

	BoolVar(cmd.Flags(), &tconf.EnableUpsert, "enable-upsert", false,
		"Enable UPSERT mode on target tables. WARNING: Ensure that tables on target YugabyteDB do not have secondary indexes. If a table has secondary indexes, setting this flag to true may lead to corruption of the indexes. (default false)")
	BoolVar(cmd.Flags(), &tconf.UsePublicIP, "use-public-ip", false,
		"Use the public IPs of the nodes to distribute --parallel-jobs uniformly for data import (default false)\n"+
			"Note: you might need to configure database to have public_ip available by setting server-broadcast-addresses.\n"+
			"Refer: https://docs.yugabyte.com/preview/reference/configuration/yb-tserver/#server-broadcast-addresses")
	cmd.Flags().StringVar(&tconf.TargetEndpoints, "target-endpoints", "",
		"comma separated list of node's endpoint to use for parallel import of data(default is to use all the nodes in the cluster).\n"+
			"For example: \"host1:port1,host2:port2\" or \"host1,host2\"\n"+
			"Note: use-public-ip flag will be ignored if this is used.")
	// flag existence depends on fix of this gh issue: https://github.com/yugabyte/yugabyte-db/issues/12464
	BoolVar(cmd.Flags(), &tconf.DisableTransactionalWrites, "disable-transactional-writes", false,
		"Disable transactional writes in tables for faster data ingestion (default false)\n"+
			"(Note: this is a interim flag until the issues related to 'yb_disable_transactional_writes' session variable are fixed. Refer: https://github.com/yugabyte/yugabyte-db/issues/12464)")
	// Hidden for beta2.0 release (and onwards until further notice).
	cmd.Flags().MarkHidden("disable-transactional-writes")

	BoolVar(cmd.Flags(), &truncateSplits, "truncate-splits", true,
		"Truncate splits after importing")
	cmd.Flags().MarkHidden("truncate-splits")
}

func registerImportUsePartitionRootFlagToTarget(cmd *cobra.Command) {
	BoolVar(cmd.Flags(), &importUsePartitionRoot, "use-partition-root", true,
		"For partitioned tables during live migration:\n"+
			"  - true (default): Import CDC data only via the root table.\n"+
			"  - false: Import CDC data only via child partitions\n(Note: this flag is only supported for YugabyteDB target version 2025.2.3.0 and above)")
}

func registerImportUsePartitionRootFlagToSource(cmd *cobra.Command) {
	BoolVar(cmd.Flags(), &importUsePartitionRoot, "use-partition-root", true,
		"For partitioned tables during live migration:\n"+
			"  - true (default): Import CDC data only via the root table.\n"+
			"  - false: Import CDC data only via child partitions\n")
}

func registerImportDataToTargetFlags(cmd *cobra.Command) {
	BoolVar(cmd.Flags(), &startClean, "start-clean", false,
		`Starts a fresh import with exported data files present in the export-dir/data directory. 
If any table on YugabyteDB database is non-empty, it prompts whether you want to continue the import without truncating those tables; 
If you go ahead without truncating, then yb-voyager starts ingesting the data present in the data files with upsert mode.
Note that for the cases where a table doesn't have a primary key, this may lead to insertion of duplicate data. To avoid this, exclude the table using the --exclude-file-list or truncate those tables manually before using the start-clean flag (default false)`)
	BoolVar(cmd.Flags(), &truncateTables, "truncate-tables", false,
		"Truncate tables on target YugabyteDB before importing data. Only applicable along with --start-clean true (default false)")

	cmd.Flags().Var(&errorPolicySnapshotFlag, "error-policy-snapshot",
		"The desired behavior when there is an error while processing and importing rows to target YugabyteDB in the snapshot phase. The errors can be while reading from file, transforming rows, or ingesting rows into YugabyteDB.\n"+
			"\tabort: immediately abort the process. (default)\n"+
			"\tstash-and-continue: stash the errored rows to a file and continue with the import")

	cmd.Flags().IntVar(&maxConcurrentBatchProductionsConfig, "max-concurrent-batch-productions", 10, "Maximum number of concurrent batch productions to allow while importing data (default 10)")
	cmd.Flags().MarkHidden("max-concurrent-batch-productions")

	BoolVar(cmd.Flags(), &enableRandomBatchProduction, "enable-random-batch-production", true, "Enable random batch production during data import (default true)")
	cmd.Flags().MarkHidden("enable-random-batch-production")

	cmd.Flags().StringVar(&cdcPartitioningStrategy, "cdc-partitioning-strategy", "auto",
		`The desired partitioning strategy to use while importing cdc events parallelly. The supported values are: pk, table. (default auto-detect)
		\tauto: Automatically detect the partitioning strategy based on the table having expression or normal unique indexes.
		\tpk: Partition the cdc events by primary key.
		\ttable: Partition the cdc events by table.`)
	cmd.Flags().MarkHidden("cdc-partitioning-strategy")

	BoolVar(cmd.Flags(), &disableNullConflicts, "disable-null-conflicts", false, "Disable conflict detection for null values during data import (default false), UNSAFE to use this flag if you have unique key constraints with NULLS NOT DISTINCT property")
	cmd.Flags().MarkHidden("disable-null-conflicts")

	cmd.Flags().IntVar(&prometheusMetricsPort, "prometheus-metrics-port", 0,
		"Port for Prometheus metrics server (default: 9101)")
	cmd.Flags().MarkHidden("prometheus-metrics-port")
}

func registerImportSchemaFlags(cmd *cobra.Command) {
	BoolVar(cmd.Flags(), &startClean, "start-clean", false,
		"Delete all schema objects and start a fresh import (default false)")
	cmd.Flags().StringVar(&tconf.ImportObjects, "object-type-list", "",
		"comma separated list of schema object types to include while importing schema")
	cmd.Flags().StringVar(&tconf.ExcludeImportObjects, "exclude-object-type-list", "",
		"comma separated list of schema object types to exclude while importing schema (ignored if --object-type-list is used)")
	BoolVar(cmd.Flags(), &importObjectsInStraightOrder, "straight-order", false,
		"Imports the schema objects in the order specified via the --object-type-list flag (default false)")
	BoolVar(cmd.Flags(), &flagPostSnapshotImport, "post-snapshot-import", false,
		"Perform schema related tasks on target YugabyteDB after data import is complete. Use --refresh-mviews along with this flag to refresh materialized views.")
	BoolVar(cmd.Flags(), &tconf.IgnoreIfExists, "ignore-exist", false,
		"ignore errors if object already exists (default false)")
	BoolVar(cmd.Flags(), &flagRefreshMViews, "refresh-mviews", false,
		"Refreshes the materialised views on target during post snapshot import phase (default false)")
	BoolVar(cmd.Flags(), &enableOrafce, "enable-orafce", true,
		"enable Orafce extension on target(if source db type is Oracle)")

	// --post-snapshot-import and --refresh-mviews flags will now be handled by the command post-data-import-finalize-schema
	// Not removing these flags and just deprecating them for backward compatibility.
	cmd.Flags().MarkDeprecated("post-snapshot-import",
		"use the command 'finalize-schema-post-data-import' instead. \nFor more details, refer to the documentation: \nhttps://docs.yugabyte.com/preview/yugabyte-voyager/reference/schema-migration/finalize-schema-post-data-import/\n")
	cmd.Flags().MarkDeprecated("refresh-mviews",
		"it is no longer supported in the 'import schema' command. Use the 'finalize-schema-post-data-import' command instead. \nFor more details, refer to the documentation: \nhttps://docs.yugabyte.com/preview/yugabyte-voyager/reference/schema-migration/finalize-schema-post-data-import/\n")

}

func validateTargetPortRange() {
	if tconf.Port == 0 {
		if tconf.TargetDBType == ORACLE {
			tconf.Port = ORACLE_DEFAULT_PORT
		} else if tconf.TargetDBType == YUGABYTEDB {
			tconf.Port = YUGABYTEDB_YSQL_DEFAULT_PORT
		} else if tconf.TargetDBType == POSTGRESQL {
			tconf.Port = POSTGRES_DEFAULT_PORT
		} else if tconf.TargetDBType == YUGABYTEDB_AMP {
			// yb-amp compute endpoints are assigned deployment-specific ports
			// (there is no canonical default like YSQL's 5433), so require it.
			utils.ErrExit("--target-db-port is required for --target-db-type %s (yb-amp compute endpoints use deployment-specific ports)", YUGABYTEDB_AMP)
		}
		return
	}

	if tconf.Port < 0 || tconf.Port > 65535 {
		utils.ErrExit("Invalid port number %d. Valid range is 0-65535", tconf.Port)
	}
}

func validateTargetSchemaFlag() {
	// we want to run this check only for import-data-to-target and import-schema commands.
	// This is not applicable for import-data-to-source-replica (validateFFDBSchemaFlag)/import-data-to-source (no ability to pass schema).
	// For import-data-file, we allow this flag and source is PG(dummy)
	if !slices.Contains([]string{SOURCE_REPLICA_DB_IMPORTER_ROLE, SOURCE_DB_IMPORTER_ROLE, IMPORT_FILE_ROLE}, importerRole) {
		if tconf.SchemaConfig != "" && sourceDBType == "postgresql" {
			utils.ErrExit("Error --target-db-schema flag is not valid for export from 'postgresql' db type")
		}
	}

	if tconf.SchemaConfig == "" {
		if tconf.TargetDBType == YUGABYTEDB || tconf.TargetDBType == YUGABYTEDB_AMP {
			// yb-amp follows the PostgreSQL/YugabyteDB convention: default
			// schema is "public" and PG-source schemas are preserved.
			tconf.SchemaConfig = YUGABYTEDB_DEFAULT_SCHEMA
		} else if tconf.TargetDBType == ORACLE {
			tconf.SchemaConfig = tconf.User
		}
		return
	} else if tconf.TargetDBType != POSTGRESQL && tconf.TargetDBType != YUGABYTEDB_AMP {
		splits := strings.Split(tconf.SchemaConfig, ",")
		if len(splits) > 1 {
			utils.ErrExit("Error --target-db-schema flag can only contain one schema name. Got: %s", tconf.SchemaConfig)
		}
	}
}

// validateTargetDBTypeFlag ensures --target-db-type holds a value that is
// supported for import-to-target. Fall-forward / fall-back roles derive
// TargetDBType from the source DB type (oracle/postgresql/yugabytedb), so
// this guardrail only applies to the target-import roles.
func validateTargetDBTypeFlag() {
	if importerRole != TARGET_DB_IMPORTER_ROLE && importerRole != IMPORT_FILE_ROLE {
		return
	}
	switch tconf.TargetDBType {
	case YUGABYTEDB, YUGABYTEDB_AMP:
		// supported target types for import-to-target
	default:
		utils.ErrExit("unsupported --target-db-type %q for import to target. Supported values: %s, %s",
			tconf.TargetDBType, YUGABYTEDB, YUGABYTEDB_AMP)
	}
}

func getTargetPassword(cmd *cobra.Command) {
	var err error
	tconf.Password, err = getPassword(cmd, "target-db-password", "TARGET_DB_PASSWORD")
	if err != nil {
		utils.ErrExit("error in getting target-db-password: %w", err)
	}
}

func getSourceReplicaDBPassword(cmd *cobra.Command) {
	var err error
	tconf.Password, err = getPassword(cmd, "source-replica-db-password", "SOURCE_REPLICA_DB_PASSWORD")
	if err != nil {
		utils.ErrExit("error while getting source-replica-db-password: %w", err)
	}
}

func getSourceDBPassword(cmd *cobra.Command) {
	var err error
	tconf.Password, err = getPassword(cmd, "source-db-password", "SOURCE_DB_PASSWORD")
	if err != nil {
		utils.ErrExit("error while getting source-db-password: %w", err)
	}
}

func validateImportObjectsFlag(importObjectsString string, flagName string) {
	if importObjectsString == "" {
		return
	}

	availableObjects := utils.GetSchemaObjectList(GetSourceDBTypeFromMSR())
	objectList := utils.CsvStringToSlice(importObjectsString)
	for _, object := range objectList {
		if !slices.Contains(availableObjects, strings.ToUpper(object)) {
			utils.ErrExit("Error Invalid object type '%v' specified wtih --%s flag. Supported object types are: %v", object, flagName, availableObjects)
		}
	}
}

func checkOrSetDefaultTargetSSLMode() {
	tconf.SSLMode = strings.ToLower(tconf.SSLMode) // normalize before comparing

	if tconf.SSLMode == "" {
		tconf.SSLMode = constants.PREFER
		return
	}

	var sslModes []string
	if importerRole == TARGET_DB_IMPORTER_ROLE || importerRole == IMPORT_FILE_ROLE {
		sslModes = supportedSSLModesOnTargetForImport
	} else if importerRole == SOURCE_REPLICA_DB_IMPORTER_ROLE || importerRole == SOURCE_DB_IMPORTER_ROLE {
		sslModes = supportedSSLModesOnSourceOrSourceReplica
	} // there should be no other else case

	if !slices.Contains(sslModes, tconf.SSLMode) {
		utils.ErrExit("Invalid sslmode %q. Required one of [%s]", tconf.SSLMode, strings.Join(sslModes, ", "))
	}
}

func registerFlagsForTarget(cmd *cobra.Command) {
	cmd.Flags().Int64Var(&batchSizeInNumRows, "batch-size", 0,
		fmt.Sprintf("Size of batches in the number of rows generated for ingestion during import. default(%d)", DEFAULT_BATCH_SIZE_YUGABYTEDB))
	cmd.Flags().IntVar(&tconf.Parallelism, "parallel-jobs", 0,
		"number of parallel jobs to use while importing data. By default, voyager will try to determine the total "+
			"number of cores N across the cluster and use N/4 as parallel jobs. "+
			"If core detection fails, it estimates N using the number of nodes * 16 assumed vCPUs per node. "+
			"Any value less than 1 reverts to the default calculation.")

	cmd.Flags().Var(&tconf.AdaptiveParallelismMode, "adaptive-parallelism",
		"Adapt parallelism based on the resource usage (CPU, memory) of the target YugabyteDB cluster."+
			"\n"+
			"Specify the mode for adaptive parallelism behavior: disabled, balanced, aggressive "+
			"(default: balanced for YugabyteDB, disabled for YugabyteDB AMP)"+
			"\n"+
			"\tbalanced: Operate with moderate thresholds. Recommended to be used when there are other workloads running on the cluster.\n"+
			"\taggressive: Operate with aggressive max-CPU thresholds for better performance. Recommended to be used when there are no other workloads running on the cluster.\n"+
			"\tdisabled: Disable adaptive parallelism.")

	cmd.Flags().IntVar(&tconf.MaxParallelism, "adaptive-parallelism-max", 0,
		"number of max parallel jobs to use while importing data when adaptive parallelism is enabled. "+
			"By default, voyager will try to determine the total number of cores N and use N as the max parallel jobs.")
	BoolVar(cmd.Flags(), &skipReplicationChecks, "skip-replication-checks", false,
		"It is NOT recommended to have any form of replication (CDC/xCluster) running on the target YugabyteDB cluster during data import. "+
			"If detected, data import is aborted. Use this flag to turn off the checks and continue importing data.")
	BoolVar(cmd.Flags(), &skipNodeHealthChecks, "skip-node-health-checks", false,
		"Skips the monitoring of the Node status checks on the target YugabyteDB cluster. "+
			"By default, voyager will keep monitoring the node status to keep the cluster stable.")
	BoolVar(cmd.Flags(), &skipDiskUsageHealthChecks, "skip-disk-usage-health-checks", false,
		"Skips the monitoring of the disk usage on the target YugabyteDB cluster. "+
			"By default, voyager will keep monitoring the disk usage on the nodes to keep the cluster stable.")

	cmd.Flags().StringVar(&tconf.OnPrimaryKeyConflictAction, "on-primary-key-conflict", "ERROR-POLICY",
		`Action to take on primary key conflict during data import during snapshot phase.
Supported values:
ERROR-POLICY(default): Handle error as per configured error-policy, if any primary key conflict is encountered.
IGNORE		: Skip rows where the primary key already exists and continue importing remaining data.`)

	cmd.Flags().MarkHidden("skip-disk-usage-health-checks")
	cmd.Flags().MarkHidden("skip-node-health-checks")
}

func registerFlagsForSourceAndSourceReplica(cmd *cobra.Command) {
	cmd.Flags().Int64Var(&batchSizeInNumRows, "batch-size", 0,
		fmt.Sprintf("Size of batches in the number of rows generated for ingestion during import. default: ORACLE(%d), POSTGRESQL(%d)", DEFAULT_BATCH_SIZE_ORACLE, DEFAULT_BATCH_SIZE_POSTGRESQL))
	cmd.Flags().IntVar(&tconf.Parallelism, "parallel-jobs", 0,
		"number of parallel jobs to use while importing data. default: For PostgreSQL(voyager will try if it can determine the total "+
			"number of cores N and use N/2 as parallel jobs else it will fall back to 8) and Oracle(16). "+
			"Any value less than 1 reverts to the default calculation.")
}

func validateBatchSizeFlag(numLinesInASplit int64) {
	if batchSizeInNumRows <= 0 {
		if tconf.TargetDBType == ORACLE {
			batchSizeInNumRows = DEFAULT_BATCH_SIZE_ORACLE
		} else if tconf.TargetDBType == POSTGRESQL {
			batchSizeInNumRows = DEFAULT_BATCH_SIZE_POSTGRESQL
		} else {
			batchSizeInNumRows = DEFAULT_BATCH_SIZE_YUGABYTEDB
		}
		return
	}

	var defaultBatchSize int64
	if tconf.TargetDBType == ORACLE {
		defaultBatchSize = DEFAULT_BATCH_SIZE_ORACLE
	} else if tconf.TargetDBType == POSTGRESQL {
		defaultBatchSize = DEFAULT_BATCH_SIZE_POSTGRESQL
	} else {
		defaultBatchSize = DEFAULT_BATCH_SIZE_YUGABYTEDB
	}

	// TODO: we might want to lift this restriction for non-transactional COPY (depends on testing of --batch-size flag)
	if numLinesInASplit > defaultBatchSize {
		utils.ErrExit("Error invalid batch size %v. The batch size cannot be greater than %v", numLinesInASplit, defaultBatchSize)
	}
}

func validateFFDBSchemaFlag() {
	if tconf.SchemaConfig == "" && tconf.TargetDBType == ORACLE {
		utils.ErrExit("Error --source-replica-db-schema flag is mandatory for import data to source-replica")
	}
}

// defaultAdaptiveParallelismMode returns the adaptive-parallelism mode to use when the
// user did NOT pass --adaptive-parallelism. Adaptive parallelism relies on the YugabyteDB
// cluster control API (yb_servers(), tserver metrics), so it is the recommended default
// (Balanced) ONLY for a real YugabyteDB target. Every other target — yb-amp (stateless
// PG17 compute) and the PostgreSQL fall-forward/fall-back targets — has no such API, so it
// defaults to Disabled; --parallel-jobs controls import parallelism there.
//
// Defaulting non-YB targets to Disabled is also what lets a user pass --parallel-jobs for
// them without having to also pass --adaptive-parallelism disabled (validateParallelismFlags
// only conflicts --parallel-jobs with an *enabled* adaptive mode).
func defaultAdaptiveParallelismMode(targetDBType string) types.AdaptiveParallelismMode {
	if targetDBType == YUGABYTEDB {
		return types.BalancedAdaptiveParallelismMode
	}
	return types.DisabledAdaptiveParallelismMode
}

func validateParallelismFlags(cmd *cobra.Command) {
	// yb-amp has no YB cluster control API, so adaptive parallelism cannot work
	// there. Reject any explicit request for it (CLI or config — Flags().Changed()
	// is true in both, since config values are applied via Flags().Set()), pointing
	// the user to --parallel-jobs. An explicit `--adaptive-parallelism disabled` is
	// fine (not IsEnabled()).
	if tconf.TargetDBType == YUGABYTEDB_AMP {
		if cmd.Flags().Changed("adaptive-parallelism") && tconf.AdaptiveParallelismMode.IsEnabled() {
			utils.ErrExit("adaptive parallelism is only supported for YugabyteDB targets. For --target-db-type %s, use --parallel-jobs to control import parallelism.", YUGABYTEDB_AMP)
		}
		if cmd.Flags().Changed("adaptive-parallelism-max") {
			utils.ErrExit("--adaptive-parallelism-max is only supported for YugabyteDB targets. For --target-db-type %s, use --parallel-jobs.", YUGABYTEDB_AMP)
		}
	}
	if tconf.AdaptiveParallelismMode.IsEnabled() {
		if tconf.Parallelism > 0 {
			utils.ErrExit("Error --parallel-jobs flag cannot be used when adaptive-parallelism is enabled (balanced/aggressive). If you wish to set the number of parallel jobs explicitly, disable adaptive parallelism using --adaptive-parallelism disabled")
		}
	}
	if tconf.MaxParallelism > 0 {
		if !tconf.AdaptiveParallelismMode.IsEnabled() {
			utils.ErrExit("Error --adaptive-parallelism-max flag can only be used when adaptive-parallelism is enabled (balanced/aggressive)")
		}
	}

}

func validateTruncateTablesFlag() error {
	if truncateTables && !startClean {
		return goerrors.Errorf("Error --truncate-tables true can only be specified along with --start-clean true")
	}
	return nil
}

var onPrimaryKeyConflictActions = []string{
	constants.PRIMARY_KEY_CONFLICT_ACTION_ERROR_POLICY,
	constants.PRIMARY_KEY_CONFLICT_ACTION_IGNORE,
	// constants.PRIMARY_KEY_CONFLICT_ACTION_UPDATE,
}

func validateOnPrimaryKeyConflictFlag() error {
	log.Infof("passed value for --on-primary-key-conflict: %s", tconf.OnPrimaryKeyConflictAction)
	tconf.OnPrimaryKeyConflictAction = strings.ToUpper(tconf.OnPrimaryKeyConflictAction)

	// flag only applicable for import-data-to-target and import-data-file commands
	// ignore for import-data-to-source-replica and import-data-to-source commands
	if !isPrimaryKeyConflictModeValid() {
		return nil
	}

	// Check if the provided OnPrimaryKeyConflictAction is valid
	if tconf.OnPrimaryKeyConflictAction != "" {
		if !slices.Contains(onPrimaryKeyConflictActions, tconf.OnPrimaryKeyConflictAction) {
			return goerrors.Errorf("invalid value for --on-primary-key-conflict. Allowed values are: [%s]", strings.Join(onPrimaryKeyConflictActions, ", "))
		}
	}

	// ensure that OnPrimaryKeyConflictAction is not changed in case of resumption
	/*
		Most of our cmd validations(including this) run in PreRun phase
		which can happen before the migration status record or the metaDB is created.
		For example, in case of import data file command
	*/
	if !bool(startClean) && metaDBIsCreated(exportDir) {
		msr, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			return fmt.Errorf("error getting migration status record: %w", err)
		} else if msr == nil {
			return goerrors.Errorf("migration status record is nil, cannot validate --on-primary-key-conflict flag")
		}

		if msr.OnPrimaryKeyConflictAction != "" && msr.OnPrimaryKeyConflictAction != tconf.OnPrimaryKeyConflictAction {
			return goerrors.Errorf("--on-primary-key-conflict flag cannot be changed after the import has started. "+
				"Previous value was %s, current value is %s", msr.OnPrimaryKeyConflictAction, tconf.OnPrimaryKeyConflictAction)
		}
	}

	// --enable-upsert true and on-primary-key-conflict ignore is conflicting, therefore we only allow it if on-primary-key-conflict is set to ERROR-POLICY
	if tconf.EnableUpsert && tconf.OnPrimaryKeyConflictAction != constants.PRIMARY_KEY_CONFLICT_ACTION_ERROR_POLICY {
		return goerrors.Errorf("--enable-upsert=true can only be used with --on-primary-key-conflict=ERROR-POLICY")
	}

	if tconf.OnPrimaryKeyConflictAction == constants.PRIMARY_KEY_CONFLICT_ACTION_IGNORE {
		utils.PrintAndLogf("Note: --on-primary-key-conflict is set as 'IGNORE'. Rows with existing primary keys will be skipped during import.")
	}
	return nil
}
