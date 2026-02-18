package cmd

import (
	"github.com/spf13/cobra"
)

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate data consistency and performance",
	Long:  PARENT_COMMAND_USAGE,
}

func init() {
	rootCmd.AddCommand(validateCmd)
}
