package srcdb

import (
	"os/exec"
	"unicode"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

func checkTools(tools ...string) {
	for _, tool := range tools {
		execPath, err := exec.LookPath(tool)
		if err != nil {
			utils.ErrExit("%q not found. Check if it is installed and included in the path.", tool)
		}
		log.Infof("Found %q", execPath)
	}
}

func nameContainsCapitalLetter(name string) bool {
	for _, c := range name {
		if unicode.IsUpper(c) {
			return true
		}
	}
	return false
}
