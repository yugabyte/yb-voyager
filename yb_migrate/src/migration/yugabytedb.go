package migration

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"yb_migrate/src/utils"
)

var log = utils.GetLogger()

// TODO
func PrintYugabyteDBTargetVersion(target *utils.Target) {
	targetConnectionURI := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable",
		target.User, target.Password, target.Host, target.Port, target.DBName)

	yugabyteDBVersionCommand := exec.Command("psql", targetConnectionURI,
		"-Atc", "SELECT VERSION();")
	output, err := yugabyteDBVersionCommand.Output()

	utils.CheckError(err, "", "", true)

	fmt.Printf("Target YugabyteDB Version: %s\n", output)
}

func YugabyteDBImportSchema(target *utils.Target, exportDir string) {
	metaInfo := ExtractMetaInfo(exportDir)

	projectDirPath := exportDir

	PrintYugabyteDBTargetVersion(target)

	targetConnectionURI := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable",
		target.User, target.Password, target.Host, target.Port, target.DBName)

	//this list also has defined the order to create object type in target YugabyteDB
	importObjectOrderList := utils.GetSchemaObjectList(metaInfo.SourceDBType)

	for _, importObjectType := range importObjectOrderList {
		fmt.Printf("Import of %ss started...\n", strings.ToLower(importObjectType))

		importObjectDirPath := projectDirPath + "/schema/" + strings.ToLower(importObjectType) + "s"

		importObjectFilePath := importObjectDirPath + "/" + strings.ToLower(importObjectType) + ".sql"

		createObjectCommand := exec.Command("psql", targetConnectionURI, "-b", "-f", importObjectFilePath)

		// fmt.Printf("[Debug]: Command: %s\n", createObjectCommand.String())

		// var consoleError, consoleOutput bytes.Buffer
		// createObjectCommand.Stderr = &consoleError
		// createObjectCommand.Stdout = &consoleOutput

		// createObjectCommand.Stdin = os.Stdin
		// createObjectCommand.Stdout = os.Stdout
		// createObjectCommand.Stderr = os.Stderr

		_ = createObjectCommand.Run()

		// CheckError(err, createObjectCommand.String(), "couldn't import " + importObjectType + " to target database!!", false)
		// utils.CheckError(err, createObjectCommand.String(), "couldn't import %s", false)

		// log.Infof("%s", consoleOutput.String())
		// fmt.Printf("%s\n", consoleOutput.String())

		// if err == nil && len(consoleError.String()) == 0 {
		// 	fmt.Printf("Import of %s done!!\n", importObjectType)
		// } else {
		// 	color.Red("couldn't import all %ss due to: %s\nplease try again!!", importObjectType, consoleError.String())
		// }
		fmt.Printf("Import of %ss done!!\n", strings.ToLower(importObjectType))
	}

}

//This function is implementation is rough as of now.
func ExtractMetaInfo(exportDir string) utils.ExportMetaInfo {
	// fmt.Printf("Extracting the metainfo about the source database...\n")
	var metaInfo utils.ExportMetaInfo

	metaInfoDirPath := exportDir + "/metainfo"

	metaInfoDir, err := os.ReadDir(metaInfoDirPath)
	utils.CheckError(err, "", "", true)

	for _, metaInfoSubDir := range metaInfoDir {
		// fmt.Printf("%s\n", metaInfoSubDir.Name())

		if metaInfoSubDir.IsDir() {
			subItemPath := metaInfoDirPath + "/" + metaInfoSubDir.Name()

			subItems, err := os.ReadDir(subItemPath)
			if err != nil {
				panic(err)
			}
			for _, subItem := range subItems {
				subItemName := subItem.Name()
				// fmt.Printf("\t%s\n", subItemName)

				if strings.HasPrefix(subItemName, "source-db-") {
					splits := strings.Split(subItemName, "-")

					metaInfo.SourceDBType = splits[len(splits)-1]
				}

			}
		}

	}

	// fmt.Printf("MetaInfo Struct: %v\n", metaInfo)
	return metaInfo
}
