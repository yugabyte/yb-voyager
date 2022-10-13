package srcdb

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"
	"unicode"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
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

func findAllExecutablesInPath(executableName string) ([]string, error) {
	pathString := os.Getenv("PATH")
	if pathString == "" {
		return nil, fmt.Errorf("PATH environment variable is not set")
	}
	paths := strings.Split(pathString, string(os.PathListSeparator))
	var result []string
	for _, dir := range paths {
		fullPath := path.Join(dir, executableName)
		if _, err := os.Stat(fullPath); err == nil {
			result = append(result, fullPath)
		}
	}
	return result, nil
}
