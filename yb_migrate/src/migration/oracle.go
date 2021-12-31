package migration

import (
	"context"
	_ "embed"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"yb_migrate/src/utils"
)

//[ALTERNATE WAY] Use select banner from v$version; from oracle database to get version
func PrintOracleSourceDBVersion(source *utils.Source, exportDir string) {
	sourceDSN := getSourceDSN(source)

	testDBVersionCommandString := fmt.Sprintf("ora2pg -t SHOW_VERSION --source \"%s\" --user %s --password %s",
		sourceDSN, source.User, source.Password)

	testDBVersionCommand := exec.Command("/bin/bash", "-c", testDBVersionCommandString)

	fmt.Printf("[Debug]: Test oracle version command: %s\n", testDBVersionCommandString)

	dbVersionBytes, err := testDBVersionCommand.CombinedOutput()

	utils.CheckError(err, testDBVersionCommand.String(), string(dbVersionBytes), true)

	fmt.Printf("DB Version: %s\n", string(dbVersionBytes))
}

func Ora2PgExtractSchema(source *utils.Source, exportDir string) {
	projectDirPath := exportDir //utils.GetProjectDirPath(source, exportDir)

	//[Internal]: Decide whether to keep ora2pg.conf file hidden or not
	configFilePath := projectDirPath + "/temp/.ora2pg.conf"
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

func Ora2PgExportDataOffline(ctx context.Context, source *utils.Source, exportDir string, quitChan chan bool, exportDataStart chan bool) {
	defer utils.WaitGroup.Done()

	utils.CheckToolsRequiredInstalledOrNot(source.DBType)

	utils.CheckSourceDbAccessibility(source)

	utils.CreateMigrationProjectIfNotExists(source, exportDir)

	projectDirPath := exportDir

	//TODO: Decide where to keep this
	configFilePath := projectDirPath + "/temp/.ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, source)

	exportDataCommandString := fmt.Sprintf("ora2pg -t COPY -o data.sql -b %s/data -c %s",
		projectDirPath, configFilePath)

	//TODO: Exporting only those tables provided in tablelist

	//Exporting all the tables in the schema
	exportDataCommand := exec.Command("/bin/bash", "-c", exportDataCommandString)
	// log.Debugf("exportDataCommand: %s", exportDataCommandString)

	stdOutFile, err := os.OpenFile(exportDir+"/temp/export-data-stdout", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer stdOutFile.Close()

	stdErrFile, err := os.OpenFile(exportDir+"/temp/export-data-stderr", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer stdErrFile.Close()

	exportDataCommand.Stdout = stdOutFile
	exportDataCommand.Stderr = stdErrFile

	err = exportDataCommand.Start()
	fmt.Println("ora2pg for data export started")
	if err != nil {
		quitChan <- true
		runtime.Goexit()
	}

	exportDataStart <- true

	err = exportDataCommand.Wait()
	if err != nil {
		quitChan <- true
		runtime.Goexit()
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
