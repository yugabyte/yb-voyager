package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
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
	supportedFileTypes  = []string{datafile.CSV, datafile.SQL}
	importDataFileMode  bool
)

var importDataFileCmd = &cobra.Command{
	Use:   "file",
	Short: "This command imports data from given files into YugabyteDB database",

	Run: func(cmd *cobra.Command, args []string) {
		importDataFileMode = true
		checkImportDataFileFlags()
		parseFileTableMapping()
		prepareForImportDataCmd()
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

	sourceDBType = ORACLE // dummy value

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
	tableFileSize := getFileSizeInfo()
	dfd := &datafile.Descriptor{
		FileType:      fileType,
		TableRowCount: tableFileSize,
		Delimiter:     delimiter,
		HasHeader:     hasHeader,
		ExportDir:     exportDir,
	}
	dfd.Save()

	createDataFileSymLinks()
	prepareCopyCommands()
	setImportTableListFlag()
	createExportDataDoneFlag()
}

func getFileSizeInfo() map[string]int64 {
	tableFileSize := make(map[string]int64)
	for table, filePath := range tableNameVsFilePath {
		fileInfo, err := os.Stat(filePath)
		if err != nil {
			utils.ErrExit("calculating file size of %q in bytes: %v", filePath, err)
		}
		tableFileSize[table] = int64(fileInfo.Size())

		log.Infof("File size of %q for table %q: %d", filePath, table, tableFileSize[table])
	}

	return tableFileSize
}

func createDataFileSymLinks() {
	log.Infof("creating symlinks to original data files")
	for table, filePath := range tableNameVsFilePath {
		symLinkPath := filepath.Join(exportDir, "data", table+"_data.sql")

		filePath, err := filepath.Abs(filePath)
		if err != nil {
			utils.ErrExit("absolute original filepath for table %q: %v", table, err)
		}
		log.Infof("absolute filepath for %q: %q", table, filePath)
		log.Infof("symlink path for file %q is %q", filePath, symLinkPath)
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
			utils.ErrExit("error creating symlink to data file %q: %v", filePath, err)
		}
	}
}

func prepareCopyCommands() {
	log.Infof("preparing copy commands for the tables to import")
	for table, filePath := range tableNameVsFilePath {
		var copyCommand string
		if !hasHeader || fileType != datafile.CSV {
			copyCommand = fmt.Sprintf(`COPY %s FROM STDIN DELIMITER '%c'`, table, []rune(delimiter)[0])
		} else {
			dataFileDescriptor = datafile.OpenDescriptor(exportDir)
			df, err := datafile.OpenDataFile(filePath, dataFileDescriptor)
			if err != nil {
				utils.ErrExit("opening datafile to prepare copy command: %v", err)
			}
			copyCommand = fmt.Sprintf(`COPY %s(%s) FROM STDIN DELIMITER '%c' CSV HEADER`, table, df.GetCopyHeader(), []rune(delimiter)[0])
		}
		copyTableFromCommands[table] = copyCommand
	}

	log.Infof("copyTableFromCommands map: %+v", copyTableFromCommands)
}

func setImportTableListFlag() {
	tableList := []string{}
	for key := range tableNameVsFilePath {
		tableList = append(tableList, key)
	}

	target.TableList = strings.Join(tableList, ",")
}

func parseFileTableMapping() {
	if fileTableMapping != "" {
		keyValuePairs := strings.Split(fileTableMapping, ",")
		for _, keyValuePair := range keyValuePairs {
			fileName, table := strings.Split(keyValuePair, ":")[0], strings.Split(keyValuePair, ":")[1]
			tableNameVsFilePath[table] = filepath.Join(dataDir, fileName)
		}
	} else {
		// TODO: replace "link" with docs link
		utils.PrintAndLog("Note: --file-table-map flag is not provided, default will assume the file names in format as mentioned in the docs. Refer - link")
		// get matching file in data-dir
		files, err := filepath.Glob(filepath.Join(dataDir, "*_data.sql"))
		if err != nil {
			utils.ErrExit("finding data files to import: %v", err)
		}

		if len(files) == 0 {
			utils.ErrExit("No data files found to import in %q", dataDir)
		} else {
			var tableFiles []string
			for _, file := range files {
				tableFiles = append(tableFiles, filepath.Base(file))
			}
			utils.PrintAndLog("Table data files identified to import from data-dir(%q) are: [%s]\n\n", dataDir, strings.Join(tableFiles, ", "))
		}

		reTableName := regexp.MustCompile(`(\S+)_data.sql`)
		for _, file := range files {
			fileName := filepath.Base(file)

			matches := reTableName.FindAllStringSubmatch(fileName, -1)
			if len(matches) == 0 {
				utils.ErrExit("datafile names in %q are not in right format, refer docs", dataDir)
			}

			tableName := matches[0][1]
			tableNameVsFilePath[tableName] = file
		}
	}
}

func checkImportDataFileFlags() {
	checkFileType()
	checkDataDirFlag()
	checkDelimiterFlag()
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

func checkDataDirFlag() {
	if dataDir == "" {
		fmt.Fprintln(os.Stderr, `Error: required flag "data-dir" not set`)
		os.Exit(1)
	}
	if !utils.FileOrFolderExists(dataDir) {
		fmt.Fprintf(os.Stderr, "Directory: %s doesn't exists!!\n", dataDir)
		os.Exit(1)
	} else if dataDir == "." {
		fmt.Println("Note: Using current working directory as data directory")
	} else {
		dataDir = strings.TrimRight(dataDir, "/")
	}
}

func checkDelimiterFlag() {
	if len(delimiter) > 1 {
		utils.ErrExit("Not a valid delimiter: %q", delimiter)
	}
}

func init() {
	importDataCmd.AddCommand(importDataFileCmd)
	registerCommonImportFlags(importDataFileCmd)

	importDataFileCmd.Flags().StringVar(&fileType, "file-type", "csv",
		"type of data file: csv, sql")

	importDataFileCmd.Flags().StringVar(&delimiter, "delimiter", "\t",
		"character used as delimiter in rows of the table(s)")

	importDataFileCmd.Flags().StringVar(&dataDir, "data-dir", "",
		"path to the directory contains data files to import into table(s)")
	err := importDataFileCmd.MarkFlagRequired("data-dir")
	if err != nil {
		utils.ErrExit("mark 'data-dir' flag required: %v", err)
	}

	importDataFileCmd.Flags().StringVar(&fileTableMapping, "file-table-map", "",
		"mapping between file name in 'data-dir' to table name.\n"+
			"Note: default will assume the file names in format as mentioned in the docs. Refer - link") // TODO: if not given default should be file names

	importDataFileCmd.Flags().BoolVar(&hasHeader, "has-header", false,
		"true - if first line of data file is a list of columns for rows (default false)\n"+
			"(Note: only works for csv file type)")
}
