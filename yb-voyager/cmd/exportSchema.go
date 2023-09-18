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
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var exportSchemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "This command is used to export the schema from source database into .sql files",
	Long:  ``,

	PreRun: func(cmd *cobra.Command, args []string) {
		setExportFlagsDefaults()
		validateExportFlags(cmd, SOURCE_DB_EXPORTER_ROLE)
		markFlagsRequired(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		var err error
		metaDB, err = metadb.NewMetaDB(exportDir)
		if err != nil {
			utils.ErrExit("Failed to initialize meta db: %s", err)
		}
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
	defer source.DB().Disconnect()
	checkSourceDBCharset()
	source.DB().CheckRequiredToolsAreInstalled()
	sourceDBVersion := source.DB().GetVersion()

	utils.PrintAndLog("%s version: %s\n", source.DBType, sourceDBVersion)

	CreateMigrationProjectIfNotExists(source.DBType, exportDir)
	err = retrieveMigrationUUID(exportDir)
	if err != nil {
		utils.ErrExit("failed to get migration UUID: %w", err)
	}
	source.DB().ExportSchema(exportDir)
	utils.PrintAndLog("\nExported schema files created under directory: %s\n", filepath.Join(exportDir, "schema"))

	payload := callhome.GetPayload(exportDir, migrationUUID)
	payload.SourceDBType = source.DBType
	payload.SourceDBVersion = sourceDBVersion
	callhome.PackAndSendPayload(exportDir)

	setSchemaIsExported(exportDir)

	miginfo := &MigInfo{
		SourceDBType:    source.DBType,
		SourceDBName:    source.DBName,
		SourceDBSchema:  source.Schema,
		SourceDBVersion: source.DB().GetVersion(),
		SourceDBSid:     source.DBSid,
		SourceTNSAlias:  source.TNSAlias,
		exportDir:       exportDir,
	}
	err = SaveMigInfo(miginfo)
	if err != nil {
		utils.ErrExit("unable to save migration info: %s", err)
	}
}

func init() {
	exportCmd.AddCommand(exportSchemaCmd)
	registerCommonGlobalFlags(exportSchemaCmd)
	registerCommonExportFlags(exportSchemaCmd)
	registerSourceDBConnFlags(exportSchemaCmd)
	exportSchemaCmd.Flags().BoolVar(&source.UseOrafce, "use-orafce", true,
		"enable using orafce extension in export schema")

	exportSchemaCmd.Flags().BoolVar(&source.CommentsOnObjects, "comments-on-objects", false,
		"enable export of comments associated with database objects (default false)")
}

func schemaIsExported(exportDir string) bool {
	flagFilePath := filepath.Join(exportDir, "metainfo", "flags", "exportSchemaDone")
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
	flagFilePath := filepath.Join(exportDir, "metainfo", "flags", "exportSchemaDone")
	fh, err := os.Create(flagFilePath)
	if err != nil {
		utils.ErrExit("create %q: %s", flagFilePath, err)
	}
	fh.Close()
}

func clearSchemaIsExported(exportDir string) {
	flagFilePath := filepath.Join(exportDir+"metainfo", "flags", "exportSchemaDone")
	os.Remove(flagFilePath)
}

// clear and set source db type flag
func setSourceDbType(sourceDbType string, exportDir string) {
	// remove any possible source-db-* flags, as there could be some previous from indepedent migration
	for _, supportedSourceDBType := range supportedSourceDBTypes {
		flagFilePath := filepath.Join(exportDir, "metainfo", "schema", fmt.Sprintf("source-db-%s", supportedSourceDBType))
		os.Remove(flagFilePath)
	}

	flagFilePath := filepath.Join(exportDir, "metainfo", "schema", fmt.Sprintf("source-db-%s", sourceDbType))
	fh, err := os.Create(flagFilePath)
	if err != nil {
		utils.ErrExit("create %q: %s", flagFilePath, err)
	}
	fh.Close()
}
