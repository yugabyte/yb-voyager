/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var target Target
// importCmd represents the import command
var importCmd = &cobra.Command{
	Use:   "import",
	Short: "A brief description of your command",
	Long: `A longer description that spans multiple lines and likely contains examples
and usage of using your command. For example:

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("import called")
	},
}

var targetDBHost string

func init() {
	rootCmd.AddCommand(importCmd)

	importCmd.PersistentFlags().StringVar(&target.Host, "target-db-host", "",
		"Host on which the YugabyteDB server is running")
	importCmd.PersistentFlags().StringVar(&target.Port, "target-db-port", "",
		"Port on which the YugabyteDB database is running")
	importCmd.PersistentFlags().StringVar(&target.User, "target-db-user", "",
		"Username with which to connect to the target YugabyteDB server")
	importCmd.PersistentFlags().StringVar(&target.Password, "target-db-password", "",
		"Password with which to connect to the target YugabyteDB server")
	importCmd.PersistentFlags().StringVar(&target.Database, "target-db-name", "",
		"Name of the database on the target YugabyteDB server on which import needs to be done")
	importCmd.PersistentFlags().StringVar(&target.Uri, "target-db-uri", "",
		"Complete connection uri to the target YugabyteDB server")
}
