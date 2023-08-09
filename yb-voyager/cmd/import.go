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
	"os"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"
	"golang.org/x/term"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var sourceDBType string
var enableOrafce bool

// tconf struct will be populated by CLI arguments parsing
var tconf tgtdb.TargetConf

var tdb tgtdb.TargetDB

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "Import schema and data from compatible source database(Oracle, MySQL, PostgreSQL)",
	Long:  `Import has various sub-commands i.e. import schema and import data to import into YugabyteDB from various compatible source databases(Oracle, MySQL, PostgreSQL).`,
}

func init() {
	rootCmd.AddCommand(importCmd)
}

// If any changes are made to this function, verify if the change is also needed for importDataFileCommand.go
func validateImportFlags(cmd *cobra.Command) {
	validateExportDirFlag()
	validateTargetDBType()
	checkOrSetDefaultTargetSSLMode()
	validateTargetPortRange()
	if tconf.TableList != "" && tconf.ExcludeTableList != "" {
		utils.ErrExit("Error: Only one of --table-list and --exclude-table-list are allowed")
	}
	validateTableListFlag(tconf.TableList, "table-list")
	validateTableListFlag(tconf.ExcludeTableList, "exclude-table-list")
	if tconf.ImportObjects != "" && tconf.ExcludeImportObjects != "" {
		utils.ErrExit("Error: Only one of --object-list and --exclude-object-list are allowed")
	}
	validateImportObjectsFlag(tconf.ImportObjects, "object-list")
	validateImportObjectsFlag(tconf.ExcludeImportObjects, "exclude-object-list")
	validateTargetSchemaFlag()
	// For beta2.0 release (and onwards until further notice)
	if tconf.DisableTransactionalWrites {
		fmt.Println("WARNING: The --disable-transactional-writes feature is in the experimental phase, not for production use case.")
	}
	validateBatchSizeFlag(batchSize)
	validateTargetPassword(cmd)
}

func registerCommonImportFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&tconf.TargetDBType, "target-db-type", "",
		"type of the target database (oracle, yugabytedb)")

	cmd.Flags().StringVar(&tconf.Host, "target-db-host", "127.0.0.1",
		"host on which the YugabyteDB server is running")

	cmd.Flags().IntVar(&tconf.Port, "target-db-port", -1,
		"port on which the YugabyteDB YSQL API is running")

	cmd.Flags().StringVar(&tconf.User, "target-db-user", "",
		"username with which to connect to the target YugabyteDB server")
	cmd.MarkFlagRequired("target-db-user")

	cmd.Flags().StringVar(&tconf.Password, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server")

	cmd.Flags().StringVar(&tconf.DBName, "target-db-name", "",
		"name of the database on the target YugabyteDB server on which import needs to be done")

	cmd.Flags().StringVar(&tconf.DBSid, "target-db-sid", "",
		"[For Oracle Only] Oracle System Identifier (SID) that you wish to use while importing data to Oracle instances")

	cmd.Flags().StringVar(&tconf.OracleHome, "oracle-home", "",
		"[For Oracle Only] Path to set $ORACLE_HOME environment variable. tnsnames.ora is found in $ORACLE_HOME/network/admin")

	cmd.Flags().StringVar(&tconf.TNSAlias, "oracle-tns-alias", "",
		"[For Oracle Only] Name of TNS Alias you wish to use to connect to Oracle instance. Refer to documentation to learn more about configuring tnsnames.ora and aliases")

	cmd.Flags().StringVar(&tconf.Schema, "target-db-schema", "",
		"target schema name in YugabyteDB (Note: works only for source as Oracle and MySQL, in case of PostgreSQL you can ALTER schema name post import)")

	// TODO: SSL related more args might come. Need to explore SSL part completely.
	cmd.Flags().StringVar(&tconf.SSLCertPath, "target-ssl-cert", "",
		"provide target SSL Certificate Path")

	cmd.Flags().StringVar(&tconf.SSLMode, "target-ssl-mode", "prefer",
		"specify the target SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full")

	cmd.Flags().StringVar(&tconf.SSLKey, "target-ssl-key", "",
		"target SSL Key Path")

	cmd.Flags().StringVar(&tconf.SSLRootCert, "target-ssl-root-cert", "",
		"target SSL Root Certificate Path")

	cmd.Flags().StringVar(&tconf.SSLCRL, "target-ssl-crl", "",
		"target SSL Root Certificate Revocation List (CRL)")

	cmd.Flags().BoolVar(&startClean, "start-clean", false,
		"import schema: delete all existing schema objects \nimport data / import data file: starts a fresh import of data or incremental data load")

	cmd.Flags().BoolVar(&tconf.VerboseMode, "verbose", false,
		"verbose mode for some extra details during execution of command")

	cmd.Flags().BoolVar(&tconf.ContinueOnError, "continue-on-error", false,
		"If set, this flag will ignore errors and continue with the import")
}

func registerImportDataFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&disablePb, "disable-pb", false,
		"true - to disable progress bar during data import (default false)")
	cmd.Flags().StringVar(&tconf.ExcludeTableList, "exclude-table-list", "",
		"list of tables to exclude while importing data (ignored if --table-list is used)")
	cmd.Flags().StringVar(&tconf.TableList, "table-list", "",
		"list of tables to import data")
	cmd.Flags().Int64Var(&batchSize, "batch-size", -1,
		"maximum number of rows in each batch generated during import.")
	cmd.Flags().IntVar(&tconf.Parallelism, "parallel-jobs", -1,
		"number of parallel copy command jobs to target database. "+
			"By default, voyager will try if it can determine the total number of cores N and use N/2 as parallel jobs. "+
			"Otherwise, it fall back to using twice the number of nodes in the cluster")
	cmd.Flags().BoolVar(&tconf.EnableUpsert, "enable-upsert", true,
		"true - to enable UPSERT mode on target tables\n"+
			"false - to disable UPSERT mode on target tables")
	cmd.Flags().BoolVar(&tconf.UsePublicIP, "use-public-ip", false,
		"true - to use the public IPs of the nodes to distribute --parallel-jobs uniformly for data import (default false)\n"+
			"Note: you might need to configure database to have public_ip available by setting server-broadcast-addresses.\n"+
			"Refer: https://docs.yugabyte.com/latest/reference/configuration/yb-tserver/#server-broadcast-addresses")
	cmd.Flags().StringVar(&tconf.TargetEndpoints, "target-endpoints", "",
		"comma separated list of node's endpoint to use for parallel import of data(default is to use all the nodes in the cluster).\n"+
			"For example: \"host1:port1,host2:port2\" or \"host1,host2\"\n"+
			"Note: use-public-ip flag will be ignored if this is used.")
	// flag existence depends on fix of this gh issue: https://github.com/yugabyte/yugabyte-db/issues/12464
	cmd.Flags().BoolVar(&tconf.DisableTransactionalWrites, "disable-transactional-writes", false,
		"true - to disable transactional writes in tables for faster data ingestion (default false)\n"+
			"(Note: this is a interim flag until the issues related to 'yb_disable_transactional_writes' session variable are fixed. Refer: https://github.com/yugabyte/yugabyte-db/issues/12464)")
	// Hidden for beta2.0 release (and onwards until further notice).
	cmd.Flags().MarkHidden("disable-transactional-writes")

	cmd.Flags().BoolVar(&truncateSplits, "truncate-splits", true,
		"true - to truncate splits after importing\n"+
			"false - to not truncate splits after importing (required for debugging)")
	cmd.Flags().MarkHidden("truncate-splits")

	cmd.Flags().BoolVar(&liveMigration, "live-migration", false,
		"set this flag to true to enable streaming data from source to target database")

	cmd.Flags().MarkHidden("live-migration")
}

func registerImportSchemaFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&tconf.ImportObjects, "object-list", "",
		"list of schema object types to include while importing schema")
	cmd.Flags().StringVar(&tconf.ExcludeImportObjects, "exclude-object-list", "",
		"list of schema object types to exclude while importing schema (ignored if --object-list is used)")
	cmd.Flags().BoolVar(&importObjectsInStraightOrder, "straight-order", false,
		"If set, objects will be imported in the order specified with the --object-list flag (default false)")
	cmd.Flags().BoolVar(&flagPostImportData, "post-import-data", false,
		"If set, creates indexes, foreign-keys, and triggers in target db")
	cmd.Flags().BoolVar(&tconf.IgnoreIfExists, "ignore-exist", false,
		"true - to ignore errors if object already exists\n"+
			"false - throw those errors to the standard output (default false)")
	cmd.Flags().BoolVar(&flagRefreshMViews, "refresh-mviews", false,
		"If set, refreshes the materialised views on target during post import data phase (default false)")
	cmd.Flags().BoolVar(&enableOrafce, "enable-orafce", true,
		"true - to enable Orafce extension on target(if source db type is Oracle)")
}

func validateTargetPortRange() {
	if tconf.Port == -1 {
		if tconf.TargetDBType == ORACLE {
			tconf.Port = ORACLE_DEFAULT_PORT
		} else if tconf.TargetDBType == YUGABYTEDB {
			tconf.Port = YUGABYTEDB_YSQL_DEFAULT_PORT
		}
	}

	if tconf.Port < 0 || tconf.Port > 65535 {
		utils.ErrExit("Invalid port number %d. Valid range is 0-65535", tconf.Port)
	}
}

func validateTargetSchemaFlag() {
	if tconf.Schema == "" {
		return
	}
	if tconf.Schema != YUGABYTEDB_DEFAULT_SCHEMA && sourceDBType == "postgresql" {
		utils.ErrExit("Error: --target-db-schema flag is not valid for export from 'postgresql' db type")
	}
}

func validateTargetPassword(cmd *cobra.Command) {
	if cmd.Flags().Changed("target-db-password") {
		return
	}
	if os.Getenv("TARGET_DB_PASSWORD") != "" {
		tconf.Password = os.Getenv("TARGET_DB_PASSWORD")
		return
	}
	fmt.Print("Password to connect to target:")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		utils.ErrExit("read password: %v", err)
		return
	}
	fmt.Print("\n")
	tconf.Password = string(bytePassword)
}

func validateImportObjectsFlag(importObjectsString string, flagName string) {
	if importObjectsString == "" {
		return
	}
	//We cannot access sourceDBType variable at this point, but exportDir has been validated
	availableObjects := utils.GetSchemaObjectList(ExtractMetaInfo(exportDir).SourceDBType)
	objectList := utils.CsvStringToSlice(importObjectsString)
	for _, object := range objectList {
		if !slices.Contains(availableObjects, strings.ToUpper(object)) {
			utils.ErrExit("Error: Invalid object type '%v' specified wtih --%s flag. Supported object types are: %v", object, flagName, availableObjects)
		}
	}
}

func checkOrSetDefaultTargetSSLMode() {
	if tconf.SSLMode == "" {
		tconf.SSLMode = "prefer"
	} else if tconf.SSLMode != "disable" && tconf.SSLMode != "prefer" && tconf.SSLMode != "require" && tconf.SSLMode != "verify-ca" && tconf.SSLMode != "verify-full" {
		utils.ErrExit("Invalid sslmode %q. Required one of [disable, allow, prefer, require, verify-ca, verify-full]", tconf.SSLMode)
	}
}

func validateBatchSizeFlag(numLinesInASplit int64) {
	if batchSize == -1 {
		if tconf.TargetDBType == ORACLE {
			batchSize = DEFAULT_BATCH_SIZE_ORACLE
		} else {
			batchSize = DEFAULT_BATCH_SIZE_YUGABYTEDB
		}
		return
	}

	var defaultBatchSize int64
	if tconf.TargetDBType == ORACLE {
		defaultBatchSize = DEFAULT_BATCH_SIZE_ORACLE
	} else {
		defaultBatchSize = DEFAULT_BATCH_SIZE_YUGABYTEDB
	}

	if numLinesInASplit > defaultBatchSize {
		utils.ErrExit("Error: Invalid batch size %v. The batch size cannot be greater than %v", numLinesInASplit, defaultBatchSize)
	}
}

func validateTargetDBType() {
	if tconf.TargetDBType == "" {
		utils.ErrExit("Error: required flag \"target-db-type\" not set")
	}

	tconf.TargetDBType = strings.ToLower(tconf.TargetDBType)
	if !slices.Contains(supportedTargetDBTypes, tconf.TargetDBType) {
		utils.ErrExit("Error: Invalid target-db-type: %q. Supported target db types are: %s", tconf.TargetDBType, supportedTargetDBTypes)
	}
}
