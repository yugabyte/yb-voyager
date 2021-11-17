package migration

// TODO: fill it
//check for ysqldump and psql/ysqlsh
func CheckPostgresToolsInstalled() {

}

// TODO: fill it, how to using the tools? [psql -c "Select * from version()"]
func PrintPostgresSourceDBVersion(host string, port string, schema string, user string, password string, dbName string, exportDir string) {

	/*
		projectDirectory := exportDir + "/" + schema
		currentDirectory := "cd " + projectDirectory + ";"
		sourceDSN := "dbi:Oracle:host=" + host + ";service_name=" + dbName + ";port=" + port

		testDBVersionCommandString := fmt.Sprintf(currentDirectory+"ora2pg -t SHOW_VERSION --source \"%s\" --user %s --password %s;",
			sourceDSN, user, password)

		testVersionCommand := exec.Command("/bin/bash", "-c", testDBVersionCommandString)

		dbVersionBytes, err := testVersionCommand.Output()

		migrationutil.CheckError(err, testVersionCommand.String(), "Couldn't export schema", true)

		fmt.Printf("[Debug] Output of export schema script: %s and %s\n", testVersionCommand.String(), dbVersionBytes)
		fmt.Println("DB Version: ", string(dbVersionBytes))
	*/
}

func ExportPostgresSchema(host string, port string, schema string, user string, password string, dbName string, exportDir string, projectDirName string) {
	// projectDirPath := exportDir + "/" + projectDirName

	//Dump the schema into one file first

	//Parsing the single file to generate multiple database object files

}
