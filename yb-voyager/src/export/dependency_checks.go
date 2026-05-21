/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package export

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	goerrors "github.com/go-errors/errors"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const MinRequiredJavaVersion = 17

var PGExportCommands = []string{"pg_dump", "pg_restore", "psql"}

func ChangeStreamingIsEnabled(exportType string) bool {
	return exportType == utils.CHANGES_ONLY || exportType == utils.SNAPSHOT_AND_CHANGES
}

func CheckDependencies(sourceDBType, sourceDBVersion, exportType string, useDebezium bool) ([]string, error) {
	var binaryCheckIssues []string
	var missingTools []string
	streamingOrDebezium := ChangeStreamingIsEnabled(exportType) || useDebezium

	switch sourceDBType {
	case "postgresql":
		for _, binary := range PGExportCommands {
			_, binaryCheckIssue, err := srcdb.GetAbsPathOfPGCommandAboveVersion(binary, sourceDBVersion)
			if err != nil {
				return nil, err
			} else if binaryCheckIssue != "" {
				binaryCheckIssues = append(binaryCheckIssues, binaryCheckIssue)
			}
		}
		missingTools = utils.CheckTools("strings")

	case "mysql":
		if !streamingOrDebezium {
			missingTools = utils.CheckTools("ora2pg")
		}

	case "oracle":
		if !streamingOrDebezium {
			missingTools = utils.CheckTools("ora2pg", "sqlplus")
		} else {
			missingTools = utils.CheckTools("sqlplus")
		}

	case "yugabytedb":
		missingTools = utils.CheckTools("strings")

	default:
		return nil, goerrors.Errorf("unknown source database type %q", sourceDBType)
	}

	binaryCheckIssues = append(binaryCheckIssues, missingTools...)

	if streamingOrDebezium {
		javaIssue, err := CheckJavaVersion(MinRequiredJavaVersion)
		if err != nil {
			return nil, err
		}
		if javaIssue != "" {
			binaryCheckIssues = append(binaryCheckIssues, javaIssue)
		}
	}

	if len(binaryCheckIssues) > 0 {
		binaryCheckIssues = append(binaryCheckIssues, "Install or Add the required dependencies to PATH and try again")
	}

	if streamingOrDebezium {
		err := dbzm.FindDebeziumDistribution(sourceDBType, false)
		if err != nil {
			if len(binaryCheckIssues) > 0 {
				binaryCheckIssues = append(binaryCheckIssues, "")
			}
			binaryCheckIssues = append(binaryCheckIssues, strings.ToUpper(err.Error()[:1])+err.Error()[1:])
			binaryCheckIssues = append(binaryCheckIssues, "Please check your Voyager installation and try again")
		}
	}

	return binaryCheckIssues, nil
}

func CheckJavaVersion(minRequiredVersion int) (string, error) {
	javaBinary := "java"
	if javaHome := os.Getenv("JAVA_HOME"); javaHome != "" {
		javaBinary = javaHome + "/bin/java"
	}

	cmd := exec.Command(javaBinary, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Sprintf("java: required version >= %d", minRequiredVersion), nil
	}

	versionOutput := string(output)

	var versionLine string
	lines := strings.Split(versionOutput, "\n")
	for _, line := range lines {
		if strings.Contains(line, "version") {
			versionLine = line
			break
		}
	}
	if versionLine == "" {
		return "", goerrors.Errorf("unable to find java version in output: %s", versionOutput)
	}

	startIndex := strings.Index(versionLine, "\"")
	endIndex := strings.LastIndex(versionLine, "\"")
	if startIndex == -1 || endIndex == -1 || startIndex >= endIndex {
		return "", goerrors.Errorf("unexpected java version output: %s", versionOutput)
	}
	version := versionLine[startIndex+1 : endIndex]

	versionNumbers := strings.Split(version, ".")
	if len(versionNumbers) < 1 {
		return "", goerrors.Errorf("unexpected java version output: %s", versionOutput)
	}
	majorVersion, err := strconv.Atoi(versionNumbers[0])
	if err != nil {
		return "", goerrors.Errorf("unexpected java version output: %s", versionOutput)
	}

	if majorVersion < minRequiredVersion {
		return fmt.Sprintf("java: required version >= %d; current version: %s", minRequiredVersion, version), nil
	}

	return "", nil
}
