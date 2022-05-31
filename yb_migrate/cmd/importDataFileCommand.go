package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/datafile"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/tgtdb"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

var (
	fileType            string
	delimiter           string
	dataDir             string
	fileTableMapping    string
	hasHeader           bool
	tableNameVsFilePath = make(map[string]string)
	supportedFileTypes  = []string{"csv", "sql"}
)

var importDataFileCmd = &cobra.Command{
	Use:   "file",
	Short: "This command imports data from given files into YugabyteDB database",

	Run: func(cmd *cobra.Command, args []string) {
		checkFileType()
		// / checkDelimiter()
		parseFileTableMapping()
		prepareForImportDataCmd()
		// importDataCmd.Run(importDataCmd, args)
		importDataFile()
	},
}

func importDataFile() {
	utils.PrintAndLog("import of data in %q database started", target.DBName)
	err := target.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the target DB: %s", err)
	}
	fmt.Printf("Target YugabyteDB version: %s\n", target.DB().GetVersion())
	// sourceDBType = ExtractMetaInfo(exportDir).SourceDBType
	sourceDBType = POSTGRESQL
	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	targets := getYBServers()

	var parallelism = parallelImportJobs
	if parallelism == -1 {
		parallelism = len(targets)
	}
	log.Infof("parallelism=%v", parallelism)

	if loadBalancerUsed {
		clone := target.Clone()
		clone.Uri = getTargetConnectionUri(clone)
		targets = []*tgtdb.Target{clone}
	}
	if target.VerboseMode {
		fmt.Printf("Number of parallel imports jobs at a time: %d\n", parallelism)
	}

	if parallelism > SPLIT_FILE_CHANNEL_SIZE {
		splitFileChannelSize = parallelism + 1
	}
	splitFilesChannel := make(chan *SplitFileImportTask, splitFileChannelSize)
	targetServerChannel := make(chan *tgtdb.Target, 1)

	go roundRobinTargets(targets, targetServerChannel)
	generateSmallerSplits(splitFilesChannel)
	go doImport(splitFilesChannel, parallelism, targetServerChannel)
	checkForDone()

	time.Sleep(time.Second * 2)
	fmt.Printf("\nexiting...\n")
}

func prepareForImportDataCmd() {
	CreateMigrationProjectIfNotExists("postgresql", exportDir)
	createExportDataDoneFlag()
	createDataFileSymLinks()
	prepareCopyCommands()

	prepareImportTableList()

	disablePb = true // TODO: estimate the total row count for PB
	dfd := datafile.Descriptor{
		FileType:      fileType,
		TableRowCount: nil,
		Delimiter:     delimiter,
		HasHeader:     hasHeader,
		ExportDir:     exportDir,
	}
	dfd.Save()
}

func createDataFileSymLinks() {
	log.Infof("creating symlinks to original data files")
	for table, filePath := range tableNameVsFilePath {
		symLinkPath := exportDir + "/data/" + table + "_data.sql"

		filePath, err := filepath.Abs(filePath)
		if err != nil {
			utils.ErrExit("absolute original filepath for table %q: %v", table, err)
		}
		utils.PrintAndLog("absolute filepath for %q: %q", table, filePath)
		utils.PrintAndLog("symlink path for file %q is %q", filePath, symLinkPath)
		if utils.FileOrFolderExists(symLinkPath) {
			resolvedFilePath, err := os.Readlink(symLinkPath)
			if err != nil {
				utils.ErrExit("resolving symlink %q: %v", symLinkPath, err)
			}
			if resolvedFilePath == filePath {
				log.Infof("using symlink %q already present", symLinkPath)
				continue
			} else {
				log.Infof("removing symlink: %q to create fresh link", symLinkPath)
				err = os.Remove(symLinkPath)
				if err != nil {
					utils.ErrExit("removing symlink %q: %v", symLinkPath, err)
				}
			}
		}

		err = os.Symlink(filePath, symLinkPath)
		if err != nil {
			utils.ErrExit("error creating data file %q symlink: %v", filePath, err)
		}
	}
}

func prepareCopyCommands() {
	log.Infof("preparing copy commands for the tables to import")
	for table := range tableNameVsFilePath {
		copyCommand := fmt.Sprintf(`COPY %s FROM STDIN DELIMITER '%c'`, table, []rune(delimiter)[0])
		if hasHeader {
			copyCommand += " CSV HEADER"
		}
		copyTableFromCommands[table] = copyCommand
	}

	log.Infof("copyTableFromCommands map: %+v", copyTableFromCommands)
}

func prepareImportTableList() {
	tableList := []string{}
	for key := range tableNameVsFilePath {
		tableList = append(tableList, key)
	}

	target.TableList = strings.Join(tableList, ",")
}

func parseFileTableMapping() {
	keyValuePairs := strings.Split(fileTableMapping, ",")
	for _, keyValuePair := range keyValuePairs {
		fileName, table := strings.Split(keyValuePair, ":")[0], strings.Split(keyValuePair, ":")[1]
		tableNameVsFilePath[table] = dataDir + fileName
	}
}

func checkFileType() {
	supported := false
	for _, supportedFileType := range supportedFileTypes {
		if fileType == supportedFileType {
			supported = true
			break
		}
	}

	if !supported {
		utils.ErrExit("given file-type %q is not supported", fileType)
	}
}

func init() {
	importDataCmd.AddCommand(importDataFileCmd)
	registerCommonImportFlags(importDataFileCmd)

	importDataFileCmd.Flags().StringVar(&fileType, "file-type", "",
		"type of data file: csv, sql")
	err := importDataFileCmd.MarkFlagRequired("file-type")
	if err != nil {
		utils.ErrExit("mark 'data-dir' flag required: %v", err)
	}

	importDataFileCmd.Flags().StringVar(&delimiter, "delimiter", "\t",
		"character used as delimiter in rows of the table(s)")

	importDataFileCmd.Flags().StringVar(&dataDir, "data-dir", "",
		"path to the directory contains data files to import into table(s)")
	err = importDataFileCmd.MarkFlagRequired("data-dir")
	if err != nil {
		utils.ErrExit("mark 'data-dir' flag required: %v", err)
	}

	importDataFileCmd.Flags().StringVar(&fileTableMapping, "file-table-map", "",
		"mapping between file name in 'data-dir' to table name") // TODO: if not given default should be file names
	err = importDataFileCmd.MarkFlagRequired("file-table-map")
	if err != nil {
		utils.ErrExit("mark 'file-table-map' flag required: %v", err)
	}

	importDataFileCmd.Flags().BoolVar(&hasHeader, "has-header", false,
		"true - if first line of data file is a list of columns for rows (default false)")
}
