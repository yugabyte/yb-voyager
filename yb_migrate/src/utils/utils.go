/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
package utils

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/yosssi/gohtml"
)

var projectSubdirs = []string{"schema", "data", "reports", "metainfo", "metainfo/data", "metainfo/schema", "metainfo/flags", "temp"}

//report.json format
type Report struct {
	Summary Summary `json:"summary"`
	Issues  []Issue `json:"issues"`
}

type Summary struct {
	DBName     string     `json:"dbName"`
	SchemaName string     `json:"schemaName"`
	DBObjects  []DBObject `json:"databaseObjects"`
}

type DBObject struct {
	ObjectType   string `json:"objectType"`
	TotalCount   int    `json:"totalCount"`
	InvalidCount int    `json:"invalidCount"`
	ObjectNames  string `json:"objectNames"`
	Details      string `json:"details"`
}

type Issue struct {
	ObjectType string `json:"objectType"`
	Reason     string `json:"reason"`
	FilePath   string `json:"filePath"`
	GH         string `json:"GH"`
}

func Wait(c chan int) {
	fmt.Print("\033[?25l") // Hide the cursor
	chars := [4]byte{'|', '/', '-', '\\'}
	var i = 0
	for true {
		i++
		select {
		case <-c:
			fmt.Println("\r")
			fmt.Print("\033[?25h") // enable the cursor
			return
		default:
			fmt.Print("\b" + string(chars[i%4]))
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func Readline(r *bufio.Reader) (string, error) {
	var (
		isPrefix bool  = true
		err      error = nil
		line, ln []byte
	)
	for isPrefix && err == nil {
		line, isPrefix, err = r.ReadLine()
		ln = append(ln, line...)
	}
	return string(ln), err
}

func checkSourceEndpointsReachability(host string, port string) {
	sourceEndPointConnectivityCommandString := "nc -z -w 30 " + host + " " + port
	lastCommandExitStatusCommandString := "echo $?"

	finalString := sourceEndPointConnectivityCommandString + "; " + lastCommandExitStatusCommandString

	checkSourceDBConnectivityCommand := exec.Command("/bin/sh", "-c", finalString)

	// fmt.Printf("[Debug] Final Command is : %s\n", checkSourceDBConnectivityCommand.String())

	err := checkSourceDBConnectivityCommand.Run()
	if err != nil {
		fmt.Println("Error: source database endpoints are not reachable")
		os.Exit(1)
	}

}

func CheckSourceDbAccessibility(source *Source) {
	//sanity check - network connectivity to source endpoints(host, port)
	checkSourceEndpointsReachability(source.Host, source.Port)

	//Now check for DB accessibility
	var checkConnectivityCommand string

	if source.DBType == "oracle" {
		checkConnectivityCommand = fmt.Sprintf("sqlcl '%s/%s@(DESCRIPTION=(ADDRESS="+
			"(PROTOCOL=TCP)(HOST=%s)(PORT=%s))(CONNECT_DATA=(SID=%s)))'", source.User,
			source.Password, source.Host, source.Port, source.DBName)
		//PROTOCOL=TCPS if SSL is enabled

	} else if source.DBType == "postgresql" {
		//URI syntax - "postgresql://user:password@host:port/dbname?sslmode=mode"
		checkConnectivityCommand = fmt.Sprintf("psql postgresql://%s:%s@%s:%s/%s?sslmode=%s",
			source.User, source.Password, source.Host, source.Port, source.DBName, source.SSLMode)

	} else if source.DBType == "mysql" {
		/*
			TODO:
				if mysql client version >= 8.0
					use flag: ssl-mode=VERIFY_CA
				else
					use flag: ssl-verify-server-cert
		*/
		checkConnectivityCommand = fmt.Sprintf("mysql --user=%s --password=%s --host=%s --port=%s "+
			"--database=%s --ssl-mode=%s --ssl-cert=%s", source.User, source.Password,
			source.Host, source.Port, source.DBName, source.SSLMode, source.SSLCertPath)

	}

	_, err := exec.Command("/bin/bash", "-c", checkConnectivityCommand).Output()

	// fmt.Printf("[Debug] checkConnectivityCommand: %s\n", checkConnectivityCommand)
	// fmt.Printf("[Debug] command output: %s\n", cmdOutput)
	CheckError(err, checkConnectivityCommand, "Unable to connect to the source database", true)

	PrintIfTrue("Source DB is accessible!!\n", !source.GenerateReportMode)
}

//Called before export schema command
func DeleteProjectDirIfPresent(source *Source, exportDir string) {
	log.Debugf("Deleting existing project related directories under: \"%s\"", exportDir)

	projectDirPath := exportDir
	// projectDirPath := GetProjectDirPath(source, exportDir)

	err := exec.Command("rm", "-rf", projectDirPath+"/schema").Run()
	CheckError(err, "", "Couldn't clean project directory, first clean it to proceed", true)

	err = exec.Command("rm", "-rf", projectDirPath+"/data").Run()
	CheckError(err, "", "Couldn't clean project directory, first clean it to proceed", true)

	err = exec.Command("rm", "-rf", projectDirPath+"/reports").Run()
	CheckError(err, "", "Couldn't clean project directory, first clean it to proceed", true)

	err = exec.Command("rm", "-rf", projectDirPath+"/metadata").Run()
	CheckError(err, "", "Couldn't clean project directory, first clean it to proceed", true)

	err = exec.Command("rm", "-rf", projectDirPath+"/temp").Run()
	CheckError(err, "", "Couldn't clean project directory, first clean it to proceed", true)
}

//setup a project having subdirs for various database objects IF NOT EXISTS
func CreateMigrationProjectIfNotExists(source *Source, exportDir string) {
	// log.Debugf("Creating a project directory...")
	//Assuming export directory as a project directory
	projectDirPath := exportDir

	for _, subdir := range projectSubdirs {
		err := exec.Command("mkdir", "-p", projectDirPath+"/"+subdir).Run()
		CheckError(err, "", "couldn't create sub-directories under "+projectDirPath, true)
	}

	// Put info to metainfo/schema about the source db
	sourceInfoFile := projectDirPath + "/metainfo/schema/" + "source-db-" + source.DBType
	cmdOutput, err := exec.Command("touch", sourceInfoFile).CombinedOutput()

	CheckError(err, "", string(cmdOutput), true)

	schemaObjectList := GetSchemaObjectList(source.DBType)

	// creating subdirs under schema dir
	for _, schemaObjectType := range schemaObjectList {
		databaseObjectDirName := strings.ToLower(schemaObjectType) + "s"

		err := exec.Command("mkdir", "-p", projectDirPath+"/schema/"+databaseObjectDirName).Run()
		CheckError(err, "", "couldn't create sub-directories under "+projectDirPath+"/schema", true)
	}

	// creating subdirs under temp/schema dir
	if source.GenerateReportMode {
		for _, schemaObjectType := range schemaObjectList {
			databaseObjectDirName := strings.ToLower(schemaObjectType) + "s"

			err := exec.Command("mkdir", "-p", projectDirPath+"/temp/schema/"+databaseObjectDirName).Run()
			CheckError(err, "", "couldn't create sub-directories under "+projectDirPath+"/schema", true)
		}
	}

	// log.Debugf("Created a project directory...")
}

func AskPrompt(args ...string) bool {
	var input string
	var argsLen int = len(args)

	for i := 0; i < argsLen; i++ {
		if i != argsLen-1 {
			fmt.Printf("%s ", args[i])
		} else {
			fmt.Printf("%s", args[i])
		}

	}
	fmt.Printf("?[Y/N]:")

	_, err := fmt.Scan(&input)

	if err != nil {
		panic(err)
	}

	input = strings.TrimSpace(input)
	input = strings.ToUpper(input)

	if input == "Y" || input == "YES" {
		return true
	}
	return false
}

func GetSchemaObjectList(sourceDBType string) []string {
	var requiredList []string
	switch sourceDBType {
	case "oracle":
		requiredList = oracleSchemaObjectList
	case "postgresql":
		requiredList = postgresSchemaObjectList
	case "mysql":
		requiredList = mysqlSchemaObjectList
	default:
		fmt.Printf("Unsupported %s source db type\n", sourceDBType)
		os.Exit(1)
	}
	return requiredList
}

func CheckToolsRequiredInstalledOrNot(source *Source) {
	var toolsRequired []string

	switch source.DBType {
	case "oracle":
		toolsRequired = []string{"ora2pg", "sqlcl"}
	case "postgresql":
		toolsRequired = []string{"pg_dump", "strings", "psql"}
	case "mysql":
		toolsRequired = []string{"ora2pg", "mysql"}
	case "yugabytedb":
		toolsRequired = []string{"psql"}
	default:
		log.Fatalf("Invalid DB Type!!")
	}

	commandNotFoundRegexp := regexp.MustCompile(`(?i)not[ ]+found[ ]+in[ ]+\$PATH`)

	for _, tool := range toolsRequired {
		var versionFlag string
		if tool == "sqlcl" {
			versionFlag = "-V"
		} else {
			versionFlag = "--version"
		}

		checkToolPresenceCommand := exec.Command(tool, versionFlag)
		err := checkToolPresenceCommand.Run()

		if err != nil {
			if commandNotFoundRegexp.MatchString(err.Error()) {
				log.Fatalf("%s command not found. Check if %s is installed and included in PATH variable", tool, tool)
			} else {
				panic(err)
			}
		}
	}

	PrintIfTrue(fmt.Sprintf("Required tools %v are present...\n", toolsRequired), source.VerboseMode, !source.GenerateReportMode)
}

func ProjectSubdirsExists(exportDir string) bool {
	for _, subdir := range projectSubdirs {
		if FileOrFolderExists(exportDir + "/" + subdir) {
			return true
		}
	}
	return false
}

func FileOrFolderExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		} else {
			panic(err)
		}
	} else {
		return true
	}
}

func CleanDir(dir string) {
	if FileOrFolderExists(dir) {
		files, _ := filepath.Glob(dir + "/*")
		fmt.Printf("cleaning directory: %s ...\n", dir)
		for _, file := range files {
			err := os.RemoveAll(file)
			if err != nil {
				fmt.Printf("%s\n", err.Error())
				os.Exit(1)
			}
		}
	}
}

func ClearMatchingFiles(filePattern string) {
	files, err := filepath.Glob(filePattern)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
		os.Exit(1)
	}
	for _, file := range files {
		err := os.RemoveAll(file)
		if err != nil {
			fmt.Printf("%s\n", err.Error())
			os.Exit(1)
		}
	}
}

func PrintIfTrue(message string, args ...bool) {
	for i := 0; i < len(args); i++ {
		if args[i] == false {
			return
		}
	}
	fmt.Printf("%s\n", message)
}

func ParseJsonFromString(jsonString string) Report {
	byteJson := []byte(jsonString)
	var report Report
	err := json.Unmarshal(byteJson, &report)
	if err != nil {
		fmt.Printf("%s\n", err.Error())
	}
	return report
}

func GetObjectNameListFromReport(jsonString string, objType string) []string {
	var tableList []string
	report := ParseJsonFromString(jsonString)

	for _, dbObject := range report.Summary.DBObjects {
		if dbObject.ObjectType == objType {
			rawTableList := dbObject.ObjectNames

			tableList = strings.Split(rawTableList, ", ")
			break
		}
	}

	return tableList
}

func PrettifyHtmlString(htmlStr string) string {
	return gohtml.Format(htmlStr)
}

func PrettifyJsonString(jsonStr string) string {
	var prettyJSON bytes.Buffer
	if err := json.Indent(&prettyJSON, []byte(jsonStr), "", "    "); err != nil {
		panic(err)
	}
	return prettyJSON.String()
}
