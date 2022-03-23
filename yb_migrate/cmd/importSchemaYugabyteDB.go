package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/yugabyte/ybm/yb_migrate/src/migration"
	"github.com/yugabyte/ybm/yb_migrate/src/utils"

	"github.com/jackc/pgx/v4"
)

func PrintTargetYugabyteDBVersion(target *utils.Target) {
	targetConnectionURI := target.GetConnectionUri()

	version := migration.SelectVersionQuery("yugabytedb", targetConnectionURI)
	fmt.Printf("YugabyteDB Version: %s\n", version)
}

func YugabyteDBImportSchema(target *utils.Target, exportDir string) {
	metaInfo := ExtractMetaInfo(exportDir)

	projectDirPath := exportDir

	targetConnectionURI := ""
	if target.Uri == "" {
		targetConnectionURI = fmt.Sprintf("postgresql://%s:%s@%s:%d/%s?%s",
			target.User, target.Password, target.Host, target.Port, target.DBName, generateSSLQueryStringIfNotExists(target))
	} else {
		targetConnectionURI = target.Uri
	}

	//this list also has defined the order to create object type in target YugabyteDB
	importObjectOrderList := utils.GetSchemaObjectList(metaInfo.SourceDBType)

	for _, importObjectType := range importObjectOrderList {
		var importObjectDirPath, importObjectFilePath string

		if importObjectType != "INDEX" {
			importObjectDirPath = projectDirPath + "/schema/" + strings.ToLower(importObjectType) + "s"
			importObjectFilePath = importObjectDirPath + "/" + strings.ToLower(importObjectType) + ".sql"
		} else {
			if target.ImportIndexesAfterData {
				continue
			}
			importObjectDirPath = projectDirPath + "/schema/" + "tables"
			importObjectFilePath = importObjectDirPath + "/" + "INDEXES_table.sql"
		}

		if !utils.FileOrFolderExists(importObjectFilePath) {
			continue
		}

		fmt.Printf("importing %10s %5s", importObjectType, "")
		go utils.Wait("done\n", "")

		conn, err := pgx.Connect(context.Background(), targetConnectionURI)
		if err != nil {
			utils.WaitChannel <- 1
			fmt.Println(err)
			os.Exit(1)
		}

		sqlStrArray := createSqlStrArray(importObjectFilePath, importObjectType)
		errOccured := 0
		for _, sqlStr := range sqlStrArray {
			// fmt.Printf("Execute STATEMENT: %s\n", sqlStr[1])
			_, err := conn.Exec(context.Background(), sqlStr[0])
			if err != nil {
				if strings.Contains(err.Error(), "already exists") {
					if !target.IgnoreIfExists {
						fmt.Printf("\b \n    %s\n", err.Error())
						fmt.Printf("    STATEMENT: %s\n", sqlStr[1])
					}
				} else {
					errOccured = 1
					fmt.Printf("\b \n    %s\n", err.Error())
					fmt.Printf("    STATEMENT: %s\n", sqlStr[1])
				}
				if !target.ContinueOnError { //non-default case
					break
				}
			}
		}

		utils.WaitChannel <- errOccured

		conn.Close(context.Background())
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
