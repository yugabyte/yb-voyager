package srcdb

import (
	"fmt"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func pgdumpExtractSchema(source *Source, exportDir string) {
	fmt.Printf("exporting the schema %10s", "")
	go utils.Wait("done\n", "error\n")
	SSLQueryString := generateSSLQueryStringIfNotExists(source)
	preparePgdumpCommandString := ""

	if source.Uri != "" {
		preparePgdumpCommandString = fmt.Sprintf(`pg_dump "%s" --schema-only --no-owner -f %s/temp/schema.sql`, source.Uri, exportDir)
	} else {
		preparePgdumpCommandString = fmt.Sprintf(`pg_dump "postgresql://%s:%s@%s:%d/%s?%s" --schema-only --no-owner -f %s/temp/schema.sql`, source.User, source.Password, source.Host,
			source.Port, source.DBName, SSLQueryString, exportDir)
	}

	log.Infof("Running command: %s", preparePgdumpCommandString)
	preparedYsqldumpCommand := exec.Command("/bin/bash", "-c", preparePgdumpCommandString)

	stdout, err := preparedYsqldumpCommand.CombinedOutput()
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
	parseSchemaFile(source, exportDir)

	log.Info("Export of schema completed.")
	utils.WaitChannel <- 0
	<-utils.WaitChannel
}

//NOTE: This is for case when --schema-only option is provided with pg_dump[Data shouldn't be there]
func parseSchemaFile(source *Source, exportDir string) {
	log.Info("Begun parsing the schema file.")
	schemaFilePath := exportDir + "/temp" + "/schema.sql"
	schemaDirPath := exportDir + "/schema"
	schemaFileData, err := ioutil.ReadFile(schemaFilePath)
	if err != nil {
		utils.ErrExit("Failed to read file %q: %v", schemaFilePath, err)
	}

	schemaFileLines := strings.Split(string(schemaFileData), "\n")
	numLines := len(schemaFileLines)

	sessionVariableStartPattern := regexp.MustCompile("-- Dumped by pg_dump.*")

	//For example: -- Name: address address_city_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
	sqlTypeInfoCommentPattern := regexp.MustCompile("--.*Type:.*")

	var createTableSqls, createFunctionSqls, createTriggerSqls,
		createIndexSqls, createTypeSqls, createSequenceSqls, createDomainSqls,
		createRuleSqls, createAggregateSqls, createViewSqls, createMatViewSqls, uncategorizedSqls,
		createSchemaSqls, createExtensionSqls, createProcedureSqls, setSessionVariables strings.Builder

	var isPossibleFlag bool = true
	for i := 0; i < numLines; i++ {
		if sqlTypeInfoCommentPattern.MatchString(schemaFileLines[i]) {
			sqlType := extractSqlTypeFromSqlInfoComment(schemaFileLines[i])

			i += 2 //jumping to start of sql statement
			sqlStatement := extractSqlStatements(schemaFileLines, &i)

			//Missing: PARTITION, PROCEDURE, MVIEW, TABLESPACE, ROLE, GRANT ...
			switch sqlType {
			case "TABLE", "DEFAULT", "CONSTRAINT", "FK CONSTRAINT":
				createTableSqls.WriteString(sqlStatement)
			case "INDEX":
				createIndexSqls.WriteString(sqlStatement)

			case "FUNCTION":
				createFunctionSqls.WriteString(sqlStatement)

			case "PROCEDURE":
				createProcedureSqls.WriteString(sqlStatement)

			case "TRIGGER":
				createTriggerSqls.WriteString(sqlStatement)

			case "TYPE":
				createTypeSqls.WriteString(sqlStatement)
			case "DOMAIN":
				createDomainSqls.WriteString(sqlStatement)

			case "AGGREGATE":
				createAggregateSqls.WriteString(sqlStatement)
			case "RULE":
				createRuleSqls.WriteString(sqlStatement)
			case "SEQUENCE":
				createSequenceSqls.WriteString(sqlStatement)
			case "VIEW":
				createViewSqls.WriteString(sqlStatement)
			case "MATERIALIZED VIEW":
				createMatViewSqls.WriteString(sqlStatement)
			case "SCHEMA":
				createSchemaSqls.WriteString(sqlStatement)
			case "EXTENSION":
				createExtensionSqls.WriteString(sqlStatement)
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

	//TODO: convert below code into a for-loop

	//writing to .sql files in project
	ioutil.WriteFile(schemaDirPath+"/tables/table.sql", []byte(setSessionVariables.String()+createTableSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/tables/INDEXES_table.sql", []byte(setSessionVariables.String()+createIndexSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/functions/function.sql", []byte(setSessionVariables.String()+createFunctionSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/procedures/procedure.sql", []byte(setSessionVariables.String()+createProcedureSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/triggers/trigger.sql", []byte(setSessionVariables.String()+createTriggerSqls.String()), 0644)

	ioutil.WriteFile(schemaDirPath+"/types/type.sql", []byte(setSessionVariables.String()+createTypeSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/domains/domain.sql", []byte(setSessionVariables.String()+createDomainSqls.String()), 0644)

	ioutil.WriteFile(schemaDirPath+"/aggregates/aggregate.sql", []byte(setSessionVariables.String()+createAggregateSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/rules/rule.sql", []byte(setSessionVariables.String()+createRuleSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/sequences/sequence.sql", []byte(setSessionVariables.String()+createSequenceSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/views/view.sql", []byte(setSessionVariables.String()+createViewSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/mviews/mview.sql", []byte(setSessionVariables.String()+createMatViewSqls.String()), 0644)

	if uncategorizedSqls.Len() > 0 {
		ioutil.WriteFile(schemaDirPath+"/uncategorized.sql", []byte(setSessionVariables.String()+uncategorizedSqls.String()), 0644)
	}

	ioutil.WriteFile(schemaDirPath+"/schemas/schema.sql", []byte(setSessionVariables.String()+createSchemaSqls.String()), 0644)
	ioutil.WriteFile(schemaDirPath+"/extensions/extension.sql", []byte(setSessionVariables.String()+createExtensionSqls.String()), 0644)

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
		if isSqlComment(schemaFileLines[(*index)]) || isPG13statement(schemaFileLines[(*index)]) {
			break
		} else {
			sqlStatement.WriteString(schemaFileLines[(*index)] + "\n")
		}

		(*index)++
	}
	return sqlStatement.String()
}

func isSqlComment(line string) bool {
	return len(line) >= 2 && line[:2] == "--"
}

func isPG13statement(line string) bool {
	return strings.HasPrefix(line, "SET default_table_access_method")
}
