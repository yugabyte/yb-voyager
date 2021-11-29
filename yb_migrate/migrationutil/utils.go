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
package migrationutil

import (
	"fmt"
	"log"
	"os/exec"
	"strings"
	"time"
)

func Wait(c chan *int) {
	fmt.Print("\033[?25l") // Hide the cursor
	chars := [4]byte{'|', '/', '-', '\\'}
	var i = 0
	for true {
		i++
		select {
		case <-c:
			fmt.Printf("\nGot Data on channel. Export Done\n")
			return
		default:
			fmt.Print("\b" + string(chars[i%4]))
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func checkSourceEndpointsReachability(host string, port string) {
	sourceEndPointConnectivityCommandString := "nc -z -w 30 " + host + " " + port
	lastCommandExitStatusCommandString := "echo $?"

	finalString := sourceEndPointConnectivityCommandString + "; " + lastCommandExitStatusCommandString

	checkSourceDBConnectivityCommand := exec.Command("/bin/sh", "-c", finalString)

	fmt.Printf("[Debug] Final Command is : %s\n", checkSourceDBConnectivityCommand.String())

	outputbytes, err := checkSourceDBConnectivityCommand.Output()

	//try to beautify the command.String() print statement here
	CheckError(err, checkSourceDBConnectivityCommand.String(), "Source Endpoints are not reachable", true)

	fmt.Printf("Source Connectivity check command exit status: %s\n", outputbytes)
}

func CheckSourceDbAccessibility(source *Source) {
	//sanity check - network connectivity to source endpoints(host, port)
	checkSourceEndpointsReachability(source.Host, source.Port)

	//Now check for DB accessibility
	var checkConnectivityCommand string

	if source.DBType == "oracle" {
		checkConnectivityCommand = fmt.Sprintf("sqlplus '%s/%s@(DESCRIPTION=(ADDRESS="+
			"(PROTOCOL=TCP)(HOST=%s)(PORT=%s))(CONNECT_DATA=(SID=%s)))'", source.User,
			source.Password, source.Host, source.Port, source.DBName)

	} else if source.DBType == "postgres" {
		//URI syntax - "postgresql://user:password@host:port/dbname?sslmode=mode"
		checkConnectivityCommand = fmt.Sprintf("psql postgresql://%s:%s@%s:%s/%s?sslmode=%s",
			source.User, source.Password, source.Host, source.Port, source.DBName, source.SSLMode)

	}
	// else if source.DBType == "mysql" {
	// 	checkConnectivityCommand = fmt.Sprintf("mysql")
	// }

	cmdOutput, err := exec.Command("/bin/bash", "-c", checkConnectivityCommand).Output()

	CheckError(err, checkConnectivityCommand, "Unable to connect to the source database", true)

	fmt.Printf("Output of checkConnectivityCommand : %s\n", cmdOutput)
}

//setup a project having various subdirs for various database objects
func CreateMigrationProject(ExportDir string, projectDirName string, schemaName string) string {
	fmt.Println("Creating a project directory: ", projectDirName)

	projectDirPath := ExportDir + "/" + projectDirName

	projectDirectoryCreationCommand := exec.Command("mkdir", projectDirPath)

	// var stdin bytes.Buffer
	// var stdout bytes.Buffer
	// var stderr bytes.Buffer
	// projectDirectoryCreationCommand.Stdin = os.Stdin
	// projectDirectoryCreationCommand.Stdout = os.Stdout
	// projectDirectoryCreationCommand.Stderr = &stderr

	cmdOutput, err := projectDirectoryCreationCommand.CombinedOutput()

	if err != nil {
		log.Fatalf("%s", cmdOutput)
		// fmt.Printf("%s : %s \n", err, stderr.String())
	}

	//creating directories: schema, data, temp inside project
	//TODO: use a for-loop here
	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/data"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/metainfo"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/temp"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	//Under schema dir, creating subdirs for DB objects[TABLES, VIEWS, TYPES, FUNCTIONS, PROCEDURES, SEQUENCES, MVIEWS, GRANTS?]
	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema/tables"),
		"Couldn't create sub-directories under "+projectDirPath, true)
	//TODO: add subdirs under tables/ for PKs, FKs, INDEXs - name of dir should be uniform/valid for all sources
	//Maybe this TODO step can done while parsing or generating schema

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema/views"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema/types"),
		"Couldn't create sub-directories under "+projectDirPath, true)
	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema/functions"),
		"Couldn't create sub-directories under "+projectDirPath, true)
	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema/procedures"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema/sequences"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema/triggers"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema/mviews"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/schema/grants"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	fmt.Println("Created a project directory: ", projectDirName)
	return ""
}

func executeCommandAndErrorCheck(command *exec.Cmd, errorPrintStatement string, stopOnError bool) {
	err := command.Run()

	CheckErrorSimple(err, errorPrintStatement, stopOnError)
}

func GetProjectDirPath(source *Source, ExportDir string) string {
	projectDirName := GetProjectDirName(source)

	projectDirPath := ExportDir + "/" + projectDirName
	// fmt.Printf("Returned Export Dir Path: %s\n", projectDirPath)
	return projectDirPath
}

func GetProjectDirName(source *Source) string {
	if source.DBType == "oracle" {
		//schema in oracle is equivalent to database in postgres, mysql
		return "project-" + source.Schema + "-migration"
	} else {
		return "project-" + source.DBName + "-migration"
	}
}

func AskPrompt(str string) bool {
	var input string
	fmt.Printf("%s(Y/N):", str)
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
