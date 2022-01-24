package migration

import (
	"fmt"
	"os/exec"
	"yb_migrate/src/utils"
)

//TODO: Reuse similar function in oracle instead of this
func PrintMySQLSourceDBVersion(source *utils.Source) {
	sourceDSN := getSourceDSN(source)

	testDBVersionCommandString := fmt.Sprintf("ora2pg -t SHOW_VERSION --source \"%s\" --user %s --password %s;",
		sourceDSN, source.User, source.Password)

	testDBVersionCommand := exec.Command("bin/bash", "-c", testDBVersionCommandString)

	dbVersionBytes, err := testDBVersionCommand.Output()

	utils.CheckError(err, testDBVersionCommand.String(), string(dbVersionBytes), true)

	fmt.Printf("SourceDB Version: %s\n", string(dbVersionBytes))
}
