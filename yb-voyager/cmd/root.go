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
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"
	"time"

	"github.com/fatih/color"
	goerrors "github.com/go-errors/errors"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/tebeka/atexit"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/config"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp/noopcp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp/ybaeon"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp/yugabyted"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/lockfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	cfgFile                            string
	exportDir                          string
	schemaDir                          string
	startClean                         utils.BoolStr
	truncateTables                     utils.BoolStr
	lockFile                           *lockfile.Lockfile
	migrationUUID                      uuid.UUID
	perfProfile                        utils.BoolStr
	ProcessShutdownRequested           bool
	controlPlane                       cp.ControlPlane
	currentCommand                     string
	callHomeErrorOrCompletePayloadSent bool
	controlPlaneConfig                 map[string]string // Holds control plane configuration from config file
)

var envVarValuesToObfuscateInLogs = []string{
	"SOURCE_DB_PASSWORD",
	"TARGET_DB_PASSWORD",
	"SOURCE_REPLICA_DB_PASSWORD",
}

var configKeyValuesToObfuscateInLogs = []string{
	"source.db-password",
	"target.db-password",
	"source-replica.db-password",
}

var rootCmd = &cobra.Command{
	Use:   "yb-voyager",
	Short: "A CLI based migration engine to migrate complete database(schema + data) from some source database to YugabyteDB",
	Long: `A CLI based migration engine for complete database migration from a source database to YugabyteDB. Currently supported source databases are Oracle, MySQL, PostgreSQL.
Refer to docs (https://docs.yugabyte.com/preview/migrate/) for more details like setting up source/target, migration workflow etc.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// Initialize the config file (also loads control plane config)
		envVarsAlreadyExported, err := initConfig(cmd)
		if err != nil {
			// not using utils.ErrExit as logging is not initialized yet
			fmt.Printf("ERROR: Failed to initialize config: %v\n", err)
			atexit.Exit(1)
		}

		currentCommand = cmd.CommandPath()

		if isLiveMigrationIterationCommand(cmd) {
			err := resolveToActiveIterationIfRequired(cmd)
			if err != nil {
				utils.ErrExit("failed to resolve to active iteration: %w", err)
			}
		}

		if !shouldRunPersistentPreRun(cmd) {
			return
		}

		if isBulkAssessmentCommand(cmd) {
			validateBulkAssessmentDirFlag()
			err := config.ValidateLogLevel()
			if err != nil {
				// not using utils.ErrExit as logging is not initialized yet
				fmt.Printf("ERROR: %v\n", err)
				atexit.Exit(1)
			}
			if shouldLock(cmd) {
				lockFPath := filepath.Join(bulkAssessmentDir, fmt.Sprintf(".%sLockfile.lck", GetCommandID(cmd)))
				lockFile = lockfile.NewLockfile(lockFPath)
				lockFile.Lock()
			}
			err = InitLogging(bulkAssessmentDir, config.LogLevel, cmd.Use == "status", GetCommandID(cmd))
			if err != nil {
				utils.ErrExit("Failed to initialize logging: %w", err)
			}
			startTime = time.Now()
			log.Infof("Start time: %s\n", startTime)

			// Initialize the metaDB variable only if the metaDB is already created. For example, resumption of a command.
			metaDB = initMetaDB(bulkAssessmentDir)
			if perfProfile {
				go startPprofServer()
			}
			// no info/payload is collected/supported for assess-migration-bulk
			err = setControlPlane("")
			if err != nil {
				utils.ErrExit("ERROR: setting up control plane: %w", err)
			}
		} else {
			validateExportDirFlag()
			err := config.ValidateLogLevel()
			if err != nil {
				// not using utils.ErrExit as logging is not initialized yet
				fmt.Printf("ERROR: %v\n", err)
				atexit.Exit(1)
			}
			schemaDir = filepath.Join(exportDir, "schema")
			if shouldLock(cmd) {
				lockFPath := filepath.Join(exportDir, fmt.Sprintf(".%sLockfile.lck", GetCommandID(cmd)))
				lockFile = lockfile.NewLockfile(lockFPath)
				lockFile.Lock()
			}
			err = InitLogging(exportDir, config.LogLevel, cmd.Use == "status", GetCommandID(cmd))
			if err != nil {
				utils.ErrExit("Failed to initialize logging: %w", err)
			}
			startTime = time.Now()
			log.Infof("Start time: %s\n", startTime)

			if shouldRunExportDirInitialisedCheck(cmd) {
				checkExportDirInitialised()
			}

			if callhome.SendDiagnostics {
				go sendCallhomePayloadAtIntervals()
			}

			// Initialize the metaDB variable only if the metaDB is already created. For example, resumption of a command.
			if metaDBIsCreated(exportDir) {
				metaDB = initMetaDB(exportDir)
				msr, err := metaDB.GetMigrationStatusRecord()
				if err != nil {
					utils.ErrExit("get migration status record: %w", err)
				}

				msrVoyagerVersionString := msr.VoyagerVersion

				detectVersionCompatibility(msrVoyagerVersionString, exportDir)

			}

			if perfProfile {
				go startPprofServer()
			}
			// Set up the control plane
			err = setControlPlane(getControlPlaneType())
			if err != nil {
				utils.ErrExit("ERROR: setting up control plane: %w", err)
			}
		}

		// Log the flag values set from the config file
		for _, f := range resolvedConfig.fromConfigFile {
			if slices.Contains(configKeyValuesToObfuscateInLogs, f.ConfigKey) {
				f.Value = "********"
			}
			log.Infof("Flag '%s' set from config key '%s' with value '%s'\n", f.FlagName, f.ConfigKey, f.Value)
		}
		// Log the env variables already set in the environment by the user
		for envVar, val := range envVarsAlreadyExported {
			if slices.Contains(envVarValuesToObfuscateInLogs, envVar) {
				val = "********"
			}
			log.Infof("Environment variable '%s' already set with value '%s'\n", envVar, val)
		}
		// Log the env variables set from the config file
		for _, val := range resolvedConfig.fromEnvVar {
			if slices.Contains(envVarValuesToObfuscateInLogs, val.EnvVar) {
				val.Value = "********"
			}
			log.Infof("Environment variable '%s' set from config key '%s' with value '%s'\n", val.EnvVar, val.ConfigKey, val.Value)
		}

	},

	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},

	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if shouldLock(cmd) {
			lockFile.Unlock()
		}
		if shouldRunPersistentPreRun(cmd) {
			controlPlane.Finalize()
		}
		atexit.Exit(0)
	},
}

var liveMigrationIterationCommandList = []string{
	"yb-voyager import data",
	"yb-voyager export data",
	"yb-voyager export data from source",
	"yb-voyager import data to target",
	"yb-voyager export data from target",
	"yb-voyager import data to source",
	"yb-voyager initiate cutover to source",
	"yb-voyager initiate cutover to target",
}

func isLiveMigrationIterationCommand(cmd *cobra.Command) bool {
	return slices.Contains(liveMigrationIterationCommandList, cmd.CommandPath())
}

func resolveToActiveIterationIfRequired(cmd *cobra.Command) error {
	if !metaDBIsCreated(exportDir) {
		return nil
	}
	err := InitLogging(exportDir, config.LogLevel, cmd.Use == "status", GetCommandID(cmd))
	if err != nil {
		utils.ErrExit("Failed to initialize logging: %w", err)
	}
	metaDB = initMetaDB(exportDir)
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil || msr == nil {
		return nil
	}
	if !msr.IsParentMigration() || msr.LatestIterationNumber == 0 {
		return nil
	}
	iterationsDir := msr.GetIterationsDir(exportDir)
	iterationExportDir := GetIterationExportDir(iterationsDir, msr.LatestIterationNumber)
	if !utils.FileOrFolderExists(iterationExportDir) {
		return goerrors.Errorf("iteration export directory does not exist")
	}
	iterationMetaDB, err := metadb.NewMetaDB(iterationExportDir)
	if err != nil {
		return fmt.Errorf("failed to create iteration meta db: %w", err)
	}

	log.Infof("Resolving to iteration %d: %s", msr.LatestIterationNumber, iterationExportDir)
	//update the export dir and meta db to the iteration export dir and meta db
	exportDir = iterationExportDir
	metaDB = iterationMetaDB
	exportType = CHANGES_ONLY

	return nil

}

func shouldRunExportDirInitialisedCheck(cmd *cobra.Command) bool {
	return slices.Contains(exportDirInitialisedCheckNeededList, cmd.CommandPath())
}

func checkExportDirInitialised() {
	// Check to ensure that this is not the first command in the migration process
	isMetaDBPresent := metaDBIsCreated(exportDir)
	if !isMetaDBPresent {
		utils.ErrExit("Migration has not started yet. Run the commands in the order specified in the documentation: %s", color.BlueString("https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/"))
	}
}

func startPprofServer() {
	// Server for pprof
	err := http.ListenAndServe("localhost:6060", nil)
	if err != nil {
		fmt.Println("Error starting pprof server")
	}
	/*
		Steps to use pprof for profiling yb-voyager:
		1. install graphviz on the machine using - sudo yum install graphviz gv
		2. start voyager with profile flag - yb-voyager ... --profile true
		3. use the following command to start a web ui for the profile data-
			go tool pprof -http=[<client_machine_ip>:<port>] http://localhost:6060/debug/pprof/profile
	*/
}

var exportDirInitialisedCheckNeededList = []string{
	"yb-voyager analyze-schema",
	"yb-voyager import data",
	"yb-voyager import data to target",
	"yb-voyager import data to source",
	"yb-voyager import data to source-replica",
	"yb-voyager import data status",
	"yb-voyager export data from target",
	"yb-voyager export data status",
	"yb-voyager cutover status",
	"yb-voyager get data-migration-report",
	"yb-voyager compare-performance",
	"yb-voyager archive changes",
	"yb-voyager end migration",
	"yb-voyager initiate cutover to source",
	"yb-voyager initiate cutover to source-replica",
	"yb-voyager initiate cutover to target",
}

var noLockNeededList = []string{
	"yb-voyager",
	"yb-voyager version",
	"yb-voyager help",
	"yb-voyager import",
	"yb-voyager import data to",
	"yb-voyager import data status",
	"yb-voyager export",
	"yb-voyager export data from",
	"yb-voyager export data status",
	"yb-voyager cutover",
	"yb-voyager cutover status",
	"yb-voyager get data-migration-report",
	"yb-voyager initiate",
	"yb-voyager end",
	"yb-voyager archive",
}

var noPersistentPreRunNeededList = []string{
	"yb-voyager",
	"yb-voyager version",
	"yb-voyager help",
	"yb-voyager import",
	"yb-voyager import data to",
	"yb-voyager export",
	"yb-voyager export data from",
	"yb-voyager initiate",
	"yb-voyager initiate cutover",
	"yb-voyager initiate cutover to",
	"yb-voyager cutover",
	"yb-voyager archive",
	"yb-voyager end",
}

// used for registering the --config-file flag
var offlineCommands = []string{
	"yb-voyager assess-migration",
	"yb-voyager export schema",
	"yb-voyager analyze-schema",
	"yb-voyager import schema",
	"yb-voyager export data",
	"yb-voyager export data from source",
	"yb-voyager import data",
	"yb-voyager import data to target",
	"yb-voyager finalize-schema-post-data-import",
	"yb-voyager end migration",
	"yb-voyager compare-performance",
}

func shouldLock(cmd *cobra.Command) bool {
	return !slices.Contains(noLockNeededList, cmd.CommandPath())
}

func shouldRunPersistentPreRun(cmd *cobra.Command) bool {
	return !slices.Contains(noPersistentPreRunNeededList, cmd.CommandPath())
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	callhome.ReadEnvSendDiagnostics()
}

var globalFlags = []string{}

// Note: assess-migration-bulk and get data-migration-report commands do not call this function.
func registerCommonGlobalFlags(cmd *cobra.Command) {
	BoolVar(cmd.Flags(), &perfProfile, "profile", false,
		"profile yb-voyager for performance analysis")
	cmd.Flags().MarkHidden("profile")

	registerExportDirFlag(cmd)
	globalFlags = append(globalFlags, "export-dir")
	registerConfigFileFlag(cmd)
	globalFlags = append(globalFlags, "config-file")

	cmd.PersistentFlags().StringVarP(&config.LogLevel, "log-level", "l", "info",
		"log level for yb-voyager. Accepted values: (trace, debug, info, warn, error, fatal, panic)")
	globalFlags = append(globalFlags, "log-level")

	cmd.PersistentFlags().BoolVarP(&utils.DoNotPrompt, "yes", "y", false,
		"assume answer as yes for all questions during migration (default false)")

	BoolVar(cmd.Flags(), &callhome.SendDiagnostics, "send-diagnostics", true,
		"enable or disable the 'send-diagnostics' feature that sends analytics data to YugabyteDB.(default true)")
	globalFlags = append(globalFlags, "send-diagnostics")

	//Any global flags added here should be added to the globalFlags slice
}

func registerConfigFileFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&cfgFile, "config-file", "c", "",
		"path of the config file which is used to set the various parameters for yb-voyager commands")
}

func registerExportDirFlag(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")
}

func validateExportDirFlag() {
	if exportDir == "" {
		utils.ErrExit(`ERROR required flag "export-dir" not set`)
	}
	if !utils.FileOrFolderExists(exportDir) {
		utils.ErrExit("export-dir doesn't exist: %q\n", exportDir)
	} else {
		if exportDir == "." {
			fmt.Println("Note: Using current directory as export-dir")
		}
		var err error
		exportDir, err = filepath.Abs(exportDir)
		if err != nil {
			utils.ErrExit("Failed to get absolute path for export-dir: %q: %w\n", exportDir, err)
		}
		exportDir = filepath.Clean(exportDir)
	}

	fmt.Printf("Using export-dir: %s\n\n", color.BlueString(exportDir))
}

func GetCommandID(c *cobra.Command) string {
	if c.HasParent() {
		p := GetCommandID(c.Parent())
		if p == "" {
			return c.Name()
		} else {
			return p + "-" + c.Name()
		}
	}
	return ""
}

func BoolVar(flagSet *pflag.FlagSet, p *utils.BoolStr, name string, value bool, usage string) {
	*p = utils.BoolStr(value)
	flagSet.AddFlag(&pflag.Flag{
		Name:     name,
		Usage:    usage + "; accepted values (true, false, yes, no, 0, 1)",
		Value:    p,
		DefValue: fmt.Sprintf("%t", value),
	})
}

func metaDBIsCreated(exportDir string) bool {
	return utils.FileOrFolderExists(filepath.Join(exportDir, "metainfo", "meta.db"))
}

func setControlPlane(cpType string) error {
	switch cpType {
	case "":
		log.Infof("'control-plane-type' not set. Setting cp to NoopControlPlane.")
		controlPlane = noopcp.New()
	case YUGABYTED:
		ybdConnString := os.Getenv("YUGABYTED_DB_CONN_STRING")
		if ybdConnString == "" {
			return goerrors.Errorf("yugabyted-control-plane.db-conn-string config param (or YUGABYTED_DB_CONN_STRING environment variable) needs to be set if control plane type is 'yugabyted'.")
		}
		controlPlane = yugabyted.New(exportDir)
		log.Infof("Migration UUID %s", migrationUUID)
		err := controlPlane.Init()
		if err != nil {
			return fmt.Errorf("initialize yugabyted control plane for visualization: %w", err)
		}
		log.Infof("Yugabyted control plane initialized successfully")
	case YBAEON:
		// Get YB-Aeon config from nested section
		// Default to production domain if not specified in config
		domain := controlPlaneConfig["ybaeon-control-plane.domain"]
		if domain == "" {
			domain = "https://cloud.yugabyte.com"
		}
		accountID := controlPlaneConfig["ybaeon-control-plane.account-id"]
		projectID := controlPlaneConfig["ybaeon-control-plane.project-id"]
		clusterID := controlPlaneConfig["ybaeon-control-plane.cluster-id"]
		apiKey := controlPlaneConfig["ybaeon-control-plane.api-key"]

		// Validate required YB-Aeon configuration
		var missingKeys []string
		if accountID == "" {
			missingKeys = append(missingKeys, "ybaeon-control-plane.account-id")
		}
		if projectID == "" {
			missingKeys = append(missingKeys, "ybaeon-control-plane.project-id")
		}
		if clusterID == "" {
			missingKeys = append(missingKeys, "ybaeon-control-plane.cluster-id")
		}
		if apiKey == "" {
			missingKeys = append(missingKeys, "ybaeon-control-plane.api-key")
		}
		if len(missingKeys) > 0 {
			return goerrors.Errorf("YB-Aeon control plane requires the following configuration keys: %v", missingKeys)
		}

		ybaeonConfig := &ybaeon.YBAeonConfig{
			Domain:    domain,
			AccountID: accountID,
			ProjectID: projectID,
			ClusterID: clusterID,
			APIKey:    apiKey,
		}

		controlPlane = ybaeon.New(exportDir, ybaeonConfig)
		log.Infof("Migration UUID %s", migrationUUID)
		err := controlPlane.Init()
		if err != nil {
			return fmt.Errorf("initialize YB-Aeon control plane for visualization: %w", err)
		}
		log.Infof("YB-Aeon control plane initialized successfully")
	default:
		return goerrors.Errorf("invalid value of control plane type: %q. Allowed values: %v", cpType, []string{YUGABYTED, YBAEON})
	}
	return nil
}

func getControlPlaneType() string {
	return os.Getenv("CONTROL_PLANE_TYPE")
}
