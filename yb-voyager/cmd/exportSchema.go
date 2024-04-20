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
	"regexp"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/jsonfile"

	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var skipRecommendations utils.BoolStr
var assessmentReportPath string

var exportSchemaCmd = &cobra.Command{
	Use: "schema",
	Short: "Export schema from source database into export-dir as .sql files\n" +
		"For more details and examples, visit https://docs.yugabyte.com/preview/yugabyte-voyager/reference/schema-migration/export-schema/",
	Long: ``,

	PreRun: func(cmd *cobra.Command, args []string) {
		if source.StrExportObjectTypeList != "" && source.StrExcludeObjectTypeList != "" {
			utils.ErrExit("Error: only one of --object-type-list and --exclude-object-type-list is allowed")
		}
		setExportFlagsDefaults()
		err := validateExportFlags(cmd, SOURCE_DB_EXPORTER_ROLE)
		if err != nil {
			utils.ErrExit("Error: %s", err.Error())
		}
		markFlagsRequired(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		source.ApplyExportSchemaObjectListFilter()
		err := exportSchema()
		if err != nil {
			utils.ErrExit("failed to export schema: %v", err)
		}
	},
}

func exportSchema() error {
	if metaDBIsCreated(exportDir) && schemaIsExported() {
		if startClean {
			proceed := utils.AskPrompt(
				"CAUTION: Using --start-clean will overwrite any manual changes done to the " +
					"exported schema. Do you want to proceed")
			if !proceed {
				return nil
			}

			for _, dirName := range []string{"schema", "reports", "temp", "metainfo/schema"} {
				utils.CleanDir(filepath.Join(exportDir, dirName))
			}
			clearSchemaIsExported()
		} else {
			fmt.Fprintf(os.Stderr, "Schema is already exported. "+
				"Use --start-clean flag to export schema again -- "+
				"CAUTION: Using --start-clean will overwrite any manual changes done to the exported schema.\n")
			return nil
		}
	} else if startClean {
		utils.PrintAndLog("Schema is not exported yet. Ignoring --start-clean flag.\n\n")
	}
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	utils.PrintAndLog("export of schema for source type as '%s'\n", source.DBType)
	// Check connection with source database.
	err := source.DB().Connect()
	if err != nil {
		log.Errorf("failed to connect to the source db: %s", err)
		return fmt.Errorf("failed to connect to the source db: %w", err)
	}
	defer source.DB().Disconnect()
	checkSourceDBCharset()
	source.DB().CheckRequiredToolsAreInstalled()
	sourceDBVersion := source.DB().GetVersion()
	utils.PrintAndLog("%s version: %s\n", source.DBType, sourceDBVersion)
	err = retrieveMigrationUUID()
	if err != nil {
		log.Errorf("failed to get migration UUID: %v", err)
		return fmt.Errorf("failed to get migration UUID: %w", err)
	}

	exportSchemaStartEvent := createExportSchemaStartedEvent()
	controlPlane.ExportSchemaStarted(&exportSchemaStartEvent)

	source.DB().ExportSchema(exportDir, schemaDir)

	err = updateIndexesInfoInMetaDB()
	if err != nil {
		return err
	}
	utils.PrintAndLog("\nExported schema files created under directory: %s\n\n", filepath.Join(exportDir, "schema"))

	err = applyMigrationAssessmentRecommendations()
	if err != nil {
		return fmt.Errorf("failed to apply migration assessment recommendation to the schema files: %w", err)
	}

	payload := callhome.GetPayload(exportDir, migrationUUID)
	payload.SourceDBType = source.DBType
	payload.SourceDBVersion = sourceDBVersion
	callhome.PackAndSendPayload(exportDir)

	saveSourceDBConfInMSR()
	setSchemaIsExported()

	exportSchemaCompleteEvent := createExportSchemaCompletedEvent()
	controlPlane.ExportSchemaCompleted(&exportSchemaCompleteEvent)
	return nil
}

func init() {
	exportCmd.AddCommand(exportSchemaCmd)
	registerCommonGlobalFlags(exportSchemaCmd)
	registerCommonExportFlags(exportSchemaCmd)
	registerSourceDBConnFlags(exportSchemaCmd, false, true)
	BoolVar(exportSchemaCmd.Flags(), &source.UseOrafce, "use-orafce", true,
		"enable using orafce extension in export schema")

	BoolVar(exportSchemaCmd.Flags(), &source.CommentsOnObjects, "comments-on-objects", false,
		"enable export of comments associated with database objects (default false)")

	exportSchemaCmd.Flags().StringVar(&source.StrExportObjectTypeList, "object-type-list", "",
		"comma separated list of objects to export. ")

	exportSchemaCmd.Flags().StringVar(&source.StrExcludeObjectTypeList, "exclude-object-type-list", "",
		"comma separated list of objects to exclude from export. ")

	BoolVar(exportSchemaCmd.Flags(), &skipRecommendations, "skip-recommendations", false,
		"disable applying recommendations in the exported schema suggested by the migration assessment report")

	exportSchemaCmd.Flags().StringVar(&assessmentReportPath, "assessment-report-path", "",
		"path to the generated assessment report file(JSON format) to be used for applying recommendation to exported schema")
}

func schemaIsExported() bool {
	if !metaDBIsCreated(exportDir) {
		return false
	}
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

func updateIndexesInfoInMetaDB() error {
	log.Infof("updating indexes info in metaDB")
	if !utils.ContainsString(source.ExportObjectTypeList, "TABLE") {
		log.Infof("skipping updating indexes info in metaDB since TABLE object type is not being exported")
		return nil
	}
	indexesInfo := source.DB().GetIndexesInfo()
	if indexesInfo == nil {
		return nil
	}
	err := metadb.UpdateJsonObjectInMetaDB(metaDB, metadb.SOURCE_INDEXES_INFO_KEY, func(record *[]utils.IndexInfo) {
		*record = indexesInfo
	})
	if err != nil {
		return fmt.Errorf("failed to update indexes info in meta db: %w", err)
	}
	return nil
}

func applyMigrationAssessmentRecommendations() error {
	if skipRecommendations {
		log.Infof("not apply recommendations due to flag --skip-recommendations=true")
		return nil
	}

	assessmentReportPath := lo.Ternary(assessmentReportPath != "", assessmentReportPath,
		filepath.Join(exportDir, "assessment", "reports", "assessmentReport.json"))
	if !utils.FileOrFolderExists(assessmentReportPath) {
		utils.PrintAndLog("migration assessment report file doesn't exists at %q, skipping apply recommendations step...", assessmentReportPath)
		return nil
	}

	log.Infof("parsing assessment report json file for applying recommendations")
	var report AssessmentReport
	err := jsonfile.NewJsonFile[AssessmentReport](assessmentReportPath).Load(&report)
	if err != nil {
		return fmt.Errorf("failed to parse json report file %q: %w", assessmentReportPath, err)
	}

	err = applyColocatedVsShardedTableRecommendation(report.Sharding)
	if err != nil {
		return fmt.Errorf("failed to apply colocated vs sharded table recommendation: %w", err)
	}
	return nil
}

func applyColocatedVsShardedTableRecommendation(shardingReport *migassessment.ShardingReport) error {
	filePath := utils.GetObjectFilePath(schemaDir, "TABLE")
	if !utils.FileOrFolderExists(filePath) {
		log.Warnf("required schema file %s does not exists, returning without applying the recommendations", filePath)
		return nil
	}

	log.Infof("applying colocated vs sharded table recommendation")
	var newSQLFileContent strings.Builder
	sqlInfoArr := parseSqlFileForObjectType(filePath, "TABLE")
	setOrSelectRegexp := regexp.MustCompile(`(?m)^SET .+?;$|^SELECT .+?;$`)
	lastStmtSetOrSelect := false
	for _, sqlInfo := range sqlInfoArr {
		newSQL := sqlInfo.formattedStmt
		if setOrSelectRegexp.MatchString(sqlInfo.formattedStmt) {
			newSQL += "\n"
			lastStmtSetOrSelect = true
		} else {
			if createTableRegex.MatchString(sqlInfo.stmt) &&
				slices.Contains(shardingReport.ShardedTables, sqlInfo.objName) {
				newSQL = applyShardingRecommendation(sqlInfo, lastStmtSetOrSelect)
			} else {
				newSQL = appendSpacing(newSQL, lastStmtSetOrSelect)
			}
			lastStmtSetOrSelect = false
		}
		_, err := newSQLFileContent.WriteString(newSQL)
		if err != nil {
			return fmt.Errorf("write SQL string to string builder: %w", err)
		}
	}

	// rename existing table.sql file to table.sql.orig
	backupPath := filePath + ".orig"
	log.Infof("renaming existing file '%s' --> '%s.orig'", filePath, backupPath)
	err := os.Rename(filePath, filePath+".orig")
	if err != nil {
		return fmt.Errorf("error renaming file %s: %w", filePath, err)
	}

	// create new table.sql file for modified schema
	log.Infof("creating file %q to store the modified recommended schema", filePath)
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("error creating file '%q' storing the modified recommended schema: %w", filePath, err)
	}
	if _, err = file.WriteString(newSQLFileContent.String()); err != nil {
		return fmt.Errorf("error writing to file '%q' storing the modified recommended schema: %w", filePath, err)
	}
	if err = file.Close(); err != nil {
		return fmt.Errorf("error closing file '%q' storing the modified recommended schema: %w", filePath, err)
	}

	utils.PrintAndLog("Modified CREATE TABLE statements in %q according to the colocation and sharding recommendations of the assessment report.",
		utils.GetRelativePathFromCwd(filePath))
	utils.PrintAndLog("The original DDLs have been preserved in %q for reference.", utils.GetRelativePathFromCwd(backupPath))
	return nil
}

func applyShardingRecommendation(sqlInfo sqlInfo, lastStmtSetOrSelect bool) string {
	newSQL := strings.TrimRight(sqlInfo.formattedStmt, "; ")
	newSQL += "WITH (COLOCATION = false);\n\n\n"
	return prependSpacing(newSQL, lastStmtSetOrSelect)
}

func appendSpacing(sql string, lastStmtSetOrSelect bool) string {
	return prependSpacing(sql+"\n\n\n", lastStmtSetOrSelect)
}

func prependSpacing(sql string, condition bool) string {
	if condition {
		return "\n\n" + sql
	}
	return sql
}

func createExportSchemaStartedEvent() cp.ExportSchemaStartedEvent {

	result := cp.ExportSchemaStartedEvent{}
	initBaseSourceEvent(&result.BaseEvent, "EXPORT SCHEMA")
	return result
}

func createExportSchemaCompletedEvent() cp.ExportSchemaCompletedEvent {

	result := cp.ExportSchemaCompletedEvent{}
	initBaseSourceEvent(&result.BaseEvent, "EXPORT SCHEMA")
	return result
}
