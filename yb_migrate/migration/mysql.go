package migration

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"yb_migrate/migrationutil"
)

//TODO: Try to reuse similar function in oracle
func PrintMySQLSourceDBVersion(source *migrationutil.Source, ExportDir string) {
	sourceDSN := getSourceDSN(source)

	testDBVersionCommandString := fmt.Sprintf("ora2pg -t SHOW_VERSION --source \"%s\" --user %s --password %s;",
		sourceDSN, source.User, source.Password)

	testDBVersionCommand := exec.Command("bin/bash", "-c", testDBVersionCommandString)

	dbVersionBytes, err := testDBVersionCommand.Output()

	migrationutil.CheckError(err, testDBVersionCommand.String(), string(dbVersionBytes), true)

	fmt.Printf("DB Version: %s\n", string(dbVersionBytes))
}

/*
	TODO: MySQLExtractSchema() and OracleExtractSchema() are almost similar,
	Maybe the code reusability can be done here.
*/
func MySQLExtractSchema(source *migrationutil.Source, ExportDir string) {
	projectDirPath := migrationutil.GetProjectDirPath(source, ExportDir)

	//[Internal]: Decide whether to keep ora2pg.conf file hidden or not
	configFilePath := projectDirPath + "/metainfo/schema/ora2pg.conf"
	populateOra2pgConfigFile(configFilePath, source)

	exportObjectList := migrationutil.GetSchemaObjectList(source.DBType)

	for _, exportObject := range exportObjectList {
		exportObjectFileName := strings.ToLower(exportObject) + ".sql"
		exportObjectDirName := strings.ToLower(exportObject) + "s"
		exportObjectDirPath := projectDirPath + "/schema/" + exportObjectDirName

		exportSchemaObjectCommand := exec.Command("ora2pg", "-m", "-t", exportObject, "-o",
			exportObjectFileName, "-b", exportObjectDirPath, "-c", configFilePath)

		exportSchemaObjectCommand.Stdout = os.Stdout
		exportSchemaObjectCommand.Stderr = os.Stderr

		fmt.Printf("[Debug] exportSchemaObjectCommand: %s\n", exportSchemaObjectCommand.String())
		err := exportSchemaObjectCommand.Run()

		//TODO: Maybe we can suggest some smart HINT for the error happenend here
		migrationutil.CheckError(err, exportSchemaObjectCommand.String(),
			"Exporting of "+exportObject+" didn't happen, Retry exporting the schema", false)

		if err == nil {
			fmt.Printf("Export of %s schema done...\n", exportObject)
		}

	}
}

// TODO:
func MySQLDataExportOffline(source *migrationutil.Source, ExportDir string) {

}
