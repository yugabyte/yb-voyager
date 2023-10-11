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
	"path/filepath"

	log "github.com/sirupsen/logrus"
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
		err := validateExportFlags(cmd, SOURCE_DB_EXPORTER_ROLE)
		if err != nil {
			utils.ErrExit("Error: %s", err.Error())
		}
		markFlagsRequired(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		source.ApplyExportSchemaObjectListFilter()
		exportSchema()
	},
}

func exportSchema() {
	if metaDBIsCreated(exportDir) && schemaIsExported() {
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
			clearSchemaIsExported()
		} else {
			fmt.Fprintf(os.Stderr, "Schema is already exported. "+
				"Use --start-clean flag to export schema again -- "+
				"CAUTION: Using --start-clean will overwrite any manual changes done to the exported schema.\n")
			return
		}
	} else if startClean {
		utils.PrintAndLog("Schema is not exported yet. Ignoring --start-clean flag.\n")
	}
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

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
	err = retrieveMigrationUUID()
	if err != nil {
		utils.ErrExit("failed to get migration UUID: %w", err)
	}
	source.DB().ExportSchema(exportDir)
	updateIndexesInfoInMetaDB()
	utils.PrintAndLog("\nExported schema files created under directory: %s\n", filepath.Join(exportDir, "schema"))

	payload := callhome.GetPayload(exportDir, migrationUUID)
	payload.SourceDBType = source.DBType
	payload.SourceDBVersion = sourceDBVersion
	callhome.PackAndSendPayload(exportDir)

	metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		// setting irrespective of the current value
		record.SourceDBConf = source.Clone()
		record.SourceDBConf.Password = ""
	})
	setSchemaIsExported()
}

func init() {
	exportCmd.AddCommand(exportSchemaCmd)
	registerCommonGlobalFlags(exportSchemaCmd)
	registerCommonExportFlags(exportSchemaCmd)
	registerSourceDBConnFlags(exportSchemaCmd)
	BoolVar(exportSchemaCmd.Flags(), &source.UseOrafce, "use-orafce", true,
		"enable using orafce extension in export schema")

	BoolVar(exportSchemaCmd.Flags(), &source.CommentsOnObjects, "comments-on-objects", false,
		"enable export of comments associated with database objects (default false)")

	exportSchemaCmd.Flags().StringVar(&source.StrExportObjectTypesList, "object-types-list", "",
		"comma separated list of objects to export. ")
}

func schemaIsExported() bool {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("check if schema is exported: load migration status record: %s", err)
	}

	return msr.ExportSchemaDone
}

func setSchemaIsExported() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportSchemaDone = true
	})
	if err != nil {
		utils.ErrExit("set schema is exported: update migration status record: %s", err)
	}
}

func clearSchemaIsExported() {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.ExportSchemaDone = false
	})
	if err != nil {
		utils.ErrExit("clear schema is exported: update migration status record: %s", err)
	}
}

func updateIndexesInfoInMetaDB() {
	log.Infof("updating indexes info in meta db")
	indexesInfo := source.DB().GetIndexesInfo()
	if indexesInfo == nil {
		return
	}
	err := metadb.UpdateJsonObjectInMetaDB(metaDB, metadb.SOURCE_INDEXES_INFO_KEY, func(record *[]utils.IndexInfo) {
		*record = indexesInfo
	})
	if err != nil {
		utils.ErrExit("update indexes info in meta db: %s", err)
	}
}
