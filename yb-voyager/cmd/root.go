/*
Copyright (c) YugaByte, Inc.

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
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	cfgFile       string
	exportDir     string
	migrationMode string
	startClean    bool
	logLevel      string
)

var rootCmd = &cobra.Command{
	Use:   "yb-voyager",
	Short: "A tool to migrate a database to YugabyteDB",
	Long:  `Currently supports PostgreSQL, Oracle, MySQL. Soon support for DB2 and MSSQL will come`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if exportDir != "" && utils.FileOrFolderExists(exportDir) {
			InitLogging(exportDir)
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	cobra.CheckErr(rootCmd.Execute())
}

func init() {
	cobra.OnInitialize(initConfig)

	// Here you will define your flags and configuration settings.
	// Cobra supports persistent flags, which, if defined here,
	// will be global for your application.
	rootCmd.CompletionOptions.DisableDefaultCmd = true

	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", "INFO",
		"Logging levels: TRACE, DEBUG, INFO, WARN")

	rootCmd.PersistentFlags().BoolVar(&source.VerboseMode, "verbose", false,
		"enable verbose mode for the console output")

	rootCmd.PersistentFlags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory to keep all the dump files and metainfo")

	rootCmd.PersistentFlags().BoolVarP(&utils.DoNotPrompt, "yes", "y", false,
		"assume answer as yes for all questions during migration (default false)")

	rootCmd.PersistentFlags().BoolVar(&callhome.SendDiagnostics, "send-diagnostics", true,
		"enable or disable the 'send-diagnostics' feature that sends analytics data to Yugabyte.")
	callhome.ReadEnvSendDiagnostics()
}

// initConfig reads in config file and ENV variables if set.
func initConfig() {
	if cfgFile != "" {
		// Use config file from the flag.
		viper.SetConfigFile(cfgFile)
	} else {
		// Find home directory.
		home, err := os.UserHomeDir()
		cobra.CheckErr(err)

		// Search config in home directory with name ".yb-voyager" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".yb-voyager")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}

func validateExportDirFlag() {
	if exportDir == "" {
		fmt.Fprintln(os.Stderr, `ERROR: required flag "export-dir" not set`)
		os.Exit(1)
	}
	if !utils.FileOrFolderExists(exportDir) {
		fmt.Fprintf(os.Stderr, "Directory %q doesn't exists.\n", exportDir)
		os.Exit(1)
	} else if exportDir == "." {
		fmt.Println("Note: Using current working directory as export directory")
	} else {
		exportDir = strings.TrimRight(exportDir, "/")
	}
}
