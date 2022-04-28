package srcdb

import (
	"fmt"
	"os"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

func ErrExit(formatString string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, formatString+"\n", args...)
	log.Errorf(formatString+"\n", args...)
	os.Exit(1)
}

func checkTools(tools ...string) {
	for _, tool := range tools {
		execPath, err := exec.LookPath(tool)
		if err != nil {
			ErrExit("%q not found. Check if it is installed and included in the path.", tool)
		}
		log.Infof("Found %q", execPath)
	}
}
