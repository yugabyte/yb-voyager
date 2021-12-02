package migration

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"yb_migrate/migrationutil"

	"github.com/fatih/color"
)

//TODO
func CheckYugabyteDBToolsInstalled() {

}

// TODO
func PrintYugabyteDBTargetVersion() {

}

func YugabyteDBImportSchema(target *migrationutil.Target, ExportDir string) {
	metaInfo := extractMetaInfo(ExportDir)

	projectDirPath := ExportDir

	targetConnectionURI := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable",
		target.User, target.Password, target.Host, target.Port, target.DBName)

	//this list defined the order to create object type in target
	importObjectOrderList := migrationutil.GetSchemaObjectList(metaInfo.SourceDBType)

	// []string{ /*if source = postgres*/ "SCHEMA", "TYPE",
	// 	/*if source = postgres*/ "DOMAIN", "SEQUENCE", "TABLE", "FUNCTION", "VIEW", "TRIGGER",
	// 	/*"MVIEW" , "INDEXES", "FKEYS", GRANT, ROLE, RULE, AGGREGATE */}

	red := color.New(color.FgRed).PrintfFunc()
	yellow := color.New(color.FgYellow).PrintfFunc()
	blue := color.New(color.FgBlue).PrintfFunc()
	green := color.New(color.FgGreen).PrintfFunc()

	for _, importObjectType := range importObjectOrderList {
		yellow("[Debug]: Import of %s started...\n", importObjectType)
		// fmt.Printf("[Debug]: Import of %s started...\n", importObjectType)

		importObjectDirPath := projectDirPath + "/schema/" + strings.ToLower(importObjectType) + "s"

		importObjectFilePath := importObjectDirPath + "/" + strings.ToLower(importObjectType) + ".sql"

		createObjectCommand := exec.Command("psql", targetConnectionURI, "-b", "-f", importObjectFilePath)

		fmt.Printf("[Debug]: Command: %s\n", createObjectCommand.String())

		var consoleOutput bytes.Buffer
		createObjectCommand.Stderr = &consoleOutput
		createObjectCommand.Stdout = &consoleOutput

		// createObjectCommand.Stdin = os.Stdin
		// createObjectCommand.Stdout = os.Stdout
		// createObjectCommand.Stderr = os.Stderr

		err := createObjectCommand.Run()

		// CheckError(err, createObjectCommand.String(), "couldn't import " + importObjectType + " to target database!!", false)
		// migrationutil.CheckError(err, createObjectCommand.String(), "couldn't import %s", false)

		blue("%s\n", consoleOutput.String())
		// fmt.Printf("%s\n", consoleOutput.String())

		if err == nil {
			green("Import of %s done!!\n", importObjectType)
		} else {
			red("couldn't import any %s, please try again!!", importObjectType)
		}
	}

}

//This function is implementation is rough as of now.
func extractMetaInfo(ExportDir string) migrationutil.MetaInfo {
	fmt.Printf("Extracting the metainfo about the source database...\n")
	var metaInfo migrationutil.MetaInfo

	metaInfoDirPath := ExportDir + "/metainfo"

	metaInfoDir, err := os.ReadDir(metaInfoDirPath)
	migrationutil.CheckError(err, "", "", true)

	for _, metaInfoSubDir := range metaInfoDir {
		fmt.Printf("%s\n", metaInfoSubDir.Name())

		if metaInfoSubDir.IsDir() {
			subItemPath := metaInfoDirPath + "/" + metaInfoSubDir.Name()

			subItems, err := os.ReadDir(subItemPath)
			if err != nil {
				panic(err)
			}
			for _, subItem := range subItems {
				subItemName := subItem.Name()
				fmt.Printf("\t%s\n", subItemName)

				if strings.HasPrefix(subItemName, "source-db-") {
					splits := strings.Split(subItemName, "-")

					metaInfo.SourceDBType = splits[len(splits)-1]
				}

			}
		}

	}

	fmt.Printf("MetaInfo Struct: %v\n", metaInfo)
	return metaInfo
}
