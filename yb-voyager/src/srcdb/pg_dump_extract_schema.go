package srcdb

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func pgdumpExtractSchema(schemaList string, connectionUri string, exportDir string) {
	fmt.Printf("exporting the schema %10s", "")
	go utils.Wait("done\n", "error\n")

	pgDumpPath, err := GetAbsPathOfPGCommand("pg_dump")
	if err != nil {
		utils.ErrExit("could not get absolute path of pg_dump command: %v", err)
	}
	cmd := fmt.Sprintf(`%s "%s" --schema-only --schema "%s" --no-owner -f %s --no-privileges --no-tablespaces`, pgDumpPath,
		connectionUri, schemaList, filepath.Join(exportDir, "temp", "schema.sql"))
	log.Infof("Running command: %s", cmd)
	preparedPgdumpCommand := exec.Command("/bin/bash", "-c", cmd)

	stdout, err := preparedPgdumpCommand.CombinedOutput()
	//pg_dump formats its stdout messages, %s is sufficient.
	if string(stdout) != "" {
		log.Infof("%s", string(stdout))
	}
	if err != nil {
		utils.WaitChannel <- 1
		<-utils.WaitChannel
		utils.ErrExit("data export unsuccessful: %v", err)
	}

	//Parsing the single file to generate multiple database object files
	parseSchemaFile(exportDir)

	log.Info("Export of schema completed.")
	utils.WaitChannel <- 0
	<-utils.WaitChannel
}

//NOTE: This is for case when --schema-only option is provided with pg_dump[Data shouldn't be there]
func parseSchemaFile(exportDir string) {
	log.Info("Begun parsing the schema file.")
	schemaFilePath := filepath.Join(exportDir, "temp", "schema.sql")
	schemaDirPath := filepath.Join(exportDir, "schema")
	schemaFileData, err := ioutil.ReadFile(schemaFilePath)
	if err != nil {
		utils.ErrExit("Failed to read file %q: %v", schemaFilePath, err)
	}

	schemaFileLines := strings.Split(string(schemaFileData), "\n")
	numLines := len(schemaFileLines)

	sessionVariableStartPattern := regexp.MustCompile("-- Dumped by pg_dump.*")

	// For example: -- Name: address address_city_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
	sqlTypeInfoCommentPattern := regexp.MustCompile("--.*Type:.*")

	// map to store the sql statements for each db object type
	// map's key are based on the elements of 'utils.postgresSchemaObjectList' array
	objSqlStmts := make(map[string]*strings.Builder)

	// initialize the map
	pgObjList := utils.GetSchemaObjectList("postgresql")
	for _, objType := range pgObjList {
		objSqlStmts[objType] = &strings.Builder{}
	}

	var uncategorizedSqls, setSessionVariables strings.Builder

	var isPossibleFlag bool = true
	for i := 0; i < numLines; i++ {
		if sqlTypeInfoCommentPattern.MatchString(schemaFileLines[i]) {
			sqlType := extractSqlTypeFromSqlInfoComment(schemaFileLines[i])

			i += 2 // jumping to start of sql statement
			sqlStatement := extractSqlStatements(schemaFileLines, &i)

			// TODO: TABLESPACE
			switch sqlType {
			case "SCHEMA", "TYPE", "DOMAIN", "SEQUENCE", "INDEX", "RULE", "FUNCTION",
				"AGGREGATE", "PROCEDURE", "VIEW", "TRIGGER", "EXTENSION", "COMMENT":
				objSqlStmts[sqlType].WriteString(sqlStatement)
			case "TABLE", "DEFAULT", "CONSTRAINT", "FK CONSTRAINT", "TABLE ATTACH":
				objSqlStmts["TABLE"].WriteString(sqlStatement)
			case "MATERIALIZED VIEW":
				objSqlStmts["MVIEW"].WriteString(sqlStatement)
			default:
				uncategorizedSqls.WriteString(sqlStatement)
			}
		} else if isPossibleFlag && sessionVariableStartPattern.MatchString(schemaFileLines[i]) {
			i++

			setSessionVariables.WriteString("-- setting variables for current session")
			sqlStatement := extractSqlStatements(schemaFileLines, &i)
			setSessionVariables.WriteString(sqlStatement)

			isPossibleFlag = false
		}
	}

	for objType, sqlStmts := range objSqlStmts {
		if sqlStmts.Len() == 0 { // create .sql file only if there are DDLs
			continue
		}
		filePath := utils.GetObjectFilePath(schemaDirPath, objType)
		dataBytes := []byte(setSessionVariables.String() + sqlStmts.String())

		err := ioutil.WriteFile(filePath, dataBytes, 0644)
		if err != nil {
			utils.ErrExit("Failed to create sql file for for %q: %v", objType, err)
		}
	}

	if uncategorizedSqls.Len() > 0 {
		filePath := filepath.Join(schemaDirPath, "uncategorized.sql")
		// TODO: add it to the analyze-schema report in case of postgresql
		utils.PrintAndLog("Some uncategorized sql statements are present in %q, Needs to review and import them manually!!", filePath)
		ioutil.WriteFile(filePath, []byte(setSessionVariables.String()+uncategorizedSqls.String()), 0644)
	}
}

func extractSqlTypeFromSqlInfoComment(sqlInfoComment string) string {
	sqlInfoCommentSlice := strings.Split(sqlInfoComment, "; ")
	var sqlType strings.Builder
	for _, info := range sqlInfoCommentSlice {
		if info[:4] == "Type" {
			typeInfo := strings.Split(info, ": ")
			typeInfoValue := typeInfo[1]

			for i := 0; i < len(typeInfoValue) && typeInfoValue[i] != ';'; i++ {
				sqlType.WriteByte(typeInfoValue[i])
			}
		}
	}

	return sqlType.String()
}

func extractSqlStatements(schemaFileLines []string, index *int) string {
	var sqlStatement strings.Builder
	for (*index) < len(schemaFileLines) {
		if isSqlComment(schemaFileLines[(*index)]) {
			break
		} else if shouldSkipLine(schemaFileLines[(*index)]) {
			(*index)++
			continue
		} else {
			sqlStatement.WriteString(schemaFileLines[(*index)] + "\n")
			(*index)++
		}
	}
	return sqlStatement.String()
}

func isSqlComment(line string) bool {
	return len(line) >= 2 && line[:2] == "--"
}

func shouldSkipLine(line string) bool {
	return strings.HasPrefix(line, "SET default_table_access_method")
}
