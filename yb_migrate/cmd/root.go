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
	"os"
	"strings"

	"github.com/yugabyte/ybm/yb_migrate/src/utils"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var (
	cfgFile       string
	exportDir     string
	migrationMode string
	startClean    bool
	logLevel      string
)

var log = utils.GetLogger()

var rootCmd = &cobra.Command{
	Use:   "yb_migrate",
	Short: "A tool to migrate a database to YugabyteDB",
	Long:  `Currently supports PostgreSQL, Oracle, MySQL. Soon support for DB2 and MSSQL will come`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		// fmt.Println("Root cmd PersistentPreRun")

		if !utils.FileOrFolderExists(exportDir) {
			fmt.Printf("Directory: %s doesn't exists!!\n", exportDir)
			os.Exit(1)
		} else if exportDir == "." {
			fmt.Println("Note: Using current working directory as export directory")
		} else {
			exportDir = strings.TrimRight(exportDir, "/") //cleaning the string
		}

	},

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("config = %s\nexportDir = %s\n", cfgFile, exportDir)
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

	rootCmd.PersistentFlags().StringVarP(&cfgFile, "config", "c", "",
		"config file (default is $HOME/.yb_migrate.yaml)")

	rootCmd.PersistentFlags().StringVarP(&logLevel, "log-level", "", "INFO",
		"Logging levels: TRACE, DEBUG, INFO, WARN")

	rootCmd.PersistentFlags().BoolVar(&source.VerboseMode, "verbose", false,
		"enable verbose mode for the console output")
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

		// Search config in home directory with name ".yb_migrate" (without extension).
		viper.AddConfigPath(home)
		viper.SetConfigType("yaml")
		viper.SetConfigName(".yb_migrate")
	}

	viper.AutomaticEnv() // read in environment variables that match

	// If a config file is found, read it in.
	if err := viper.ReadInConfig(); err == nil {
		fmt.Fprintln(os.Stderr, "Using config file:", viper.ConfigFileUsed())
	}
}
