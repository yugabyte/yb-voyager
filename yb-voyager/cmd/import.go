/*
Copyright (c) YugaByte, Inc.

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
	"syscall"

	"golang.org/x/exp/slices"
	"golang.org/x/term"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var sourceDBType string
var enableOrafce bool

// target struct will be populated by CLI arguments parsing
var target tgtdb.Target

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
	checkOrSetDefaultTargetSSLMode()
	validateTargetPortRange()
	if target.TableList != "" && target.ExcludeTableList != "" {
		utils.ErrExit("Error: Only one of --table-list and --exclude-table-list are allowed")
	}
	validateTableListFlag(target.TableList, "table-list")
	validateTableListFlag(target.ExcludeTableList, "exclude-table-list")
	if target.ImportObjects != "" && target.ExcludeImportObjects != "" {
		utils.ErrExit("Error: Only one of --object-list and --exclude-object-list are allowed")
	}
	validateImportObjectsFlag(target.ImportObjects, "object-list")
	validateImportObjectsFlag(target.ExcludeImportObjects, "exclude-object-list")
	validateTargetSchemaFlag()
	// For beta2.0 release (and onwards until further notice)
	if disableTransactionalWrites {
		fmt.Println("WARNING: The --disable-transactional-writes feature is in the experimental phase, not for production use case.")
	}
	validateBatchSizeFlag(numLinesInASplit)
	validateTargetPassword(cmd)
}

func registerCommonImportFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&target.Host, "target-db-host", "127.0.0.1",
		"host on which the YugabyteDB server is running")

	cmd.Flags().IntVar(&target.Port, "target-db-port", YUGABYTEDB_YSQL_DEFAULT_PORT,
		"port on which the YugabyteDB YSQL API is running")

	cmd.Flags().StringVar(&target.User, "target-db-user", "",
		"username with which to connect to the target YugabyteDB server")
	cmd.MarkFlagRequired("target-db-user")

	cmd.Flags().StringVar(&target.Password, "target-db-password", "",
		"password with which to connect to the target YugabyteDB server")

	cmd.Flags().StringVar(&target.DBName, "target-db-name", YUGABYTEDB_DEFAULT_DATABASE,
		"name of the database on the target YugabyteDB server on which import needs to be done")

	cmd.Flags().StringVar(&target.Schema, "target-db-schema", YUGABYTEDB_DEFAULT_SCHEMA,
		"target schema name in YugabyteDB (Note: works only for source as Oracle and MySQL, in case of PostgreSQL you can ALTER schema name post import)")

	// TODO: SSL related more args might come. Need to explore SSL part completely.
	cmd.Flags().StringVar(&target.SSLCertPath, "target-ssl-cert", "",
		"provide target SSL Certificate Path")

	cmd.Flags().StringVar(&target.SSLMode, "target-ssl-mode", "prefer",
		"specify the target SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full")

	cmd.Flags().StringVar(&target.SSLKey, "target-ssl-key", "",
		"target SSL Key Path")

	cmd.Flags().StringVar(&target.SSLRootCert, "target-ssl-root-cert", "",
		"target SSL Root Certificate Path")

	cmd.Flags().StringVar(&target.SSLCRL, "target-ssl-crl", "",
		"target SSL Root Certificate Revocation List (CRL)")

	cmd.Flags().BoolVar(&startClean, "start-clean", false,
		"delete all existing schema objects before importing schema. Delete all existing data before importing the data")

	cmd.Flags().BoolVar(&target.VerboseMode, "verbose", false,
		"verbose mode for some extra details during execution of command")

	cmd.Flags().BoolVar(&target.ContinueOnError, "continue-on-error", false,
		"If set, this flag will ignore errors and continue with the import")
}

func registerImportDataFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(&disablePb, "disable-pb", false,
		"true - to disable progress bar during data import (default false)")
	cmd.Flags().StringVar(&target.ExcludeTableList, "exclude-table-list", "",
		"list of tables to exclude while importing data (ignored if --table-list is used)")
	cmd.Flags().StringVar(&target.TableList, "table-list", "",
		"list of tables to import data")
	cmd.Flags().Int64Var(&numLinesInASplit, "batch-size", DEFAULT_BATCH_SIZE,
		"maximum number of rows in each batch generated during import.")
	cmd.Flags().IntVar(&parallelImportJobs, "parallel-jobs", -1,
		"number of parallel copy command jobs to target database. "+
			"By default, voyager will try if it can determine the total number of cores N and use N/2 as parallel jobs. "+
			"Otherwise, it fall back to using twice the number of nodes in the cluster")
	cmd.Flags().BoolVar(&enableUpsert, "enable-upsert", true,
		"true - to enable UPSERT mode on target tables\n"+
			"false - to disable UPSERT mode on target tables")
	cmd.Flags().BoolVar(&usePublicIp, "use-public-ip", false,
		"true - to use the public IPs of the nodes to distribute --parallel-jobs uniformly for data import (default false)\n"+
			"Note: you might need to configure database to have public_ip available by setting server-broadcast-addresses.\n"+
			"Refer: https://docs.yugabyte.com/latest/reference/configuration/yb-tserver/#server-broadcast-addresses")
	cmd.Flags().StringVar(&targetEndpoints, "target-endpoints", "",
		"comma separated list of node's endpoint to use for parallel import of data(default is to use all the nodes in the cluster).\n"+
			"For example: \"host1:port1,host2:port2\" or \"host1,host2\"\n"+
			"Note: use-public-ip flag will be ignored if this is used.")
	// flag existence depends on fix of this gh issue: https://github.com/yugabyte/yugabyte-db/issues/12464
	cmd.Flags().BoolVar(&disableTransactionalWrites, "disable-transactional-writes", false,
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
	cmd.Flags().StringVar(&target.ImportObjects, "object-list", "",
		"list of schema object types to include while importing schema")
	cmd.Flags().StringVar(&target.ExcludeImportObjects, "exclude-object-list", "",
		"list of schema object types to exclude while importing schema (ignored if --object-list is used)")
	cmd.Flags().BoolVar(&importObjectsInStraightOrder, "straight-order", false,
		"If set, objects will be imported in the order specified with the --object-list flag (default false)")
	cmd.Flags().BoolVar(&flagPostImportData, "post-import-data", false,
		"If set, creates indexes, foreign-keys, and triggers in target db")
	cmd.Flags().BoolVar(&target.IgnoreIfExists, "ignore-exist", false,
		"true - to ignore errors if object already exists\n"+
			"false - throw those errors to the standard output (default false)")
	cmd.Flags().BoolVar(&flagRefreshMViews, "refresh-mviews", false,
		"If set, refreshes the materialised views on target during post import data phase (default false")
	cmd.Flags().BoolVar(&enableOrafce, "enable-orafce", true,
		"true - to enable Orafce extension on target(if source db type is Oracle)")
}

func validateTargetPortRange() {
	if target.Port < 0 || target.Port > 65535 {
		utils.ErrExit("Invalid port number %d. Valid range is 0-65535", target.Port)
	}
}

func validateTargetSchemaFlag() {
	if target.Schema == "" {
		return
	}
	if target.Schema != YUGABYTEDB_DEFAULT_SCHEMA && sourceDBType == "postgresql" {
		utils.ErrExit("Error: --target-db-schema flag is not valid for export from 'postgresql' db type")
	}
}

func validateTargetPassword(cmd *cobra.Command) {
	if cmd.Flags().Changed("target-db-password") {
		return
	}
	fmt.Print("Password to connect to target:")
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		utils.ErrExit("read password: %v", err)
		return
	}
	fmt.Print("\n")
	target.Password = string(bytePassword)
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
	if target.SSLMode == "" {
		target.SSLMode = "prefer"
	} else if target.SSLMode != "disable" && target.SSLMode != "prefer" && target.SSLMode != "require" && target.SSLMode != "verify-ca" && target.SSLMode != "verify-full" {
		utils.ErrExit("Invalid sslmode %q. Required one of [disable, allow, prefer, require, verify-ca, verify-full]", target.SSLMode)
	}
}

func validateBatchSizeFlag(numLinesInASplit int64) {
	if numLinesInASplit > 20000 {
		utils.ErrExit("Error: Invalid batch size %v. The batch size cannot be greater than %v", numLinesInASplit, DEFAULT_BATCH_SIZE)
	}
}
