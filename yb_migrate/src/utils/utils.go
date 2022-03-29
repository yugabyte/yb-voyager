/*
Copyright (c) YugaByte, Inc.

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
	"sort"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yosssi/gohtml"
)

var projectSubdirs = []string{"schema", "data", "reports", "metainfo", "metainfo/data", "metainfo/schema", "metainfo/flags", "temp"}

func Wait(args ...string) {
	var successMsg, failureMsg string
	if len(args) > 0 {
		successMsg = args[0]
	}
	if len(args) > 1 {
		failureMsg = args[1]
	}

	chars := [4]byte{'|', '/', '-', '\\'}
	var i = 0
	for {
		i++
		select {
		case channelCode := <-WaitChannel:
			fmt.Print("\b ")
			if channelCode == 0 {
				fmt.Printf("%s", successMsg)
			} else if channelCode == 1 {
				fmt.Printf("%s", failureMsg)
			}
			WaitChannel <- -1
			return
		default:
			fmt.Printf("\b" + string(chars[i%4]))
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
		if schemaObjectType == "INDEX" { //no separate dir for indexes
			continue
		}
		databaseObjectDirName := strings.ToLower(schemaObjectType) + "s"

		err := exec.Command("mkdir", "-p", projectDirPath+"/schema/"+databaseObjectDirName).Run()
		CheckError(err, "", "couldn't create sub-directories under "+projectDirPath+"/schema", true)
	}

	// creating subdirs under temp/schema dir
	if source.GenerateReportMode {
		for _, schemaObjectType := range schemaObjectList {
			if schemaObjectType == "INDEX" { //no separate dir for indexes
				continue
			}
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
		toolsRequired = []string{"ora2pg", "sqlplus"}
	case "postgresql":
		toolsRequired = []string{"pg_dump", "strings", "psql"}
	case "mysql":
		toolsRequired = []string{"ora2pg", "mysql"}
	case "yugabytedb":
		toolsRequired = []string{"psql"}
	default:
		fmt.Println("Invalid DB Type!!")
		log.Fatalf("Invalid DB Type!!")
	}

	commandNotFoundRegexp := regexp.MustCompile(`(?i)not[ ]+found[ ]+in[ ]+\$PATH`)

	for _, tool := range toolsRequired {
		versionFlag := "--version"

		checkToolPresenceCommand := exec.Command(tool, versionFlag)
		err := checkToolPresenceCommand.Run()

		if err != nil {
			if commandNotFoundRegexp.MatchString(err.Error()) {
				fmt.Printf("%s command not found. Check if %s is installed and included in PATH variable", tool, tool)
				log.Fatalf("%s command not found. Check if %s is installed and included in PATH variable", tool, tool)
			} else {
				panic(err)
			}
		}
	}

	// PrintIfTrue(fmt.Sprintf("Required tools %v are present...\n", toolsRequired), source.VerboseMode, !source.GenerateReportMode)
}

func ProjectSubdirsExists(exportDir string) bool {
	for _, subdir := range projectSubdirs {
		if FileOrFolderExists(exportDir + "/" + subdir) {
			return true
		}
	}
	return false
}

func IsDirectoryEmpty(pathPattern string) bool {
	files, _ := filepath.Glob(pathPattern + "/*")
	return len(files) == 0
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
	fmt.Printf("%s", message)
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

func GetObjectNameListFromReport(report Report, objType string) []string {
	var tableList []string
	// fmt.Printf("Report: %v\n", report)
	for _, dbObject := range report.Summary.DBObjects {
		if dbObject.ObjectType == objType {
			rawTableList := dbObject.ObjectNames
			// fmt.Println("RawTableList: ", rawTableList)
			tableList = strings.Split(rawTableList, ", ")
			break
		}
	}

	sort.Strings(tableList)
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

func GetObjectDirPath(schemaDirPath string, objType string) string {
	var requiredPath string
	if objType == "INDEX" {
		requiredPath = schemaDirPath + "/tables"
	} else {
		requiredPath = schemaDirPath + "/" + strings.ToLower(objType) + "s"
	}
	return requiredPath
}

func GetObjectFilePath(schemaDirPath string, objType string) string {
	var requiredPath string
	if objType == "INDEX" {
		requiredPath = schemaDirPath + "/tables/INDEXES_table.sql"
	} else {
		requiredPath = schemaDirPath + "/" + strings.ToLower(objType) + "s/" +
			strings.ToLower(objType) + ".sql"
	}
	return requiredPath
}

func GetObjectFileName(schemaDirPath string, objType string) string {
	return filepath.Base(GetObjectFilePath(schemaDirPath, objType))
}

func IsQuotedString(str string) bool {
	return str[0] == '"' && str[len(str)-1] == '"'
}
