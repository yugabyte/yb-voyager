package cmd

import (
	"github.com/spf13/cobra"
)

var schemaCmd = &cobra.Command{
	Use:   "schema",
	Short: "Migrate schema from source to YugabyteDB.",
	Long:  PARENT_COMMAND_USAGE,
}

func init() {
	rootCmd.AddCommand(schemaCmd)
}
