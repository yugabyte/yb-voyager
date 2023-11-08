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

	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var importDataToSourceCmd = &cobra.Command{
	Use:   "source",
	Short: "This command will set up and import data into fall back database",
	Long:  `This command connects to the fall back database using the parameters provided and starts the importing process.`,

	Run: func(cmd *cobra.Command, args []string) {
		validateMetaDBCreated()
		importType = SNAPSHOT_AND_CHANGES
		importerRole = FB_DB_IMPORTER_ROLE
		err := initTargetConfFromSourceConf()
		if err != nil {
			utils.ErrExit("failed to setup target conf from source conf in MSR: %v", err)
		}
		validateFFDBSchemaFlag()
		importDataCmd.PreRun(cmd, args)
		importDataCmd.Run(cmd, args)
	},
}

func init() {
	importDataToCmd.AddCommand(importDataToSourceCmd)
	registerCommonGlobalFlags(importDataToSourceCmd)
	registerCommonImportFlags(importDataToSourceCmd)
	registerSourceDBAsTargetConnFlags(importDataToSourceCmd)
	registerImportDataCommonFlags(importDataToSourceCmd)
	hideImportFlagsInFallForwardOrBackCmds(importDataToSourceCmd)
}

func initTargetConfFromSourceConf() error {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("get migration status record: %v", err)
	}
	sconf := msr.SourceDBConf
	tconf.TargetDBType = sconf.DBType
	tconf.Host = sconf.Host
	tconf.Port = sconf.Port
	tconf.User = sconf.User
	tconf.DBName = sconf.DBName
	tconf.Schema = sconf.Schema
	tconf.SSLMode = sconf.SSLMode
	tconf.SSLMode = sconf.SSLMode
	tconf.SSLCertPath = sconf.SSLCertPath
	tconf.SSLKey = sconf.SSLKey
	tconf.SSLRootCert = sconf.SSLRootCert
	tconf.SSLCRL = sconf.SSLCRL
	tconf.SSLQueryString = sconf.SSLQueryString
	tconf.DBSid = sconf.DBSid
	tconf.TNSAlias = sconf.TNSAlias
	tconf.OracleHome = sconf.OracleHome
	tconf.Uri = sconf.Uri
	return nil
}
