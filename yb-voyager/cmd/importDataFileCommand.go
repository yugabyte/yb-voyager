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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	fileType            string
	delimiter           string
	dataDir             string
	fileTableMapping    string
	hasHeader           bool
	tableNameVsFilePath = make(map[string]string)
	supportedFileTypes  = []string{datafile.CSV}
)

var importDataFileCmd = &cobra.Command{
	Use:   "file",
	Short: "This command imports data from given files into YugabyteDB database",

	Run: func(cmd *cobra.Command, args []string) {
		checkImportDataFileFlags()
		parseFileTableMapping()
		prepareForImportDataCmd()
		importData()
	},
}

func prepareForImportDataCmd() {
	sourceDBType = ORACLE // dummy value - this command is not affected by it
	CreateMigrationProjectIfNotExists(sourceDBType, exportDir)
	tableFileSize := getFileSizeInfo()
	dfd := &datafile.Descriptor{
		FileType:      fileType,
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
		if fileType == datafile.CSV {
			if hasHeader {
				df, err := datafile.OpenDataFile(filePath, dataFileDescriptor)
				if err != nil {
					utils.ErrExit("opening datafile %q to prepare copy command: %v", err)
				}
				copyTableFromCommands[table] = fmt.Sprintf(`COPY %s(%s) FROM STDIN DELIMITER '%c' CSV HEADER`, table, df.GetHeader(), []rune(delimiter)[0])
			} else {
				copyTableFromCommands[table] = fmt.Sprintf(`COPY %s FROM STDIN DELIMITER '%c' CSV`, table, []rune(delimiter)[0])
			}
		} else {
			panic(fmt.Sprintf("File Type %q not implemented\n", fileType))
		}
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
	var err error
	delimiter, err = strconv.Unquote(`"` + delimiter + `"`)
	if err != nil || len(delimiter) > 1 {
		utils.ErrExit("Invalid sytax of flag value in --delimiter %q. It should be a valid single-byte value.", delimiter)
	}
	log.Infof("resolved delimiter value: %q", delimiter)
}

func init() {
	importDataCmd.AddCommand(importDataFileCmd)
	registerCommonImportFlags(importDataFileCmd)

	importDataFileCmd.Flags().StringVar(&fileType, "file-type", "csv",
		fmt.Sprintf("supported data file types: %s", supportedFileTypes))

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
}
