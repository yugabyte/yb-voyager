package migration

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"yb_migrate/migrationutil"
)

//check for ora2pg and psql/ysqlsh installed
func CheckOracleToolsInstalled() {
	testCommand := exec.Command("ora2pg", "--version")
	preparedCommand := testCommand.String()

	log.Printf("[DEBUG] Prepared Command is: %s\n", preparedCommand)

	outputbytes, err := testCommand.Output()

	migrationutil.CheckError(err, preparedCommand, "Ora2pg is not installed in the machine", true)
	log.Println("[DEBUG] Command output: ", string(outputbytes))
}

//[ALTERNATE WAY] Use select banner from v$version; from oracle database to get version
func PrintOracleSourceDBVersion(source *migrationutil.Source, exportDir string) {
	sourceDSN := "dbi:Oracle:host=" + source.Host + ";service_name=" + source.DBName +
		";port=" + source.Port

	testDBVersionCommandString := fmt.Sprintf("ora2pg -t SHOW_VERSION --source \"%s\" --user %s --password %s;",
		sourceDSN, source.User, source.Password)

	testDBVersionCommand := exec.Command("/bin/bash", "-c", testDBVersionCommandString)

	fmt.Printf("[Debug]: Test oracle version command: %s\n", testDBVersionCommandString)

	dbVersionBytes, err := testDBVersionCommand.Output()

	migrationutil.CheckError(err, testDBVersionCommand.String(), string(dbVersionBytes), true)

	fmt.Printf("DB Version: %s\n", string(dbVersionBytes))
}

func ExportOracleSchema(source *migrationutil.Source, exportDir string, projectDirName string) {
	projectDirPath := migrationutil.GetProjectDirPath(source, exportDir)

	//[Internal]: Decide whether to keep ora2pg.conf file hidden or not
	configFilePath := projectDirPath + "/temp/.ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, source)

	//Currently Missing: PARTITION, TABLESPACE, MVIEWs, PACKAGEs(exported as schema)
	exportObjects := []string{"TABLE", "VIEW", "TYPE", "TRIGGER", "FUNCTION", "PROCEDURE", "SEQUENCE", "GRANT"}

	for _, exportObject := range exportObjects {
		exportObjectFileName := strings.ToLower(exportObject) + ".sql"
		exportObjectDirName := strings.ToLower(exportObject) + "s"
		exportSchemaObjectCommandString := fmt.Sprintf("ora2pg -p -t %s -o %s -b %s/schema/%s/ -c %s",
			exportObject, exportObjectFileName, projectDirPath, exportObjectDirName, configFilePath)

		exportSchemaObjectCommand := exec.Command("/bin/bash", "-c", exportSchemaObjectCommandString)

		fmt.Printf("[Debug] exportSchemaObjectCommand: %s\n", exportSchemaObjectCommand.String())
		err := exportSchemaObjectCommand.Run()

		//TODO: Maybe we can suggest some smart HINT for the error happenend here
		migrationutil.CheckError(err, exportSchemaObjectCommand.String(),
			"Exporting "+exportObject+" didn't happen, Retry exporting the schema", false)

		if err == nil {
			fmt.Printf("Export of %s schema done...\n", exportObjectDirName)
		}

	}
}

//go:embed data/sample-ora2pg.conf
var sampleOra2pgConfigFile string

func populateOra2pgConfigFile(configFilePath string, source *migrationutil.Source) {
	sourceDSN := "dbi:Oracle:host=" + source.Host + ";service_name=" +
		source.DBName + ";port=" + source.Port

	lines := strings.Split(string(sampleOra2pgConfigFile), "\n")

	for i, line := range lines {
		// fmt.Printf("[Debug]: %d %s\n", i, line)
		if strings.HasPrefix(line, "ORACLE_DSN") {
			lines[i] = "ORACLE_DSN	" + sourceDSN
		} else if strings.HasPrefix(line, "ORACLE_USER") {
			// fmt.Println(line)
			lines[i] = "ORACLE_USER	" + source.User
		} else if strings.HasPrefix(line, "ORACLE_PWD") {
			lines[i] = "ORACLE_PWD	" + source.Password
		} else if strings.HasPrefix(line, "SCHEMA") {
			lines[i] = "SCHEMA	" + source.Schema
		}
		// else if strings.HasPrefix(line, "TYPE") {
		// 	lines[i] = "TYPE	" + "TABLE VIEW TYPE" //all the database objects to export
		// }
	}

	output := strings.Join(lines, "\n")
	err := ioutil.WriteFile(configFilePath, []byte(output), 0644)

	migrationutil.CheckError(err, "Not able to update the config file", "", true)
}

//Using ora2pg tool
func OracleExportDataOffline(source *migrationutil.Source, exportDir string) {
	CheckOracleToolsInstalled()

	migrationutil.CheckSourceDbAccessibility(source)

	projectDirPath := migrationutil.GetProjectDirPath(source, exportDir)

	//[Internal]: Decide where to keep it
	configFilePath := projectDirPath + "/temp/.ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, source)

	exportDataCommandString := fmt.Sprintf("ora2pg -t COPY -o data.sql -b %s/data -c %s",
		projectDirPath, configFilePath)

	//TODO: Exporting tables provided in tablelist
	//TODO: use some number of jobs by default or as provided by the user

	//Exporting all the tables in the schema
	exportDataCommand := exec.Command("/bin/bash", "-c", exportDataCommandString)
	fmt.Printf("[Debug] exportDataCommand: %s\n", exportDataCommandString)

	err := exportDataCommand.Run()
	migrationutil.CheckError(err, exportDataCommandString,
		"Exporting of data failed, retry exporting it", false)

	if err == nil {
		fmt.Printf("Data export complete...\n")
	}
}
