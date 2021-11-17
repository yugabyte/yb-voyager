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

func CheckError(err error, executedCommand string, possibleReason string, stop bool) {
	if err != nil {
		if stop {
			log.Fatalf("%s \n", err)
		} else {
			log.Printf("%s \n", err)
		}

		if executedCommand != "" {
			log.Printf("Error caused by : %s\n", executedCommand)
		}

		if possibleReason != "" {
			fmt.Printf("HINT: %s\n", possibleReason)
		}
	}
}

func CheckErrorSimple(err error, printStatement string, stop bool) {
	if err != nil {
		if stop {
			log.Fatalf("%s: %s\n", printStatement, err)
		} else {
			log.Printf("%s: %s\n", printStatement, err)
		}
	}
}

func CheckSourceDBConnectivity(host string, port string, schema string, user string, password string) {

	//sanity check - event if the machine is reachable or not
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

func CheckRequiredToolsInstalled(sourceDBType string) {
	if sourceDBType == "oracle" || sourceDBType == "mysql" {
		// migration.CheckOra2pgInstalled() import cycle not allowed
	} else if sourceDBType == "postgres" {
		// migration.CheckYsqldumpInstalled()
	}

}

//setup a project having various subdirs for various database objects
func CreateMigrationProject(exportDir string, projectDirName string, schemaName string) {
	fmt.Println("Creating a project directory: ", projectDirName)

	projectDirPath := exportDir + "/" + projectDirName
	err := exec.Command("mkdir", "-p", projectDirPath).Run()

	if err != nil {
		log.Fatalf("Could not create a project directory under %s: %s\n", projectDirName, err)
	}

	//creating empty dirs for DB objects[TABLES, VIEWS, TYPES, FUNCTIONS, PROCEDURES, SEQUENCES, MVIEWS, GRANTS?]

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/tables"),
		"Couldn't create sub-directories under "+projectDirPath, true)
	//TODO: add subdirs under tables/ for PKs, FKs, INDEXs - name of dir should be uniform/valid for all sources
	//Maybe this TODO step can done while parsing or generating schema

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/views"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/types"),
		"Couldn't create sub-directories under "+projectDirPath, true)
	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/functions"),
		"Couldn't create sub-directories under "+projectDirPath, true)
	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/procedures"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/triggers"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/mviews"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/grants"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/data"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	executeCommandAndErrorCheck(exec.Command("mkdir", "-p", projectDirPath+"/temp"),
		"Couldn't create sub-directories under "+projectDirPath, true)

	fmt.Println("Created a project directory: ", projectDirName)
}

func executeCommandAndErrorCheck(command *exec.Cmd, errorPrintStatement string, stopOnError bool) {
	err := command.Run()

	CheckErrorSimple(err, errorPrintStatement, stopOnError)
}
