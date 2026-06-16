package cmd

import (
	"github.com/spf13/cobra"
)

var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "Migrate data from source to YugabyteDB.",
	Long:  PARENT_COMMAND_USAGE,
}

func init() {
	rootCmd.AddCommand(dataCmd)
}
