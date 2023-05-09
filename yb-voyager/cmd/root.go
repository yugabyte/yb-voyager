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
	"path/filepath"
	"strings"

	"github.com/nightlyone/lockfile"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	cfgFile    string
	exportDir  string
	startClean bool
	lockFile   lockfile.Lockfile
)

var rootCmd = &cobra.Command{
	Use:   "yb-voyager",
	Short: "A CLI based migration engine to migrate complete database(schema + data) from some source database to YugabyteDB",
	Long: `A CLI based migration engine for complete database migration from a source database to YugabyteDB. Currently supported source databases are Oracle, MySQL, PostgreSQL.
Refer to docs (https://docs.yugabyte.com/preview/migrate/) for more details like setting up source/target, migration workflow etc.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {

		if exportDir != "" && utils.FileOrFolderExists(exportDir) {
			if cmd.Use != "version" && cmd.Use != "status" {
				lockExportDir(cmd)
			}
			InitLogging(exportDir, cmd.Use == "status")
		}
	},

	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},

	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if exportDir != "" && utils.FileOrFolderExists(exportDir) && cmd.Use != "version" && cmd.Use != "status" {
			unlockExportDir()
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

	callhome.ReadEnvSendDiagnostics()
}

func registerCommonGlobalFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(&source.VerboseMode, "verbose", false,
		"enable verbose mode for the console output")

	cmd.PersistentFlags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")

	cmd.PersistentFlags().BoolVarP(&utils.DoNotPrompt, "yes", "y", false,
		"assume answer as yes for all questions during migration (default false)")

	cmd.PersistentFlags().BoolVar(&callhome.SendDiagnostics, "send-diagnostics", true,
		"enable or disable the 'send-diagnostics' feature that sends analytics data to Yugabyte.")
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
		utils.ErrExit(`ERROR: required flag "export-dir" not set`)
	}
	if !utils.FileOrFolderExists(exportDir) {
		utils.ErrExit("export-dir %q doesn't exists.\n", exportDir)
	} else if exportDir == "." {
		fmt.Println("Note: Using current working directory as export directory")
	} else {
		exportDir = strings.TrimRight(exportDir, "/")
	}
}

func lockExportDir(cmd *cobra.Command) {
	lockFileName := ".lockfile.lck"
	// using different lockfile as import data can be run in parallel with export data(for live migration)
	if cmd.Use == "data" && cmd.Parent().Use == "import" {
		lockFileName = ".importDataLockfile.lck"
	}

	lockFilePath, err := filepath.Abs(filepath.Join(exportDir, lockFileName))
	if err != nil {
		utils.ErrExit("Failed to get absolute path for lockfile %q: %v\n", lockFileName, err)
	}
	createLock(lockFilePath)
}

func createLock(lockFileName string) {
	var err error
	lockFile, err = lockfile.New(lockFileName)
	if err != nil {
		utils.ErrExit("Failed to create lockfile %q: %v\n", lockFileName, err)
	}

	err = lockFile.TryLock()
	if err == nil {
		return
	} else if err == lockfile.ErrBusy {
		utils.ErrExit("Another instance of yb-voyager is running in the export-dir = %s\n", exportDir)
	} else {
		utils.ErrExit("Unable to lock the export-dir: %v\n", err)
	}
}

func unlockExportDir() {
	err := lockFile.Unlock()
	if err != nil {
		utils.ErrExit("Unable to unlock %q: %v\n", lockFile, err)
	}
}
