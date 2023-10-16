/*
Copyright (c) YugabyteDB, Inc.

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

import "github.com/spf13/cobra"

var fallBackCmd = &cobra.Command{
	Use:   "fall-back",
	Short: "fall-back is used to setup and synchronize the fall back database",
	Long:  `Fall-back has three commands: setup, synchronize and switchover.`,
}

func init() {
	rootCmd.AddCommand(fallBackCmd)
}

func hideFlagsInFallBackCmds(cmd *cobra.Command) {
	var flags = []string{"target-db-type", "import-type", "source-db-type", "export-type", "target-endpoints", "use-public-ip"}
	for _, flagName := range flags {
		flag := cmd.Flags().Lookup(flagName)
		if flag != nil {
			flag.Hidden = true
		}
	}
}
