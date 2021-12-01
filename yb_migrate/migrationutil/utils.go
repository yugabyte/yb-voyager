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

//setup a project having subdirs for various database objects
func CreateMigrationProject(source *Source, ExportDir string) {
	fmt.Println("Creating a project directory...")

	projectDirPath := GetProjectDirPath(source, nil, ExportDir)

	err := exec.Command("mkdir", projectDirPath).Run()
	CheckError(err, "", "couldn't create sub-directories under "+ExportDir, true)

	subdirs := []string{"schema", "data", "metainfo", "metainfo/data", "metainfo/schema", "temp"}
	for _, subdir := range subdirs {
		err := exec.Command("mkdir", projectDirPath+"/"+subdir).Run()
		CheckError(err, "", "couldn't create sub-directories under "+projectDirPath, true)
	}

	// Put info to metainfo/schema about the source db
	sourceInfoFile := projectDirPath + "/metainfo/schema/" + "source-db-" + source.DBType
	_ = exec.Command("touch", sourceInfoFile).Run()

	// creating subdirs under schema dir
	databaseObjectTypes := []string{"TABLE", "VIEW", "TYPE", "FUNCTION", "PROCEDURE",
		"SEQUENCE", "SCHEMA", "TRIGGER", "MVIEW", "PACKAGE", "SYNONYM", "OTHER",
		/*Test Schemas for -> "GRANT", PARTITION, ROLE, TABLESPACE*/}

	for _, databaseObjectType := range databaseObjectTypes {

		if source.DBType == "postgres" && (databaseObjectType == "PACKAGE" || databaseObjectType == "SYNONYM") {
			continue
		} else if source.DBName == "oracle" && (databaseObjectType == "SCHEMA" || databaseObjectType == "OTHER") {
			continue
		}

		databaseObjectDirName := strings.ToLower(databaseObjectType) + "s"

		err := exec.Command("mkdir", projectDirPath+"/schema/"+databaseObjectDirName).Run()
		CheckError(err, "", "couldn't create sub-directories under "+projectDirPath, true)
	}

	fmt.Println("Created a project directory...")
}

func GetProjectDirPath(source *Source, target *Target, ExportDir string) string {
	projectDirName := GetProjectDirName(source, target)

	projectDirPath := ExportDir + "/" + projectDirName
	// fmt.Printf("Returned Export Dir Path: %s\n", projectDirPath)
	return projectDirPath
}

func GetProjectDirName(source *Source, target *Target) string {
	if target != nil {
		return "project-" + target.DBName + "-migration"
	} else if source.DBType == "oracle" {
		//schema in oracle is equivalent to database in postgres, mysql
		return source.DBType + "-" + source.Schema + "-migration"
	} else {
		return source.DBType + "-" + source.DBName + "-migration"
	}
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
