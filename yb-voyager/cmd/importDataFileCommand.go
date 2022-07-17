package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/libmig"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/semaphore"
)

var (
	fileFormat           string
	delimiter            string
	dataDir              string
	fileTableMapping     string
	hasHeader            bool
	tableNameVsFilePath  = make(map[string]string)
	supportedFileFormats = []string{datafile.CSV, datafile.TEXT}
	fileOpts             string
	fileOptsMap          = make(map[string]string)
	supportedCsvFileOpts = []string{"escape_char", "quote_char"}
)

var importDataFileCmd = &cobra.Command{
	Use:   "file",
	Short: "This command imports data from given files into YugabyteDB database",

	Run: func(cmd *cobra.Command, args []string) {
		checkImportDataFileFlags()
		parseFileTableMapping()
		importDataFiles()
		//prepareForImportDataCmd()
		//importData()
	},
}

func importDataFiles() {
	ctx := context.Background()
	sema := semaphore.NewWeighted(20)
	migstate := libmig.NewMigrationState(exportDir)
	progressReporter := libmig.NewProgressReporter()
	connPool := newConnPool()
	tdb := libmig.NewTargetDB(connPool)
	dfd := &libmig.DataFileDescriptor{
		FileType: fileFormat,
		// TODO Fill up other data descriptor options.
	}
	dbName, schemaName := target.DBName, target.Schema
	for tableName, filePath := range tableNameVsFilePath {
		tableID := libmig.NewTableID(dbName, schemaName, tableName)
		op := libmig.NewImportFileOp(migstate, progressReporter, tdb, filePath, tableID, dfd, sema)
		op.BatchSize = int(numLinesInASplit)
		err := op.Run(ctx)
		if err != nil {
			utils.ErrExit("Failed to import %s: %s", tableID, err)
		}
	}
	// Let the progress bars end properly.
	time.Sleep(time.Second)
}

func newConnPool() *libmig.ConnectionPool {
	targets := getYBServers()
	var targetUriList []string
	for _, t := range targets {
		targetUriList = append(targetUriList, t.Uri)
	}
	log.Infof("targetUriList: %s", targetUriList)
	params := &libmig.ConnectionParams{
		NumConnections: parallelImportJobs + 1,
		ConnUriList:    targetUriList,
		SessionVars: map[string]string{
			"yb_disable_transactional_writes": fmt.Sprintf("%v", disableTransactionalWrites),
			"yb_enable_upsert_mode":           fmt.Sprintf("%v", enableUpsert),
		},
	}
	connPool := libmig.NewConnectionPool(params)
	return connPool
}

func prepareForImportDataCmd() {
	sourceDBType = ORACLE // dummy value - this command is not affected by it
	CreateMigrationProjectIfNotExists(sourceDBType, exportDir)
	tableFileSize := getFileSizeInfo()
	dfd := &datafile.Descriptor{
		FileFormat:    fileFormat,
		TableFileSize: tableFileSize,
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
			log.Infof("removing symlink: %q to create fresh link", symLinkPath)
			err = os.Remove(symLinkPath)
			if err != nil {
				utils.ErrExit("removing symlink %q: %v", symLinkPath, err)
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
	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	for table, filePath := range tableNameVsFilePath {
		if fileFormat == datafile.CSV {
			if hasHeader {
				df, err := datafile.OpenDataFile(filePath, dataFileDescriptor)
				if err != nil {
					utils.ErrExit("opening datafile %q to prepare copy command: %v", err)
				}
				copyTableFromCommands[table] = fmt.Sprintf(`COPY %s(%s) FROM STDIN WITH (FORMAT %s, DELIMITER '%c', ESCAPE '%s', QUOTE '%s', HEADER,`,
					table, df.GetHeader(), fileFormat, []rune(delimiter)[0], fileOptsMap["escape_char"], fileOptsMap["quote_char"])
			} else {
				copyTableFromCommands[table] = fmt.Sprintf(`COPY %s FROM STDIN WITH (FORMAT %s, DELIMITER '%c', ESCAPE '%s', QUOTE '%s', `,
					table, fileFormat, []rune(delimiter)[0], fileOptsMap["escape_char"], fileOptsMap["quote_char"])
			}
		} else if fileFormat == datafile.TEXT {
			copyTableFromCommands[table] = fmt.Sprintf(`COPY %s FROM STDIN WITH (FORMAT %s, DELIMITER '%c', `, table, fileFormat, []rune(delimiter)[0])
		} else {
			panic(fmt.Sprintf("File Type %q not implemented\n", fileFormat))
		}
		copyTableFromCommands[table] += ` ROWS_PER_TRANSACTION %v)`
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
		files, err := filepath.Glob(filepath.Join(dataDir, "*_data.csv"))
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

		reTableName := regexp.MustCompile(`(\S+)_data.csv`)
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
	validateExportDirFlag()
	fileFormat = strings.ToLower(fileFormat)
	checkFileFormat()
	checkDataDirFlag()
	checkDelimiterFlag()
	checkHasHeader()
	checkFileOpts()
}

func checkFileFormat() {
	supported := false
	for _, supportedFileFormat := range supportedFileFormats {
		if fileFormat == supportedFileFormat {
			supported = true
			break
		}
	}

	if !supported {
		utils.ErrExit("--format %q is not supported", fileFormat)
	}
}

func checkDataDirFlag() {
	if dataDir == "" {
		fmt.Fprintln(os.Stderr, `ERROR: required flag "data-dir" not set`)
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
	var err error
	delimiter, err = strconv.Unquote(`"` + delimiter + `"`)
	if err != nil || len(delimiter) != 1 {
		utils.ErrExit("ERROR: invalid syntax of flag value in --delimiter %q. It should be a valid single-byte value.", delimiter)
	}
	log.Infof("resolved delimiter value: %q", delimiter)
}

func checkHasHeader() {
	if hasHeader && fileFormat != datafile.CSV {
		utils.ErrExit("--has-header flag is only supported for CSV file format")
	}
}

func checkFileOpts() {
	switch fileFormat {
	case datafile.CSV:
		if fileOpts == "" { // set defaults
			fileOptsMap = map[string]string{
				"escape_char": "\"",
				"quote_char":  "\"",
			}
			return
		}

		keyValuePairs := strings.Split(fileOpts, ",")
		for _, keyValuePair := range keyValuePairs {
			key, value := strings.Split(keyValuePair, "=")[0], strings.Split(keyValuePair, "=")[1]
			key = strings.ToLower(key)
			if !slices.Contains(supportedCsvFileOpts, key) {
				utils.ErrExit("ERROR: %q is not a valid csv file option", key)
			} else if len(value) != 1 {
				utils.ErrExit("ERROR: invalid syntax of opt '%s=%s' in --file-opts flag. It should be a valid single-byte value.", key, value)
			}
			fileOptsMap[key] = value
		}

	case datafile.TEXT:
		if fileOpts != "" {
			utils.ErrExit("ERROR: --file-opts flag is invalid for %q format", fileFormat)
		}
	default:
		if fileOpts != "" {
			panic(fmt.Sprintf("ERROR: --file-opts not implemented for %q format\n", fileFormat))
		}
	}

	log.Infof("fileOptsMap: %v", fileOptsMap)
}

func init() {
	importDataCmd.AddCommand(importDataFileCmd)
	registerCommonImportFlags(importDataFileCmd)

	importDataFileCmd.Flags().StringVar(&fileFormat, "format", "csv",
		fmt.Sprintf("supported data file types: %s", supportedFileFormats))

	importDataFileCmd.Flags().StringVar(&delimiter, "delimiter", ",",
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

	importDataFileCmd.Flags().StringVar(&fileOpts, "file-opts", "",
		`comma separated options for csv file format:
		1. escape_char: escape character (default is double quotes '"')
		2. quote_char: 	character used to quote the values (default double quotes '"')
		for eg: --file-opts "escape_char=\",quote_char=\" or --file-opts 'escape_char=",quote_char="'`)

	importDataFileCmd.Flags().BoolVar(&disablePb, "disable-pb", false,
		"true - to disable progress bar during data import (default false)")
}
