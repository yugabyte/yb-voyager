package cmd

import (
	"github.com/spf13/cobra"
)

var assessCmd = &cobra.Command{
	Use:   "assess",
	Short: "Analyze source database for compatibility and get cluster sizing recommendations",
	Long:  PARENT_COMMAND_USAGE,
}

func init() {
	rootCmd.AddCommand(assessCmd)
}
