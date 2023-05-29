package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/s3"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
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
	nullString           string
	fileOptsMap          = make(map[string]string)
	supportedCsvFileOpts = []string{"escape_char", "quote_char"}
	dataStore            datastore.DataStore
)

var importDataFileCmd = &cobra.Command{
	Use:   "file",
	Short: "This command imports data from given files into YugabyteDB database",

	Run: func(cmd *cobra.Command, args []string) {
		checkImportDataFileFlags(cmd)
		dataStore = datastore.NewDataStore(dataDir)
		parseFileTableMapping()
		prepareForImportDataCmd()
		importData(nil)
	},
}

func prepareForImportDataCmd() {
	sourceDBType = POSTGRESQL // dummy value - this command is not affected by it
	sqlname.SourceDBType = sourceDBType
	CreateMigrationProjectIfNotExists(sourceDBType, exportDir)
	tableFileSize := getFileSizeInfo()
	dataFileDescriptor = &datafile.Descriptor{
		FileFormat:    fileFormat,
		TableFileSize: tableFileSize,
		Delimiter:     delimiter,
		HasHeader:     hasHeader,
		ExportDir:     exportDir,
	}
	if fileOptsMap["quote_char"] != "" {
		quoteCharBytes := []byte(fileOptsMap["quote_char"])
		dataFileDescriptor.QuoteChar = quoteCharBytes[0]
	}
	if fileOptsMap["escape_char"] != "" {
		escapeCharBytes := []byte(fileOptsMap["escape_char"])
		dataFileDescriptor.EscapeChar = escapeCharBytes[0]
	}

	escapeFileOptsCharsIfRequired() // escaping for COPY command should be done after saving fileOpts in data file descriptor
	createDataFileSymLinks()
	prepareCopyCommands()
	setImportTableListFlag()
	createExportDataDoneFlag()
}

func getFileSizeInfo() map[string]int64 {
	tableFileSize := make(map[string]int64)
	for table, filePath := range tableNameVsFilePath {
		fileSize, err := dataStore.FileSize(filePath)
		if err != nil {
			utils.ErrExit("calculating file size of %q in bytes: %v", filePath, err)
		}
		tableFileSize[table] = fileSize

		log.Infof("File size of %q for table %q: %d", filePath, table, tableFileSize[table])
	}

	return tableFileSize
}

func createDataFileSymLinks() {
	log.Infof("creating symlinks to original data files")
	for table, filePath := range tableNameVsFilePath {
		symLinkPath := filepath.Join(exportDir, "data", table+"_data.sql")

		filePath, err := dataStore.AbsolutePath(filePath)
		if err != nil {
			utils.ErrExit("absolute original filepath for table %q: %v", table, err)
		}
		log.Infof("absolute filepath for %q: %q", table, filePath)
		log.Infof("symlink path for file %q is %q", filePath, symLinkPath)

		log.Infof("removing symlink: %q to create fresh link", symLinkPath)
		err = os.Remove(symLinkPath)
		if err != nil {
			if os.IsNotExist(err) {
				log.Infof("symlink %q does not exist: %v", symLinkPath, err)
			} else {
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
		cmd := ""
		if fileFormat == datafile.CSV {
			if hasHeader {
				reader, err := dataStore.Open(filePath)
				if err != nil {
					utils.ErrExit("preparing reader for copy commands on file %q: %v", filePath, err)
				}
				df, err := datafile.NewDataFile(filePath, reader, dataFileDescriptor)
				if err != nil {
					utils.ErrExit("opening datafile %q to prepare copy command: %v", filePath, err)
				}
				cmd = fmt.Sprintf(`COPY %s(%s) FROM STDIN WITH (FORMAT %s, DELIMITER '%c', ESCAPE '%s', QUOTE '%s', HEADER,`,
					table, df.GetHeader(), fileFormat, []rune(delimiter)[0], fileOptsMap["escape_char"], fileOptsMap["quote_char"])
				df.Close()
			} else {
				cmd = fmt.Sprintf(`COPY %s FROM STDIN WITH (FORMAT %s, DELIMITER '%c', ESCAPE '%s', QUOTE '%s', `,
					table, fileFormat, []rune(delimiter)[0], fileOptsMap["escape_char"], fileOptsMap["quote_char"])
			}
		} else if fileFormat == datafile.TEXT {
			cmd = fmt.Sprintf(`COPY %s FROM STDIN WITH (FORMAT %s, DELIMITER '%c', `, table, fileFormat, []rune(delimiter)[0])
		} else {
			panic(fmt.Sprintf("File Type %q not implemented\n", fileFormat))
		}
		cmd += ` ROWS_PER_TRANSACTION %v)`
		copyTableFromCommands.Store(table, cmd)
	}
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
			tableNameVsFilePath[table] = dataStore.Join(dataDir, fileName)
		}
	} else {
		// TODO: replace "link" with docs link
		utils.PrintAndLog("Note: --file-table-map flag is not provided, default will assume the file names in format as mentioned in the docs. Refer - link")

		// get matching file in data-dir
		files, err := dataStore.Glob("*_data.csv")
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

func checkImportDataFileFlags(cmd *cobra.Command) {
	validateExportDirFlag()
	fileFormat = strings.ToLower(fileFormat)
	checkFileFormat()
	checkDataDirFlag()
	checkDelimiterFlag()
	checkHasHeader()
	checkAndParseFileOpts()
	setDefaultForNullString()
	validateTargetPassword(cmd)
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
		utils.ErrExit(`Error: required flag "data-dir" not set`)
	}
	if strings.HasPrefix(dataDir, "s3://") {
		s3.ValidateObjectURL(dataDir)
		return
	}
	if !utils.FileOrFolderExists(dataDir) {
		utils.ErrExit("data-dir: %s doesn't exists!!", dataDir)
	}
	dataDirAbs, err := filepath.Abs(dataDir)
	if err != nil {
		utils.ErrExit("unable to resolve absolute path for data-dir(%q): %v", dataDir, err)
	}

	exportDirAbs, err := filepath.Abs(exportDir)
	if err != nil {
		utils.ErrExit("unable to resolve absolute path for export-dir(%q): %v", exportDir, err)
	}

	if strings.HasPrefix(dataDirAbs, exportDirAbs) {
		utils.ErrExit("ERROR: data-dir must be outside the export-dir")
	}
	if dataDir == "." {
		fmt.Println("Note: Using current working directory as data directory")
	}
}

func checkDelimiterFlag() {
	resolvedDelimiter, ok := resolveAndCheckSingleByteChar(delimiter)
	if !ok {
		utils.ErrExit("ERROR: invalid syntax of flag value in --delimiter %s. It should be a valid single-byte value.", delimiter)
	}
	log.Infof("resolved delimiter value: %q", resolvedDelimiter)
	delimiter = resolvedDelimiter
}

func checkHasHeader() {
	if hasHeader && fileFormat != datafile.CSV {
		utils.ErrExit("--has-header flag is only supported for CSV file format")
	}
}

func checkAndParseFileOpts() {
	switch fileFormat {
	case datafile.CSV:
		// setting default values for escape and quote, will be updated if provided in fileOpts flag
		fileOptsMap = map[string]string{
			"escape_char": "\"",
			"quote_char":  "\"",
		}
		if strings.Trim(fileOpts, " ") == "" { // if fileOpts is empty, then return
			return
		}

		keyValuePairs := strings.Split(fileOpts, ",")
		for _, keyValuePair := range keyValuePairs {
			key, value := strings.Split(keyValuePair, "=")[0], strings.Split(keyValuePair, "=")[1]
			key = strings.ToLower(key)
			if !slices.Contains(supportedCsvFileOpts, key) {
				utils.ErrExit("ERROR: %q is not a valid csv file option", key)
			} else {
				resolvedValue, ok := resolveAndCheckSingleByteChar(value)
				if !ok {
					utils.ErrExit("ERROR: invalid syntax of opt '%s=%s' in --file-opts flag. It should be a valid single-byte value.", key, value)
				}
				fileOptsMap[key] = resolvedValue
			}
		}

	case datafile.TEXT:
		if fileOpts != "" {
			utils.ErrExit("ERROR: --file-opts flag is invalid for %q format", fileFormat)
		}
	default:
		if fileOpts != "" {
			panic(fmt.Sprintf("ERROR: --file-opts flag not implemented for %q format\n", fileFormat))
		}
	}

	log.Infof("fileOptsMap: %v", fileOptsMap)
}

func setDefaultForNullString() {
	if nullString == "" {
		switch fileFormat {
		case datafile.CSV:
			nullString = ""
		case datafile.TEXT:
			nullString = "\\N"
		default:
			panic("unsupported file format")
		}
	}
}

// resolves and check the given string is a single byte character
func resolveAndCheckSingleByteChar(value string) (string, bool) {
	if len(value) == 1 {
		return value, true
	}
	resolvedValue, err := strconv.Unquote(`"` + value + `"`)
	if err != nil || len(resolvedValue) != 1 {
		return resolvedValue, false
	}
	return resolvedValue, true
}

// in case of csv file format, escape and quote characters are required to be escaped with
// backslash if there are single quote or backslash provided with E in copy Command
func escapeFileOptsCharsIfRequired() {
	if fileOptsMap["escape_char"] == `'` {
		fileOptsMap["escape_char"] = `\'`
	}

	if fileOptsMap["quote_char"] == `'` {
		fileOptsMap["quote_char"] = `\'`
	}

	if fileOptsMap["escape_char"] == `\` {
		fileOptsMap["escape_char"] = `\\`
	}

	if fileOptsMap["quote_char"] == `\` {
		fileOptsMap["quote_char"] = `\\`
	}

}

func init() {
	importDataCmd.AddCommand(importDataFileCmd)
	registerCommonImportFlags(importDataFileCmd)
	registerImportDataFlags(importDataFileCmd)

	importDataFileCmd.Flags().StringVar(&fileFormat, "format", "csv",
		fmt.Sprintf("supported data file types: %v", supportedFileFormats))

	importDataFileCmd.Flags().StringVar(&delimiter, "delimiter", ",",
		"character used as delimiter in rows of the table(s)")

	importDataFileCmd.Flags().StringVar(&dataDir, "data-dir", "",
		"path to the directory contains data files to import into table(s)")
	err := importDataFileCmd.MarkFlagRequired("data-dir")
	if err != nil {
		utils.ErrExit("mark 'data-dir' flag required: %v", err)
	}

	importDataFileCmd.Flags().StringVar(&fileTableMapping, "file-table-map", "",
		"comma separated list of mapping between file name in '--data-dir' to a table in database.\n"+
			"Note: default will be to import all the files in --data-dir with given format, for eg: table1.csv to import into table1 on target database.")

	importDataFileCmd.Flags().BoolVar(&hasHeader, "has-header", false,
		"true - if first line of data file is a list of columns for rows (default false)\n"+
			"(Note: only works for csv file type)")

	importDataFileCmd.Flags().StringVar(&fileOpts, "file-opts", "",
		`comma separated options for csv file format:
		1. escape_char: escape character (default is double quotes '"')
		2. quote_char: 	character used to quote the values (default double quotes '"')
		for eg: --file-opts "escape_char=\",quote_char=\"" or --file-opts 'escape_char=",quote_char="'`)

	importDataFileCmd.Flags().StringVar(&nullString, "null-string", "",
		`string that represents null value in the data file (default for csv: ""(empty string), for text: '\N')`)
}
