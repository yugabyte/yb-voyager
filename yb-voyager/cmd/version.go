package cmd

import (
	"fmt"
	"runtime/debug"

	"github.com/spf13/cobra"
)

const (
	// This constant must be updated on every release.
	YB_VOYAGER_VERSION = "1.0.0-beta"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print yb-voyager version info.",

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("VERSION=%s\n", YB_VOYAGER_VERSION)

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
