package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/dustin/go-humanize"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	backupSchemaFiles    utils.BoolStr
	backupDataFiles      utils.BoolStr
	backupMigrationFiles utils.BoolStr
	backupLogFiles       utils.BoolStr
	backupDir            string
)

var endMigrationCmd = &cobra.Command{
	Use:   "migration",
	Short: "End the current migration and cleanup all metadata stored in databases(Target, FF and FB) and export-dir",
	Long:  "End the current migration and cleanup all metadata stored in databases(Target, FF and FB) and export-dir",

	PreRun: func(cmd *cobra.Command, args []string) {
		err := validateEndMigrationFlags(cmd)
		if err != nil {
			utils.ErrExit(err.Error())
		}
	},

	Run: endMigrationCommandFn,
}

func endMigrationCommandFn(cmd *cobra.Command, args []string) {
	if utils.AskPrompt("Migration can't be resumed or continued after this.", "Are you sure you want to end the migration") {
		log.Info("ending the migration")
	} else {
		log.Info("Aborting the end migration command")
		return
	}

	checkIfEndCommandCanBePerformed()

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("end migration: getting migration status record: %v", err)
	} else if msr == nil {
		utils.ErrExit("end migration: migration status record not found. Is the migration initialized?")
	}

	// cleaning only the migration state wherever and  whatever required
	cleanupSourceDB(msr)
	cleanupTargetDB(msr)
	cleanupFallForwardDB(msr)
	cleanupFallBackDB(msr)

	// backing up the state from the export directory
	retrieveMigrationUUID()
	backupSchemaFilesFn(msr)
	backupDataFilesFn()
	backupMigrationReportsFn()
	backupLogFilesFn()

	utils.CleanDir(exportDir)
	utils.PrintAndLog("Migration ended successfully")
}

func backupSchemaFilesFn(msr *metadb.MigrationStatusRecord) {
	if !bool(backupSchemaFiles) || !msr.ExportSchemaDone {
		return
	}

	log.Infof("backing up schema files to %s", backupDir)
	err := os.MkdirAll(filepath.Join(backupDir, "schema-backup"), 0755)
	if err != nil {
		utils.ErrExit("end migration: creating schema backup directory: %v", err)
	}

	cmd := exec.Command("mv", filepath.Join(exportDir, "schema"), filepath.Join(backupDir, "schema-backup"))
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.ErrExit("end migration: moving schema files: %s:\n%v", string(output), err)
	}
}

func backupDataFilesFn() {
	if !bool(backupDataFiles) {
		return
	}

	log.Infof("backing up sql data files to %s", backupDir)
	err := os.MkdirAll(filepath.Join(backupDir, "data-backup"), 0755)
	if err != nil {
		utils.ErrExit("end migration: creating data backup directory: %v", err)
	}

	files, err := os.ReadDir(filepath.Join(exportDir, "data"))
	if err != nil {
		utils.ErrExit("end migration: reading data directory: %v", err)
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), ".sql") {
			continue
		}

		err = os.Rename(filepath.Join(exportDir, "data", file.Name()), filepath.Join(backupDir, "data-backup", file.Name()))
		if err != nil {
			utils.ErrExit("end migration: moving data files: %v", err)
		}
	}
}

func backupMigrationReportsFn() {
	if !bool(backupMigrationFiles) {
		return
	}
	log.Infof("backing up migration reports to %s", backupDir)
	err := os.MkdirAll(filepath.Join(backupDir, "reports-backup"), 0755)
	if err != nil {
		utils.ErrExit("end migration: creating reports backup directory: %v", err)
	}

	err = os.Rename(filepath.Join(exportDir, "reports"), filepath.Join(backupDir, "reports-backup"))
	if err != nil {
		utils.ErrExit("end migration: moving migration reports: %v", err)
	}

	utils.PrintAndLog("backing up export data status report...")
	file, err := os.Create(filepath.Join(backupDir, "reports", "export_data_status.txt"))
	if err != nil {
		utils.ErrExit("end migration: creating export data status report: %v", err)
	}
	originalStdout := os.Stdout
	os.Stdout = file
	exportDataStatusCmd.Run(exportDataStatusCmd, nil)
	os.Stdout = originalStdout
	file.Close()

	utils.PrintAndLog("backing up export data status report...")
	file, err = os.Create(filepath.Join(backupDir, "reports", "import_data_status.txt"))
	if err != nil {
		utils.ErrExit("end migration: creating import data status report: %v", err)
	}
	originalStdout = os.Stdout
	os.Stdout = file
	importDataStatusCmd.Run(importDataStatusCmd, nil)
	os.Stdout = originalStdout
	file.Close()
}

func backupLogFilesFn() {
	if !bool(backupLogFiles) {
		return
	}
	log.Infof("backing up log files to %s", backupDir)
	// err := os.MkdirAll(filepath.Join(backupDir, "logs-backup"), 0755)
	// if err != nil {
	// 	utils.ErrExit("end migration: creating logs backup directory: %v", err)
	// }

	err := os.Rename(filepath.Join(exportDir, "logs"), filepath.Join(backupDir, "logs-backup"))
	if err != nil {
		utils.ErrExit("end migration: moving log files: %v", err)
	}
}

func cleanupSourceDB(msr *metadb.MigrationStatusRecord) {
	source := msr.SourceDBConf
	if source == nil {
		log.Infof("source db conf is not set. skipping cleanup")
		return
	}
	err := source.DB().Connect()
	if err != nil {
		utils.ErrExit("end migration: connecting to source db: %v", err)
	}
	defer source.DB().Disconnect()
	source.DB().ClearMigrationState(migrationUUID, exportDir)

	err = dbzm.NewYugabyteDBCDCClient(exportDir, "", source.SSLRootCert, source.DBName, strings.Split(source.TableList, ",")[0], metaDB).DeleteStreamID()
	if err != nil {
		utils.ErrExit("end migration: deleting yugabytedb cdc stream id: %v", err)
	}
}

func cleanupTargetDB(msr *metadb.MigrationStatusRecord) {
	if msr.TargetDBConf == nil {
		log.Infof("target db conf is not set. skipping cleanup")
		return
	}
	tdb := tgtdb.NewTargetDB(msr.TargetDBConf)
	err := tdb.Init()
	if err != nil {
		utils.ErrExit("end migration: initializing target db: %v", err)
	}
	defer tdb.Finalize()
	tdb.ClearMigrationState(migrationUUID, exportDir)
}

func cleanupFallForwardDB(msr *metadb.MigrationStatusRecord) {
	if msr.FallForwardEnabled {
		ffdb := tgtdb.NewTargetDB(msr.FallForwardDBConf)
		err := ffdb.Init()
		if err != nil {
			utils.ErrExit("end migration: initializing fallforward db: %v", err)
		}
		defer ffdb.Finalize()
		ffdb.ClearMigrationState(migrationUUID, exportDir)
	}
}

func cleanupFallBackDB(msr *metadb.MigrationStatusRecord) {
	if msr.FallbackEnabled {
		fbdb := tgtdb.NewTargetDB(msr.SourceDBAsTargetConf)
		err := fbdb.Init()
		if err != nil {
			utils.ErrExit("end migration: initializing fallback db: %v", err)
		}
		defer fbdb.Finalize()
		fbdb.ClearMigrationState(migrationUUID, exportDir)
	}
}

func validateEndMigrationFlags(cmd *cobra.Command) error {
	flags := []string{"backup-schema-files", "backup-data-files", "backup-migration-reports", "backup-log-files"}
	for _, flag := range flags {
		if cmd.Flag(flag).Value.String() == "true" && !cmd.Flag("backup-dir").Changed {
			return fmt.Errorf("flag %s requires --backup-dir flag to be set", flag)
		}
	}
	return nil
}

func checkIfEndCommandCanBePerformed() {
	// check if any ongoing voyager command
	matches, err := filepath.Glob(filepath.Join(exportDir, ".*.lck"))
	if err != nil {
		utils.ErrExit("end migration: checking for ongoing voyager commands: %v", err)
	}
	if len(matches) > 0 {
		var ongoingCmds []string
		for _, match := range matches {
			match = strings.TrimPrefix(match, ".")
			match = strings.TrimSuffix(match, "Lockfile.lck")
			if match == "end-migration" {
				continue
			}
			ongoingCmds = append(ongoingCmds, match)
		}
		if len(ongoingCmds) > 0 &&
			!utils.AskPrompt(fmt.Sprintf("found ongoing voyager commands: %s. Do you want to continue", strings.Join(ongoingCmds, ", "))) {
			utils.ErrExit("aborting the end migration command")
		}
	} else {
		log.Infof("no ongoing voyager commands found")
	}

	if backupDataFiles {
		// verify that the size of backup-data dir to be greater the export-dir/data dir
		exportDirDataSize, err := calculateDirSizeWithPattern(exportDir, "data/*.sql")
		if err != nil {
			utils.ErrExit("end migration: calculating export dir data size: %v", err)
		}

		backupDirSize, err := getFreeDiskSpace(backupDir)
		if err != nil {
			utils.ErrExit("end migration: calculating backup dir size: %v", err)
		}

		if exportDirDataSize >= int64(backupDirSize) {
			utils.ErrExit(`end migration: backup directory free space is less than the export directory data size.
			Please provide a backup directory with more free space than the export directory data size(%s).`, humanize.Bytes(uint64(exportDirDataSize)))

		}
	}

	// TODO: more checks like if the migration is in progress or not
}

func calculateDirSizeWithPattern(dirPath string, filePattern string) (int64, error) {
	var size int64

	fileMatches, err := filepath.Glob(filepath.Join(dirPath, filePattern))
	if err != nil {
		return 0, err
	}

	for _, filePath := range fileMatches {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return 0, err
		}

		if !fileInfo.IsDir() {
			size += fileInfo.Size()
		}
	}

	return size, nil
}

func getFreeDiskSpace(path string) (uint64, error) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0, err
	}
	// calculate the free space in bytes
	freeSpace := stat.Bfree * uint64(stat.Bsize)
	return freeSpace, nil
}

func init() {
	endCmd.AddCommand(endMigrationCmd)

	BoolVar(endMigrationCmd.Flags(), &backupSchemaFiles, "backup-schema-files", false, "backup migration schema files")
	BoolVar(endMigrationCmd.Flags(), &backupDataFiles, "backup-data-files", false, "backup migration data files")
	// Name of flag: backup-migration-reports or save-migration-reports
	BoolVar(endMigrationCmd.Flags(), &backupMigrationFiles, "backup-migration-reports", false, "backup migration reports")
	BoolVar(endMigrationCmd.Flags(), &backupLogFiles, "backup-log-files", false, "backup migration log files")
	endMigrationCmd.Flags().StringVar(&backupDir, "backup-dir", "", "backup directory")
	endMigrationCmd.Flags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")

	endMigrationCmd.MarkFlagRequired("backup-schema-files")
	endMigrationCmd.MarkFlagRequired("backup-data-files")
	endMigrationCmd.MarkFlagRequired("backup-migration-reports")
	endMigrationCmd.MarkFlagRequired("backup-log-files")
	endMigrationCmd.MarkFlagRequired("export-dir")
}
