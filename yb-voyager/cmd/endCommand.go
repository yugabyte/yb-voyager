package cmd

import (
	"github.com/spf13/cobra"
)

var endCmd = &cobra.Command{
	Use:   "end",
	Short: PARENT_COMMAND_USAGE,
	Long:  ``,
}

func init() {
	rootCmd.AddCommand(endCmd)
}
