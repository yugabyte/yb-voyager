package cmd

import (
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Export, analyze, and import schema",
	Long:  PARENT_COMMAND_USAGE,
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
