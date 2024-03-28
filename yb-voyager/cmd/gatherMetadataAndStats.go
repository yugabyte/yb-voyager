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
package cmd

import (
	"fmt"
	"os/exec"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func gatherMetadataAndStats() error {
	assessmentDataDir := filepath.Join(exportDir, "assessment", "data")

	utils.PrintAndLog("gathering metadata and stats from '%s' source database...", source.DBType)
	switch source.DBType {
	case POSTGRESQL:
		err := gatherMetadataAndStatsFromPG()
		if err != nil {
			return fmt.Errorf("error gathering metadata and stats from source PG database: %w", err)
		}
	default:
		return fmt.Errorf("source DB Type %s is not yet supported for metadata and stats gathering", source.DBType)
	}
	utils.PrintAndLog("gathered metadata and stats files at '%s'", assessmentDataDir)
	return nil
}

func gatherMetadataAndStatsFromPG() error {
	psqlBinPath, err := srcdb.GetAbsPathOfPGCommand("psql")
	if err != nil {
		log.Errorf("could not get absolute path of psql command: %v", err)
		return fmt.Errorf("could not get absolute path of psql command: %w", err)
	}

	homebrewVoyagerDir := fmt.Sprintf("yb-voyager@%s", utils.YB_VOYAGER_VERSION)
	possiblePathsForPsqlScript := []string{
		filepath.Join("/", "etc", "yb-voyager", "scripts", "yb-voyager-gather-metadata-and-stats.psql"),
		filepath.Join("/", "opt", "homebrew", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, "etc", "yb-voyager", "scripts", "yb-voyager-gather-metadata-and-stats.psql"),
		filepath.Join("/", "usr", "local", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, "etc", "yb-voyager", "scripts", "yb-voyager-gather-metadata-and-stats.psql"),
	}
	psqlScriptPath := ""
	for _, path := range possiblePathsForPsqlScript {
		if utils.FileOrFolderExists(path) {
			psqlScriptPath = path
			break
		}
	}

	if psqlScriptPath == "" {
		log.Errorf("psql script not found in possible paths: %v", possiblePathsForPsqlScript)
		return fmt.Errorf("psql script not found in possible paths: %v", possiblePathsForPsqlScript)
	}

	log.Infof("using psql script: %s", psqlScriptPath)
	if source.Password == "" {
		sourcePassword, err := askPassword("source DB", source.User, "SOURCE_DB_PASSWORD")
		if err != nil {
			log.Errorf("error getting source DB password: %v", err)
			return err
		}
		source.Password = sourcePassword
	}

	args := []string{
		source.DB().GetConnectionUriWithoutPassword(),
		"-f", psqlScriptPath,
		"-v", "schema_list=" + source.Schema,
	}

	preparedPsqlCmd := exec.Command(psqlBinPath, args...)
	log.Infof("running psql command: %s", preparedPsqlCmd.String())
	preparedPsqlCmd.Env = append(preparedPsqlCmd.Env, "PGPASSWORD="+source.Password)
	preparedPsqlCmd.Dir = filepath.Join(exportDir, "assessment", "data")

	stdout, err := preparedPsqlCmd.CombinedOutput()
	if err != nil {
		log.Errorf("output of postgres metadata and stats gathering script\n%s", string(stdout))
		return err
	}
	log.Infof("output of postgres metadata and stats gathering script\n%s", string(stdout))
	return nil
}
