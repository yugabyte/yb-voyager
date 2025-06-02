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
	"bufio"
	_ "embed"
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"text/template"

	"github.com/fatih/color"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

var (
	assessmentMetadataDir            string
	assessmentMetadataDirFlag        string
	assessmentReport                 AssessmentReport
	assessmentDB                     *migassessment.AssessmentDB
	intervalForCapturingIOPS         int64
	assessMigrationSupportedDBTypes  = []string{POSTGRESQL, ORACLE}
	referenceOrTablePartitionPresent = false
	pgssEnabledForAssessment         = false
	invokedByExportSchema            utils.BoolStr
)

var sourceConnectionFlags = []string{
	"source-db-host",
	"source-db-password",
	"source-db-name",
	"source-db-port",
	"source-db-schema",
	"source-db-user",
	"source-ssl-cert",
	"source-ssl-crl",
	"source-ssl-key",
	"source-ssl-mode",
	"source-ssl-root-cert",
}

var assessMigrationCmd = &cobra.Command{
	Use:   "assess-migration",
	Short: fmt.Sprintf("Assess the migration from source (%s) database to YugabyteDB.", strings.Join(assessMigrationSupportedDBTypes, ", ")),
	Long:  fmt.Sprintf("Assess the migration from source (%s) database to YugabyteDB.", strings.Join(assessMigrationSupportedDBTypes, ", ")),

	PreRun: func(cmd *cobra.Command, args []string) {
		CreateMigrationProjectIfNotExists(source.DBType, exportDir)
		err := retrieveMigrationUUID()
		if err != nil {
			utils.ErrExit("failed to get migration UUID: %w", err)
		}
		validateSourceDBTypeForAssessMigration()
		setExportFlagsDefaults()
		validateSourceSchema()
		validatePortRange()
		validateSSLMode()
		validateOracleParams()
		err = validateAndSetTargetDbVersionFlag()
		if err != nil {
			utils.ErrExit("failed to validate target db version: %v", err)
		}
		if cmd.Flags().Changed("assessment-metadata-dir") {
			validateAssessmentMetadataDirFlag()
			for _, f := range sourceConnectionFlags {
				if cmd.Flags().Changed(f) {
					utils.ErrExit("Cannot pass `--source-*` connection related flags when `--assessment-metadata-dir` is provided.\nPlease re-run the command without these flags")
				}
			}
		} else {
			cmd.MarkFlagRequired("source-db-user")
			cmd.MarkFlagRequired("source-db-name")
			//Update this later as per db-types TODO
			cmd.MarkFlagRequired("source-db-schema")
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		err := assessMigration()
		if err != nil {
			utils.ErrExit("%s", err)
		}
		packAndSendAssessMigrationPayload(COMPLETE, "")
	},
}

func packAndSendAssessMigrationPayload(status string, errMsg string) {
	if !shouldSendCallhome() {
		return
	}

	payload := createCallhomePayload()
	payload.MigrationPhase = ASSESS_MIGRATION_PHASE
	payload.Status = status
	if assessmentMetadataDirFlag == "" {
		sourceDBDetails := callhome.SourceDBDetails{
			DBType:    source.DBType,
			DBVersion: source.DBVersion,
			DBSize:    source.DBSize,
		}
		payload.SourceDBDetails = callhome.MarshalledJsonString(sourceDBDetails)
	}

	var tableSizingStats, indexSizingStats []callhome.ObjectSizingStats
	if assessmentReport.TableIndexStats != nil {
		for _, stat := range *assessmentReport.TableIndexStats {
			newStat := callhome.ObjectSizingStats{
				//redacting schema and object name
				ObjectName:      constants.OBFUSCATE_STRING,
				ReadsPerSecond:  utils.SafeDereferenceInt64(stat.ReadsPerSecond),
				WritesPerSecond: utils.SafeDereferenceInt64(stat.WritesPerSecond),
				SizeInBytes:     utils.SafeDereferenceInt64(stat.SizeInBytes),
			}
			if stat.IsIndex {
				indexSizingStats = append(indexSizingStats, newStat)
			} else {
				tableSizingStats = append(tableSizingStats, newStat)
			}
		}
	}
	schemaSummaryCopy := utils.SchemaSummary{
		Notes: assessmentReport.SchemaSummary.Notes,
		DBObjects: lo.Map(schemaAnalysisReport.SchemaSummary.DBObjects, func(dbObject utils.DBObject, _ int) utils.DBObject {
			dbObject.ObjectNames = ""
			dbObject.Details = "" // not useful, either static or sometimes sensitive(oracle indexes) information
			return dbObject
		}),
	}

	var obfuscatedIssues []callhome.AssessmentIssueCallhome
	for _, issue := range assessmentReport.Issues {
		obfuscatedIssue := callhome.NewAsssesmentIssueCallhome(issue.Category, issue.CategoryDescription, issue.Type, issue.Name, issue.Impact, issue.ObjectType, issue.Details)

		// special handling for extensions issue: adding extname to issue.Name
		if issue.Type == queryissue.UNSUPPORTED_EXTENSION {
			obfuscatedIssue.Name = queryissue.AppendObjectNameToIssueName(issue.Name, issue.ObjectName)
		}

		// appending the issue after obfuscating sensitive information
		obfuscatedIssues = append(obfuscatedIssues, obfuscatedIssue)
	}

	var callhomeSizingAssessment callhome.SizingCallhome
	if assessmentReport.Sizing != nil {
		sizingRecommedation := &assessmentReport.Sizing.SizingRecommendation
		callhomeSizingAssessment = callhome.SizingCallhome{
			NumColocatedTables:              len(sizingRecommedation.ColocatedTables),
			ColocatedReasoning:              sizingRecommedation.ColocatedReasoning,
			NumShardedTables:                len(sizingRecommedation.ShardedTables),
			NumNodes:                        sizingRecommedation.NumNodes,
			VCPUsPerInstance:                sizingRecommedation.VCPUsPerInstance,
			MemoryPerInstance:               sizingRecommedation.MemoryPerInstance,
			OptimalSelectConnectionsPerNode: sizingRecommedation.OptimalSelectConnectionsPerNode,
			OptimalInsertConnectionsPerNode: sizingRecommedation.OptimalInsertConnectionsPerNode,
			EstimatedTimeInMinForImport:     sizingRecommedation.EstimatedTimeInMinForImport,
		}
	}

	assessPayload := callhome.AssessMigrationPhasePayload{
		PayloadVersion:                 callhome.ASSESS_MIGRATION_CALLHOME_PAYLOAD_VERSION,
		TargetDBVersion:                assessmentReport.TargetDBVersion,
		Sizing:                         &callhomeSizingAssessment,
		MigrationComplexity:            assessmentReport.MigrationComplexity,
		MigrationComplexityExplanation: assessmentReport.MigrationComplexityExplanation,
		SchemaSummary:                  callhome.MarshalledJsonString(schemaSummaryCopy),
		Issues:                         obfuscatedIssues,
		Error:                          callhome.SanitizeErrorMsg(errMsg),
		TableSizingStats:               callhome.MarshalledJsonString(tableSizingStats),
		IndexSizingStats:               callhome.MarshalledJsonString(indexSizingStats),
		SourceConnectivity:             assessmentMetadataDirFlag == "",
		IopsInterval:                   intervalForCapturingIOPS,
		ControlPlaneType:               getControlPlaneType(),
	}

	payload.PhasePayload = callhome.MarshalledJsonString(assessPayload)
	err := callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

func registerSourceDBConnFlagsForAM(cmd *cobra.Command) {
	cmd.Flags().StringVar(&source.DBType, "source-db-type", "",
		fmt.Sprintf("source database type: (%s)\n", strings.Join(assessMigrationSupportedDBTypes, ", ")))

	cmd.MarkFlagRequired("source-db-type")

	cmd.Flags().StringVar(&source.Host, "source-db-host", "localhost",
		"source database server host")

	cmd.Flags().IntVar(&source.Port, "source-db-port", 0,
		"source database server port number. Default: PostgreSQL(5432), Oracle(1521)")

	cmd.Flags().StringVar(&source.User, "source-db-user", "",
		"connect to source database as the specified user")

	// TODO: All sensitive parameters can be taken from the environment variable
	cmd.Flags().StringVar(&source.Password, "source-db-password", "",
		"source password to connect as the specified user. Alternatively, you can also specify the password by setting the environment variable SOURCE_DB_PASSWORD. If you don't provide a password via the CLI, yb-voyager will prompt you at runtime for a password. If the password contains special characters that are interpreted by the shell (for example, # and $), enclose the password in single quotes.")

	cmd.Flags().StringVar(&source.DBName, "source-db-name", "",
		"source database name to be migrated to YugabyteDB")

	cmd.Flags().StringVar(&source.Schema, "source-db-schema", "",
		"source schema name(s) to export\n"+
			`Note: in case of PostgreSQL, it can be a single or comma separated list of schemas: "schema1,schema2,schema3"`)

	// TODO SSL related more args will come. Explore them later.
	cmd.Flags().StringVar(&source.SSLCertPath, "source-ssl-cert", "",
		"Path of the file containing source SSL Certificate")

	cmd.Flags().StringVar(&source.SSLMode, "source-ssl-mode", "prefer",
		fmt.Sprintf("specify the source SSL mode out of: [%s]",
			strings.Join(supportedSSLModesOnSourceOrSourceReplica, ", ")))

	cmd.Flags().StringVar(&source.SSLKey, "source-ssl-key", "",
		"Path of the file containing source SSL Key")

	cmd.Flags().StringVar(&source.SSLRootCert, "source-ssl-root-cert", "",
		"Path of the file containing source SSL Root Certificate")

	cmd.Flags().StringVar(&source.SSLCRL, "source-ssl-crl", "",
		"Path of the file containing source SSL Root Certificate Revocation List (CRL)")

	cmd.Flags().StringVar(&source.DBSid, "oracle-db-sid", "",
		"[For Oracle Only] Oracle System Identifier (SID) that you wish to use while exporting data from Oracle instances")

	cmd.Flags().StringVar(&source.OracleHome, "oracle-home", "",
		"[For Oracle Only] Path to set $ORACLE_HOME environment variable. tnsnames.ora is found in $ORACLE_HOME/network/admin")

	cmd.Flags().StringVar(&source.TNSAlias, "oracle-tns-alias", "",
		"[For Oracle Only] Name of TNS Alias you wish to use to connect to Oracle instance. Refer to documentation to learn more about configuring tnsnames.ora and aliases")
}

func init() {
	rootCmd.AddCommand(assessMigrationCmd)
	registerCommonGlobalFlags(assessMigrationCmd)
	registerSourceDBConnFlagsForAM(assessMigrationCmd)

	BoolVar(assessMigrationCmd.Flags(), &startClean, "start-clean", false,
		"cleans up the project directory for schema or data files depending on the export command (default false)")

	// optional flag to take metadata and stats directory path in case it is not in exportDir
	assessMigrationCmd.Flags().StringVar(&assessmentMetadataDirFlag, "assessment-metadata-dir", "",
		"Directory path where assessment metadata like source DB metadata and statistics are stored. Optional flag, if not provided, "+
			"it will be assumed to be present at default path inside the export directory.")

	assessMigrationCmd.Flags().Int64Var(&intervalForCapturingIOPS, "iops-capture-interval", 120,
		"Interval (in seconds) at which voyager will gather IOPS metadata from source database for the given schema(s). (only valid for PostgreSQL)")

	BoolVar(assessMigrationCmd.Flags(), &source.RunGuardrailsChecks, "run-guardrails-checks", true, "run guardrails checks before assess migration. (only valid for PostgreSQL)")

	assessMigrationCmd.Flags().StringVar(&targetDbVersionStrFlag, "target-db-version", "",
		fmt.Sprintf("Target YugabyteDB version to assess migration for (in format A.B.C.D). Defaults to latest stable version (%s)", ybversion.LatestStable.String()))

	BoolVar(assessMigrationCmd.Flags(), &invokedByExportSchema, "invoked-by-export-schema", false,
		"Flag to indicate if the assessment is invoked by export schema command. ")
	assessMigrationCmd.Flags().MarkHidden("invoked-by-export-schema") // mark hidden
}

func assessMigration() (err error) {
	assessmentMetadataDir = lo.Ternary(assessmentMetadataDirFlag != "", assessmentMetadataDirFlag,
		filepath.Join(exportDir, "assessment", "metadata"))
	// setting schemaDir to use later on - gather assessment metadata, segregating into schema files per object etc..
	schemaDir = filepath.Join(assessmentMetadataDir, "schema")

	checkStartCleanForAssessMigration(assessmentMetadataDirFlag != "")
	utils.PrintAndLog("Assessing for migration to target YugabyteDB version %s\n", targetDbVersion)

	assessmentDir := filepath.Join(exportDir, "assessment")
	migassessment.AssessmentDir = assessmentDir
	migassessment.SourceDBType = source.DBType
	migassessment.IntervalForCapturingIops = intervalForCapturingIOPS

	if source.Password == "" {
		source.Password, err = askPassword("source DB", source.User, "SOURCE_DB_PASSWORD")
		if err != nil {
			return fmt.Errorf("failed to get source DB password for assessing migration: %w", err)
		}
	}

	if assessmentMetadataDirFlag == "" { // only in case of source connectivity
		err := source.DB().Connect()
		if err != nil {
			return fmt.Errorf("failed to connect source db for assessing migration: %v", err)
		}

		// We will require source db connection for the below checks
		// Check if required binaries are installed.
		if source.RunGuardrailsChecks {
			// Check source database version.
			log.Info("checking source DB version")
			err = source.DB().CheckSourceDBVersion(exportType)
			if err != nil {
				return fmt.Errorf("failed to check source DB version for assess migration: %w", err)
			}

			// Check if required binaries are installed.
			binaryCheckIssues, err := checkDependenciesForExport()
			if err != nil {
				return fmt.Errorf("failed to check dependencies for assess migration: %w", err)
			} else if len(binaryCheckIssues) > 0 {
				return fmt.Errorf("\n%s\n%s", color.RedString("\nMissing dependencies for assess migration:"), strings.Join(binaryCheckIssues, "\n"))
			}
		}

		res := source.DB().CheckSchemaExists()
		if !res {
			return fmt.Errorf("failed to check if source schema exist: %q", source.Schema)
		}

		// Check if source db has permissions to assess migration
		if source.RunGuardrailsChecks {
			checkIfSchemasHaveUsagePermissions()
			var missingPerms []string
			missingPerms, pgssEnabledForAssessment, err = source.DB().GetMissingAssessMigrationPermissions()
			if err != nil {
				return fmt.Errorf("failed to get missing assess migration permissions: %w", err)
			}
			if len(missingPerms) > 0 {
				color.Red("\nPermissions missing in the source database for assess migration:\n")
				output := strings.Join(missingPerms, "\n")
				utils.PrintAndLog("%s\n\n", output)

				link := "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-source-database"
				fmt.Println("Check the documentation to prepare the database for migration:", color.BlueString(link))

				reply := utils.AskPrompt("\nDo you want to continue anyway")
				if !reply {
					return fmt.Errorf("grant the required permissions and try again")
				}
			}
		}

		fetchSourceInfo()

		source.DB().Disconnect()
	}

	startEvent := createMigrationAssessmentStartedEvent()
	controlPlane.MigrationAssessmentStarted(startEvent)

	initAssessmentDB() // Note: migassessment.AssessmentDir needs to be set beforehand

	err = gatherAssessmentMetadata()
	if err != nil {
		return fmt.Errorf("failed to gather assessment metadata: %w", err)
	}

	parseExportedSchemaFileForAssessmentIfRequired()

	err = populateMetadataCSVIntoAssessmentDB()
	if err != nil {
		return fmt.Errorf("failed to populate metadata CSV into SQLite DB: %w", err)
	}

	err = validateSourceDBIOPSForAssessMigration()
	if err != nil {
		return fmt.Errorf("failed to validate source database IOPS: %w", err)
	}

	err = runAssessment()
	if err != nil {
		utils.PrintAndLog("failed to run assessment: %v", err)
	}

	err = generateAssessmentReport()
	if err != nil {
		return fmt.Errorf("failed to generate assessment report: %w", err)
	}

	log.Infof("number of assessment issues detected: %d\n", len(assessmentReport.Issues))

	utils.PrintAndLog("Migration assessment completed successfully.")
	completedEvent := createMigrationAssessmentCompletedEvent()
	controlPlane.MigrationAssessmentCompleted(completedEvent)
	err = SetMigrationAssessmentDoneInMSR()
	if err != nil {
		return fmt.Errorf("failed to set migration assessment completed in MSR: %w", err)
	}
	return nil
}

func fetchSourceInfo() {
	var err error
	source.DBVersion = source.DB().GetVersion()
	source.DBSize, err = source.DB().GetDatabaseSize()
	if err != nil {
		log.Errorf("error getting database size: %v", err) //can just log as this is used for call-home only
	}
}

func SetMigrationAssessmentDoneInMSR() error {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		if invokedByExportSchema {
			record.MigrationAssessmentDoneViaExportSchema = true
			record.MigrationAssessmentDone = false
		} else {
			record.MigrationAssessmentDone = true
			record.MigrationAssessmentDoneViaExportSchema = false
		}
	})
	if err != nil {
		return fmt.Errorf("failed to update migration status record with migration assessment done flag: %w", err)
	}
	return nil
}

func IsMigrationAssessmentDoneDirectly(metaDBInstance *metadb.MetaDB) (bool, error) {
	record, err := metaDBInstance.GetMigrationStatusRecord()
	if err != nil {
		return false, fmt.Errorf("failed to get migration status record: %w", err)
	}
	return record.MigrationAssessmentDone, nil
}

func IsMigrationAssessmentDoneViaExportSchema() (bool, error) {
	if !metaDBIsCreated(exportDir) {
		return false, fmt.Errorf("metaDB is not created in export directory: %s", exportDir)
	}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return false, fmt.Errorf("failed to get migration status record: %w", err)
	}
	if msr == nil {
		return false, nil
	}

	return msr.MigrationAssessmentDoneViaExportSchema, nil
}

func ClearMigrationAssessmentDone() error {
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		if record.MigrationAssessmentDone {
			record.MigrationAssessmentDone = false
		}
	})
	if err != nil {
		return fmt.Errorf("failed to clear migration status record with migration assessment done flag: %w", err)
	}
	return nil
}

func createMigrationAssessmentStartedEvent() *cp.MigrationAssessmentStartedEvent {
	ev := &cp.MigrationAssessmentStartedEvent{}
	initBaseSourceEvent(&ev.BaseEvent, "ASSESS MIGRATION")
	return ev
}

func createMigrationAssessmentCompletedEvent() *cp.MigrationAssessmentCompletedEvent {
	ev := &cp.MigrationAssessmentCompletedEvent{}
	initBaseSourceEvent(&ev.BaseEvent, "ASSESS MIGRATION")

	totalColocatedSize, err := assessmentReport.GetTotalColocatedSize(source.DBType)
	if err != nil {
		utils.PrintAndLog("failed to calculate the total colocated table size from tableIndexStats: %v", err)
	}

	totalShardedSize, err := assessmentReport.GetTotalShardedSize(source.DBType)
	if err != nil {
		utils.PrintAndLog("failed to calculate the total sharded table size from tableIndexStats: %v", err)
	}

	assessmentIssues := convertAssessmentIssueToYugabyteDAssessmentIssue(assessmentReport)

	payload := AssessMigrationPayload{
		PayloadVersion:                 ASSESS_MIGRATION_YBD_PAYLOAD_VERSION,
		VoyagerVersion:                 assessmentReport.VoyagerVersion,
		TargetDBVersion:                assessmentReport.TargetDBVersion,
		MigrationComplexity:            assessmentReport.MigrationComplexity,
		MigrationComplexityExplanation: assessmentReport.MigrationComplexityExplanation,
		SchemaSummary:                  assessmentReport.SchemaSummary,
		AssessmentIssues:               assessmentIssues,
		SourceSizeDetails: SourceDBSizeDetails{
			TotalIndexSize:     assessmentReport.GetTotalIndexSize(),
			TotalTableSize:     assessmentReport.GetTotalTableSize(),
			TotalTableRowCount: assessmentReport.GetTotalTableRowCount(),
			TotalDBSize:        source.DBSize,
		},
		TargetRecommendations: TargetSizingRecommendations{
			TotalColocatedSize: totalColocatedSize,
			TotalShardedSize:   totalShardedSize,
		},
		ConversionIssues: schemaAnalysisReport.Issues,
		Sizing:           assessmentReport.Sizing,
		TableIndexStats:  assessmentReport.TableIndexStats,
		Notes:            assessmentReport.Notes,
		AssessmentJsonReport: AssessmentReportYugabyteD{
			VoyagerVersion:             assessmentReport.VoyagerVersion,
			TargetDBVersion:            assessmentReport.TargetDBVersion,
			MigrationComplexity:        assessmentReport.MigrationComplexity,
			SchemaSummary:              assessmentReport.SchemaSummary,
			Sizing:                     assessmentReport.Sizing,
			TableIndexStats:            assessmentReport.TableIndexStats,
			Notes:                      assessmentReport.Notes,
			UnsupportedDataTypes:       assessmentReport.UnsupportedDataTypes,
			UnsupportedDataTypesDesc:   assessmentReport.UnsupportedDataTypesDesc,
			UnsupportedFeatures:        assessmentReport.UnsupportedFeatures,
			UnsupportedFeaturesDesc:    assessmentReport.UnsupportedFeaturesDesc,
			UnsupportedQueryConstructs: assessmentReport.UnsupportedQueryConstructs,
			UnsupportedPlPgSqlObjects:  assessmentReport.UnsupportedPlPgSqlObjects,
			MigrationCaveats:           assessmentReport.MigrationCaveats,
		},
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		utils.PrintAndLog("Failed to serialise the final report to json (ERR IGNORED): %s", err)
	}

	ev.Report = string(payloadBytes)
	log.Infof("assess migration payload send to yugabyted: %s", ev.Report)
	return ev
}

func convertAssessmentIssueToYugabyteDAssessmentIssue(ar AssessmentReport) []AssessmentIssueYugabyteD {
	var result []AssessmentIssueYugabyteD
	for _, issue := range ar.Issues {

		ybdIssue := AssessmentIssueYugabyteD{
			Category:               issue.Category,
			CategoryDescription:    issue.CategoryDescription,
			Type:                   issue.Type, // Ques: should we be just sending Name in AssessmentIssueYugabyteD payload
			Name:                   issue.Name,
			Description:            issue.Description,
			Impact:                 issue.Impact,
			ObjectType:             issue.ObjectType,
			ObjectName:             issue.ObjectName,
			SqlStatement:           issue.SqlStatement,
			DocsLink:               issue.DocsLink,
			MinimumVersionsFixedIn: issue.MinimumVersionsFixedIn,

			Details: issue.Details,
		}
		result = append(result, ybdIssue)
	}
	return result
}

func runAssessment() error {
	log.Infof("running assessment for migration from '%s' to YugabyteDB", source.DBType)

	err := migassessment.SizingAssessment(targetDbVersion)
	if err != nil {
		log.Errorf("failed to perform sizing and sharding assessment: %v", err)
		return fmt.Errorf("failed to perform sizing and sharding assessment: %w", err)
	}

	assessmentReport.Sizing = migassessment.SizingReport

	shardedTables, _ := assessmentReport.GetShardedTablesRecommendation()
	colocatedTables, _ := assessmentReport.GetColocatedTablesRecommendation()
	log.Infof("Recommendation: colocated tables: %v", colocatedTables)
	log.Infof("Recommendation: sharded tables: %v", shardedTables)
	log.Infof("Recommendation: Cluster size: %s", assessmentReport.GetClusterSizingRecommendation())
	return nil
}

func checkStartCleanForAssessMigration(metadataDirPassedByUser bool) {
	assessmentDir := filepath.Join(exportDir, "assessment")
	reportsFilePattern := filepath.Join(assessmentDir, "reports", fmt.Sprintf("%s.*", ASSESSMENT_FILE_NAME))
	metadataFilesPattern := filepath.Join(assessmentMetadataDir, "*.csv")
	schemaFilesPattern := filepath.Join(assessmentMetadataDir, "schema", "*", "*.sql")
	dbsFilePattern := filepath.Join(assessmentDir, "dbs", "*.db")

	assessmentAlreadyDone := utils.FileOrFolderExistsWithGlobPattern(reportsFilePattern) || utils.FileOrFolderExistsWithGlobPattern(dbsFilePattern)
	if !metadataDirPassedByUser {
		assessmentAlreadyDone = assessmentAlreadyDone || utils.FileOrFolderExistsWithGlobPattern(metadataFilesPattern) ||
			utils.FileOrFolderExistsWithGlobPattern(schemaFilesPattern)
	}

	if assessmentAlreadyDone {
		if startClean {
			utils.CleanDir(filepath.Join(assessmentDir, "metadata"))
			utils.CleanDir(filepath.Join(assessmentDir, "reports"))
			utils.CleanDir(filepath.Join(assessmentDir, "dbs"))
			err := ClearMigrationAssessmentDone()
			if err != nil {
				utils.ErrExit("failed to clear migration assessment completed flag in msr during start clean: %v", err)
			}
		} else {
			utils.ErrExit("assessment metadata or reports files already exist in the assessment directory: '%s'. Use the --start-clean flag to clear the directory before proceeding.", assessmentDir)
		}
	}
}

func gatherAssessmentMetadata() (err error) {
	if assessmentMetadataDirFlag != "" {
		return nil // assessment metadata files are provided by the user inside assessmentMetadataDir
	}

	// setting schema objects types to export before creating the project directories
	source.ExportObjectTypeList = utils.GetExportSchemaObjectList(source.DBType)
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	utils.PrintAndLog("gathering metadata and stats from '%s' source database...", source.DBType)
	switch source.DBType {
	case POSTGRESQL:
		err := gatherAssessmentMetadataFromPG()
		if err != nil {
			return fmt.Errorf("error gathering metadata and stats from source PG database: %w", err)
		}
	case ORACLE:
		err := gatherAssessmentMetadataFromOracle()
		if err != nil {
			return fmt.Errorf("error gathering metadata and stats from source Oracle database: %w", err)
		}
	default:
		return fmt.Errorf("source DB Type %s is not yet supported for metadata and stats gathering", source.DBType)
	}
	utils.PrintAndLog("gathered assessment metadata files at '%s'", assessmentMetadataDir)
	return nil
}

func gatherAssessmentMetadataFromOracle() (err error) {
	if assessmentMetadataDirFlag != "" {
		return nil
	}

	scriptPath, err := findGatherMetadataScriptPath(ORACLE)
	if err != nil {
		return err
	}

	tnsAdmin, err := getTNSAdmin(source)
	if err != nil {
		return fmt.Errorf("error getting tnsAdmin: %v", err)
	}
	envVars := []string{fmt.Sprintf("ORACLE_PASSWORD=%s", source.Password),
		fmt.Sprintf("TNS_ADMIN=%s", tnsAdmin),
		fmt.Sprintf("ORACLE_HOME=%s", source.GetOracleHome()),
	}
	log.Infof("environment variables passed to oracle gather metadata script: %v", envVars)
	return runGatherAssessmentMetadataScript(scriptPath, envVars,
		source.DB().GetConnectionUriWithoutPassword(), strings.ToUpper(source.Schema), assessmentMetadataDir)
}

func gatherAssessmentMetadataFromPG() (err error) {
	if assessmentMetadataDirFlag != "" {
		return nil
	}

	scriptPath, err := findGatherMetadataScriptPath(POSTGRESQL)
	if err != nil {
		return err
	}

	return runGatherAssessmentMetadataScript(scriptPath, []string{fmt.Sprintf("PGPASSWORD=%s", source.Password)},
		source.DB().GetConnectionUriWithoutPassword(), source.Schema, assessmentMetadataDir, fmt.Sprintf("%t", pgssEnabledForAssessment), fmt.Sprintf("%d", intervalForCapturingIOPS))
}

func findGatherMetadataScriptPath(dbType string) (string, error) {
	var defaultScriptPath string
	switch dbType {
	case POSTGRESQL:
		defaultScriptPath = "/etc/yb-voyager/gather-assessment-metadata/postgresql/yb-voyager-pg-gather-assessment-metadata.sh"
	case ORACLE:
		defaultScriptPath = "/etc/yb-voyager/gather-assessment-metadata/oracle/yb-voyager-oracle-gather-assessment-metadata.sh"
	default:
		panic(fmt.Sprintf("invalid source db type %q", dbType))
	}

	homebrewVoyagerDir := fmt.Sprintf("yb-voyager@%s", utils.YB_VOYAGER_VERSION)
	possiblePathsForScript := []string{
		defaultScriptPath,
		filepath.Join("/", "opt", "homebrew", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, defaultScriptPath),
		filepath.Join("/", "usr", "local", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, defaultScriptPath),
	}

	for _, path := range possiblePathsForScript {
		if utils.FileOrFolderExists(path) {
			log.Infof("found the gather assessment metadata script at: %s", path)
			return path, nil
		}
	}

	return "", fmt.Errorf("script not found in possible paths: %v", possiblePathsForScript)
}

func runGatherAssessmentMetadataScript(scriptPath string, envVars []string, scriptArgs ...string) error {
	cmd := exec.Command(scriptPath, scriptArgs...)
	log.Infof("running script: %s", cmd.String())
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, envVars...)
	cmd.Dir = assessmentMetadataDir
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting gather assessment metadata script: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Errorf("[stderr of script]: %s", scanner.Text())
			fmt.Printf("%s\n", scanner.Text())
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		log.Infof("[stdout of script]: %s", scanner.Text())
		fmt.Printf("%s\n", scanner.Text())
	}

	err = cmd.Wait()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 2 {
					log.Infof("Exit without error as user opted not to continue in the script.")
					os.Exit(0)
				}
			}
		}
		return fmt.Errorf("error waiting for gather assessment metadata script to complete: %w", err)
	}
	return nil
}

/*
It is due to the differences in how tools like ora2pg, and pg_dump exports the schema
pg_dump - export schema in single .sql file which is later on segregated by voyager in respective .sql file
ora2pg - export schema in given .sql file, and we have to call it for each object type to export schema
*/
func parseExportedSchemaFileForAssessmentIfRequired() {
	if source.DBType == ORACLE {
		return // already parsed into schema files while exporting
	}

	log.Infof("set 'schemaDir' as: %s", schemaDir)
	source.ApplyExportSchemaObjectListFilter()
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)
	source.DB().ExportSchema(exportDir, schemaDir)
}

func populateMetadataCSVIntoAssessmentDB() error {
	metadataFilesPath, err := filepath.Glob(filepath.Join(assessmentMetadataDir, "*.csv"))
	if err != nil {
		return fmt.Errorf("error looking for csv files in directory %s: %w", assessmentMetadataDir, err)
	}

	for _, metadataFilePath := range metadataFilesPath {
		baseFileName := filepath.Base(metadataFilePath)
		metric := strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName))
		tableName := strings.Replace(metric, "-", "_", -1)
		// collecting both initial and final measurement in the same table
		tableName = lo.Ternary(strings.Contains(tableName, migassessment.TABLE_INDEX_IOPS),
			migassessment.TABLE_INDEX_IOPS, tableName)

		log.Infof("populating metadata from file %s into table %s", metadataFilePath, tableName)
		file, err := os.Open(metadataFilePath)
		if err != nil {
			log.Warnf("error opening file %s: %v", metadataFilePath, err)
			return nil
		}
		csvReader := csv.NewReader(file)
		csvReader.ReuseRecord = true
		rows, err := csvReader.ReadAll()
		if err != nil {
			log.Errorf("error reading csv file %s: %v", metadataFilePath, err)
			return fmt.Errorf("error reading csv file %s: %w", metadataFilePath, err)
		}

		err = assessmentDB.BulkInsert(tableName, rows)
		if err != nil {
			return fmt.Errorf("error bulk inserting data into %s table: %w", tableName, err)
		}
		log.Infof("populated metadata from file %s into table %s", metadataFilePath, tableName)
	}

	err = assessmentDB.PopulateMigrationAssessmentStats()
	if err != nil {
		return fmt.Errorf("failed to populate migration assessment stats: %w", err)
	}
	return nil
}

//go:embed templates/migration_assessment_report.template
var bytesTemplate []byte

func generateAssessmentReport() (err error) {
	utils.PrintAndLog("Generating assessment report...")

	assessmentReport.VoyagerVersion = utils.YB_VOYAGER_VERSION
	assessmentReport.TargetDBVersion = targetDbVersion

	err = getAssessmentReportContentFromAnalyzeSchema()
	if err != nil {
		return fmt.Errorf("failed to generate assessment report content from analyze schema: %w", err)
	}

	unsupportedFeatures, err := fetchUnsupportedObjectTypes()
	if err != nil {
		return fmt.Errorf("failed to fetch unsupported object types: %w", err)
	}
	assessmentReport.UnsupportedFeatures = append(assessmentReport.UnsupportedFeatures, unsupportedFeatures...)

	if utils.GetEnvAsBool("REPORT_UNSUPPORTED_QUERY_CONSTRUCTS", true) {
		unsupportedQueries, err := fetchUnsupportedQueryConstructs()
		if err != nil {
			return fmt.Errorf("failed to fetch unsupported queries on YugabyteDB: %w", err)
		}
		assessmentReport.UnsupportedQueryConstructs = unsupportedQueries
	}

	unsupportedDataTypes, unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB, err := fetchColumnsWithUnsupportedDataTypes()
	if err != nil {
		return fmt.Errorf("failed to fetch columns with unsupported data types: %w", err)
	}
	assessmentReport.UnsupportedDataTypes = unsupportedDataTypes
	assessmentReport.UnsupportedDataTypesDesc = DATATYPE_CATEGORY_DESCRIPTION

	addAssessmentIssuesForUnsupportedDatatypes(unsupportedDataTypes)

	addMigrationCaveatsToAssessmentReport(unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB)

	err = addAssessmentIssuesForRedundantIndex()
	if err != nil {
		return fmt.Errorf("error in getting redundant index issues: %v", err)
	}
	// calculating migration complexity after collecting all assessment issues
	complexity, explanation := calculateMigrationComplexityAndExplanation(source.DBType, schemaDir, assessmentReport)
	log.Infof("migration complexity: %q and explanation: %q", complexity, explanation)
	assessmentReport.MigrationComplexity = complexity
	assessmentReport.MigrationComplexityExplanation = explanation

	assessmentReport.Sizing = migassessment.SizingReport
	assessmentReport.TableIndexStats, err = assessmentDB.FetchAllStats()
	if err != nil {
		return fmt.Errorf("fetching all stats info from AssessmentDB: %w", err)
	}

	addNotesToAssessmentReport()
	postProcessingOfAssessmentReport()

	assessmentReportDir := filepath.Join(exportDir, "assessment", "reports")
	err = generateAssessmentReportJson(assessmentReportDir)
	if err != nil {
		return fmt.Errorf("failed to generate assessment report JSON: %w", err)
	}

	err = generateAssessmentReportHtml(assessmentReportDir)
	if err != nil {
		return fmt.Errorf("failed to generate assessment report HTML: %w", err)
	}
	return nil
}

func fetchRedundantIndexInfo() ([]utils.RedundantIndexesInfo, error) {
	query := fmt.Sprintf(`SELECT redundant_schema_name,redundant_table_name,redundant_index_name,
	existing_schema_name,existing_table_name,existing_index_name,
	redundant_ddl,existing_ddl from %s`,
		migassessment.REDUNDANT_INDEXES)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying-%s on assessmentDB for redundant indexes: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching redundant indexes %v", err)
		}
	}()

	var redundantIndexesInfo []utils.RedundantIndexesInfo
	for rows.Next() {
		var redundantIndex utils.RedundantIndexesInfo
		err := rows.Scan(&redundantIndex.RedundantSchemaName, &redundantIndex.RedundantTableName, &redundantIndex.RedundantIndexName,
			&redundantIndex.ExistingSchemaName, &redundantIndex.ExistingTableName, &redundantIndex.ExistingIndexName,
			&redundantIndex.RedundantIndexDDL, &redundantIndex.ExistingIndexDDL)
		if err != nil {
			return nil, fmt.Errorf("error scanning rows for redundant indexes: %w", err)
		}
		redundantIndex.DBType = source.DBType
		redundantIndexesInfo = append(redundantIndexesInfo, redundantIndex)
	}
	return redundantIndexesInfo, nil
}

func fetchColumnStatisticsInfo() ([]utils.ColumnStatistics, error) {
	query := fmt.Sprintf(`SELECT schema_name, table_name, column_name, null_frac, effective_n_distinct, most_common_freq, most_common_val from %s`,
		migassessment.COLUMN_STATISTICS)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying-%s on assessmentDB for column statistics: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching column statistics %v", err)
		}
	}()

	var columnStats []utils.ColumnStatistics
	for rows.Next() {
		var stat utils.ColumnStatistics
		err := rows.Scan(&stat.SchemaName, &stat.TableName, &stat.ColumnName, &stat.NullFraction, &stat.DistinctValues, &stat.MostCommonFrequency, &stat.MostCommonValue)
		if err != nil {
			return nil, fmt.Errorf("error scanning rows for most frequent values indexes: %w", err)
		}
		stat.DBType = source.DBType
		columnStats = append(columnStats, stat)
	}
	return columnStats, nil
}

func fetchAndSetColumnStatisticsForIndexIssues() error {
	if source.DBType != POSTGRESQL {
		return nil
	}
	var err error
	//Fetching the column stats from assessment db
	columnStats, err := fetchColumnStatisticsInfo()
	if err != nil {
		return fmt.Errorf("error fetching column stats from assessement db: %v", err)
	}
	//passing it on to the parser issue detector to enable it for detecting issues using this.
	parserIssueDetector.SetColumnStatistics(columnStats)
	return nil
}

func addAssessmentIssuesForRedundantIndex() error {
	if source.DBType != POSTGRESQL {
		return nil
	}
	redundantIndexesInfo, err := fetchRedundantIndexInfo()
	if err != nil {
		return fmt.Errorf("error fetching redundant index information: %v", err)
	}

	var redundantIssues []queryissue.QueryIssue
	redundantIssues = append(redundantIssues, queryissue.GetRedundantIndexIssues(redundantIndexesInfo)...)
	for _, issue := range redundantIssues {

		convertedAnalyzeIssue := convertIssueInstanceToAnalyzeIssue(issue, "", false, false)
		convertedIssue := convertAnalyzeSchemaIssueToAssessmentIssue(convertedAnalyzeIssue, issue.MinimumVersionsFixedIn)
		assessmentReport.AppendIssues(convertedIssue)
	}
	return nil
}

func getAssessmentReportContentFromAnalyzeSchema() error {

	var err error
	//fetching column stats from assessment db and then passing it on to the parser issue detector for detecting issues
	err = fetchAndSetColumnStatisticsForIndexIssues()
	if err != nil {
		return fmt.Errorf("error parsing column statistics information: %v", err)
	}

	/*
		Here we are generating analyze schema report which converts issue instance to analyze schema issue
		Then in assessment codepath we extract the required information from analyze schema issue which could have been done directly from issue instance(TODO)

		But current Limitation is analyze schema currently uses regexp etc to detect some issues(not using parser).
	*/
	schemaAnalysisReport := analyzeSchemaInternal(&source, true, true)
	assessmentReport.SchemaSummary = schemaAnalysisReport.SchemaSummary
	assessmentReport.SchemaSummary.Description = lo.Ternary(source.DBType == ORACLE, SCHEMA_SUMMARY_DESCRIPTION_ORACLE, SCHEMA_SUMMARY_DESCRIPTION)

	var unsupportedFeatures []UnsupportedFeature
	switch source.DBType {
	case ORACLE:
		unsupportedFeatures, err = fetchUnsupportedOracleFeaturesFromSchemaReport(schemaAnalysisReport)
	case POSTGRESQL:
		unsupportedFeatures, err = fetchUnsupportedPGFeaturesFromSchemaReport(schemaAnalysisReport)
	default:
		panic(fmt.Sprintf("unsupported source db type %q", source.DBType))
	}
	if err != nil {
		return fmt.Errorf("failed to fetch '%s' unsupported features: %w", source.DBType, err)
	}
	assessmentReport.UnsupportedFeatures = append(assessmentReport.UnsupportedFeatures, unsupportedFeatures...)
	assessmentReport.UnsupportedFeaturesDesc = FEATURE_CATEGORY_DESCRIPTION

	// Ques: Do we still need this and REPORT_UNSUPPORTED_QUERY_CONSTRUCTS env var
	if utils.GetEnvAsBool("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS", true) {
		assessmentReport.UnsupportedPlPgSqlObjects = fetchUnsupportedPlPgSQLObjects(schemaAnalysisReport)
	}
	return nil
}

// when we group multiple Issue instances into a single bucket of UnsupportedFeature.
// Ideally, all the issues in the same bucket should have the same minimum version fixed in.
// We want to validate that and fail if not.
func areMinVersionsFixedInEqual(m1 map[string]*ybversion.YBVersion, m2 map[string]*ybversion.YBVersion) bool {
	if m1 == nil && m2 == nil {
		return true
	}
	if m1 == nil || m2 == nil {
		return false
	}

	if len(m1) != len(m2) {
		return false
	}
	for k, v := range m1 {
		if m2[k] == nil || !m2[k].Equal(v) {
			return false
		}
	}
	return true
}

func getUnsupportedFeaturesFromSchemaAnalysisReport(featureName string, issueDescription string, issueType string, schemaAnalysisReport utils.SchemaReport, displayDDLInHTML bool) UnsupportedFeature {
	log.Info("filtering issues for feature: ", featureName)
	objects := make([]ObjectInfo, 0)
	link := "" // for oracle we shouldn't display any line for links
	var minVersionsFixedIn map[string]*ybversion.YBVersion
	var minVersionsFixedInSet bool

	for _, analyzeIssue := range schemaAnalysisReport.Issues {
		if slices.Contains([]string{UNSUPPORTED_DATATYPES_CATEGORY, UNSUPPORTED_PLPGSQL_OBJECTS_CATEGORY}, analyzeIssue.IssueType) {
			//In case the category is Datatypes or PLPGSQL issues, the no need to check for the issue in those as these are reported separately in other places
			//e.g. fetchUnsupportedPlPgSQLObjects(),fetchColumnsWithUnsupportedDataTypes()
			continue
		}

		// Reason in analyze is equivalent to Description of IssueInstance or AssessmentIssue
		issueMatched := lo.Ternary[bool](issueType != "", issueType == analyzeIssue.Type, strings.Contains(analyzeIssue.Reason, issueDescription))
		if issueMatched {
			if !minVersionsFixedInSet {
				minVersionsFixedIn = analyzeIssue.MinimumVersionsFixedIn
				minVersionsFixedInSet = true
			}
			if !areMinVersionsFixedInEqual(minVersionsFixedIn, analyzeIssue.MinimumVersionsFixedIn) {
				utils.ErrExit("Issues belonging to UnsupportedFeature %s have different minimum versions fixed in: %v, %v", analyzeIssue.Name, minVersionsFixedIn, analyzeIssue.MinimumVersionsFixedIn)
			}

			objectInfo := ObjectInfo{
				ObjectName:   analyzeIssue.ObjectName,
				SqlStatement: analyzeIssue.SqlStatement,
			}
			link = analyzeIssue.DocsLink
			objects = append(objects, objectInfo)
			issueDescription = analyzeIssue.Reason
			assessmentReport.AppendIssues(convertAnalyzeSchemaIssueToAssessmentIssue(analyzeIssue, minVersionsFixedIn))
		}
	}

	return UnsupportedFeature{featureName, objects, displayDDLInHTML, link, issueDescription, minVersionsFixedIn}
}

func convertAnalyzeSchemaIssueToAssessmentIssue(analyzeSchemaIssue utils.AnalyzeSchemaIssue, minVersionsFixedIn map[string]*ybversion.YBVersion) AssessmentIssue {
	return AssessmentIssue{
		Category:            analyzeSchemaIssue.IssueType,
		CategoryDescription: GetCategoryDescription(analyzeSchemaIssue.IssueType),
		Type:                analyzeSchemaIssue.Type,
		Name:                analyzeSchemaIssue.Name,

		// Reason in analyze is equivalent to Description of IssueInstance or AssessmentIssue
		// and we don't use any Suggestion field in AssessmentIssue. Combination of Description + DocsLink should be enough
		Description: lo.Ternary(analyzeSchemaIssue.Suggestion == "", analyzeSchemaIssue.Reason, utils.JoinSentences(analyzeSchemaIssue.Reason, analyzeSchemaIssue.Suggestion)),

		Impact:                 analyzeSchemaIssue.Impact,
		ObjectType:             analyzeSchemaIssue.ObjectType,
		ObjectName:             analyzeSchemaIssue.ObjectName,
		SqlStatement:           analyzeSchemaIssue.SqlStatement,
		DocsLink:               analyzeSchemaIssue.DocsLink,
		MinimumVersionsFixedIn: minVersionsFixedIn,
		Details:                analyzeSchemaIssue.Details,
	}
}

func fetchUnsupportedPGFeaturesFromSchemaReport(schemaAnalysisReport utils.SchemaReport) ([]UnsupportedFeature, error) {
	log.Infof("fetching unsupported features for PG...")
	unsupportedFeatures := make([]UnsupportedFeature, 0)

	for _, indexMethod := range queryissue.UnsupportedIndexMethods {
		displayIndexMethod := strings.ToUpper(indexMethod)
		featureName := fmt.Sprintf("%s indexes", displayIndexMethod)
		reason := fmt.Sprintf(queryissue.UNSUPPORTED_INDEX_METHOD_DESCRIPTION, displayIndexMethod)
		unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(featureName, reason, "", schemaAnalysisReport, false))
	}
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(CONSTRAINT_TRIGGERS_FEATURE, "", queryissue.CONSTRAINT_TRIGGER, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(INHERITED_TABLES_FEATURE, "", queryissue.INHERITANCE, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(GENERATED_COLUMNS_FEATURE, "", queryissue.STORED_GENERATED_COLUMNS, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(CONVERSIONS_OBJECTS_FEATURE, "", CREATE_CONVERSION_ISSUE_TYPE, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(MULTI_COLUMN_GIN_INDEX_FEATURE, "", queryissue.MULTI_COLUMN_GIN_INDEX, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(ALTER_SETTING_ATTRIBUTE_FEATURE, "", queryissue.ALTER_TABLE_SET_COLUMN_ATTRIBUTE, schemaAnalysisReport, true))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(DISABLING_TABLE_RULE_FEATURE, "", queryissue.ALTER_TABLE_DISABLE_RULE, schemaAnalysisReport, true))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(CLUSTER_ON_FEATURE, "", queryissue.ALTER_TABLE_CLUSTER_ON, schemaAnalysisReport, true))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(STORAGE_PARAMETERS_FEATURE, "", queryissue.STORAGE_PARAMETERS, schemaAnalysisReport, true))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(EXTENSION_FEATURE, "", queryissue.UNSUPPORTED_EXTENSION, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(EXCLUSION_CONSTRAINT_FEATURE, "", queryissue.EXCLUSION_CONSTRAINTS, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(DEFERRABLE_CONSTRAINT_FEATURE, "", queryissue.DEFERRABLE_CONSTRAINTS, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(VIEW_CHECK_FEATURE, "", VIEW_WITH_CHECK_OPTION_ISSUE_TYPE, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getIndexesOnComplexTypeUnsupportedFeature(schemaAnalysisReport)...)
	unsupportedFeatures = append(unsupportedFeatures, getPKandUKOnComplexTypeUnsupportedFeature(schemaAnalysisReport)...)
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(UNLOGGED_TABLE_FEATURE, "", queryissue.UNLOGGED_TABLES, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(REFERENCING_TRIGGER_FEATURE, "", queryissue.REFERENCING_CLAUSE_IN_TRIGGER, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(BEFORE_FOR_EACH_ROW_TRIGGERS_ON_PARTITIONED_TABLE_FEATURE, "", queryissue.BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.ADVISORY_LOCKS_ISSUE_NAME, "", queryissue.ADVISORY_LOCKS, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.XML_FUNCTIONS_ISSUE_NAME, "", queryissue.XML_FUNCTIONS, schemaAnalysisReport, false))

	// TODO: test if the issues are getting populated correctly for UnsupportedFeatures struct wrt system columns
	for _, issueType := range queryissue.UnsupportedSystemColumnsIssueTypes {
		featureName := "System Columns"
		unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(featureName, "", issueType, schemaAnalysisReport, false))
	}

	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.LARGE_OBJECT_FUNCTIONS_ISSUE_NAME, "", queryissue.LARGE_OBJECT_FUNCTIONS, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(REGEX_FUNCTIONS_FEATURE, "", queryissue.REGEX_FUNCTIONS, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(FETCH_WITH_TIES_FEATURE, "", queryissue.FETCH_WITH_TIES, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.JSON_QUERY_FUNCTIONS_ISSUE_NAME, "", queryissue.JSON_QUERY_FUNCTION, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.JSON_CONSTRUCTOR_FUNCTION_ISSUE_NAME, "", queryissue.JSON_CONSTRUCTOR_FUNCTION, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.ANY_VALUE_AGGREGATE_FUNCTION_ISSUE_NAME, "", queryissue.ANY_VALUE_AGGREGATE_FUNCTION, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.RANGE_AGGREGATE_FUNCTION_ISSUE_NAME, "", queryissue.RANGE_AGGREGATE_FUNCTION, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.SECURITY_INVOKER_VIEWS_ISSUE_NAME, "", queryissue.SECURITY_INVOKER_VIEWS, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.DETERMINISTIC_OPTION_WITH_COLLATION_ISSUE_NAME, "", queryissue.DETERMINISTIC_OPTION_WITH_COLLATION, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.NON_DETERMINISTIC_COLLATION_ISSUE_NAME, "", queryissue.NON_DETERMINISTIC_COLLATION, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.UNIQUE_NULLS_NOT_DISTINCT_ISSUE_NAME, "", queryissue.UNIQUE_NULLS_NOT_DISTINCT, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.JSONB_SUBSCRIPTING_ISSUE_NAME, "", queryissue.JSONB_SUBSCRIPTING, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE_ISSUE_NAME, "", queryissue.FOREIGN_KEY_REFERENCES_PARTITIONED_TABLE, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.JSON_TYPE_PREDICATE_ISSUE_NAME, "", queryissue.JSON_TYPE_PREDICATE, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.SQL_BODY_IN_FUNCTION_ISSUE_NAME, "", queryissue.SQL_BODY_IN_FUNCTION, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.CTE_WITH_MATERIALIZED_CLAUSE_ISSUE_NAME, "", queryissue.CTE_WITH_MATERIALIZED_CLAUSE, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.NON_DECIMAL_INTEGER_LITERAL_ISSUE_NAME, "", queryissue.NON_DECIMAL_INTEGER_LITERAL, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.COMPRESSION_CLAUSE_IN_TABLE_ISSUE_NAME, "", queryissue.COMPRESSION_CLAUSE_IN_TABLE, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.HOTSPOTS_ON_DATE_INDEX_ISSUE, "", queryissue.HOTSPOTS_ON_DATE_INDEX, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.HOTSPOTS_ON_TIMESTAMP_INDEX_ISSUE, "", queryissue.HOTSPOTS_ON_TIMESTAMP_INDEX, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.LOW_CARDINALITY_INDEX_ISSUE_NAME, "", queryissue.LOW_CARDINALITY_INDEXES, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.MOST_FREQUENT_VALUE_INDEXES_ISSUE_NAME, "", queryissue.MOST_FREQUENT_VALUE_INDEXES, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.NULL_VALUE_INDEXES_ISSUE_NAME, "", queryissue.NULL_VALUE_INDEXES, schemaAnalysisReport, false))

	return lo.Filter(unsupportedFeatures, func(f UnsupportedFeature, _ int) bool {
		return len(f.Objects) > 0
	}), nil
}

func getPKandUKOnComplexTypeUnsupportedFeature(schemaAnalysisReport utils.SchemaReport) []UnsupportedFeature {
	log.Infof("fetching unsupported features for PK/UK on complex datatypes...")
	unsupportedFeatures := make([]UnsupportedFeature, 0)

	for _, issueTypeAndName := range queryissue.PkOrUkOnComplexDatatypesIssues {
		unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(issueTypeAndName.IssueName, "", issueTypeAndName.IssueType, schemaAnalysisReport, false))
	}

	return lo.Filter(unsupportedFeatures, func(f UnsupportedFeature, _ int) bool {
		return len(f.Objects) > 0
	})
}

func getIndexesOnComplexTypeUnsupportedFeature(schemaAnalysisReport utils.SchemaReport) []UnsupportedFeature {
	// TODO: include MinimumVersionsFixedIn
	log.Infof("fetching unsupported features for Index on complex datatypes...")
	unsupportedFeatures := make([]UnsupportedFeature, 0)

	for _, issueTypeAndName := range queryissue.IndexOnComplexDatatypesIssues {
		unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(issueTypeAndName.IssueName, "", issueTypeAndName.IssueType, schemaAnalysisReport, false))
	}

	return lo.Filter(unsupportedFeatures, func(f UnsupportedFeature, _ int) bool {
		return len(f.Objects) > 0
	})
}

func fetchUnsupportedOracleFeaturesFromSchemaReport(schemaAnalysisReport utils.SchemaReport) ([]UnsupportedFeature, error) {
	log.Infof("fetching unsupported features for Oracle...")
	unsupportedFeatures := make([]UnsupportedFeature, 0)
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(COMPOUND_TRIGGER_FEATURE, "", COMPOUND_TRIGGER_ISSUE_TYPE, schemaAnalysisReport, false))
	return lo.Filter(unsupportedFeatures, func(f UnsupportedFeature, _ int) bool {
		return len(f.Objects) > 0
	}), nil
}

var OracleUnsupportedIndexTypes = []string{"CLUSTER INDEX", "DOMAIN INDEX", "FUNCTION-BASED DOMAIN INDEX", "IOT - TOP INDEX", "NORMAL/REV INDEX", "FUNCTION-BASED NORMAL/REV INDEX"}

func fetchUnsupportedObjectTypes() ([]UnsupportedFeature, error) {
	if source.DBType != ORACLE {
		return nil, nil
	}

	query := fmt.Sprintf(`SELECT schema_name, object_name, object_type FROM %s`, migassessment.OBJECT_TYPE_MAPPING)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying-%s: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching object type mapping metadata: %v", err)
		}
	}()

	var unsupportedIndexes, virtualColumns, inheritedTypes, unsupportedPartitionTypes []ObjectInfo
	for rows.Next() {
		var schemaName, objectName, objectType string
		err = rows.Scan(&schemaName, &objectName, &objectType)
		if err != nil {
			return nil, fmt.Errorf("error scanning rows:%w", err)
		}

		if slices.Contains(OracleUnsupportedIndexTypes, objectType) {
			unsupportedIndexes = append(unsupportedIndexes, ObjectInfo{
				ObjectName: fmt.Sprintf("Index Name: %s, Index Type=%s", objectName, objectType),
			})
			// For oracle migration complexity comes from ora2pg, so defining Impact not required right now
			assessmentReport.AppendIssues(AssessmentIssue{
				Category:            UNSUPPORTED_FEATURES_CATEGORY,
				CategoryDescription: GetCategoryDescription(UNSUPPORTED_FEATURES_CATEGORY),
				Type:                UNSUPPORTED_INDEXES_ISSUE_TYPE,
				Name:                UNSUPPORTED_INDEXES_FEATURE,
				Description:         "", // TODO
				ObjectType:          constants.INDEX,
				// TODO: here it should be only ObjectName, to populate Index Type there should be a separate field
				ObjectName: fmt.Sprintf("Index Name: %s, Index Type=%s", objectName, objectType),
			})
		} else if objectType == VIRTUAL_COLUMN {
			virtualColumns = append(virtualColumns, ObjectInfo{ObjectName: objectName})
			assessmentReport.AppendIssues(AssessmentIssue{
				Category:            UNSUPPORTED_FEATURES_CATEGORY,
				CategoryDescription: GetCategoryDescription(UNSUPPORTED_FEATURES_CATEGORY),
				Type:                VIRTUAL_COLUMNS_ISSUE_TYPE,
				Name:                VIRTUAL_COLUMNS_FEATURE,
				Description:         "", // TODO
				ObjectType:          constants.COLUMN,
				ObjectName:          objectName,
			})
		} else if objectType == INHERITED_TYPE {
			inheritedTypes = append(inheritedTypes, ObjectInfo{ObjectName: objectName})
			assessmentReport.AppendIssues(AssessmentIssue{
				Category:            UNSUPPORTED_FEATURES_CATEGORY,
				CategoryDescription: GetCategoryDescription(UNSUPPORTED_FEATURES_CATEGORY),
				Type:                INHERITED_TYPES_ISSUE_TYPE,
				Name:                INHERITED_TYPES_FEATURE,
				Description:         "", // TODO
				ObjectType:          constants.TYPE,
				ObjectName:          objectName,
			})
		} else if objectType == REFERENCE_PARTITION || objectType == SYSTEM_PARTITION {
			referenceOrTablePartitionPresent = true
			unsupportedPartitionTypes = append(unsupportedPartitionTypes, ObjectInfo{ObjectName: fmt.Sprintf("Table Name: %s, Partition Method: %s", objectName, objectType)})

			assessmentReport.AppendIssues(AssessmentIssue{
				Category:            UNSUPPORTED_FEATURES_CATEGORY,
				CategoryDescription: GetCategoryDescription(UNSUPPORTED_FEATURES_CATEGORY),
				Type:                UNSUPPORTED_PARTITIONING_METHODS_ISSUE_TYPE,
				Name:                UNSUPPORTED_PARTITIONING_METHODS_FEATURE,
				Description:         "", // TODO
				ObjectType:          constants.TABLE,
				// TODO: here it should be only ObjectName, to populate Partition Method there should be a separate field
				ObjectName: fmt.Sprintf("Table Name: %s, Partition Method: %s", objectName, objectType),
			})
		}
	}

	unsupportedFeatures := make([]UnsupportedFeature, 0)
	unsupportedFeatures = append(unsupportedFeatures, UnsupportedFeature{UNSUPPORTED_INDEXES_FEATURE, unsupportedIndexes, false, "", "", nil})
	unsupportedFeatures = append(unsupportedFeatures, UnsupportedFeature{VIRTUAL_COLUMNS_FEATURE, virtualColumns, false, "", "", nil})
	unsupportedFeatures = append(unsupportedFeatures, UnsupportedFeature{INHERITED_TYPES_FEATURE, inheritedTypes, false, "", "", nil})
	unsupportedFeatures = append(unsupportedFeatures, UnsupportedFeature{UNSUPPORTED_PARTITIONING_METHODS_FEATURE, unsupportedPartitionTypes, false, "", "", nil})
	return lo.Filter(unsupportedFeatures, func(f UnsupportedFeature, _ int) bool {
		return len(f.Objects) > 0
	}), nil
}

func fetchUnsupportedPlPgSQLObjects(schemaAnalysisReport utils.SchemaReport) []UnsupportedFeature {
	if source.DBType != POSTGRESQL {
		return nil
	}

	plpgsqlIssues := lo.Filter(schemaAnalysisReport.Issues, func(issue utils.AnalyzeSchemaIssue, _ int) bool {
		return issue.IssueType == UNSUPPORTED_PLPGSQL_OBJECTS_CATEGORY
	})
	groupPlpgsqlIssuesByIssueName := lo.GroupBy(plpgsqlIssues, func(issue utils.AnalyzeSchemaIssue) string {
		return issue.Name
	})
	var unsupportedPlpgSqlObjects []UnsupportedFeature
	for issueName, analyzeSchemaIssues := range groupPlpgsqlIssuesByIssueName {
		var objects []ObjectInfo
		var docsLink string
		var minVersionsFixedIn map[string]*ybversion.YBVersion
		var minVersionsFixedInSet bool

		for _, issue := range analyzeSchemaIssues {
			if !minVersionsFixedInSet {
				minVersionsFixedIn = issue.MinimumVersionsFixedIn
				minVersionsFixedInSet = true
			}
			if !areMinVersionsFixedInEqual(minVersionsFixedIn, issue.MinimumVersionsFixedIn) {
				utils.ErrExit("Issues belonging to UnsupportedFeature %s have different minimum versions fixed in: %v, %v", issueName, minVersionsFixedIn, issue.MinimumVersionsFixedIn)
			}

			objects = append(objects, ObjectInfo{
				ObjectType:   issue.ObjectType,
				ObjectName:   issue.ObjectName,
				SqlStatement: issue.SqlStatement,
			})
			docsLink = issue.DocsLink
			assessmentReport.AppendIssues(convertAnalyzeSchemaIssueToAssessmentIssue(issue, issue.MinimumVersionsFixedIn))
		}
		feature := UnsupportedFeature{
			FeatureName:            issueName,
			DisplayDDL:             true,
			DocsLink:               docsLink,
			Objects:                objects,
			MinimumVersionsFixedIn: minVersionsFixedIn,
		}
		unsupportedPlpgSqlObjects = append(unsupportedPlpgSqlObjects, feature)
	}

	return unsupportedPlpgSqlObjects
}

func fetchUnsupportedQueryConstructs() ([]utils.UnsupportedQueryConstruct, error) {
	if source.DBType != POSTGRESQL {
		return nil, nil
	}
	query := fmt.Sprintf("SELECT DISTINCT query from %s", migassessment.DB_QUERIES_SUMMARY)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying=%s on assessmentDB: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching database queries summary metadata: %v", err)
		}
	}()

	var executedQueries []string
	for rows.Next() {
		var executedQuery string
		err := rows.Scan(&executedQuery)
		if err != nil {
			return nil, fmt.Errorf("error scanning rows: %w", err)
		}
		executedQueries = append(executedQueries, executedQuery)
	}

	if len(executedQueries) == 0 {
		log.Infof("queries info not present in the assessment metadata for detecting unsupported query constructs")
		return nil, nil
	}

	var result []utils.UnsupportedQueryConstruct
	for i := 0; i < len(executedQueries); i++ {
		query := executedQueries[i]
		log.Debugf("fetching unsupported query constructs for query - [%s]", query)
		collectedSchemaList, err := queryparser.GetSchemaUsed(query)
		if err != nil { // no need to error out if failed to get schemas for a query
			log.Errorf("failed to get schemas used for query [%s]: %v", query, err)
			continue
		}

		log.Infof("collected schema list %v(len=%d) for query [%s]", collectedSchemaList, len(collectedSchemaList), query)
		if !considerQueryForIssueDetection(collectedSchemaList) {
			log.Infof("ignoring query due to difference in collected schema list %v(len=%d) vs source schema list %v(len=%d)",
				collectedSchemaList, len(collectedSchemaList), source.GetSchemaList(), len(source.GetSchemaList()))
			continue
		}

		issues, err := parserIssueDetector.GetDMLIssues(query, targetDbVersion)
		if err != nil {
			log.Errorf("failed while trying to fetch query issues in query - [%s]: %v",
				query, err)
		}

		for _, issue := range issues {
			uqc := utils.UnsupportedQueryConstruct{
				Query:                  issue.SqlStatement,
				ConstructTypeName:      issue.Name,
				DocsLink:               issue.DocsLink,
				MinimumVersionsFixedIn: issue.MinimumVersionsFixedIn,
			}
			result = append(result, uqc)

			assessmentReport.AppendIssues(AssessmentIssue{
				Category:               UNSUPPORTED_QUERY_CONSTRUCTS_CATEGORY,
				CategoryDescription:    GetCategoryDescription(UNSUPPORTED_QUERY_CONSTRUCTS_CATEGORY),
				Type:                   issue.Type,
				Name:                   issue.Name,
				Impact:                 issue.Impact,
				Description:            issue.Description,
				SqlStatement:           issue.SqlStatement,
				DocsLink:               issue.DocsLink,
				MinimumVersionsFixedIn: issue.MinimumVersionsFixedIn,
			})
		}
	}

	// sort the slice to group same constructType in html and json reports
	log.Infof("sorting the result slice based on construct type")
	sort.Slice(result, func(i, j int) bool {
		return result[i].ConstructTypeName <= result[j].ConstructTypeName
	})
	return result, nil
}

func fetchColumnsWithUnsupportedDataTypes() ([]utils.TableColumnsDataTypes, []utils.TableColumnsDataTypes, []utils.TableColumnsDataTypes, error) {
	var unsupportedDataTypes, unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB []utils.TableColumnsDataTypes

	query := fmt.Sprintf(`SELECT schema_name, table_name, column_name, data_type FROM %s`,
		migassessment.TABLE_COLUMNS_DATA_TYPES)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("error querying-%s on assessmentDB: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching unsupported datatypes metadata: %v", err)
		}
	}()

	var allColumnsDataTypes []utils.TableColumnsDataTypes
	for rows.Next() {
		var columnDataTypes utils.TableColumnsDataTypes
		err := rows.Scan(&columnDataTypes.SchemaName, &columnDataTypes.TableName,
			&columnDataTypes.ColumnName, &columnDataTypes.DataType)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("error scanning rows: %w", err)
		}

		allColumnsDataTypes = append(allColumnsDataTypes, columnDataTypes)
	}

	var sourceUnsupportedDatatypes, liveUnsupportedDatatypes, liveWithFForFBUnsupportedDatatypes []string

	switch source.DBType {
	case POSTGRESQL:
		sourceUnsupportedDatatypes = srcdb.PostgresUnsupportedDataTypes
		liveUnsupportedDatatypes = srcdb.GetPGLiveMigrationUnsupportedDatatypes()
		liveWithFForFBUnsupportedDatatypes = srcdb.GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes()
	case ORACLE:
		sourceUnsupportedDatatypes = srcdb.OracleUnsupportedDataTypes
	default:
		panic(fmt.Sprintf("invalid source db type %q", source.DBType))
	}
	// filter columns with unsupported data types using sourceUnsupportedDataTypes
	for i := 0; i < len(allColumnsDataTypes); i++ {
		//Using this ContainsAnyStringFromSlice as the catalog we use for fetching datatypes uses the data_type only
		// which just contains the base type for example VARCHARs it won't include any length, precision or scale information
		//of these types there are other columns available for these information so we just do string match of types with our list
		splits := strings.Split(allColumnsDataTypes[i].DataType, ".")
		typeName := splits[len(splits)-1] //using typename only for the cases we are checking it from the static list of type names
		typeName = strings.TrimSuffix(typeName, "[]")

		isUnsupportedDatatype := utils.ContainsAnyStringFromSlice(sourceUnsupportedDatatypes, typeName)
		isUnsupportedDatatypeInLive := utils.ContainsAnyStringFromSlice(liveUnsupportedDatatypes, typeName)

		isUnsupportedDatatypeInLiveWithFFOrFBList := utils.ContainsAnyStringFromSlice(liveWithFForFBUnsupportedDatatypes, typeName)
		isUDTDatatype := utils.ContainsAnyStringFromSlice(parserIssueDetector.GetCompositeTypes(), allColumnsDataTypes[i].DataType)
		isArrayDatatype := strings.HasSuffix(allColumnsDataTypes[i].DataType, "[]")                                                                       //if type is array
		isEnumDatatype := utils.ContainsAnyStringFromSlice(parserIssueDetector.GetEnumTypes(), strings.TrimSuffix(allColumnsDataTypes[i].DataType, "[]")) //is ENUM type
		isArrayOfEnumsDatatype := isArrayDatatype && isEnumDatatype

		allColumnsDataTypes[i].IsArrayType = isArrayDatatype
		allColumnsDataTypes[i].IsEnumType = isEnumDatatype
		allColumnsDataTypes[i].IsUDTType = isUDTDatatype

		isUnsupportedDatatypeInLiveWithFFOrFB := isUnsupportedDatatypeInLiveWithFFOrFBList || isUDTDatatype || isArrayOfEnumsDatatype

		switch true {
		case isUnsupportedDatatype:
			unsupportedDataTypes = append(unsupportedDataTypes, allColumnsDataTypes[i])
		case isUnsupportedDatatypeInLive:
			unsupportedDataTypesForLiveMigration = append(unsupportedDataTypesForLiveMigration, allColumnsDataTypes[i])
		case isUnsupportedDatatypeInLiveWithFFOrFB:
			/*
				TODO test this for Oracle case if there is any special handling required
				For Live mgiration with FF or FB, It is meant to be for the datatypes that are going to be in YB after migration
				so it makes sense to use the analyzeSchema `compositeTypes` or `enumTypes` and check from there but some information
				we are still using from Source which might need a better way in case of Oracle as for PG it doesn't really makes a difference in
				source or analyzeSchema's results.
			*/
			//reporting types in the list YugabyteUnsupportedDataTypesForDbzm, UDT and array on ENUMs columns as unsupported with live migration with ff/fb
			unsupportedDataTypesForLiveMigrationWithFForFB = append(unsupportedDataTypesForLiveMigrationWithFForFB, allColumnsDataTypes[i])

		}
	}

	return unsupportedDataTypes, unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB, nil
}

func addAssessmentIssuesForUnsupportedDatatypes(unsupportedDatatypes []utils.TableColumnsDataTypes) {
	for _, colInfo := range unsupportedDatatypes {
		qualifiedColName := fmt.Sprintf("%s.%s.%s", colInfo.SchemaName, colInfo.TableName, colInfo.ColumnName)
		switch source.DBType {
		case ORACLE:
			issue := AssessmentIssue{
				Category:               UNSUPPORTED_DATATYPES_CATEGORY,
				CategoryDescription:    GetCategoryDescription(UNSUPPORTED_DATATYPES_CATEGORY),
				Type:                   colInfo.DataType, // TODO: maybe name it like "unsupported datatype - geometry"
				Name:                   colInfo.DataType, // TODO: maybe name it like "unsupported datatype - geometry"
				Description:            "",               // TODO
				Impact:                 constants.IMPACT_LEVEL_3,
				ObjectType:             constants.COLUMN,
				ObjectName:             qualifiedColName,
				DocsLink:               "",  // TODO
				MinimumVersionsFixedIn: nil, // TODO
			}
			assessmentReport.AppendIssues(issue)
		case POSTGRESQL:
			// Datatypes can be of form public.geometry, so we need to extract the datatype from it
			datatype, ok := utils.SliceLastElement(strings.Split(colInfo.DataType, "."))
			if !ok {
				log.Warnf("failed to get datatype from %s", colInfo.DataType)
				continue
			}

			// We obtain the queryissue from the Report function. This queryissue is first converted to AnalyzeIssue and then to AssessmentIssue using pre existing function
			// Coneverting queryissue directly to AssessmentIssue would have lead to the creation of a new function which would have required a lot of cases to be handled and led to code duplication
			// This converted AssessmentIssue is then appended to the assessmentIssues slice
			queryissue := queryissue.ReportUnsupportedDatatypes(datatype, colInfo.ColumnName, constants.COLUMN, qualifiedColName)
			checkIsFixedInAndAddIssueToAssessmentIssues(queryissue)

		default:
			panic(fmt.Sprintf("invalid source db type %q", source.DBType))
		}

	}
}

func checkIsFixedInAndAddIssueToAssessmentIssues(queryIssue queryissue.QueryIssue) {
	fixed, err := queryIssue.IsFixedIn(targetDbVersion)
	if err != nil {
		log.Warnf("checking if issue %v is supported: %v", queryIssue, err)
	}
	if !fixed {
		convertedAnalyzeIssue := convertIssueInstanceToAnalyzeIssue(queryIssue, "", false, false)
		issue := convertAnalyzeSchemaIssueToAssessmentIssue(convertedAnalyzeIssue, queryIssue.MinimumVersionsFixedIn)
		assessmentReport.AppendIssues(issue)
	}
}

/*
Queries to ignore:
- Collected schemas is totally different than source schema list, not containing ""

Queries to consider:
- Collected schemas subset of source schema list
- Collected schemas contains some from source schema list and some extras

Caveats:
There can be a lot of false positives.
For example: standard sql functions like sum(), count() won't be qualified(pg_catalog) in queries generally.
Making the schema unknown for that object, resulting in query consider
*/
func considerQueryForIssueDetection(collectedSchemaList []string) bool {
	// filtering out pg_catalog schema, since it doesn't impact query consideration decision
	collectedSchemaList = lo.Filter(collectedSchemaList, func(item string, _ int) bool {
		return item != "pg_catalog"
	})

	sourceSchemaList := strings.Split(source.Schema, "|")

	// fallback in case: unable to collect objects or there are no object(s) in the query
	if len(collectedSchemaList) == 0 {
		return true
	}

	// empty schemaname indicates presence of unqualified objectnames in query
	if slices.Contains(collectedSchemaList, "") {
		log.Debug("considering due to empty schema\n")
		return true
	}

	for _, collectedSchema := range collectedSchemaList {
		if slices.Contains(sourceSchemaList, collectedSchema) {
			log.Debugf("considering due to '%s' schema\n", collectedSchema)
			return true
		}
	}
	return false
}

const (
	PREVIEW_FEATURES_NOTE                = `Some features listed in this report may be supported under a preview flag in the specified target-db-version of YugabyteDB. Please refer to the official <a class="highlight-link" target="_blank" href="https://docs.yugabyte.com/preview/releases/ybdb-releases/">release notes</a> for detailed information and usage guidelines.`
	RANGE_SHARDED_INDEXES_RECOMMENDATION = `If indexes are created on columns commonly used in range-based queries (e.g. timestamp columns), it is recommended to explicitly configure these indexes with range sharding. This ensures efficient data access for range queries.
By default, YugabyteDB uses hash sharding for indexes, which distributes data randomly and is not ideal for range-based predicates potentially degrading query performance. Note that range sharding is enabled by default only in <a class="highlight-link" target="_blank" href="https://docs.yugabyte.com/preview/develop/postgresql-compatibility/">PostgreSQL compatibility mode</a> in YugabyteDB.`
	COLOCATED_TABLE_RECOMMENDATION_CAVEAT = `If there are any tables that receive disproportionately high load, ensure that they are NOT colocated to avoid the colocated tablet becoming a hotspot.
For additional considerations related to colocated tables, refer to the documentation at: https://docs.yugabyte.com/preview/explore/colocation/#limitations-and-considerations`
	ORACLE_PARTITION_DEFAULT_COLOCATION = `For sharding/colocation recommendations, each partition is treated individually. During the export schema phase, all the partitions of a partitioned table are currently created as colocated by default.
To manually modify the schema, please refer: <a class="highlight-link" href="https://github.com/yugabyte/yb-voyager/issues/1581">https://github.com/yugabyte/yb-voyager/issues/1581</a>.`

	ORACLE_UNSUPPPORTED_PARTITIONING = `Reference and System Partitioned tables are created as normal tables, but are not considered for target cluster sizing recommendations.`

	GIN_INDEXES                = `There are some BITMAP indexes present in the schema that will get converted to GIN indexes, but GIN indexes are partially supported in YugabyteDB as mentioned in <a class="highlight-link" href="https://github.com/yugabyte/yugabyte-db/issues/7850">https://github.com/yugabyte/yugabyte-db/issues/7850</a> so take a look and modify them if not supported.`
	UNLOGGED_TABLE_NOTE        = `There are some Unlogged tables in the schema. They will be created as regular LOGGED tables in YugabyteDB as unlogged tables are not supported.`
	REPORTING_LIMITATIONS_NOTE = `<a class="highlight-link" target="_blank"  href="https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/#assessment-and-schema-analysis-limitations">Limitations in assessment</a>`
)

const FOREIGN_TABLE_NOTE = `There are some Foreign tables in the schema, but during the export schema phase, exported schema does not include the SERVER and USER MAPPING objects. Therefore, you must manually create these objects before import schema. For more information on each of them, run analyze-schema. `

// TODO: fix notes handling for html tags just for html and not for json
func addNotesToAssessmentReport() {
	log.Infof("adding notes to assessment report")

	assessmentReport.Notes = append(assessmentReport.Notes, PREVIEW_FEATURES_NOTE)
	// keep it as the first point in Notes
	if len(assessmentReport.Sizing.SizingRecommendation.ColocatedTables) > 0 {
		assessmentReport.Notes = append(assessmentReport.Notes, COLOCATED_TABLE_RECOMMENDATION_CAVEAT)
	}
	for _, dbObj := range schemaAnalysisReport.SchemaSummary.DBObjects {
		if dbObj.ObjectType == "INDEX" && dbObj.TotalCount > 0 {
			assessmentReport.Notes = append(assessmentReport.Notes, RANGE_SHARDED_INDEXES_RECOMMENDATION)
			break
		}
	}
	switch source.DBType {
	case ORACLE:
		partitionSqlFPath := filepath.Join(assessmentMetadataDir, "schema", "partitions", "partition.sql")
		// file exists and isn't empty (containing PARTITIONs DDLs)
		if utils.FileOrFolderExists(partitionSqlFPath) && !utils.IsFileEmpty(partitionSqlFPath) {
			assessmentReport.Notes = append(assessmentReport.Notes, ORACLE_PARTITION_DEFAULT_COLOCATION)
		}
		if referenceOrTablePartitionPresent {
			assessmentReport.Notes = append(assessmentReport.Notes, ORACLE_UNSUPPPORTED_PARTITIONING)
		}

		// checking if gin indexes are present.
		for _, dbObj := range schemaAnalysisReport.SchemaSummary.DBObjects {
			if dbObj.ObjectType == "INDEX" {
				if strings.Contains(dbObj.Details, GIN_INDEX_DETAILS) {
					assessmentReport.Notes = append(assessmentReport.Notes, GIN_INDEXES)
					break
				}
			}
		}
	case POSTGRESQL:
		if parserIssueDetector.IsUnloggedTablesIssueFiltered() {
			assessmentReport.Notes = append(assessmentReport.Notes, UNLOGGED_TABLE_NOTE)
		}
		assessmentReport.Notes = append(assessmentReport.Notes, REPORTING_LIMITATIONS_NOTE)
	}

}

func addMigrationCaveatsToAssessmentReport(unsupportedDataTypesForLiveMigration []utils.TableColumnsDataTypes, unsupportedDataTypesForLiveMigrationWithFForFB []utils.TableColumnsDataTypes) {
	switch source.DBType {
	case POSTGRESQL:
		log.Infof("add migration caveats to assessment report")
		migrationCaveats := make([]UnsupportedFeature, 0)
		migrationCaveats = append(migrationCaveats, getUnsupportedFeaturesFromSchemaAnalysisReport(ALTER_PARTITION_ADD_PK_CAVEAT_FEATURE, "", queryissue.ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE,
			schemaAnalysisReport, true))
		migrationCaveats = append(migrationCaveats, getUnsupportedFeaturesFromSchemaAnalysisReport(FOREIGN_TABLE_CAVEAT_FEATURE, "", queryissue.FOREIGN_TABLE,
			schemaAnalysisReport, false))
		migrationCaveats = append(migrationCaveats, getUnsupportedFeaturesFromSchemaAnalysisReport(POLICIES_CAVEAT_FEATURE, "", queryissue.POLICY_WITH_ROLES,
			schemaAnalysisReport, false))

		if len(unsupportedDataTypesForLiveMigration) > 0 {
			columns := make([]ObjectInfo, 0)
			for _, colInfo := range unsupportedDataTypesForLiveMigration {
				qualifiedColName := fmt.Sprintf("%s.%s.%s", colInfo.SchemaName, colInfo.TableName, colInfo.ColumnName)
				columns = append(columns, ObjectInfo{ObjectName: fmt.Sprintf("%s (%s)", qualifiedColName, colInfo.DataType)})

				datatype, ok := utils.SliceLastElement(strings.Split(colInfo.DataType, "."))
				if !ok {
					log.Warnf("failed to get datatype from %s", colInfo.DataType)
					continue
				}

				// We obtain the queryissue from the Report function. This queryissue is first converted to AnalyzeIssue and then to AssessmentIssue using pre existing function
				// Coneverting queryissue directly to AssessmentIssue would have lead to the creation of a new function which would have required a lot of cases to be handled and led to code duplication
				// This converted AssessmentIssue is then appended to the assessmentIssues slice
				queryIssue := queryissue.ReportUnsupportedDatatypesInLive(datatype, colInfo.ColumnName, constants.COLUMN, qualifiedColName)
				checkIsFixedInAndAddIssueToAssessmentIssues(queryIssue)
			}
			if len(columns) > 0 {
				migrationCaveats = append(migrationCaveats, UnsupportedFeature{UNSUPPORTED_DATATYPES_LIVE_CAVEAT_FEATURE, columns, false, UNSUPPORTED_DATATYPE_LIVE_MIGRATION_DOC_LINK, UNSUPPORTED_DATATYPES_FOR_LIVE_MIGRATION_DESCRIPTION, nil})
			}
		}
		if len(unsupportedDataTypesForLiveMigrationWithFForFB) > 0 {
			columns := make([]ObjectInfo, 0)
			for _, colInfo := range unsupportedDataTypesForLiveMigrationWithFForFB {
				qualifiedColName := fmt.Sprintf("%s.%s.%s", colInfo.SchemaName, colInfo.TableName, colInfo.ColumnName)
				columns = append(columns, ObjectInfo{ObjectName: fmt.Sprintf("%s (%s)", qualifiedColName, colInfo.DataType)})

				datatype, ok := utils.SliceLastElement(strings.Split(colInfo.DataType, "."))
				if !ok {
					log.Warnf("failed to get datatype from %s", colInfo.DataType)
					continue
				}

				var queryIssue queryissue.QueryIssue

				if colInfo.IsArrayType && colInfo.IsEnumType {
					queryIssue = queryissue.NewArrayOfEnumDatatypeIssue(
						constants.COLUMN,
						qualifiedColName,
						"",
						datatype,
						colInfo.ColumnName,
					)
				} else if colInfo.IsUDTType {
					queryIssue = queryissue.NewUserDefinedDatatypeIssue(
						constants.COLUMN,
						qualifiedColName,
						"",
						datatype,
						colInfo.ColumnName,
					)
				} else {
					queryIssue = queryissue.ReportUnsupportedDatatypesInLiveWithFFOrFB(datatype, colInfo.ColumnName, constants.COLUMN, qualifiedColName)
				}
				checkIsFixedInAndAddIssueToAssessmentIssues(queryIssue)

			}
			if len(columns) > 0 {
				migrationCaveats = append(migrationCaveats, UnsupportedFeature{UNSUPPORTED_DATATYPES_LIVE_WITH_FF_FB_CAVEAT_FEATURE, columns, false, UNSUPPORTED_DATATYPE_LIVE_MIGRATION_DOC_LINK, UNSUPPORTED_DATATYPES_FOR_LIVE_MIGRATION_WITH_FF_FB_DESCRIPTION, nil})
			}
		}
		migrationCaveats = lo.Filter(migrationCaveats, func(m UnsupportedFeature, _ int) bool {
			return len(m.Objects) > 0
		})
		if len(migrationCaveats) > 0 {
			assessmentReport.MigrationCaveats = migrationCaveats
		}

	}
}

func postProcessingOfAssessmentReport() {
	switch source.DBType {
	case ORACLE:
		log.Infof("post processing of assessment report to remove the schema name from fully qualified table names")
		for i := range assessmentReport.Sizing.SizingRecommendation.ShardedTables {
			parts := strings.Split(assessmentReport.Sizing.SizingRecommendation.ShardedTables[i], ".")
			if len(parts) > 1 {
				assessmentReport.Sizing.SizingRecommendation.ShardedTables[i] = parts[1]
			}
		}

		for i := range assessmentReport.Sizing.SizingRecommendation.ColocatedTables {
			parts := strings.Split(assessmentReport.Sizing.SizingRecommendation.ColocatedTables[i], ".")
			if len(parts) > 1 {
				assessmentReport.Sizing.SizingRecommendation.ColocatedTables[i] = parts[1]
			}
		}

		// redact Impact info from the assessment report for Oracle
		// TODO: Remove this processing step in future when supporting Explanation for Oracle
		for i := range assessmentReport.Issues {
			assessmentReport.Issues[i].Impact = "-"
		}
	case POSTGRESQL:
		//sort issues based on Category with a defined order and keep all the Performance Optimization ones at the last
		var categoryOrder = map[string]int{
			UNSUPPORTED_DATATYPES_CATEGORY:        0,
			UNSUPPORTED_FEATURES_CATEGORY:         1,
			UNSUPPORTED_QUERY_CONSTRUCTS_CATEGORY: 2,
			UNSUPPORTED_PLPGSQL_OBJECTS_CATEGORY:  3,
			MIGRATION_CAVEATS_CATEGORY:            4,
			PERFORMANCE_OPTIMIZATIONS_CATEGORY:    5,
		}

		sort.Slice(assessmentReport.Issues, func(i, j int) bool {
			rank := func(cat string) int {
				if r, ok := categoryOrder[cat]; ok {
					return r
				}
				//New categories are considered last in the ordering
				return len(categoryOrder)
			}
			return rank(assessmentReport.Issues[i].Category) < rank(assessmentReport.Issues[j].Category)
		})
	}

}

func generateAssessmentReportJson(reportDir string) error {
	jsonReportFilePath := filepath.Join(reportDir, fmt.Sprintf("%s%s", ASSESSMENT_FILE_NAME, JSON_EXTENSION))
	log.Infof("writing assessment report to file: %s", jsonReportFilePath)

	strReport, err := json.MarshalIndent(assessmentReport, "", "\t")
	if err != nil {
		return fmt.Errorf("failed to marshal the assessment report: %w", err)
	}

	err = os.WriteFile(jsonReportFilePath, strReport, 0644)
	if err != nil {
		return fmt.Errorf("failed to write assessment report to file: %w", err)
	}

	utils.PrintAndLog("generated JSON assessment report at: %s", jsonReportFilePath)
	return nil
}

func generateAssessmentReportHtml(reportDir string) error {
	htmlReportFilePath := filepath.Join(reportDir, fmt.Sprintf("%s%s", ASSESSMENT_FILE_NAME, HTML_EXTENSION))
	log.Infof("writing assessment report to file: %s", htmlReportFilePath)

	file, err := os.Create(htmlReportFilePath)
	if err != nil {
		return fmt.Errorf("failed to create file for %q: %w", filepath.Base(htmlReportFilePath), err)
	}
	defer func() {
		err := file.Close()
		if err != nil {
			log.Errorf("failed to close file %q: %v", htmlReportFilePath, err)
		}
	}()

	log.Infof("creating template for assessment report...")
	funcMap := template.FuncMap{
		"split":                            split,
		"groupByObjectType":                groupByObjectType,
		"numKeysInMapStringObjectInfo":     numKeysInMapStringObjectInfo,
		"groupByObjectName":                groupByObjectName,
		"totalUniqueObjectNamesOfAllTypes": totalUniqueObjectNamesOfAllTypes,
		"getSupportedVersionString":        getSupportedVersionString,
		"snakeCaseToTitleCase":             utils.SnakeCaseToTitleCase,
		"camelCaseToTitleCase":             utils.CamelCaseToTitleCase,
		"getSqlPreview":                    utils.GetSqlStmtToPrint,
	}
	tmpl := template.Must(template.New("report").Funcs(funcMap).Parse(string(bytesTemplate)))

	log.Infof("execute template for assessment report...")
	if source.DBType == POSTGRESQL {
		// marking this as empty to not display this in html report for PG
		assessmentReport.SchemaSummary.SchemaNames = []string{}
	}

	type CombinedStruct struct {
		AssessmentReport
		MigrationComplexityCategorySummary []MigrationComplexityCategorySummary
	}
	combined := CombinedStruct{
		AssessmentReport:                   assessmentReport,
		MigrationComplexityCategorySummary: buildCategorySummary(source.DBType, assessmentReport.Issues),
	}

	err = tmpl.Execute(file, combined)
	if err != nil {
		return fmt.Errorf("failed to render the assessment report: %w", err)
	}

	utils.PrintAndLog("generated HTML assessment report at: %s", htmlReportFilePath)
	return nil
}

func groupByObjectType(objects []ObjectInfo) map[string][]ObjectInfo {
	return lo.GroupBy(objects, func(object ObjectInfo) string {
		return object.ObjectType
	})
}

func groupByObjectName(objects []ObjectInfo) map[string][]ObjectInfo {
	return lo.GroupBy(objects, func(object ObjectInfo) string {
		return object.ObjectName
	})
}

func totalUniqueObjectNamesOfAllTypes(m map[string][]ObjectInfo) int {
	totalObjectNames := 0
	for _, objects := range m {
		totalObjectNames += len(lo.Keys(groupByObjectName(objects)))
	}
	return totalObjectNames
}

func numKeysInMapStringObjectInfo(m map[string][]ObjectInfo) int {
	return len(lo.Keys(m))
}

func split(value string, delimiter string) []string {
	return strings.Split(value, delimiter)
}

func getSupportedVersionString(minimumVersionsFixedIn map[string]*ybversion.YBVersion) string {
	if minimumVersionsFixedIn == nil {
		return ""
	}
	supportedVersions := []string{}
	for series, minVersionFixedIn := range minimumVersionsFixedIn {
		if minVersionFixedIn == nil {
			continue
		}
		supportedVersions = append(supportedVersions, fmt.Sprintf(">=%s (%s series)", minVersionFixedIn.String(), series))
	}
	return strings.Join(supportedVersions, ", ")
}

func validateSourceDBTypeForAssessMigration() {
	if source.DBType == "" {
		utils.ErrExit("Error required flag \"source-db-type\" not set")
	}

	source.DBType = strings.ToLower(source.DBType)
	if !slices.Contains(assessMigrationSupportedDBTypes, source.DBType) {
		utils.ErrExit("Error Invalid source-db-type: %q. Supported source db types for assess-migration are: [%v]",
			source.DBType, strings.Join(assessMigrationSupportedDBTypes, ", "))
	}
}

func validateAssessmentMetadataDirFlag() {
	if assessmentMetadataDirFlag != "" {
		if !utils.FileOrFolderExists(assessmentMetadataDirFlag) {
			utils.ErrExit("provided with `--assessment-metadata-dir` flag does not exist: %q ", assessmentMetadataDirFlag)
		} else {
			log.Infof("using provided assessment metadata directory: %s", assessmentMetadataDirFlag)
		}
	}
}

func validateAndSetTargetDbVersionFlag() error {
	if targetDbVersionStrFlag == "" {
		if utils.AskPrompt("No target-db-version has been specified.\nDo you want to continue with the latest stable YugabyteDB version:", ybversion.LatestStable.String()) {
			targetDbVersion = ybversion.LatestStable
			return nil
		} else {
			utils.ErrExit("Aborting..")
			return nil
		}
	}

	var err error
	targetDbVersion, err = ybversion.NewYBVersion(targetDbVersionStrFlag)

	if err == nil || !errors.Is(err, ybversion.ErrUnsupportedSeries) {
		return err
	}

	// error is ErrUnsupportedSeries
	utils.PrintAndLog("%v", err)
	if utils.AskPrompt("Do you want to continue with the latest stable YugabyteDB version:", ybversion.LatestStable.String()) {
		targetDbVersion = ybversion.LatestStable
		return nil
	} else {
		utils.ErrExit("Aborting..")
		return nil
	}
}

func validateSourceDBIOPSForAssessMigration() error {
	var totalIOPS int64

	tableIndexStats, err := assessmentDB.FetchAllStats()
	if err != nil {
		return fmt.Errorf("fetching all stats info from AssessmentDB: %w", err)
	}

	// Checking if source schema has zero objects.
	if tableIndexStats == nil || len(*tableIndexStats) == 0 {
		if utils.AskPrompt("No objects found in the specified schema(s). Do you want to continue anyway") {
			return nil
		} else {
			utils.ErrExit("Aborting..")
			return nil
		}
	}

	for _, stat := range *tableIndexStats {
		totalIOPS += utils.SafeDereferenceInt64(stat.ReadsPerSecond)
		totalIOPS += utils.SafeDereferenceInt64(stat.WritesPerSecond)
	}

	// Checking if source schema IOPS is not zero.
	if totalIOPS == 0 {
		if utils.AskPrompt("Detected 0 read/write IOPS on the tables in specified schema(s). In order to get an accurate assessment, it is recommended that the source database is actively handling its typical workloads. Do you want to continue anyway") {
			return nil
		} else {
			utils.ErrExit("Aborting..")
			return nil
		}
	}

	return nil
}
