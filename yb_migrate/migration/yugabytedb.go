package migration

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
	"yb_migrate/migrationutil"
)

//TODO
func CheckYugabyteDBToolsInstalled() {

}

// TODO
func PrintYugabyteDBTargetVersion() {

}

func YugabyteDBImportSchema(target *migrationutil.Target, ExportDir string) {
	//Modify it later, once decided project Naming
	projectDirPath := ExportDir

	targetConnectionURI := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable",
		target.User, target.Password, target.Host, target.Port, target.DBName)

	//this list defined the order to create object type in target
	importObjectOrderList := []string{"SCHEMA", "TYPE", "SEQUENCE", "TABLE", "FUNCTION", "VIEW",  "TRIGGER",
		/*"MVIEW" , "INDEXES", "FKEYS", GRANT, ROLE, SCHEMA, DOMAIN */}


	for _, importObjectType := range importObjectOrderList {
		fmt.Printf("[Debug]: Import of %s started...\n", importObjectType)
		importObjectDirPath := projectDirPath + "/schema/" + strings.ToLower(importObjectType) + "s"
		importObjectFilePath := importObjectDirPath + "/" + strings.ToLower(importObjectType) + ".sql"

		createObjectCommand := exec.Command("psql", targetConnectionURI, "-e", "-f", importObjectFilePath)
		
		fmt.Printf("[Debug]: Command: %s\n", createObjectCommand.String())

		var stderr, stdout bytes.Buffer
		createObjectCommand.Stderr = &stdout
		createObjectCommand.Stdout = &stdout

		// createObjectCommand.Stdin = os.Stdin
		// createObjectCommand.Stdout = os.Stdout
		// createObjectCommand.Stderr = os.Stderr
		

		// cmdOutputBytes, err := createObjectCommand.CombinedOutput()
		err := createObjectCommand.Run()
		// fmt.Printf("[Debug]: Output: %s\n", cmdOutputBytes)

		// CheckError(err, createObjectCommand.String(), "couldn't import " + importObjectType + " to target database!!", false)
		// migrationutil.CheckError(err, createObjectCommand.String(), string(cmdOutputBytes), false)

		fmt.Printf("==========STDERR======\n")
		fmt.Printf("%s\n", stderr.String())
		fmt.Printf("==========STDOUT======\n")
		fmt.Printf("%s\n", stdout.String())

		if err == nil {
			fmt.Printf("Import of %s done!!\n", importObjectType)
		}
	}

}
