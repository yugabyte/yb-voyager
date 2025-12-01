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
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"text/template"

	goerrors "github.com/go-errors/errors"

	"github.com/fatih/color"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/types"
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
	sourceReadReplicaEndpoints       string // CLI flag - package variable for Cobra binding
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
		validateReplicaEndpointsFlag()
		err = validateAndSetTargetDbVersionFlag()
		if err != nil {
			utils.ErrExit("failed to validate target db version: %w", err)
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
			utils.ErrExit("%w", err)
		}
		packAndSendAssessMigrationPayload(COMPLETE, nil)
	},
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

	assessMigrationCmd.Flags().StringVar(&sourceReadReplicaEndpoints, "source-read-replica-endpoints", "",
		"Comma-separated list of read replica endpoints. Each endpoint is host:port. Default port 5432. "+
			"Example: \"host1:5432, host2:5433\". (only valid for PostgreSQL)")
}

// createMigrationAssessmentStartedEvent creates a migration assessment started event
// This is common for both YBM and Yugabyted control planes
func createMigrationAssessmentStartedEvent() *cp.MigrationAssessmentStartedEvent {
	ev := &cp.MigrationAssessmentStartedEvent{}
	initBaseSourceEvent(&ev.BaseEvent, "ASSESS MIGRATION")
	return ev
}

func assessMigration() (err error) {
	assessmentMetadataDir = lo.Ternary(assessmentMetadataDirFlag != "", assessmentMetadataDirFlag,
		filepath.Join(exportDir, "assessment", "metadata"))
	// setting schemaDir to use later on - gather assessment metadata, segregating into schema files per object etc..
	schemaDir = filepath.Join(assessmentMetadataDir, "schema")

	err = handleStartCleanIfNeededForAssessMigration(assessmentMetadataDirFlag != "")
	if err != nil {
		return err
	}
	utils.PrintAndLogf("Assessing for migration to target YugabyteDB version %s\n", targetDbVersion)

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

	var validatedReplicaEndpoints []srcdb.ReplicaEndpoint
	var failedReplicaNodes []string

	if assessmentMetadataDirFlag == "" { // only in case of source connectivity
		err := source.DB().Connect()
		if err != nil {
			return fmt.Errorf("failed to connect source db for assessing migration: %w", err)
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
				return goerrors.Errorf("\n%s\n%s", color.RedString("\nMissing dependencies for assess migration:"), strings.Join(binaryCheckIssues, "\n"))
			}
		}

		res := source.DB().CheckSchemaExists()
		if !res {
			return goerrors.Errorf("failed to check if source schema exist: %q", source.Schema)
		}

		// Handle replica discovery and validation (PostgreSQL only)
		validatedReplicaEndpoints, err = migassessment.HandleReplicaDiscoveryAndValidation(&source, sourceReadReplicaEndpoints)
		if err != nil {
			return fmt.Errorf("failed to handle replica discovery and validation: %w", err)
		}

		// Check permissions on all nodes (primary + replicas) after validation
		if source.RunGuardrailsChecks {
			// Check schema usage permissions first (no-op for non-PostgreSQL databases)
			checkIfSchemasHaveUsagePermissions()
			// Check assessment-specific permissions on all nodes
			pgssEnabledForAssessment, err = migassessment.CheckAssessmentPermissionsOnAllNodes(&source, validatedReplicaEndpoints)
			if err != nil {
				return fmt.Errorf("permission check failed: %w", err)
			}
		}

		fetchSourceInfo()
	}

	startEvent := createMigrationAssessmentStartedEvent()
	controlPlane.MigrationAssessmentStarted(startEvent)

	initAssessmentDB() // Note: migassessment.AssessmentDir needs to be set beforehand

	failedReplicaNodes, err = gatherAssessmentMetadata(validatedReplicaEndpoints)
	if err != nil {
		return fmt.Errorf("failed to gather assessment metadata: %w", err)
	}

	parseExportedSchemaFileForAssessmentIfRequired()

	// Disconnect from primary DB only after all direct DB operations are complete
	// (including schema export which may check schema existence)
	if assessmentMetadataDirFlag == "" {
		source.DB().Disconnect()
	}

	err = populateMetadataCSVIntoAssessmentDB()
	if err != nil {
		return fmt.Errorf("failed to populate metadata CSV into SQLite DB: %w", err)
	}

	objectUsagesStats, err := fetchObjectUsageStats()
	if err != nil {
		return fmt.Errorf("failed to populate object usage stats: %w", err)
	}

	parserIssueDetector.PopulateObjectUsages(objectUsagesStats)

	err = validateSourceDBIOPSForAssessMigration()
	if err != nil {
		return fmt.Errorf("failed to validate source database IOPS: %w", err)
	}

	err = runAssessment()
	if err != nil {
		utils.PrintAndLogf("failed to run assessment: %v", err)
	}

	err = generateAssessmentReport(failedReplicaNodes)
	if err != nil {
		return fmt.Errorf("failed to generate assessment report: %w", err)
	}

	log.Infof("number of assessment issues detected: %d\n", len(assessmentReport.Issues))

	utils.PrintAndLog("Migration assessment completed successfully.")

	// Call the appropriate event builder based on control plane type
	var completedEvent *cp.MigrationAssessmentCompletedEvent
	controlPlaneType := os.Getenv("CONTROL_PLANE_TYPE")
	if controlPlaneType == YBAEON {
		completedEvent = createMigrationAssessmentCompletedEventForYBAeon()
	} else {
		// Default to yugabyted format (backwards compatible)
		completedEvent = createMigrationAssessmentCompletedEventForYugabyteD()
	}

	controlPlane.MigrationAssessmentCompleted(completedEvent)
	saveSourceDBConfInMSR()
	err = SetMigrationAssessmentDoneInMSR()
	if err != nil {
		return fmt.Errorf("failed to set migration assessment completed in MSR: %w", err)
	}
	return nil
}

func fetchObjectUsageStats() ([]*types.ObjectUsageStats, error) {
	// Aggregate usage stats across multiple nodes:
	// - scans: SUM across all nodes (reads happen on primary + replicas)
	// - inserts/updates/deletes: SUM from primary only (writes only on primary)
	query := fmt.Sprintf(`SELECT 
		schema_name,
		object_name,
		object_type,
		parent_table_name,
		SUM(scans) as scans,
		SUM(CASE WHEN source_node = 'primary' THEN inserts ELSE 0 END) as inserts,
		SUM(CASE WHEN source_node = 'primary' THEN updates ELSE 0 END) as updates,
		SUM(CASE WHEN source_node = 'primary' THEN deletes ELSE 0 END) as deletes
	FROM %s
	GROUP BY schema_name, object_name, object_type, parent_table_name`,
		migassessment.TABLE_INDEX_USAGE_STATS)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying-%s on assessmentDB for object usage stats: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching object usage stats %v", err)
		}
	}()

	var objectUsagesStats []*types.ObjectUsageStats
	for rows.Next() {
		var objectUsage types.ObjectUsageStats
		err = rows.Scan(&objectUsage.SchemaName, &objectUsage.ObjectName, &objectUsage.ObjectType, &objectUsage.ParentTableName, &objectUsage.Scans, &objectUsage.Inserts, &objectUsage.Updates, &objectUsage.Deletes)
		if err != nil {
			return nil, fmt.Errorf("error scanning object usage stat: %w", err)
		}
		objectUsagesStats = append(objectUsagesStats, &objectUsage)
	}
	return objectUsagesStats, nil
}

func fetchSourceInfo() {
	var err error
	source.DBVersion = source.DB().GetVersion()
	source.DBSize, err = source.DB().GetDatabaseSize()
	if err != nil {
		log.Errorf("error getting database size: %v", err) //can just log as this is used for call-home only
	}

	// Get PostgreSQL system identifier
	source.FetchDBSystemIdentifier()
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
		return false, goerrors.Errorf("metaDB is not created in export directory: %s", exportDir)
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
		record.MigrationAssessmentDone = false
		record.MigrationAssessmentDoneViaExportSchema = false
	})
	if err != nil {
		return fmt.Errorf("failed to clear migration status record with migration assessment done flag: %w", err)
	}
	return nil
}

// convertAssessmentIssueToYugabyteDAssessmentIssue converts common AssessmentIssue to Yugabyted format
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
			ObjectUsage:            issue.ObjectUsage,
			SqlStatement:           issue.SqlStatement,
			DocsLink:               issue.DocsLink,
			MinimumVersionsFixedIn: issue.MinimumVersionsFixedIn,

			Details: issue.Details,
		}
		result = append(result, ybdIssue)
	}
	return result
}

// convertAssessmentIssueToYBMAssessmentIssue converts common AssessmentIssue to YBM format
func convertAssessmentIssueToYBMAssessmentIssue(ar AssessmentReport) []AssessmentIssueYBM {
	var result []AssessmentIssueYBM
	for _, issue := range ar.Issues {
		ybmIssue := AssessmentIssueYBM{
			Category:               issue.Category,
			CategoryDescription:    issue.CategoryDescription,
			Type:                   issue.Type,
			Name:                   issue.Name,
			Description:            issue.Description,
			Impact:                 issue.Impact,
			ObjectType:             issue.ObjectType,
			ObjectName:             issue.ObjectName,
			SqlStatement:           issue.SqlStatement,
			DocsLink:               issue.DocsLink,
			MinimumVersionsFixedIn: issue.MinimumVersionsFixedIn,
			Details:                issue.Details,
		}
		result = append(result, ybmIssue)
	}
	return result
}

func runAssessment() error {
	log.Infof("running assessment for migration from '%s' to YugabyteDB", source.DBType)

	err := migassessment.SizingAssessment(targetDbVersion, source.DBType)
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

func handleStartCleanIfNeededForAssessMigration(metadataDirPassedByUser bool) error {
	assessmentDir := filepath.Join(exportDir, "assessment")
	reportsFilePattern := filepath.Join(assessmentDir, "reports", fmt.Sprintf("%s.*", ASSESSMENT_FILE_NAME))
	// Check both multi-node (node-*/*.csv) and single-node (*.csv) metadata file patterns
	multiNodeMetadataPattern := filepath.Join(assessmentMetadataDir, "node-*", "*.csv")
	singleNodeMetadataPattern := filepath.Join(assessmentMetadataDir, "*.csv")
	schemaFilesPattern := filepath.Join(assessmentMetadataDir, "schema", "*", "*.sql")
	dbsFilePattern := filepath.Join(assessmentDir, "dbs", "*.db")

	assessmentFilesExists := utils.FileOrFolderExistsWithGlobPattern(reportsFilePattern) || utils.FileOrFolderExistsWithGlobPattern(dbsFilePattern)
	if !metadataDirPassedByUser {
		assessmentFilesExists = assessmentFilesExists ||
			utils.FileOrFolderExistsWithGlobPattern(multiNodeMetadataPattern) ||
			utils.FileOrFolderExistsWithGlobPattern(singleNodeMetadataPattern) ||
			utils.FileOrFolderExistsWithGlobPattern(schemaFilesPattern)
	}

	isAssessmentDone, err := IsMigrationAssessmentDoneDirectly(metaDB)
	if err != nil {
		return fmt.Errorf("failed to check if migration assessment is done: %w", err)
	}

	needCleanupOfLeftoverFiles := assessmentFilesExists && !isAssessmentDone
	if bool(startClean) || needCleanupOfLeftoverFiles {
		utils.CleanDir(filepath.Join(assessmentDir, "metadata"))
		utils.CleanDir(filepath.Join(assessmentDir, "reports"))
		utils.CleanDir(filepath.Join(assessmentDir, "dbs"))
		err := ClearMigrationAssessmentDone()
		if err != nil {
			return fmt.Errorf("failed to start clean for assess migration: %w", err)
		}
	} else if assessmentFilesExists { // if not startClean but assessment files already exist
		return goerrors.Errorf("assessment metadata or reports files already exist in the assessment directory: '%s'. Use the --start-clean flag to clear the directory before proceeding.", assessmentDir)
	}

	return nil
}

// gatherAssessmentMetadata collects metadata from the source database.
// For PostgreSQL, it accepts validated replicas and returns the list of failed replica nodes
// (for reporting partial multi-node assessments).
func gatherAssessmentMetadata(validatedReplicas []srcdb.ReplicaEndpoint) (failedReplicaNodes []string, err error) {
	if assessmentMetadataDirFlag != "" {
		return nil, nil // assessment metadata files are provided by the user inside assessmentMetadataDir
	}

	// setting schema objects types to export before creating the project directories
	source.ExportObjectTypeList = utils.GetExportSchemaObjectList(source.DBType)
	CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	utils.PrintAndLogf("\ngathering metadata and stats from '%s' source database...\n", source.DBType)
	switch source.DBType {
	case POSTGRESQL:
		failedReplicaNodes, err = gatherAssessmentMetadataFromPG(validatedReplicas)
		if err != nil {
			return nil, fmt.Errorf("error gathering metadata and stats from source PG database: %w", err)
		}
		return failedReplicaNodes, nil
	case ORACLE:
		err = gatherAssessmentMetadataFromOracle()
		if err != nil {
			return nil, fmt.Errorf("error gathering metadata and stats from source Oracle database: %w", err)
		}
	default:
		return nil, goerrors.Errorf("source DB Type %s is not yet supported for metadata and stats gathering", source.DBType)
	}
	utils.PrintAndLogf("gathered assessment metadata files at '%s'", assessmentMetadataDir)
	return nil, nil
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
		return fmt.Errorf("error getting tnsAdmin: %w", err)
	}
	envVars := []string{fmt.Sprintf("ORACLE_PASSWORD=%s", source.Password),
		fmt.Sprintf("TNS_ADMIN=%s", tnsAdmin),
		fmt.Sprintf("ORACLE_HOME=%s", source.GetOracleHome()),
	}
	log.Infof("environment variables passed to oracle gather metadata script: %v", envVars)
	return runGatherAssessmentMetadataScript(scriptPath, envVars,
		source.DB().GetConnectionUriWithoutPassword(), strings.ToUpper(source.Schema), assessmentMetadataDir)
}

// CollectionResult tracks success/failure of metadata collection from a single node
type CollectionResult struct {
	NodeName  string
	IsPrimary bool
	Success   bool
	Error     error
}

// progressTracker manages the display of progress for parallel metadata collection
type progressTracker struct {
	nodes        []string          // Ordered list of node names for consistent display
	displayNames map[string]string // nodeName -> displayName
	statuses     map[string]*NodeProgress
	mutex        sync.Mutex
	initialized  bool // Whether initial lines have been printed
	maxNameLen   int  // Maximum length of display names for alignment
}

func newProgressTracker(nodes []collectionNode) *progressTracker {
	tracker := &progressTracker{
		nodes:        make([]string, 0, len(nodes)),
		displayNames: make(map[string]string),
		statuses:     make(map[string]*NodeProgress),
		initialized:  false,
		maxNameLen:   0,
	}

	// Calculate maximum display name length for alignment
	for _, node := range nodes {
		tracker.nodes = append(tracker.nodes, node.nodeName)
		tracker.displayNames[node.nodeName] = node.displayName
		if len(node.displayName) > tracker.maxNameLen {
			tracker.maxNameLen = len(node.displayName)
		}
		// Initialize with pending stage
		tracker.statuses[node.nodeName] = &NodeProgress{
			NodeName:    node.nodeName,
			DisplayName: node.displayName,
			Stage:       "Pending...",
		}
	}

	return tracker
}

func (pt *progressTracker) update(progress NodeProgress) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	// Update status
	pt.statuses[progress.NodeName] = &progress

	// Print/update display
	pt.printAll()
}

func (pt *progressTracker) printAll() {
	if !pt.initialized {
		// First time: print all lines
		fmt.Println() // Blank line
		for _, nodeName := range pt.nodes {
			status := pt.statuses[nodeName]
			pt.printSingleLine(*status)
		}
		pt.initialized = true
	} else {
		// Move cursor up to the first line and reprint all
		// Move up by number of nodes
		fmt.Printf("\033[%dA", len(pt.nodes))

		// Reprint all lines
		for _, nodeName := range pt.nodes {
			status := pt.statuses[nodeName]
			pt.printSingleLine(*status)
		}
	}
}

func (pt *progressTracker) printSingleLine(progress NodeProgress) {
	// Determine icon based on stage
	statusIcon := "⏳"
	if progress.Stage == "Complete" {
		statusIcon = "✓"
	} else if progress.Stage == "Failed" {
		statusIcon = "⚠"
	}

	// Use full display name with dynamic width based on longest name
	displayName := progress.DisplayName

	// Clear line and print with fixed-width column for name
	// \r returns to start, \033[K clears to end of line
	// Using %-*s for left-alignment with dynamic width
	fmt.Printf("\r\033[K  %s %-*s %s\n", statusIcon, pt.maxNameLen+1, displayName+":", progress.Stage)
}

// collectionNode represents a database node (primary or replica) that metadata will be collected from.
// It contains all the information needed to run the collection script for that node.
type collectionNode struct {
	nodeName      string // Filesystem-safe unique identifier (used for subdirectory names)
	displayName   string // User-friendly name (shown in progress UI)
	connectionUri string // Database connection string
	isPrimary     bool   // Whether this is the primary node (affects script behavior)
}

// gatherAssessmentMetadataFromPG collects metadata from PostgreSQL primary and replicas.
// Accepts the validated replicas to collect from, and returns the list of failed replica names
// (for reporting partial multi-node assessment).
// Collection is performed in parallel for better performance.
func gatherAssessmentMetadataFromPG(validatedReplicas []srcdb.ReplicaEndpoint) (failedReplicaNodes []string, err error) {
	if assessmentMetadataDirFlag != "" {
		return nil, nil
	}

	scriptPath, err := findGatherMetadataScriptPath(POSTGRESQL)
	if err != nil {
		return nil, err
	}

	// Build list of all nodes to collect from (primary + replicas)
	nodes := []collectionNode{
		{
			nodeName:      "primary",
			displayName:   "Primary",
			connectionUri: source.DB().GetConnectionUriWithoutPassword(),
			isPrimary:     true,
		},
	}

	for _, replica := range validatedReplicas {
		uniqueNodeName := fmt.Sprintf("%s-%d", replica.Host, replica.Port)
		nodes = append(nodes, collectionNode{
			nodeName:      uniqueNodeName,
			displayName:   replica.Name,
			connectionUri: replica.ConnectionUri,
			isPrimary:     false,
		})
	}

	totalNodes := len(nodes)
	if totalNodes == 1 {
		utils.PrintAndLogfInfo("\nCollecting metadata from 1 node...")
	} else {
		utils.PrintAndLogfInfo("\nCollecting metadata from %d nodes in parallel...", totalNodes)
	}

	// Initialize progress tracker
	tracker := newProgressTracker(nodes)

	// Channel for progress updates
	progressChan := make(chan NodeProgress, totalNodes*10) // Buffer for multiple updates per node

	// Channel to signal completion of progress display goroutine
	displayDone := make(chan struct{})

	// Start progress display goroutine
	go func() {
		defer close(displayDone)
		for progress := range progressChan {
			tracker.update(progress)
		}
	}()

	// WaitGroup for parallel collection
	var wg sync.WaitGroup

	// Channel for collection results
	type collectionResult struct {
		displayName string // For logging which node succeeded/failed
		isPrimary   bool   // Determines if failure is critical (primary) or warning (replica)
		err         error  // nil = success, non-nil = failure
	}
	resultChan := make(chan collectionResult, totalNodes)

	// Start parallel collection for all nodes
	for _, node := range nodes {
		wg.Add(1)
		go func(n collectionNode) {
			defer wg.Done()

			// Run collection with buffered output
			err := runGatherAssessmentMetadataScriptBuffered(
				scriptPath,
				[]string{fmt.Sprintf("PGPASSWORD=%s", source.Password)},
				n.nodeName,
				n.displayName,
				n.isPrimary,
				progressChan,
				n.connectionUri,
				source.Schema,
				assessmentMetadataDir,
				fmt.Sprintf("%t", pgssEnabledForAssessment),
				fmt.Sprintf("%d", intervalForCapturingIOPS),
				"true",     // --yes (doesn't matter since skip_checks=true)
				n.nodeName, // source_node_name
				"true",     // skip_checks - guardrails already validated
			)

			// Send result
			resultChan <- collectionResult{
				displayName: n.displayName,
				isPrimary:   n.isPrimary,
				err:         err,
			}
		}(node)
	}

	// Goroutine to close channels after all collections complete
	go func() {
		wg.Wait()
		close(progressChan)
		close(resultChan)
	}()

	// Wait for display goroutine to finish (it closes when progressChan is closed)
	<-displayDone

	// Process results
	var failedReplicasList []string
	var successfulReplicaCount int
	var primaryFailed bool
	var primaryErr error

	for result := range resultChan {
		if result.err == nil {
			log.Infof("Successfully collected metadata from %s", result.displayName)
			if !result.isPrimary {
				successfulReplicaCount++
			}
		} else {
			if result.isPrimary {
				primaryFailed = true
				primaryErr = result.err
			} else {
				log.Warnf("Failed to collect metadata from replica %s: %v", result.displayName, result.err)
				failedReplicasList = append(failedReplicasList, result.displayName)
			}
		}
	}

	// If primary failed, return error immediately
	if primaryFailed {
		return nil, fmt.Errorf("metadata collection failed on primary database (critical): %w", primaryErr)
	}

	// Print summary
	if !primaryFailed && len(failedReplicasList) > 0 {
		fmt.Println() // Blank line before warnings
		color.Yellow("WARNING: Metadata collection failed on %d replica(s): %v", len(failedReplicasList), failedReplicasList)
		utils.PrintAndLogfInfo("Continuing assessment with data from primary + %d successful replica(s)", successfulReplicaCount)
		utils.PrintAndLogfWarning("Note: Sizing and metrics will reflect only the nodes that succeeded.")
	}

	fmt.Println() // Single blank line before final success message
	if len(validatedReplicas) == 0 {
		utils.PrintAndLogfSuccess("Successfully completed metadata collection from primary node")
	} else {
		utils.PrintAndLogfSuccess("Successfully completed metadata collection from %d node(s) (primary + %d replica(s))",
			1+successfulReplicaCount, successfulReplicaCount)
	}

	return failedReplicasList, nil
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

	return "", goerrors.Errorf("script not found in possible paths: %v", possiblePathsForScript)
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

// NodeProgress tracks the progress of metadata collection for a single node
type NodeProgress struct {
	NodeName    string // Unique identifier for tracking (e.g., "primary", "host-5432")
	DisplayName string // User-friendly name for display (e.g., "Primary", "replica.aws.com:5432")
	Stage       string // Current stage (e.g., "Collecting table row counts...", "Complete", "Failed")
}

// runGatherAssessmentMetadataScriptBuffered runs the metadata collection script with buffered output
// and sends progress updates to the provided channel. This version is safe for parallel execution.
func runGatherAssessmentMetadataScriptBuffered(
	scriptPath string,
	envVars []string,
	nodeName string,
	displayName string,
	isPrimary bool,
	progressChan chan<- NodeProgress,
	scriptArgs ...string,
) error {
	cmd := exec.Command(scriptPath, scriptArgs...)
	log.Infof("[%s] running script: %s", nodeName, cmd.String())
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, envVars...)
	cmd.Dir = assessmentMetadataDir
	// Don't set stdin for parallel execution to avoid conflicts.
	// Not needed anyway since we pass skip_checks=true (no prompts).

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

	// Report starting status
	if progressChan != nil {
		progressChan <- NodeProgress{
			NodeName:    nodeName,
			DisplayName: displayName,
			Stage:       "Starting collection...",
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to read stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Errorf("[%s][stderr]: %s", nodeName, line)
		}
	}()

	// Goroutine to read stdout and detect stages
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			log.Infof("[%s][stdout]: %s", nodeName, line)

			// Detect stage changes from script output
			if progressChan != nil {
				stage := detectStageFromOutput(line, isPrimary)
				if stage != "" {
					progressChan <- NodeProgress{
						NodeName:    nodeName,
						DisplayName: displayName,
						Stage:       stage,
					}
				}
			}
		}
	}()

	// Wait for output goroutines to finish
	wg.Wait()

	// Wait for command to complete
	err = cmd.Wait()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 2 {
					log.Infof("[%s] Exit without error as user opted not to continue in the script.", nodeName)
					// For parallel execution, we don't exit the entire process
					return fmt.Errorf("user opted not to continue")
				}
			}
		}
		log.Errorf("[%s] Script failed with error: %v", nodeName, err)
		if progressChan != nil {
			progressChan <- NodeProgress{
				NodeName:    nodeName,
				DisplayName: displayName,
				Stage:       "Failed",
			}
		}
		return fmt.Errorf("error waiting for gather assessment metadata script to complete: %w", err)
	}

	// Report completion
	if progressChan != nil {
		progressChan <- NodeProgress{
			NodeName:    nodeName,
			DisplayName: displayName,
			Stage:       "Complete",
		}
	}

	return nil
}

// detectStageFromOutput parses script output to detect the current stage
// It matches the actual messages printed by print_and_log() in the shell script
func detectStageFromOutput(line string, isPrimary bool) string {
	originalLine := strings.TrimSpace(line)
	lineLower := strings.ToLower(originalLine)

	// Special case: Start of collection
	if strings.Contains(lineLower, "assessment metadata collection started") {
		return "Starting collection..."
	}

	// Special case: Completion
	if strings.Contains(lineLower, "assessment metadata collection completed") {
		return "Complete"
	}

	// Pass through any line that looks like a stage message
	// (contains keywords that indicate this is a meaningful status update)
	isStageMessage := strings.Contains(lineLower, "collecting") ||
		strings.Contains(lineLower, "skipping") ||
		strings.Contains(lineLower, "executing")

	if isStageMessage {
		// Return the original line (preserves capitalization)
		// Add "..." if not already present
		if !strings.HasSuffix(originalLine, "...") && !strings.HasSuffix(originalLine, ".") {
			return originalLine + "..."
		}
		return originalLine
	}

	return ""
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
	// Collect CSV files from metadata directory
	// Two supported structures:
	//   1. Multi-node (PostgreSQL): assessmentMetadataDir/node-*/*.csv
	//   2. Single-node (Oracle, MySQL, etc.): assessmentMetadataDir/*.csv
	var metadataFilesPath []string

	// Check for multi-node structure first (node-* directories)
	nodeDirs, err := filepath.Glob(filepath.Join(assessmentMetadataDir, "node-*"))
	if err != nil {
		return fmt.Errorf("error looking for node data directories in %s: %w", assessmentMetadataDir, err)
	}

	if len(nodeDirs) > 0 {
		// Multi-node structure: Collect CSV files from each node directory
		for _, nodeDir := range nodeDirs {
			nodeFiles, err := filepath.Glob(filepath.Join(nodeDir, "*.csv"))
			if err != nil {
				return fmt.Errorf("error looking for csv files in directory %s: %w", nodeDir, err)
			}
			metadataFilesPath = append(metadataFilesPath, nodeFiles...)
		}
		log.Infof("Found %d CSV files across %d node(s) for population into assessment DB", len(metadataFilesPath), len(nodeDirs))
	} else {
		// Single-node structure: Collect CSV files directly from metadata directory
		metadataFilesPath, err = filepath.Glob(filepath.Join(assessmentMetadataDir, "*.csv"))
		if err != nil {
			return fmt.Errorf("error looking for csv files in directory %s: %w", assessmentMetadataDir, err)
		}
		log.Infof("Found %d CSV files in metadata directory for population into assessment DB", len(metadataFilesPath))
	}

	for _, metadataFilePath := range metadataFilesPath {
		baseFileName := filepath.Base(metadataFilePath)
		metric := strings.TrimSuffix(baseFileName, filepath.Ext(baseFileName))
		tableName := strings.Replace(metric, "-", "_", -1)
		// collecting both initial and final measurement in the same table
		tableName = lo.Ternary(strings.Contains(tableName, migassessment.TABLE_INDEX_IOPS),
			migassessment.TABLE_INDEX_IOPS, tableName)

		// check if the table exist in the assessment db or not
		// possible scenario: if gather scripts are run manually, not via voyager
		err := assessmentDB.CheckIfTableExists(tableName)
		if err != nil {
			return fmt.Errorf("error checking if table %s exists: %w", tableName, err)
		}

		log.Infof("populating metadata from file %s into table %s", metadataFilePath, tableName)
		err = assessmentDB.LoadCSVFileIntoTable(metadataFilePath, tableName)
		if err != nil {
			return fmt.Errorf("error loading CSV file %s: %w", metadataFilePath, err)
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

func generateAssessmentReport(failedReplicaNodes []string) (err error) {
	utils.PrintAndLogf("Generating assessment report...")

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

	addNotesToAssessmentReport(failedReplicaNodes)
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

func fetchRedundantIndexInfoFromAssessmentDB() ([]utils.RedundantIndexesInfo, error) {
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

	resolvedRedundantIndexes := getResolvedRedundantIndexes(redundantIndexesInfo)

	return resolvedRedundantIndexes, nil
}

func getResolvedRedundantIndexes(redundantIndexes []utils.RedundantIndexesInfo) []utils.RedundantIndexesInfo {

	redundantIndexToInfo := make(map[string]utils.RedundantIndexesInfo)

	//This function helps in resolving the existing index in cases where existing index is also a redundant index on some other index
	//So in such cases we need to report the main existing index.
	/*
		e.g. INDEX idx1 on t(id); INDEX idx2 on t(id, id1); INDEX idx3 on t(id, id1,id2);
		redundant index coming from the script can have
		Redundant - idx1, Existing idx2
		Redundant - idx2, Existing idx3
		So in this case we need to report it like
		Redundant - idx1, Existing idx3
		Redundant - idx2, Existing idx3
	*/
	getRootRedundantIndexInfo := func(currRedundantIndexInfo utils.RedundantIndexesInfo) utils.RedundantIndexesInfo {
		for {
			existingIndexOfCurrRedundant := currRedundantIndexInfo.GetExistingIndexObjectName()
			nextRedundantIndexInfo, ok := redundantIndexToInfo[existingIndexOfCurrRedundant]
			if !ok {
				return currRedundantIndexInfo
			}
			currRedundantIndexInfo = nextRedundantIndexInfo
		}
	}
	for _, redundantIndex := range redundantIndexes {
		redundantIndexToInfo[redundantIndex.GetRedundantIndexObjectName()] = redundantIndex
	}
	for _, redundantIndex := range redundantIndexes {
		rootIndexInfo := getRootRedundantIndexInfo(redundantIndex)
		rootExistingIndex := rootIndexInfo.GetExistingIndexObjectName()
		currentExistingIndex := redundantIndex.GetExistingIndexObjectName()
		if rootExistingIndex != currentExistingIndex {
			//If existing index was redundant index then after figuring out the actual existing index use that to report existing index
			redundantIndex.ExistingIndexName = rootIndexInfo.ExistingIndexName
			redundantIndex.ExistingSchemaName = rootIndexInfo.ExistingSchemaName
			redundantIndex.ExistingTableName = rootIndexInfo.ExistingTableName
			redundantIndex.ExistingIndexDDL = rootIndexInfo.ExistingIndexDDL
			redundantIndexToInfo[redundantIndex.GetRedundantIndexObjectName()] = redundantIndex
		}
	}
	var redundantIndexesRes []utils.RedundantIndexesInfo
	for _, redundantIndexInfo := range redundantIndexToInfo {
		redundantIndexesRes = append(redundantIndexesRes, redundantIndexInfo)
	}
	return redundantIndexesRes
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
		return fmt.Errorf("error fetching column stats from assessement db: %w", err)
	}
	//passing it on to the parser issue detector to enable it for detecting issues using this.
	parserIssueDetector.SetColumnStatistics(columnStats)
	return nil
}

func getAssessmentReportContentFromAnalyzeSchema() error {

	var err error
	//fetching column stats from assessment db and then passing it on to the parser issue detector for detecting issues
	err = fetchAndSetColumnStatisticsForIndexIssues()
	if err != nil {
		return fmt.Errorf("error parsing column statistics information: %w", err)
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
		ObjectUsage:            analyzeSchemaIssue.ObjectUsage,
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
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.HOTSPOTS_ON_DATE_PK_UK_ISSUE, "", queryissue.HOTSPOTS_ON_DATE_PK_UK, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.HOTSPOTS_ON_TIMESTAMP_PK_UK_ISSUE, "", queryissue.HOTSPOTS_ON_TIMESTAMP_PK_UK, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.FOREIGN_KEY_DATATYPE_MISMATCH_ISSUE_NAME, "", queryissue.FOREIGN_KEY_DATATYPE_MISMATCH, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.MISSING_FOREIGN_KEY_INDEX_ISSUE_NAME, "", queryissue.MISSING_FOREIGN_KEY_INDEX, schemaAnalysisReport, false))
	unsupportedFeatures = append(unsupportedFeatures, getUnsupportedFeaturesFromSchemaAnalysisReport(queryissue.MISSING_PRIMARY_KEY_WHEN_UNIQUE_NOT_NULL_ISSUE_NAME, "", queryissue.MISSING_PRIMARY_KEY_WHEN_UNIQUE_NOT_NULL, schemaAnalysisReport, false))

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
		baseTypeName := allColumnsDataTypes[i].GetBaseTypeNameFromDatatype() // baseType of the db e.g. for xml[] -> xml / public.geometry -> geomtetry

		isUnsupportedDatatype := utils.ContainsAnyStringFromSlice(sourceUnsupportedDatatypes, baseTypeName)
		isUnsupportedDatatypeInLive := utils.ContainsAnyStringFromSlice(liveUnsupportedDatatypes, baseTypeName)

		isUnsupportedDatatypeInLiveWithFFOrFBList := utils.ContainsAnyStringFromSlice(liveWithFForFBUnsupportedDatatypes, baseTypeName)
		isUDTDatatype := utils.ContainsAnyStringFromSlice(parserIssueDetector.GetCompositeTypes(), allColumnsDataTypes[i].DataType)
		isArrayDatatype := strings.HasSuffix(allColumnsDataTypes[i].DataType, "[]")                                                                       //if type is array
		isEnumDatatype := utils.ContainsAnyStringFromSlice(parserIssueDetector.GetEnumTypes(), strings.TrimSuffix(allColumnsDataTypes[i].DataType, "[]")) //is ENUM type

		allColumnsDataTypes[i].IsArrayType = isArrayDatatype
		allColumnsDataTypes[i].IsEnumType = isEnumDatatype
		allColumnsDataTypes[i].IsUDTType = isUDTDatatype

		// Array of enums are now supported with logical connector (default), so not including them as unsupported
		isUnsupportedDatatypeInLiveWithFFOrFB := isUnsupportedDatatypeInLiveWithFFOrFBList || isUDTDatatype

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
			//reporting types in the list YugabyteUnsupportedDataTypesForDbzm and UDT columns as unsupported with live migration with ff/fb
			//Note: hstore, tsvector, and array of enums are now supported with logical connector (default)
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
			// Handle CLOB datatype issue se
			if strings.EqualFold(colInfo.DataType, "CLOB") {
				issue.Description = "Oracle CLOB data export is now supported via the experimental flag --allow-oracle-clob-data-export. This is supported only for offline migration (not Live or BETA_FAST_DATA_EXPORT) and large CLOBs may impact performance during export and import."
				issue.DocsLink = "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/oracle/#large-sized-clob-data-is-not-supported"
			}
			assessmentReport.AppendIssues(issue)
		case POSTGRESQL:
			// Datatypes can be of form public.geometry, so we need to extract the datatype from it
			// for the array types we add the '[]' to the type for distinguish between normal type and array based datatype
			baseTypeName := colInfo.GetBaseTypeNameFromDatatype() // baseType of the db e.g. for xml[] -> xml / public.geometry -> geometry

			// We obtain the queryissue from the Report function. This queryissue is first converted to AnalyzeIssue and then to AssessmentIssue using pre existing function
			// Coneverting queryissue directly to AssessmentIssue would have lead to the creation of a new function which would have required a lot of cases to be handled and led to code duplication
			// This converted AssessmentIssue is then appended to the assessmentIssues slice
			queryissue := queryissue.ReportUnsupportedDatatypes(baseTypeName, colInfo.ColumnName, constants.COLUMN, qualifiedColName)
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

var (
	// GeneralNotes
	PREVIEW_FEATURES_NOTE = NoteInfo{
		Type: GeneralNotes,
		Text: `Some features listed in this report may be supported under a preview flag in the specified target-db-version of YugabyteDB. Please refer to the official <a class="highlight-link" target="_blank" href="https://docs.yugabyte.com/preview/releases/ybdb-releases/">release notes</a> for detailed information and usage guidelines.`,
	}
	GIN_INDEXES = NoteInfo{
		Type: GeneralNotes,
		Text: `There are some BITMAP indexes present in the schema that will get converted to GIN indexes, but GIN indexes are partially supported in YugabyteDB as mentioned in <a class="highlight-link" href="https://github.com/yugabyte/yugabyte-db/issues/7850">https://github.com/yugabyte/yugabyte-db/issues/7850</a> so take a look and modify them if not supported.`,
	}
	UNLOGGED_TABLE_NOTE = NoteInfo{
		Type: GeneralNotes,
		Text: `There are some Unlogged tables in the schema. They will be created as regular LOGGED tables in YugabyteDB as unlogged tables are not supported.`,
	}
	REPORTING_LIMITATIONS_NOTE = NoteInfo{
		Type: GeneralNotes,
		Text: `<a class="highlight-link" target="_blank"  href="https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/#assessment-and-schema-analysis-limitations">Limitations in assessment</a>`,
	}
	FOREIGN_TABLE_NOTE = NoteInfo{
		Type: GeneralNotes,
		Text: `There are some Foreign tables in the schema, but during the export schema phase, exported schema does not include the SERVER and USER MAPPING objects. Therefore, you must manually create these objects before import schema. For more information on each of them, run analyze-schema. `,
	}
	PARTIAL_MULTI_NODE_ASSESSMENT = NoteInfo{
		Type: GeneralNotes,
		Text: `This assessment includes partial multi-node data. Some replicas failed during metadata collection and are excluded from all sections of this report.`,
	}

	// ColocatedShardedNotes
	COLOCATED_TABLE_RECOMMENDATION_CAVEAT = NoteInfo{
		Type: ColocatedShardedNotes,
		Text: `If there are any tables that receive disproportionately high load, ensure that they are NOT colocated to avoid the colocated tablet becoming a hotspot.
For additional considerations related to colocated tables, refer to the documentation at: <a class="highlight-link" target="_blank" href="https://docs.yugabyte.com/preview/explore/colocation/#limitations-and-considerations">https://docs.yugabyte.com/preview/explore/colocation/#limitations-and-considerations</a>`,
	}
	ORACLE_PARTITION_DEFAULT_COLOCATION = NoteInfo{
		Type: ColocatedShardedNotes,
		Text: `For sharding/colocation recommendations, each partition is treated individually. During the export schema phase, all the partitions of a partitioned table are currently created as colocated by default.
To manually modify the schema, please refer: <a class="highlight-link" href="https://github.com/yugabyte/yb-voyager/issues/1581">https://github.com/yugabyte/yb-voyager/issues/1581</a>.`,
	}

	// SizingNotes
	ORACLE_UNSUPPPORTED_PARTITIONING = NoteInfo{
		Type: SizingNotes,
		Text: `Reference and System Partitioned tables are created as normal tables, but are not considered for target cluster sizing recommendations.`,
	}

	REDUNDANT_INDEX_ESTIMATED_TIME = NoteInfo{
		Type: SizingNotes,
		Text: `Import data time estimates exclude redundant indexes since they are automatically removed during export schema phase.`,
	}
)

func addNotesToAssessmentReport(failedReplicaNodes []string) {
	log.Infof("adding notes to assessment report")

	assessmentReport.Notes = append(assessmentReport.Notes, PREVIEW_FEATURES_NOTE)

	// Add note if some replicas failed during collection
	if len(failedReplicaNodes) > 0 {
		assessmentReport.Notes = append(assessmentReport.Notes, PARTIAL_MULTI_NODE_ASSESSMENT)
	}

	// keep it as the first point in Notes
	if len(assessmentReport.Sizing.SizingRecommendation.ColocatedTables) > 0 {
		assessmentReport.Notes = append(assessmentReport.Notes, COLOCATED_TABLE_RECOMMENDATION_CAVEAT)
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
		assessmentReport.Notes = append(assessmentReport.Notes, REDUNDANT_INDEX_ESTIMATED_TIME)
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

				baseTypeName := colInfo.GetBaseTypeNameFromDatatype() // baseType of the db e.g. for xml[] -> xml / public.geometry -> geometry

				// We obtain the queryissue from the Report function. This queryissue is first converted to AnalyzeIssue and then to AssessmentIssue using pre existing function
				// Coneverting queryissue directly to AssessmentIssue would have lead to the creation of a new function which would have required a lot of cases to be handled and led to code duplication
				// This converted AssessmentIssue is then appended to the assessmentIssues slice
				queryIssue := queryissue.ReportUnsupportedDatatypesInLive(baseTypeName, colInfo.ColumnName, constants.COLUMN, qualifiedColName)
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

				baseTypeName := colInfo.GetBaseTypeNameFromDatatype() // baseType of the db e.g. for xml[] -> xml / public.geometry -> geometry

				var queryIssue queryissue.QueryIssue

				if colInfo.IsUDTType {
					queryIssue = queryissue.NewUserDefinedDatatypeIssue(
						constants.COLUMN,
						qualifiedColName,
						"",
						baseTypeName,
						colInfo.ColumnName,
					)
				} else {
					queryIssue = queryissue.ReportUnsupportedDatatypesInLiveWithFFOrFB(baseTypeName, colInfo.ColumnName, constants.COLUMN, qualifiedColName)
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

	utils.PrintAndLogf("generated JSON assessment report at: %s", jsonReportFilePath)
	return nil
}

/*
	   Template: issuesTable

		Description:
		------------
		This Go template partial renders a dynamic table for displaying assessment issues or performance optimizations in a migration assessment report. The table adapts its headings, columns, and button controls based on the context (general issues vs. performance optimizations), as determined by the `.onlyPerf` flag.

		Features:
		---------
		- Dynamically sets headings, keywords, and button IDs based on the type of issues being displayed.
		- Provides "Expand All" and "Collapse All" buttons for toggling the visibility of detailed issue information.
		- Supports sorting by category, name, and impact via clickable table headers.
		- For each issue/optimization:
			- Displays a summary row with key information (category, name, object/SQL preview, impact).
			- Allows expanding to show detailed information, including category description, object type/name, SQL statement, supported versions, description, documentation link, and additional details.
		- Handles cases where no issues are found, displaying an appropriate message.
		- Utilizes helper functions such as `filterOutPerformanceOptimizationIssues`, `getPerformanceOptimizationIssues`, `snakeCaseToTitleCase`, `camelCaseToTitleCase`, and `getSupportedVersionString` for data formatting and filtering.

		Usage:
		------
		- Include this template in a parent template using `{{ template "issuesTable" . }}`.
		- Expects the following data structure in the context:
			- .Issues: List of issue objects with fields like Category, Name, Impact, ObjectType, ObjectName, SqlStatement, Description, DocsLink, Details, MinimumVersionsFixedIn, CategoryDescription.
			- .onlyPerf: Boolean flag indicating whether to show performance optimizations or general issues.

			Differences Between the Two Tables Rendered by issuesTable
			----------------------------------------------------------

			The `issuesTable` template is used twice in the report: once for general assessment issues and once for performance optimizations. The differences between the two tables are as follows:

			1. Heading and Labels:
			- The heading is "Assessment Issues" for general issues and "Performance Optimizations" for performance-related issues.
			- The count label is "Total issues" for general issues and "Total optimizations" for performance optimizations.
			- The keyword in the table header is "Issue" or "Optimization" accordingly.

			2. Data Source:
			- For general issues, the table uses `filterOutPerformanceOptimizationIssues .Issues` to exclude performance optimizations.
			- For performance optimizations, the table uses `getPerformanceOptimizationIssues .Issues` to include only those.

			3. Table Columns:
			- The general issues table includes a "Category" column (with an expand/collapse arrow).
			- The performance optimizations table omits the "Category" column and places the expand/collapse arrow in the "Optimization" column.

			4. Button IDs:
			- The "Expand All" and "Collapse All" buttons have different IDs for each table to allow independent control.

			5. Details Display:
			- The details rows for general issues may include a "Category Description" field, which is omitted for performance optimizations.

			6. Empty State:
			- If there are no general issues, a message "No issues were found in the assessment." is shown.
			- If there are no performance optimizations, no message is shown (the table is simply omitted).

			7. Sorting:
			- Sorting by "Category" is only available in the Assessment issues table only i.e. not onlyPerf case.
			- Sorting by "Issue" / "Optimization" is available in both the tables
			- Sorting by "Impact" is available in both tables.

			These differences are controlled by the `.onlyPerf` flag passed to the template and are reflected in both the Go template logic and the rendered HTML structure.
*/
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
		"split":                                  split,
		"groupByObjectType":                      groupByObjectType,
		"numKeysInMapStringObjectInfo":           numKeysInMapStringObjectInfo,
		"groupByObjectName":                      groupByObjectName,
		"totalUniqueObjectNamesOfAllTypes":       totalUniqueObjectNamesOfAllTypes,
		"getSupportedVersionString":              getSupportedVersionString,
		"snakeCaseToTitleCase":                   utils.SnakeCaseToTitleCase,
		"camelCaseToTitleCase":                   utils.CamelCaseToTitleCase,
		"getSqlPreview":                          utils.GetSqlStmtToPrint,
		"filterOutPerformanceOptimizationIssues": filterOutPerformanceOptimizationIssues,
		"getPerformanceOptimizationIssues":       getPerformanceOptimizationIssues,
		"dict":                                   dict,
		"hasNotesByType":                         hasNotesByType,
		"filterNotesByType":                      filterNotesByType,
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

	utils.PrintAndLogf("generated HTML assessment report at: %s", htmlReportFilePath)
	return nil
}

func dict(values ...interface{}) map[string]interface{} {
	if len(values)%2 != 0 {
		panic("invalid dict call: uneven key-value pairs")
	}
	m := make(map[string]interface{}, len(values)/2)
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			panic("dict keys must be strings")
		}
		m[key] = values[i+1]
	}
	return m
}

func filterOutPerformanceOptimizationIssues(issues []AssessmentIssue) []AssessmentIssue {
	withoutPerfOptimzationIssues := lo.Filter(issues, func(issue AssessmentIssue, _ int) bool {
		return issue.Category != PERFORMANCE_OPTIMIZATIONS_CATEGORY
	})
	return withoutPerfOptimzationIssues
}

func getPerformanceOptimizationIssues(issues []AssessmentIssue) []AssessmentIssue {
	perfOptimzationIssues := lo.Filter(issues, func(issue AssessmentIssue, _ int) bool {
		return issue.Category == PERFORMANCE_OPTIMIZATIONS_CATEGORY
	})
	sort.Slice(perfOptimzationIssues, func(i, j int) bool {
		ordStates := map[string]int{"FREQUENT": 1, "MODERATE": 2, "RARE": 3, "UNUSED": 4}
		p1 := perfOptimzationIssues[i]
		p2 := perfOptimzationIssues[j]
		return ordStates[p1.ObjectUsage] < ordStates[p2.ObjectUsage]
	})
	return perfOptimzationIssues
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

// hasNotesByType checks if there are any notes of the specified type
func hasNotesByType(notes []NoteInfo, noteType NoteType) bool {
	for _, note := range notes {
		if note.Type == noteType {
			return true
		}
	}
	return false
}

// filterNotesByType returns only notes of the specified type
func filterNotesByType(notes []NoteInfo, noteType NoteType) []NoteInfo {
	var filtered []NoteInfo
	for _, note := range notes {
		if note.Type == noteType {
			filtered = append(filtered, note)
		}
	}
	return filtered
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

func validateReplicaEndpointsFlag() {
	if sourceReadReplicaEndpoints != "" && source.DBType != POSTGRESQL {
		utils.ErrExit("Error --source-read-replica-endpoints flag / source.read-replica-endpoints config parameter is only valid for 'postgresql' db type")
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
	utils.PrintAndLogf("%v", err)
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
