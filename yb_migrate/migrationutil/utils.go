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
	"os"
	"os/exec"
	"regexp"
	"strings"
)

// func Wait(c chan *int) {
// 	fmt.Print("\033[?25l") // Hide the cursor
// 	chars := [4]byte{'|', '/', '-', '\\'}
// 	var i = 0
// 	for true {
// 		i++
// 		select {
// 		case <-c:
// 			fmt.Printf("\nGot Data on channel. Export Done\n")
// 			return
// 		default:
// 			fmt.Print("\b" + string(chars[i%4]))
// 			time.Sleep(100 * time.Millisecond)
// 		}
// 	}
// }

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
		checkConnectivityCommand = fmt.Sprintf("sqlcl '%s/%s@(DESCRIPTION=(ADDRESS="+
			"(PROTOCOL=TCP)(HOST=%s)(PORT=%s))(CONNECT_DATA=(SID=%s)))'", source.User,
			source.Password, source.Host, source.Port, source.DBName)
		//PROTOCOL=TCPS if SSL is enabled

	} else if source.DBType == "postgres" {
		//URI syntax - "postgresql://user:password@host:port/dbname?sslmode=mode"
		checkConnectivityCommand = fmt.Sprintf("psql postgresql://%s:%s@%s:%s/%s?sslmode=%s",
			source.User, source.Password, source.Host, source.Port, source.DBName, source.SSLMode)

	} else if source.DBType == "mysql" {
		/*
			if mysql client >= 8.0
				use flag: ssl-mode=VERIFY_CA
			else
				use flag: ssl-verify-server-cert
		*/
		checkConnectivityCommand = fmt.Sprintf("mysql --user=%s --password=%s --host=%s --port=%s "+
			"--database=%s --ssl-mode=%s --ssl-cert=%s", source.User, source.Password,
			source.Host, source.Port, source.DBName, source.SSLMode, source.SSLCertPath)

	}

	cmdOutput, err := exec.Command("/bin/bash", "-c", checkConnectivityCommand).Output()

	fmt.Printf("[Debug] checkConnectivityCommand: %s\n", checkConnectivityCommand)
	fmt.Printf("[Debug] command output: %s\n", cmdOutput)
	CheckError(err, checkConnectivityCommand, "Unable to connect to the source database", true)

	fmt.Printf("Source DB is accessible!!\n")
}

//Called before export schema command
func DeleteProjectDirIfPresent(source *Source, ExportDir string) {
	fmt.Printf("Deleting existing project directory under: \"%s\"\n", ExportDir)

	projectDirPath := GetProjectDirPath(source, ExportDir)

	err := exec.Command("rm", "-rf", projectDirPath).Run()

	CheckError(err, "", "Project Directory already exists, remove it first to proceed", true)

}

//setup a project having subdirs for various database objects IF NOT EXISTS
func CreateMigrationProjectIfNotExists(source *Source, ExportDir string) {
	fmt.Println("Creating a project directory...")

	projectDirPath := GetProjectDirPath(source, ExportDir)

	err := exec.Command("mkdir", "-p", projectDirPath).Run()
	CheckError(err, "", "couldn't create sub-directories under "+ExportDir, true)

	subdirs := []string{"schema", "data", "metainfo", "metainfo/data", "metainfo/schema", "temp"}
	for _, subdir := range subdirs {
		err := exec.Command("mkdir", "-p", projectDirPath+"/"+subdir).Run()
		CheckError(err, "", "couldn't create sub-directories under "+projectDirPath, true)
	}

	// Put info to metainfo/schema about the source db
	sourceInfoFile := projectDirPath + "/metainfo/schema/" + "source-db-" + source.DBType
	err = exec.Command("touch", sourceInfoFile).Run()

	CheckError(err, "", "", true)

	schemaObjectList := GetSchemaObjectList(source.DBType)

	// creating subdirs under schema dir
	for _, schemaObjectType := range schemaObjectList {
		// if source.DBType == "postgres" && (databaseObjectType == "PACKAGE" || databaseObjectType == "SYNONYM") {
		// 	continue
		// } else if source.DBName == "oracle" && (databaseObjectType == "SCHEMA" || databaseObjectType == "OTHER") {
		// 	continue
		// }

		databaseObjectDirName := strings.ToLower(schemaObjectType) + "s"

		err := exec.Command("mkdir", "-p", projectDirPath+"/schema/"+databaseObjectDirName).Run()
		CheckError(err, "", "couldn't create sub-directories under "+projectDirPath, true)
	}

	fmt.Println("Created a project directory...")
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

func GetSchemaObjectList(sourceDBType string) []string {
	var requiredList []string
	switch sourceDBType {
	case "oracle":
		requiredList = oracleSchemaObjectList
	case "postgres":
		requiredList = postgresSchemaObjectList
	case "mysql":
		requiredList = mysqlSchemaObjectList
	default:
		fmt.Printf("Unsupported %s source db type\n", sourceDBType)
		os.Exit(1)
	}
	return requiredList
}

func CheckToolsRequiredInstalledOrNot(dbType string) {
	var toolsRequired []string

	switch dbType {
	case "oracle":
		toolsRequired = []string{"ora2pg", "sqlcl"}
	case "postgres":
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

	fmt.Printf("[Debug] Required tools are present...\n")
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
