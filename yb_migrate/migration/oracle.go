package migration

import (
	"fmt"
	"io/ioutil"
	"log"
	"os/exec"
	"strings"
	"yb_migrate/migrationutil"
)

func CheckOra2pgInstalled() {
	testCommand := exec.Command("ora2pg", "--version")
	preparedCommand := testCommand.String()

	log.Printf("[DEBUG] Prepared Command is: %s\n", preparedCommand)

	outputbytes, err := testCommand.Output()

	migrationutil.CheckError(err, preparedCommand, "Ora2pg is not installed in the machine", true)
	log.Println("[DEBUG] Command output: ", string(outputbytes))
}

func PrintSourceDBVersion(host string, port string, schema string, user string, password string, dbName string, exportDir string) {
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

func CreateMigrationProject(exportDir string, schemaName string) {
	//Add check if the project/directory already exisits. Delete and Create a new one

	fmt.Println("Creating project directory: ", exportDir+"/"+schemaName)

	createProjectCommand := exec.Command("ora2pg", "--project_base", exportDir, "--init", schemaName)
	databytes, err := createProjectCommand.Output()

	log.Printf("[DEBUG] Prepared Command is: %s\n", createProjectCommand)

	migrationutil.CheckError(err, createProjectCommand.String(), "could not create a migration project in "+exportDir, true)

	fmt.Println(string(databytes))
}

func ExportSchema(host string, port string, schema string, user string, password string, dbName string, exportDir string) {
	projectDirectory := exportDir + "/" + schema
	// currentWorkingDirectory := "cd " + projectDirectory

	configFilePath := projectDirectory + "/config/ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, host, port, user, password, schema, dbName)

	// exportTableSchemaCommandString := fmt.Sprintf("ora2pg -p -t TABLE -o table.sql -b %s/schema/tables/ -c %s",
	// 	projectDirectory, configFilePath)

	exportObjects := []string{"TABLE", "VIEW"}

	for _, exportObject := range exportObjects {
		exportSchemaObjectCommandString := fmt.Sprintf("ora2pg -p -t %s -o %s -b %s/schema/tables/ -c %s",
			exportObject, strings.ToLower(exportObject)+".sql", projectDirectory, configFilePath)

		exportSchemaObjectCommand := exec.Command("/bin/bash", "-c", exportSchemaObjectCommandString)
		fmt.Printf("[Debug] exportSchemaObjectCommand: %s\n", exportSchemaObjectCommand.String())
		err := exportSchemaObjectCommand.Run()

		migrationutil.CheckError(err, exportSchemaObjectCommand.String(), "Exporting table couldn't happen", false)
	}

	// stdout, err := exportTableSchemaCommand.StdoutPipe()
	// exportTableSchemaCommand.Start()

	// scanner := bufio.NewScanner(stdout)
	// scanner.Split(bufio.ScanWords)
	// for scanner.Scan() {
	// 	m := scanner.Text()
	// 	fmt.Printf(m + " ")
	// }
	// exportTableSchemaCommand.Wait()

}

func populateOra2pgConfigFile(configFilePath string, host string, port string, user string, password string, schema string, dbName string) {
	sourceDSN := "dbi:Oracle:host=" + host + ";service_name=" + dbName + ";port=" + port

	input, err := ioutil.ReadFile(configFilePath)

	migrationutil.CheckError(err, "Not able to read the config file", "", true)

	lines := strings.Split(string(input), "\n")

	for i, line := range lines {
		// fmt.Printf("[Debug]: %d %s\n", i, line)
		if strings.HasPrefix(line, "ORACLE_DSN") {
			lines[i] = "ORACLE_DSN	" + sourceDSN
		} else if strings.HasPrefix(line, "ORACLE_USER") {
			fmt.Println(line)
			lines[i] = "ORACLE_USER	" + user
		} else if strings.HasPrefix(line, "ORACLE_PWD") {
			lines[i] = "ORACLE_PWD	" + password
		} else if strings.HasPrefix(line, "SCHEMA") {
			lines[i] = "SCHEMA	" + schema
		}
		// else if strings.HasPrefix(line, "TYPE") {
		// 	fmt.Println(line)
		// 	lines[i] = "TYPE	" + "TABLE VIEW TYPE" //all the database objects to export
		// }

	}

	output := strings.Join(lines, "\n")
	err = ioutil.WriteFile(configFilePath, []byte(output), 0644)

	migrationutil.CheckError(err, "Not able to update the config file", "", true)
}
