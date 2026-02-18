package cmd

import (
	"github.com/spf13/cobra"
)

var assessCmd = &cobra.Command{
	Use:   "assess",
	Short: "Assess source database for migration to YugabyteDB",
	Long:  PARENT_COMMAND_USAGE,
}

func init() {
	rootCmd.AddCommand(assessCmd)
}
