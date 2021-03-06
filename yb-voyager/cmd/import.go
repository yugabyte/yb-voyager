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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

	"github.com/spf13/cobra"
)

var importMode string
var sourceDBType string

// target struct will be populated by CLI arguments parsing
var target tgtdb.Target

var importCmd = &cobra.Command{
	Use:   "import",
	Short: "import schema and data from compatible source database(Oracle, Mysql, Postgres)",
	Long:  ``,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateImportFlags()
		sourceDBType = ExtractMetaInfo(exportDir).SourceDBType
		markImportFlagsRequired(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		target.ImportMode = true
		importSchema()
		importData()
	},
}

func init() {
	rootCmd.AddCommand(importCmd)
	registerCommonImportFlags(importCmd)
	importCmd.Flags().BoolVar(&disablePb, "disable-pb", false,
		"true - to disable progress bar during data import (default false)")
}

func validateImportFlags() {
	validateExportDirFlag()
	checkOrSetDefaultTargetSSLMode()
	validateTargetPortRange()

	validateTableListFlag(target.TableList)
	validateTargetSchemaFlag()
}

func registerCommonImportFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(&target.Host, "target-db-host", "127.0.0.1",
		"Host on which the YugabyteDB server is running")

	cmd.Flags().IntVar(&target.Port, "target-db-port", YUGABYTEDB_DEFAULT_PORT,
		"Port on which the YugabyteDB database is running")

	cmd.Flags().StringVar(&target.User, "target-db-user", "",
		"Username with which to connect to the target YugabyteDB server")
	cmd.MarkFlagRequired("target-db-user")

	cmd.Flags().StringVar(&target.Password, "target-db-password", "",
		"Password with which to connect to the target YugabyteDB server")
	cmd.MarkFlagRequired("target-db-password")

	cmd.Flags().StringVar(&target.DBName, "target-db-name", YUGABYTEDB_DEFAULT_DATABASE,
		"Name of the database on the target YugabyteDB server on which import needs to be done")

	cmd.Flags().StringVar(&target.Schema, "target-db-schema", YUGABYTEDB_DEFAULT_SCHEMA,
		"target schema name in YugabyteDB (Note: works only for source as Oracle and MySQL, in case of PostgreSQL you can ALTER schema name post import)")

	// TODO: SSL related more args might come. Need to explore SSL part completely.
	cmd.Flags().StringVar(&target.SSLCertPath, "target-ssl-cert", "",
		"provide target SSL Certificate Path")

	cmd.Flags().StringVar(&target.SSLMode, "target-ssl-mode", "prefer",
		"specify the target SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full")

	cmd.Flags().StringVar(&target.SSLKey, "target-ssl-key", "",
		"provide SSL Key Path")

	cmd.Flags().StringVar(&target.SSLRootCert, "target-ssl-root-cert", "",
		"provide SSL Root Certificate Path")

	cmd.Flags().StringVar(&target.SSLCRL, "target-ssl-crl", "",
		"provide SSL Root Certificate Revocation List (CRL)")

	cmd.Flags().StringVar(&migrationMode, "migration-mode", "offline",
		"mode can be offline | online(applicable only for data migration)")

	cmd.Flags().BoolVar(&startClean, "start-clean", false,
		"delete all existing database-objects/table-rows to start from zero")

	cmd.Flags().Int64Var(&numLinesInASplit, "batch-size", 100000,
		"Maximum size of each batch import ")
	cmd.Flags().IntVar(&parallelImportJobs, "parallel-jobs", -1,
		"Number of parallel copy command jobs. default: -1 means number of servers in the Yugabyte cluster")

	cmd.Flags().BoolVar(&target.ImportIndexesAfterData, "import-indexes-after-data", true,
		"false - import indexes before data\n"+
			"true - create index after data i.e. index backfill")

	cmd.Flags().BoolVar(&target.VerboseMode, "verbose", false,
		"verbose mode for some extra details during execution of command")

	cmd.Flags().BoolVar(&target.ContinueOnError, "continue-on-error", false,
		"false - stop the execution in case of errors(default false)\n"+
			"true - to ignore errors and continue")

	cmd.Flags().BoolVar(&target.IgnoreIfExists, "ignore-exist", false,
		"true - to ignore errors if object already exists\n"+
			"false - throw those errors to the standard output (default false)")

	cmd.Flags().StringVar(&target.TableList, "table-list", "",
		"list of the tables to import data(Note: works only for import data command)")

	cmd.Flags().StringVar(&importMode, "mode", "",
		"By default the data migration mode is offline. Use '--mode online' to change the mode to online migration")

	cmd.Flags().BoolVar(&usePublicIp, "use-public-ip", false,
		"true - to use the public IPs of the nodes to distribute --parallel-jobs uniformly for data import (default false)\n"+
			"Note: you might need to configure database to have public_ip available by setting server-broadcast-addresses.\n"+
			"Refer: https://docs.yugabyte.com/latest/reference/configuration/yb-tserver/#server-broadcast-addresses")

	cmd.Flags().StringVar(&targetEndpoints, "target-endpoints", "",
		"comma separated list of node's endpoint to use for parallel import of data(default is to use all the nodes in the cluster).\n"+
			"For example: \"host1:port1,host2:port2\" or \"host1,host2\"\n"+
			"Note: use-public-ip flag will be ignored if this is used.")

	cmd.Flags().BoolVar(&enableUpsert, "enable-upsert", false,
		"true - to enable upsert for insert in target tables (default false)")

	// flag existence depends on fix of this gh issue: https://github.com/yugabyte/yugabyte-db/issues/12464
	cmd.Flags().BoolVar(&disableTransactionalWrites, "disable-transactional-writes", false,
		"true - to disable transactional writes in tables for faster data ingestion (default false)\n"+
			"(Note: this is a interim flag until the issues related to 'yb_disable_transactional_writes' session variable are fixed. Refer: https://github.com/yugabyte/yugabyte-db/issues/12464)")
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

func checkOrSetDefaultTargetSSLMode() {
	if target.SSLMode == "" {
		target.SSLMode = "prefer"
	} else if target.SSLMode != "disable" && target.SSLMode != "prefer" && target.SSLMode != "require" && target.SSLMode != "verify-ca" && target.SSLMode != "verify-full" {
		utils.ErrExit("Invalid sslmode %q. Required one of [disable, allow, prefer, require, verify-ca, verify-full]", target.SSLMode)
	}
}

func markImportFlagsRequired(cmd *cobra.Command) {
	switch sourceDBType {
	case ORACLE, MYSQL:
		cmd.MarkPersistentFlagRequired("target-db-schema")
	}
}
