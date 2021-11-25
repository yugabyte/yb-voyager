package migration

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"yb_migrate/migrationutil"
)

// TODO: check for pgdump and psql/ysqlsh - installed/set-in-path
func CheckPostgresToolsInstalled() {
}

// TODO: fill it, how to using the tools? [psql -c "Select * from version()"]
func PrintPostgresSourceDBVersion(host string, port string, schema string, user string, password string, dbName string, exportDir string) {
}

func ExportPostgresSchema(host string, port string, schema string, user string, password string, dbName string, exportDir string, projectDirName string) {
	fmt.Printf("Exporting Postgres schema started...\n")
	projectDirPath := exportDir + "/" + projectDirName

	//There can be other ways to provide the password like PGPASS file or connection URI
	prepareYsqldumpCommandString := fmt.Sprintf("export PGPASSWORD=%s && ysql_dump --dbname %s --host %s --port %s "+
		"--username %s --schema-only > %s/schema.sql", password, dbName, host, port, user, projectDirPath)
	preparedYsqldumpCommand := exec.Command("/bin/bash", "-c", prepareYsqldumpCommandString)

	fmt.Printf("Executing command: %s\n", preparedYsqldumpCommand)

	err := preparedYsqldumpCommand.Run()
	migrationutil.CheckError(err, prepareYsqldumpCommandString, "Retry, dump didn't happen", true)

	//Parsing the single file to generate multiple database object files
	parseSchemaFile(host, port, schema, user, password, dbName, exportDir, projectDirName)
}

//NOTE: This is for case when --schema-only option is provided with ysql_dump[Data shouldn't be there]
func parseSchemaFile(host string, port string, schema string, user string, password string, dbName string, exportDir string, projectDirName string) {
	fmt.Printf("Parsing the schema file...\n")

	projectDirPath := exportDir + "/" + projectDirName
	schemaFilePath := projectDirPath + "/schema.sql"

	//CHOOSE - bufio vs ioutil(Memory vs Performance)?
	schemaFileData, err := ioutil.ReadFile(schemaFilePath)

	migrationutil.CheckErrorSimple(err, "File not read", true)

	schemaFileLines := strings.Split(string(schemaFileData), "\n")
	numLines := len(schemaFileLines)

	//For example -> -- Name: address address_city_id_fkey; Type: FK CONSTRAINT; Schema: public; Owner: postgres
	sqlInfoCommentPattern, err := regexp.Compile("--.*Type:.*")

	migrationutil.CheckErrorSimple(err, "Couldn't generate the schema", true)

	//Naming: createTableSqls vs tableSqls?
	var createTableSqls, createFkConstraintSqls, createFunctionSqls, createTriggerSqls,
		createIndexSqls, createTypeSqls, createSequenceSqls, createDomainSqls,
		createRuleSqls, createAggregateSqls, createViewSqls, uncategorizedSqls strings.Builder

	for i := 0; i < numLines; i++ {
		if sqlInfoCommentPattern.MatchString(schemaFileLines[i]) {
			sqlType := extractSqlTypeFromSqlInfoComment(schemaFileLines[i])

			i += 2 //jumping to start of sql statement
			sqlStatement := extractSqlStatement(schemaFileLines, &i)

			//Missing Cases: PARTITIONs, PROCEDUREs, MVIEWs, TABLESPACEs ...
			switch sqlType {
			case "TABLE", "DEFAULT":
				createTableSqls.WriteString(sqlStatement)
			case "FK CONSTRAINT":
				createFkConstraintSqls.WriteString(sqlStatement)
			case "INDEX":
				createIndexSqls.WriteString(sqlStatement)

			case "FUNCTION":
				createFunctionSqls.WriteString(sqlStatement)
			case "TRIGGER":
				createTriggerSqls.WriteString(sqlStatement)

			case "TYPE", "DOMAIN":
				createTypeSqls.WriteString(sqlStatement)
			// case "DOMAIN":
			// 	createDomainSqls.WriteString(sqlStatement)

			case "AGGREGATE":
				createAggregateSqls.WriteString(sqlStatement)
			case "RULE":
				createRuleSqls.WriteString(sqlStatement)
			case "SEQUENCE":
				createSequenceSqls.WriteString(sqlStatement)
			case "VIEW":
				createViewSqls.WriteString(sqlStatement)
			default:
				uncategorizedSqls.WriteString(sqlStatement)
			}
		}
	}

	//writing to .sql files in project
	ioutil.WriteFile(projectDirPath+"/schema/tables/table.sql", []byte(createTableSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/tables/FKEYS_table.sql", []byte(createFkConstraintSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/tables/INDEXES_table.sql", []byte(createIndexSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/functions/function.sql", []byte(createFunctionSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/triggers/trigger.sql", []byte(createTriggerSqls.String()), 0644)

	//to keep the project structure consistent
	ioutil.WriteFile(projectDirPath+"/schema/types/type.sql", []byte(createTypeSqls.String()+createDomainSqls.String()), 0644)
	// ioutil.WriteFile(projectDirPath+"/schema/types/domain.sql", []byte(createDomainSqls.String()), 0644)

	ioutil.WriteFile(projectDirPath+"/schema/others/aggregate.sql", []byte(createAggregateSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/others/rule.sql", []byte(createRuleSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/sequences/sequence.sql", []byte(createSequenceSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/views/view.sql", []byte(createViewSqls.String()), 0644)
	ioutil.WriteFile(projectDirPath+"/schema/others/uncategorized.sql", []byte(uncategorizedSqls.String()), 0644)

}

func extractSqlStatement(schemaFileLines []string, index *int) string {
	// fmt.Println("extracting sql statement started...")
	var sqlStatement strings.Builder

	for (*index) < len(schemaFileLines) {
		// fmt.Println((*index), " , ", schemaFileLines[(*index)])
		if isSqlComment(schemaFileLines[(*index)]) {
			break
		} else {
			sqlStatement.WriteString(schemaFileLines[(*index)] + "\n")
		}

		(*index)++
	}

	// fmt.Println("extracting sql statement done...")
	return sqlStatement.String()
}

func isSqlComment(line string) bool {
	return len(line) >= 2 && line[:2] == "--"
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

func PostgresExportDataOffline(source *migrationutil.Source, exportDir string) {
	projectDirPath := migrationutil.GetProjectDirPath(source, exportDir)
	dataDirPath := projectDirPath + "/data"

	//using pgdump for exporting data in directory format
	//pg_dump postgresql://postgres:postgres@127.0.0.1:5432/sakila?sslmode=disable --verbose --compress=0 --data-only -Fd -f sakila-data-dir
	pgdumpDataExportCommandString := fmt.Sprintf("pg_dump postgresql://%s:%s@%s:%s/%s?"+
		"sslmode=disable --compress=0 --data-only -Fd -f %s", source.User, source.Password,
		source.Host, source.Port, source.DBName, dataDirPath)

	fmt.Printf("[Debug] Command: %s\n", pgdumpDataExportCommandString)

	pgdumpDataExportCommand := exec.Command("/bin/bash", "-c", pgdumpDataExportCommandString)

	err := pgdumpDataExportCommand.Run()

	migrationutil.CheckError(err, pgdumpDataExportCommandString,
		"Exporting of data failed, retry exporting it", true)

	//TODO: Parse the main toc.dat file. If needed

	//TODO: write the mapping creation function
	requiredMap := getMappingForTableFileNameVsTableName()

	for fileName, tableName := range requiredMap {
		oldFileName := dataDirPath + "/" + fileName + ".dat"
		newFileName := dataDirPath + "/" + tableName + ".sql"
		fmt.Printf("Renaming: %s -> %s\n", fileName+".dat", tableName+".sql")
		os.Rename(oldFileName, newFileName)
	}

	fmt.Printf("Data  export complete... \n")
}

func getMappingForTableFileNameVsTableName() map[string]string {
	requiredMap := map[string]string{
		"4030": "actor",
		"4042": "country",
		"4040": "city",
		"4038": "address",
		"4032": "category",
		"4060": "staff",
		"4062": "store",
		"4044": "customer",
		"4048": "language",
		"4034": "film",
		"4035": "film_actor",
		"4036": "film_category",
		"4046": "inventory",
		"4058": "rental",
		"4050": "payment",
		"4051": "payment_p2007_01",
		"4052": "payment_p2007_02",
		"4053": "payment_p2007_03",
		"4054": "payment_p2007_04",
		"4055": "payment_p2007_05",
		"4056": "payment_p2007_06",
	}

	return requiredMap
}
