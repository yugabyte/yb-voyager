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
	"runtime/debug"

	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print yb-voyager version info.",

	Run: func(cmd *cobra.Command, args []string) {
		info := getVersionInfo()
		fmt.Print(info)
	},
}

func getVersionInfo() string {
	versionInfo := fmt.Sprintf("VERSION=%s\n", utils.YB_VOYAGER_VERSION)
	h := utils.GitCommitHash()
	if h != "" {
		fmt.Printf("GIT_COMMIT_HASH=%s\n", h)
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return versionInfo
	}
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			versionInfo += fmt.Sprintf("GIT_COMMIT_HASH=%s\n", setting.Value)
		}
		if setting.Key == "vcs.time" {
			versionInfo += fmt.Sprintf("LAST_COMMIT_DATE=%s\n", setting.Value)
		}
	}

	return versionInfo
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
