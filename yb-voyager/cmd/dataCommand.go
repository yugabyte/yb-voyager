package cmd

import (
	"github.com/spf13/cobra"
)

var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "Export and import data, manage cutover and archival",
	Long:  PARENT_COMMAND_USAGE,
}

func init() {
	rootCmd.AddCommand(dataCmd)
}
