package cmd

import (
	"bytes"
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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/term"
)

var (
	backupSchemaFiles     utils.BoolStr
	backupDataFiles       utils.BoolStr
	saveMigrationReports  utils.BoolStr
	backupLogFiles        utils.BoolStr
	backupDir             string
	targetDBPassword      string
	fallForwardDBPassword string
	sourceDBPassword      string
)

var endMigrationCmd = &cobra.Command{
	Use:   "migration",
	Short: "End the current migration and cleanup all metadata stored in databases(Target, Fall-Forward and Fall-Back) and export-dir",
	Long:  "End the current migration and cleanup all metadata stored in databases(Target, Fall-Forward and Fall-Back) and export-dir",

	PreRun: func(cmd *cobra.Command, args []string) {
		err := validateEndMigrationFlags(cmd)
		if err != nil {
			utils.ErrExit(err.Error())
		}

		if utils.IsDirectoryEmpty(exportDir) {
			utils.ErrExit("export directory is empty, nothing to end")
		}
	},

	Run: endMigrationCommandFn,
}

func endMigrationCommandFn(cmd *cobra.Command, args []string) {
	if utils.AskPrompt("Migration can't be resumed or continued after this.", "Are you sure you want to end the migration") {
		log.Info("ending the migration")
	} else {
		utils.PrintAndLog("aborting the end migration command")
		return
	}

	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		utils.ErrExit("end migration: getting migration status record: %v", err)
	} else if msr == nil {
		utils.ErrExit("end migration: migration status record not found. Is the migration initialized?")
	}
	retrieveMigrationUUID()
	checkIfEndCommandCanBePerformed(msr)

	// backing up the state from the export directory
	backupSchemaFilesFn()
	backupDataFilesFn()
	saveMigrationReportsFn(msr)

	// cleaning only the migration state wherever and  whatever required
	cleanupSourceDB(msr)
	cleanupTargetDB(msr)
	cleanupFallForwardDB(msr)
	cleanupFallBackDB(msr)

	backupLogFilesFn()
	cleanupExportDir()
	utils.PrintAndLog("Migration ended successfully")
}

func backupSchemaFilesFn() {
	schemaDirPath := filepath.Join(exportDir, "schema")
	if !bool(backupSchemaFiles) || !utils.FileOrFolderExists(schemaDirPath) {
		return
	}

	utils.PrintAndLog("backing up schema files")
	cmd := exec.Command("mv", schemaDirPath, backupDir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.ErrExit("end migration: moving schema files: %s: %v", string(output), err)
	}
}

func backupDataFilesFn() {
	if !backupDataFiles {
		return
	}

	utils.PrintAndLog("backing up snapshot sql data files")
	err := os.MkdirAll(filepath.Join(backupDir, "data"), 0755)
	if err != nil {
		utils.ErrExit("end migration: creating data directory for backup: %v", err)
	}

	files, err := os.ReadDir(filepath.Join(exportDir, "data"))
	if err != nil {
		utils.ErrExit("end migration: reading data directory: %v", err)
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".sql") {
			continue
		}

		dataFilePath := filepath.Join(exportDir, "data", file.Name())
		backupFilePath := filepath.Join(backupDir, "data", file.Name())
		err = os.Rename(dataFilePath, backupFilePath)
		if err != nil {
			utils.ErrExit("end migration: moving data files: %v", err)
		}
	}
}

func saveMigrationReportsFn(msr *metadb.MigrationStatusRecord) {
	if !saveMigrationReports {
		return
	}

	err := os.MkdirAll(filepath.Join(backupDir, "reports"), 0755)
	if err != nil {
		utils.ErrExit("end migration: creating reports directory for backup: %v", err)
	}

	// TODO: what if there is no report.txt generated from analyze-schema step
	utils.PrintAndLog("saving schema analysis report")
	files, err := os.ReadDir(filepath.Join(exportDir, "reports"))
	if err != nil {
		utils.ErrExit("end migration: reading reports directory: %v", err)
	}
	for _, file := range files {
		if file.IsDir() || !strings.HasPrefix(file.Name(), "report.") {
			continue
		}

		err = os.Rename(filepath.Join(exportDir, "reports", file.Name()), filepath.Join(backupDir, "reports", file.Name()))
		if err != nil {
			utils.ErrExit("end migration: moving migration reports: %v", err)
		}
	}

	utils.PrintAndLog("saving data export reports...")
	exportDataReportFilePath := filepath.Join(backupDir, "reports", "export_data_report.txt")
	strCmd := fmt.Sprintf("yb-voyager export data status -e %s > %q", exportDir, exportDataReportFilePath)
	exportDataStatusCmd := exec.Command("bash", "-c", strCmd)
	var outbuf bytes.Buffer
	exportDataStatusCmd.Stderr = &outbuf
	err = exportDataStatusCmd.Run()
	if err != nil {
		log.Errorf("end migration: running export data status command: %s: %v", outbuf.String(), err)
		utils.ErrExit("end migration: running export data status command: %v", err)
	}

	utils.PrintAndLog("saving data import reports...")
	importDataReportFilePath := filepath.Join(backupDir, "reports", "import_data_report.txt")
	strCmd = fmt.Sprintf("yb-voyager import data status -e %s > %q", exportDir, importDataReportFilePath)
	importDataStatusCmd := exec.Command("bash", "-c", strCmd)
	targetDBPassword, err = askPassword("target DB", "", "TARGET_DB_PASSWORD")
	if err != nil {
		utils.ErrExit("end migration: getting target db password: %v", err)
	}
	importDataStatusCmd.Env = append(os.Environ(), fmt.Sprintf("TARGET_DB_PASSWORD=%s", targetDBPassword))

	if msr.FallForwardEnabled {
		fallForwardDBPassword, err = askPassword("fall-forward DB", "", "FF_DB_PASSWORD")
		if err != nil {
			utils.ErrExit("end migration: getting fall-forward db password: %v", err)
		}
		importDataStatusCmd.Env = append(importDataStatusCmd.Env, fmt.Sprintf("FF_DB_PASSWORD=%s", fallForwardDBPassword))
	}

	if msr.FallbackEnabled {
		sourceDBPassword, err = askPassword("fall-back DB", "", "SOURCE_DB_PASSWORD")
		if err != nil {
			utils.ErrExit("end migration: getting fall-back db password: %v", err)
		}
		importDataStatusCmd.Env = append(importDataStatusCmd.Env, fmt.Sprintf("SOURCE_DB_PASSWORD=%s", sourceDBPassword))
	}

	outbuf = bytes.Buffer{}
	importDataStatusCmd.Stderr = &outbuf
	err = importDataStatusCmd.Run()
	if err != nil {
		log.Errorf("end migration: running import data status command: %s: %v", outbuf.String(), err)
		utils.ErrExit("end migration: running import data status command: %v", err)
	}
}

func backupLogFilesFn() {
	if !backupLogFiles {
		return
	}
	// TODO: in case of failures when cmd is executed again, even if logs were backed up, new log file for end migration will come up

	backupLogDir := filepath.Join(backupDir, "logs")
	err := os.MkdirAll(backupLogDir, 0755)
	if err != nil {
		utils.ErrExit("end migration: creating logs directory for backup: %v", err)
	}

	utils.PrintAndLog("backing up log files")
	cmdStr := fmt.Sprintf("mv %s/logs/*.log %s", exportDir, backupLogDir)
	cmd := exec.Command("bash", "-c", cmdStr)
	output, err := cmd.CombinedOutput()
	if err != nil {
		utils.ErrExit("end migration: moving log files: %s: %v", string(output), err)
	}
}

func askPassword(destination string, user string, envVar string) (string, error) {
	if os.Getenv(envVar) != "" {
		return os.Getenv(envVar), nil
	}

	if user == "" {
		fmt.Printf("Password to connect to %s (In addition, you can also set the password using the environment variable '%s'): ",
			destination, envVar)
	} else {
		fmt.Printf("Password to connect to '%s' user of %s (In addition, you can also set the password using the environment variable '%s'): ",
			user, destination, envVar)
	}
	bytePassword, err := term.ReadPassword(int(syscall.Stdin))
	if err != nil {
		return "", fmt.Errorf("reading password: %w", err)
	}
	fmt.Print("\n")
	return string(bytePassword), nil
}

func cleanupSourceDB(msr *metadb.MigrationStatusRecord) {
	utils.PrintAndLog("cleaning up voyager state from source db...")
	source := msr.SourceDBConf
	if source == nil {
		log.Info("source db conf is not set. skipping cleanup")
		return
	}

	var err error
	source.Password = sourceDBPassword
	if sourceDBPassword == "" {
		source.Password, err = askPassword("source DB", source.User, "SOURCE_DB_PASSWORD")
		if err != nil {
			utils.ErrExit("end migration: getting source db password: %v", err)
		}
	}
	err = source.DB().Connect()
	if err != nil {
		utils.ErrExit("end migration: connecting to source db: %v", err)
	}
	defer source.DB().Disconnect()
	err = source.DB().ClearMigrationState(migrationUUID, exportDir)
	if err != nil {
		utils.ErrExit("end migration: clearing migration state from source db: %v", err)
	}
}

func cleanupTargetDB(msr *metadb.MigrationStatusRecord) {
	utils.PrintAndLog("cleaning up voyager state from target db...")
	if msr.TargetDBConf == nil {
		log.Info("target db conf is not set. skipping cleanup")
		return
	}

	var err error
	tconf := msr.TargetDBConf
	tconf.Password = targetDBPassword
	if targetDBPassword == "" {
		tconf.Password, err = askPassword("target DB", tconf.User, "TARGET_DB_PASSWORD")
		if err != nil {
			utils.ErrExit("end migration: getting target db password: %v", err)
		}
	}
	tdb := tgtdb.NewTargetDB(tconf)
	err = tdb.Init()
	if err != nil {
		utils.ErrExit("end migration: initializing target db: %v", err)
	}
	defer tdb.Finalize()
	err = tdb.ClearMigrationState(migrationUUID, exportDir)
	if err != nil {
		utils.ErrExit("end migration: clearing migration state from target db: %v", err)
	}

	if msr.YBCDCStreamID == "" {
		log.Info("yugabytedb cdc stream id is not set. skipping deleting stream id")
		return
	}
	deleteCDCStreamIDForEndMigration(tconf)
}

func deleteCDCStreamIDForEndMigration(tconf *tgtdb.TargetConf) {
	utils.PrintAndLog("Deleting YugabyteDB CDC stream id\n")
	source := srcdb.Source{
		DBType:         tconf.TargetDBType,
		Host:           tconf.Host,
		Port:           tconf.Port,
		User:           tconf.User,
		Password:       tconf.Password,
		DBName:         tconf.DBName,
		Schema:         tconf.Schema,
		SSLMode:        tconf.SSLMode,
		SSLCertPath:    tconf.SSLCertPath,
		SSLKey:         tconf.SSLKey,
		SSLRootCert:    tconf.SSLRootCert,
		SSLCRL:         tconf.SSLCRL,
		SSLQueryString: tconf.SSLQueryString,
		Uri:            tconf.Uri,
	}
	err := source.DB().Connect()
	if err != nil {
		utils.ErrExit("end migration: connecting to YB as source db for deleting stream id: %v", err)
	}
	defer source.DB().Disconnect()

	ybCDCClient := dbzm.NewYugabyteDBCDCClient(exportDir, strings.Join(source.DB().GetServers(), ","),
		source.SSLRootCert, source.DBName, strings.Split(source.TableList, ",")[0], metaDB)
	err = ybCDCClient.Init()
	if err != nil {
		utils.ErrExit("end migration: initializing yugabytedb cdc client: %v", err)
	}

	_, err = ybCDCClient.ListMastersNodes()
	if err != nil {
		utils.ErrExit("end migration: listing yugabytedb master nodes: %v", err)
	}

	// TODO: check the error once streamID is expirted and ignore it
	err = ybCDCClient.DeleteStreamID()
	if err != nil {
		utils.ErrExit("end migration: deleting yugabytedb cdc stream id: %v", err)
	}
}

func cleanupFallForwardDB(msr *metadb.MigrationStatusRecord) {
	if !msr.FallForwardEnabled {
		return
	}

	utils.PrintAndLog("cleaning up voyager state from fall-forward db...")
	var err error
	ffconf := msr.FallForwardDBConf
	ffconf.Password = fallForwardDBPassword
	if fallForwardDBPassword == "" {
		ffconf.Password, err = askPassword("fall-forward DB", ffconf.User, "FF_DB_PASSWORD")
		if err != nil {
			utils.ErrExit("end migration: getting fall-forward db password: %v", err)
		}
	}
	ffdb := tgtdb.NewTargetDB(ffconf)
	err = ffdb.Init()
	if err != nil {
		utils.ErrExit("end migration: initializing fallforward db: %v", err)
	}
	defer ffdb.Finalize()
	err = ffdb.ClearMigrationState(migrationUUID, exportDir)
	if err != nil {
		utils.ErrExit("end migration: clearing migration state from fallforward db: %v", err)
	}
}

func cleanupFallBackDB(msr *metadb.MigrationStatusRecord) {
	if !msr.FallbackEnabled {
		return
	}

	utils.PrintAndLog("cleaning up voyager state from fallback db...")
	var err error
	fbconf := msr.SourceDBAsTargetConf
	fbconf.Password = sourceDBPassword
	if sourceDBPassword == "" {
		fbconf.Password, err = askPassword("fallback DB", fbconf.User, "SOURCE_DB_PASSWORD")
		if err != nil {
			utils.ErrExit("end migration: getting fallback db password: %v", err)
		}
	}
	fbdb := tgtdb.NewTargetDB(fbconf)
	err = fbdb.Init()
	if err != nil {
		utils.ErrExit("end migration: initializing fallback db: %v", err)
	}
	defer fbdb.Finalize()
	err = fbdb.ClearMigrationState(migrationUUID, exportDir)
	if err != nil {
		utils.ErrExit("end migration: clearing migration state from fallback db: %v", err)
	}
}

func cleanupExportDir() {
	utils.PrintAndLog("cleaning up export dir...")
	subdirs := []string{"schema", "data", "logs", "reports", "temp", "metainfo", "sqlldr"}
	for _, subdir := range subdirs {
		err := os.RemoveAll(filepath.Join(exportDir, subdir))
		if err != nil {
			utils.ErrExit("end migration: removing %s directory: %v", subdir, err)
		}
	}
}

func validateEndMigrationFlags(cmd *cobra.Command) error {
	flags := []string{"backup-schema-files", "backup-data-files", "save-migration-reports", "backup-log-files"}
	for _, flag := range flags {
		if cmd.Flag(flag).Value.String() == "true" && !cmd.Flag("backup-dir").Changed {
			return fmt.Errorf("flag %s requires --backup-dir flag to be set", flag)
		}
	}

	if !utils.FileOrFolderExists(backupDir) {
		return fmt.Errorf("backup-dir %q doesn't exists", backupDir)
	}
	return nil
}

func checkIfEndCommandCanBePerformed(msr *metadb.MigrationStatusRecord) {
	// check if any ongoing voyager command
	matches, err := filepath.Glob(filepath.Join(exportDir, ".*.lck"))
	if err != nil {
		utils.ErrExit("end migration: checking for ongoing voyager commands: %v", err)
	}
	if len(matches) > 0 {
		var ongoingCmds []string
		for _, match := range matches {
			match = filepath.Base(match)
			match = strings.TrimPrefix(match, ".")
			match = strings.TrimSuffix(match, "Lockfile.lck")
			if match == "end-migration" {
				continue
			}
			ongoingCmds = append(ongoingCmds, match)
		}
		if len(ongoingCmds) > 0 &&
			!utils.AskPrompt(fmt.Sprintf("found other ongoing voyager commands: %s. Do you want to continue with end migration command", strings.Join(ongoingCmds, ", "))) {
			utils.ErrExit("aborting the end migration command")
		}
	} else {
		log.Info("no ongoing voyager commands found")
	}

	if bool(backupSchemaFiles) && !msr.ExportSchemaDone {
		utils.PrintAndLog("backup schema files flag is set but schema export is not done, skipping schema backup...")
		backupSchemaFiles = false
	}

	if backupDataFiles {
		if !areOnDifferentFileSystems(exportDir, backupDir) {
			return
		}

		if !msr.ExportDataDone {
			utils.PrintAndLog("backup data files flag is set but data export is not done, skipping data backup...")
			backupDataFiles = false
			return
		}

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
}

// NOTE: function is for Linux only (Windows won't work)
// TODO: verify with dockerized voyager
func areOnDifferentFileSystems(path1 string, path2 string) bool {
	stat1 := syscall.Stat_t{}
	stat2 := syscall.Stat_t{}

	err1 := syscall.Stat(path1, &stat1)
	err2 := syscall.Stat(path2, &stat2)

	if err1 != nil || err2 != nil {
		utils.ErrExit("end migration: getting file system info for %s and %s: %v, %v", path1, path2, err1, err2)
	}

	return stat1.Dev != stat2.Dev
}

func calculateDirSizeWithPattern(dirPath string, filePattern string) (int64, error) {
	var size int64

	fileMatches, err := filepath.Glob(filepath.Join(dirPath, filePattern))
	if err != nil {
		return 0, fmt.Errorf("matching the file pattern %q in the directory %q: %w", filePattern, dirPath, err)
	}

	for _, filePath := range fileMatches {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			return 0, fmt.Errorf("getting file info for %q: %w", filePath, err)
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
	BoolVar(endMigrationCmd.Flags(), &backupDataFiles, "backup-data-files", false, "backup snapshot data files")
	BoolVar(endMigrationCmd.Flags(), &saveMigrationReports, "save-migration-reports", false, "save schema and data migration reports")
	BoolVar(endMigrationCmd.Flags(), &backupLogFiles, "backup-log-files", false, "backup yb-voyager log files for this migration")
	endMigrationCmd.Flags().StringVar(&backupDir, "backup-dir", "", "backup directory")
	endMigrationCmd.Flags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")

	endMigrationCmd.MarkFlagRequired("backup-schema-files")
	endMigrationCmd.MarkFlagRequired("backup-data-files")
	endMigrationCmd.MarkFlagRequired("save-migration-reports")
	endMigrationCmd.MarkFlagRequired("backup-log-files")
	endMigrationCmd.MarkFlagRequired("export-dir")
}
