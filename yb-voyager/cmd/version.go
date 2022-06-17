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
		fmt.Printf("VERSION=%s\n", utils.YB_VOYAGER_VERSION)

		info, ok := debug.ReadBuildInfo()
		if !ok {
			return
		}
		for _, setting := range info.Settings {
			if setting.Key == "vcs.revision" {
				fmt.Printf("GIT_HASH=%s\n", setting.Value)
			}
			if setting.Key == "vcs.time" {
				fmt.Printf("LAST_COMMIT_DATE=%s\n", setting.Value)
			}
		}
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
