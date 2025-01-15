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
package srcdb

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func getExportedDataFileList(tablesMetadata map[string]*utils.TableProgressMetadata) []*datafile.FileEntry {
	fileEntries := make([]*datafile.FileEntry, 0)
	for key := range tablesMetadata {
		tableMetadata := tablesMetadata[key]
		targetTableName := strings.TrimSuffix(filepath.Base(tableMetadata.FinalFilePath), "_data.sql")
		table, err := namereg.NameReg.LookupTableName(targetTableName)
		if err != nil {
			utils.ErrExit("error while looking up table name: %q: %v", targetTableName, err)
		}
		if !utils.FileOrFolderExists(tableMetadata.FinalFilePath) {
			// This can happen in case of nested tables in Oracle.
			log.Infof("File %q does not exist. Not including table %q in the descriptor.",
				tableMetadata.FinalFilePath, targetTableName)
			continue
		}
		fileEntry := &datafile.FileEntry{
			FilePath:  filepath.Base(tableMetadata.FinalFilePath),
			TableName: table.ForKey(),
			RowCount:  tableMetadata.CountLiveRows,
			FileSize:  -1, // Not available.
		}
		fileEntries = append(fileEntries, fileEntry)
	}
	return fileEntries
}

// Invoked at the end of export schema for Oracle and MySQL to process files containing statments of the type `\i <filename>.sql`, merging them together.
func processImportDirectives(fileName string) error {
	if !utils.FileOrFolderExists(fileName) {
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
		line := scanner.Text()
		// Check if the line contains the import directive.
		if strings.HasPrefix(line, "\\i ") {
			// Check if the file exists.
			importFileName := strings.Trim(line[3:], "' ")
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
			_, err = tmpFile.WriteString(line + "\n")
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

// Strip the schema name from the names qualified by the source schema.
//
// ora2pg exports SYNONYM objects as VIEWs. The associated CREATE VIEW DDL statements
// appear in the EXPORT_DIR/schema/synonyms/synonym.sql.
//
// The problem is that the view names are qualified with the Oracle's schema name.
// Unless, the user is importing the schema in an exactly similarly named schema
// on the target, the DDL statement will fail to import.
//
// The following function goes through all the names from the file and replaces
// all occurrences of `sourceSchemaName.objectName` with just `objectName`.
func stripSourceSchemaNames(fileName string, sourceSchema string) error {
	if !utils.FileOrFolderExists(fileName) {
		return nil
	}
	tmpFileName := fileName + ".tmp"
	fileContent, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("reading %q: %w", fileName, err)
	}
	regStr := fmt.Sprintf(`(?i)("?)%s\.([a-zA-Z0-9_"]+?)`, sourceSchema)
	reg := regexp.MustCompile(regStr)
	transformedContent := reg.ReplaceAllString(string(fileContent), "$1$2")
	err = os.WriteFile(tmpFileName, []byte(transformedContent), 0644)
	if err != nil {
		return fmt.Errorf("writing to %q: %w", tmpFileName, err)
	}
	// Rename tmpFile to fileName.
	err = os.Rename(tmpFileName, fileName)
	if err != nil {
		return fmt.Errorf("rename %q as %q: %w", tmpFileName, fileName, err)
	}
	return nil
}

func removeReduntantAlterTable(fileName string) error {
	if !utils.FileOrFolderExists(fileName) {
		return nil
	}
	tmpFileName := fileName + ".tmp"
	fileContent, err := os.ReadFile(fileName)
	if err != nil {
		return fmt.Errorf("reading %q: %w", fileName, err)
	}
	regStr := `(?i)ALTER TABLE ([a-zA-Z0-9_"]+) ALTER COLUMN ([a-zA-Z0-9_"]+) SET NOT NULL;`
	reg := regexp.MustCompile(regStr)
	transformedContent := reg.ReplaceAllString(string(fileContent), "")
	err = os.WriteFile(tmpFileName, []byte(transformedContent), 0644)
	if err != nil {
		return fmt.Errorf("writing to %q: %w", tmpFileName, err)
	}
	// Rename tmpFile to fileName.
	err = os.Rename(tmpFileName, fileName)
	if err != nil {
		return fmt.Errorf("rename %q as %q: %w", tmpFileName, fileName, err)
	}
	return nil
}
