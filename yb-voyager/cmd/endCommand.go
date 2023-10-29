package cmd

import (
	"github.com/spf13/cobra"
)

var endCmd = &cobra.Command{
	Use:   "end",
	Short: "Parent of the command that ends the migration and clears all the migration state.",
	Long:  `Parent of the command that ends the migration and clears all the migration state.`,
}

func init() {
	rootCmd.AddCommand(endCmd)
}
