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
	"bytes"
	_ "embed"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var gatherMetadataAndStatsCmd = &cobra.Command{
	Use:   "gather-metadata-and-stats",
	Short: "Gather metadata and stats from the source database for migration assessment report generation.",
	Long:  `Gather metadata and stats from the source database for migration assessment report generation.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		setSourceDefaultPort()
	},

	Run: func(cmd *cobra.Command, args []string) {
		// TODO: metadb is also initialized which is not requried at this stage
		CreateMigrationProjectIfNotExists(source.DBType, exportDir)
		gatherMetadataAndStats()
	},
}

func init() {
	rootCmd.AddCommand(gatherMetadataAndStatsCmd)
	registerCommonGlobalFlags(gatherMetadataAndStatsCmd)
	registerCommonSourceDBConnFlags(gatherMetadataAndStatsCmd)
	BoolVar(gatherMetadataAndStatsCmd.Flags(), &startClean, "start-clean", false,
		"cleans up the project directory for schema or data files depending on the export command (default false)")

	// mark schema flag as mandatory
	gatherMetadataAndStatsCmd.MarkFlagRequired("source-db-schema")
}

// TODO: figure out how to access if this file is in other package of same project
//
//go:embed matscripts/pgPsql.sql
var pgScript []byte

func gatherMetadataAndStats() {
	assessmentDataDir := filepath.Join(exportDir, "assessment", "data")
	matchingFiles := filepath.Join(assessmentDataDir, "*.csv")
	if utils.FileOrFolderExistsWithGlobPattern(matchingFiles) {
		if startClean {
			utils.CleanDir(filepath.Join(exportDir, "assessment", "data"))
		} else {
			utils.ErrExit("metadata and stats files already exist in assessment/data directory. Use --start-clean flag to start fresh")
		}
	}

	utils.PrintAndLog("gathering metadata and stats from '%s' source database...", source.DBType)
	switch source.DBType {
	case POSTGRESQL:
		gatherMetadataAndStatsFromPG()
	default:
		utils.ErrExit("Source DB Type %s is not yet supported for metadata and stats gathering", source.DBType)
	}
	utils.PrintAndLog("gathering metadata and stats completed")
	utils.PrintAndLog("metadata and stats files are available in '%s' directory", assessmentDataDir)
	utils.ListFilesInDir(assessmentDataDir)
}

func gatherMetadataAndStatsFromPG() {
	// TODO: fetch pg_dump from path(with supported version check)
	cmdArgs := []string{
		"-h", source.Host,
		"-p", strconv.Itoa(source.Port),
		"-U", source.User,
		"-d", source.DBName,
		"-v", fmt.Sprintf("schema_list=%s", source.Schema),
	}

	cmd := exec.Command("psql", cmdArgs...)
	cmd.Env = append(cmd.Env, "PGPASSWORD="+source.Password)
	cmd.Stdin = bytes.NewReader(pgScript) // other option is to use -f flag with filepath
	cmd.Dir = filepath.Join(exportDir, "assessment", "data")

	stdout, err := cmd.CombinedOutput()
	if err != nil {
		log.Errorf("output of postgres metadata and stats gathering script\n%s", string(stdout))
		utils.ErrExit("error running metadata and stats gathering script: %v", err)
	}
	log.Infof("output of postgres metadata and stats gathering script\n%s", string(stdout))
}

// TODOs
// 1. implement call home for this command
// 2. ssl connectivity for gathering metadata and stats
// 3. implement start clean functionality
