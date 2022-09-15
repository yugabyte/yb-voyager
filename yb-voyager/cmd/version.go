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
