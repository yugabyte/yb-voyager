package migration

import (
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"yb_migrate/src/utils"
)

//[ALTERNATE WAY] Use select banner from v$version; from oracle database to get version
func PrintOracleSourceDBVersion(source *utils.Source, exportDir string) {
	sourceDSN := getSourceDSN(source)

	testDBVersionCommandString := fmt.Sprintf("ora2pg -t SHOW_VERSION --source \"%s\" --user %s --password %s;",
		sourceDSN, source.User, source.Password)

	testDBVersionCommand := exec.Command("/bin/bash", "-c", testDBVersionCommandString)

	fmt.Printf("[Debug]: Test oracle version command: %s\n", testDBVersionCommandString)

	dbVersionBytes, err := testDBVersionCommand.Output()

	utils.CheckError(err, testDBVersionCommand.String(), string(dbVersionBytes), true)

	fmt.Printf("DB Version: %s\n", string(dbVersionBytes))
}

func Ora2PgExtractSchema(source *utils.Source, exportDir string) {
	projectDirPath := exportDir //utils.GetProjectDirPath(source, exportDir)

	//[Internal]: Decide whether to keep ora2pg.conf file hidden or not
	configFilePath := projectDirPath + "/metainfo/schema/ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, source)

	exportObjectList := utils.GetSchemaObjectList(source.DBType)

	for _, exportObject := range exportObjectList {
		exportObjectFileName := strings.ToLower(exportObject) + ".sql"
		exportObjectDirName := strings.ToLower(exportObject) + "s"
		exportObjectDirPath := projectDirPath + "/schema/" + exportObjectDirName

		var exportSchemaObjectCommand *exec.Cmd
		if source.DBType == "oracle" {
			exportSchemaObjectCommand = exec.Command("ora2pg", "-p", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath)
		} else if source.DBType == "mysql" {
			//TODO: Test if -p flag is required or not here
			exportSchemaObjectCommand = exec.Command("ora2pg", "-m", "-t", exportObject, "-o",
				exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath)
		}

		exportSchemaObjectCommand.Stdout = os.Stdout
		exportSchemaObjectCommand.Stderr = os.Stderr

		fmt.Printf("[Debug] exportSchemaObjectCommand: %s\n", exportSchemaObjectCommand.String())
		err := exportSchemaObjectCommand.Run()

		//TODO: Maybe we can suggest some smart HINT for the error happenend here
		utils.CheckError(err, exportSchemaObjectCommand.String(),
			"Exporting of "+exportObject+" didn't happen, Retry exporting the schema", false)

		if err == nil {
			fmt.Printf("Export of %s schema done...\n", exportObject)
		}
	}
}

//go:embed data/sample-ora2pg.conf
var SampleOra2pgConfigFile string

func populateOra2pgConfigFile(configFilePath string, source *utils.Source) {
	sourceDSN := getSourceDSN(source)

	lines := strings.Split(string(SampleOra2pgConfigFile), "\n")

	//TODO: Add support for SSL Enable Connections
	for i, line := range lines {
		// fmt.Printf("[Debug]: %d %s\n", i, line)
		if strings.HasPrefix(line, "ORACLE_DSN") {
			lines[i] = "ORACLE_DSN	" + sourceDSN
		} else if strings.HasPrefix(line, "ORACLE_USER") {
			// fmt.Println(line)
			lines[i] = "ORACLE_USER	" + source.User
		} else if strings.HasPrefix(line, "ORACLE_PWD") {
			lines[i] = "ORACLE_PWD	" + source.Password
		} else if source.DBType == "oracle" && strings.HasPrefix(line, "SCHEMA") {
			lines[i] = "SCHEMA	" + source.Schema
		} else if strings.HasPrefix(line, "PARALLEL_TABLES") {
			lines[i] = "PARALLEL_TABLES " + strconv.Itoa(source.NumConnections)
		}
	}

	output := strings.Join(lines, "\n")
	err := ioutil.WriteFile(configFilePath, []byte(output), 0644)

	utils.CheckError(err, "Not able to update the config file", "", true)
}

func Ora2PgExportDataOffline(source *utils.Source, exportDir string) {
	utils.CheckToolsRequiredInstalledOrNot(source.DBType)

	utils.CheckSourceDbAccessibility(source)

	utils.CreateMigrationProjectIfNotExists(source, exportDir)

	projectDirPath := exportDir //utils.GetProjectDirPath(source, exportDir)

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

	exportDataCommand.Stdout = os.Stdout
	exportDataCommand.Stderr = os.Stderr

	err := exportDataCommand.Run()
	utils.CheckError(err, exportDataCommandString,
		"Exporting of data failed, retry exporting it", false)

	if err == nil {
		fmt.Printf("Data export complete...\n")
	}
}

func getSourceDSN(source *utils.Source) string {
	var sourceDSN string

	if source.DBType == "oracle" {
		sourceDSN = "dbi:Oracle:" + "host=" + source.Host + ";service_name=" +
			source.DBName + ";port=" + source.Port
	} else if source.DBType == "mysql" {
		sourceDSN = "dbi:mysql:" + "host=" + source.Host + ";database=" +
			source.DBName + ";port=" + source.Port
	} else {
		fmt.Println("Invalid Source DB Type!!")
		os.Exit(1)

	}

	return sourceDSN
}
