package srcdb

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func getExportedRowCount(tablesMetadata map[string]*utils.TableProgressMetadata) map[string]int64 {
	exportedRowCount := make(map[string]int64)
	for key := range tablesMetadata {
		tableMetadata := tablesMetadata[key]
		targetTableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		exportedRowCount[targetTableName] = tableMetadata.CountLiveRows

	}
	return exportedRowCount
}

// Invoked at the end of export schema for Oracle and MySQL to process files containing statments of the type `\i <filename>.sql`, merging them together.
func processImportDirectives(fileName string) error {
	if _, err := os.Stat(fileName); os.IsNotExist(err) {
		return nil
	}
	// Create a temporary file after appending .tmp extension to the fileName.
	tmpFileName := fileName + ".tmp"
	tmpFile, err := os.Create(tmpFileName)
	if err != nil {
		return fmt.Errorf("create %q: %w", tmpFileName, err)
	}
	defer tmpFile.Close()
	// Open the original file for reading.
	file, err := os.Open(fileName)
	if err != nil {
		return fmt.Errorf("open %q: %w", fileName, err)
	}
	defer file.Close()
	// Create a new scanner and read the file line by line.
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.Trim(scanner.Text(), " ")
		// Check if the line contains the import directive.
		if strings.HasPrefix(line, "\\i ") {
			// Split the line into tokens.
			tokens := strings.Split(line, " ")
			// Check if the line contains the correct number of tokens.
			if len(tokens) != 2 {
				return fmt.Errorf("invalid number of tokens in line: %s", line)
			}
			// Check if the file exists.
			importFileName := strings.Trim(tokens[1], "'")
			log.Infof("Inlining contents of %q in %q", importFileName, fileName)
			if _, err = os.Stat(importFileName); err != nil {
				return fmt.Errorf("error while opening file %s: %v", importFileName, err)
			}
			// Read the file and append its contents to the temporary file.
			importFile, err := os.Open(importFileName)
			if err != nil {
				return fmt.Errorf("open %q: %w", importFileName, err)
			}
			defer importFile.Close()
			_, err = io.Copy(tmpFile, importFile)
			if err != nil {
				return fmt.Errorf("append %q to %q: %w", importFileName, tmpFileName, err)
			}
		} else {
			// Write the line to the temporary file.
			_, err = tmpFile.WriteString(line)
			if err != nil {
				return fmt.Errorf("write a line to %q: %w", tmpFileName, err)
			}
		}
	}
	// Check if there were any errors during the scan.
	if err = scanner.Err(); err != nil {
		return fmt.Errorf("scan %q: %w", fileName, err)
	}
	// Rename tmpFile to fileName.
	err = os.Rename(tmpFileName, fileName)
	if err != nil {
		return fmt.Errorf("rename %q as %q: %w", tmpFileName, fileName, err)
	}
	return nil
}
