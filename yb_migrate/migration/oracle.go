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

//[ALTERNATE WAY] Use select banner from v$version; from oracle database
func PrintOracleSourceDBVersion(host string, port string, schema string, user string, password string, dbName string, exportDir string) {
	projectDirectory := exportDir + "/" + schema
	currentDirectory := "cd " + projectDirectory + ";"
	sourceDSN := "dbi:Oracle:host=" + host + ";service_name=" + dbName + ";port=" + port

	testDBVersionCommandString := fmt.Sprintf(currentDirectory+"ora2pg -t SHOW_VERSION --source \"%s\" --user %s --password %s;",
		sourceDSN, user, password)

	testVersionCommand := exec.Command("/bin/bash", "-c", testDBVersionCommandString)

	dbVersionBytes, err := testVersionCommand.Output()

	migrationutil.CheckError(err, testVersionCommand.String(), "Couldn't export schema", true)

	fmt.Printf("[Debug] Output of export schema script: %s and %s\n", testVersionCommand.String(), dbVersionBytes)
	fmt.Println("DB Version: ", string(dbVersionBytes))
}

// func CreateOracleMigrationProject(exportDir string, schemaName string) {
// 	//Add check if the project/directory already exisits. Delete and Create a new one

// 	fmt.Println("Creating project directory: ", exportDir+"/"+schemaName)

// 	createProjectCommand := exec.Command("ora2pg", "--project_base", exportDir, "--init", schemaName)
// 	databytes, err := createProjectCommand.Output()

// 	log.Printf("[DEBUG] Prepared Command is: %s\n", createProjectCommand)

// 	migrationutil.CheckError(err, createProjectCommand.String(), "could not create a migration project in "+exportDir, true)

// 	fmt.Println(string(databytes))
// }

func ExportOracleSchema(host string, port string, schema string, user string, password string, dbName string, exportDir string, projectDirName string) {
	projectDirPath := exportDir + "/" + projectDirName

	//[Internal]: Decide whether to keep ora2pg.conf file hidden or not
	configFilePath := projectDirPath + "/temp/.ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, host, port, user, password, schema, dbName)

	exportObjects := []string{"TABLE", "VIEW", "TYPE", "TRIGGER", "FUNCTION", "PROCEDURE"} // "TYPES"

	for _, exportObject := range exportObjects {
		exportObjectFileName := strings.ToLower(exportObject) + ".sql"
		exportObjectDirName := strings.ToLower(exportObject) + "s"
		exportSchemaObjectCommandString := fmt.Sprintf("ora2pg -p -t %s -o %s -b %s/%s/ -c %s",
			exportObject, exportObjectFileName, projectDirPath, exportObjectDirName, configFilePath)

		exportSchemaObjectCommand := exec.Command("/bin/bash", "-c", exportSchemaObjectCommandString)

		fmt.Printf("[Debug] exportSchemaObjectCommand: %s\n", exportSchemaObjectCommand.String())
		err := exportSchemaObjectCommand.Run()

		//TODO: Maybe we can suggest some smart HINT for the error happenend here
		migrationutil.CheckError(err, exportSchemaObjectCommand.String(),
			"Exporting "+exportObject+" didn't happen, Retry the export schema", false)

		if err != nil {
			fmt.Printf("Export of %s schema done...\n", exportObjectDirName)
		}

	}

	//temporary for testing
	exportDataCommandString := fmt.Sprintf("ora2pg -t %s -o %s -b %s/%s/ -c %s",
		"COPY", "data.sql", projectDirPath, "data", configFilePath)
	exportDataCommand := exec.Command("/bin/bash", "-c", exportDataCommandString)
	fmt.Printf("[Debug] exportDataCommand: %s\n", exportDataCommand.String())

	err := exportDataCommand.Run()
	migrationutil.CheckError(err, exportDataCommand.String(),
		"Exporting of data, didn't happen, Retry the export schema", false)

	if err != nil {
		fmt.Printf("Export of %s schema done...\n", "data")
	}

}

//go:embed data/sample-ora2pg.conf
var sampleOra2pgConfigFile string

func populateOra2pgConfigFile(configFilePath string, host string, port string, user string, password string, schema string, dbName string) {
	sourceDSN := "dbi:Oracle:host=" + host + ";service_name=" + dbName + ";port=" + port

	lines := strings.Split(string(sampleOra2pgConfigFile), "\n")

	for i, line := range lines {
		// fmt.Printf("[Debug]: %d %s\n", i, line)
		if strings.HasPrefix(line, "ORACLE_DSN") {
			lines[i] = "ORACLE_DSN	" + sourceDSN
		} else if strings.HasPrefix(line, "ORACLE_USER") {
			// fmt.Println(line)
			lines[i] = "ORACLE_USER	" + user
		} else if strings.HasPrefix(line, "ORACLE_PWD") {
			lines[i] = "ORACLE_PWD	" + password
		} else if strings.HasPrefix(line, "SCHEMA") {
			lines[i] = "SCHEMA	" + schema
		}
		// else if strings.HasPrefix(line, "TYPE") {
		// 	lines[i] = "TYPE	" + "TABLE VIEW TYPE" //all the database objects to export
		// }
	}

	output := strings.Join(lines, "\n")
	err := ioutil.WriteFile(configFilePath, []byte(output), 0644)

	migrationutil.CheckError(err, "Not able to update the config file", "", true)
}
