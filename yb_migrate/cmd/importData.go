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

// importDataCmd represents the importData command
var importDataCmd = &cobra.Command{
	Use:   "data",
	Short: "This command imports data into YugabyteDB database",
	Long:  `This command will import the data exported from the source database into YugabyteDB database.`,

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("import data called")
	},
}

func init() {
	importCmd.AddCommand(importDataCmd)

	importDataCmd.PersistentFlags().StringVar(&importMode, "offline", "",
		"By default the data migration mode is offline. Use '--mode online' to change the mode to online migration")
}
