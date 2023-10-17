package cmd

import (
	"github.com/spf13/cobra"
)

var endCmd = &cobra.Command{
	Use:   "end",
	Short: "This command will mark the end of the migration",
	Long:  `This command will mark the end of the migration`,
}

func init() {
	rootCmd.AddCommand(endCmd)
}
