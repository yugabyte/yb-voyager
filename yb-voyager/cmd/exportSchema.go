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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"

	"github.com/spf13/cobra"
)

var exportSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "This command is used to export the schema from source database into .sql files",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
	},

	Run: func(cmd *cobra.Command, args []string) {
		exportSchema()
	},
}

func exportSchema() {
	if schemaIsExported(exportDir) {
		if startClean {
			proceed := utils.AskPrompt(
				"CAUTION: Using --start-clean will overwrite any manual changes done to the " +
					"exported schema. Do you want to proceed")
			if !proceed {
				return
			}
			for _, dirName := range []string{"schema", "reports", "temp", "metainfo/schema"} {
				utils.CleanDir(filepath.Join(exportDir, dirName))
			}
			clearSchemaIsExported(exportDir)
		} else {
			fmt.Fprintf(os.Stderr, "Schema is already exported. "+
				"Use --start-clean flag to export schema again -- "+
				"CAUTION: Using --start-clean will overwrite any manual changes done to the exported schema.\n")
			return
		}
	}
	utils.PrintAndLog("export of schema for source type as '%s'\n", source.DBType)
	// Check connection with source database.
	err := source.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the source db: %s", err)
	}
	source.DB().CheckRequiredToolsAreInstalled()
	sourceDBVersion := source.DB().GetVersion()

	fmt.Printf("%s version: %s\n", source.DBType, sourceDBVersion)

	CreateMigrationProjectIfNotExists(source.DBType, exportDir)
	source.DB().ExportSchema(exportDir)
	utils.PrintAndLog("\nExported schema files created under directory: %s\n", exportDir+"/schema")
	//add check for diagnostic flag here...
	utils.InitJSON(exportDir)
	payload := utils.GetPayload()
	payload.SourceDBType = source.DBType
	payload.SourceDBVersion = sourceDBVersion
	utils.PackPayload(exportDir)
	utils.SendPayload()
	setSchemaIsExported(exportDir)
}

func init() {
	exportCmd.AddCommand(exportSchemaCmd)

	// Hide num-connections flag from help description from Export Schema command
	exportSchemaCmd.SetHelpFunc(func(command *cobra.Command, strings []string) {
		command.Flags().MarkHidden("parallel-jobs")
		command.Parent().HelpFunc()(command, strings)
	})

}

func schemaIsExported(exportDir string) bool {
	flagFilePath := exportDir + "/metainfo/flags/exportSchemaDone"
	_, err := os.Stat(flagFilePath)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false
		}
		utils.ErrExit("failed to check if schema import is already done: %s", err)
	}
	return true
}

func setSchemaIsExported(exportDir string) {
	flagFilePath := exportDir + "/metainfo/flags/exportSchemaDone"
	fh, err := os.Create(flagFilePath)
	if err != nil {
		utils.ErrExit("create %q: %s", flagFilePath, err)
	}
	fh.Close()
}

func clearSchemaIsExported(exportDir string) {
	flagFilePath := exportDir + "/metainfo/flags/exportSchemaDone"
	os.Remove(flagFilePath)
}
