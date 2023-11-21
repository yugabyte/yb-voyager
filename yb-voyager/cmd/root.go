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

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"golang.org/x/exp/slices"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/lockfile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var (
	cfgFile       string
	exportDir     string
	startClean    utils.BoolStr
	lockFile      *lockfile.Lockfile
	migrationUUID uuid.UUID
	perfProfile   utils.BoolStr
)

var rootCmd = &cobra.Command{
	Use:   "yb-voyager",
	Short: "A CLI based migration engine to migrate complete database(schema + data) from some source database to YugabyteDB",
	Long: `A CLI based migration engine for complete database migration from a source database to YugabyteDB. Currently supported source databases are Oracle, MySQL, PostgreSQL.
Refer to docs (https://docs.yugabyte.com/preview/migrate/) for more details like setting up source/target, migration workflow etc.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		if cmd.CommandPath() == "yb-voyager" || cmd.CommandPath() == "yb-voyager version" {
			return
		}
		validateExportDirFlag()
		if shouldLock(cmd) {
			lockFPath := filepath.Join(exportDir, fmt.Sprintf(".%sLockfile.lck", GetCommandID(cmd)))
			lockFile = lockfile.NewLockfile(lockFPath)
			lockFile.Lock()
		}
		InitLogging(exportDir, cmd.Use == "status", GetCommandID(cmd))
		if metaDBIsCreated(exportDir) {
			initMetaDB()
		}
		if perfProfile {
			go startPprofServer()
		}

	},

	Run: func(cmd *cobra.Command, args []string) {
		if len(args) == 0 {
			cmd.Help()
			os.Exit(0)
		}
	},

	PersistentPostRun: func(cmd *cobra.Command, args []string) {
		if shouldLock(cmd) {
			lockFile.Unlock()
		}
	},
}

func startPprofServer() {
	// Server for pprof
	err := http.ListenAndServe("localhost:6060", nil)
	if err != nil {
		fmt.Println("Error starting pprof server")
	}
	/*
		Steps to use pprof for profiling yb-voyager:
		1. install graphviz on the machine using - sudo yum install graphviz gv
		2. start voyager with profile flag - yb-voyager ... --profile true
		3. use the following command to start a web ui for the profile data-
			go tool pprof -http=[<client_machine_ip>:<port>] http://localhost:6060/debug/pprof/profile
	*/
}

var noLockNeededList = []string{
	"yb-voyager version",
	"yb-voyager import data status",
	"yb-voyager export data status",
	"yb-voyager cutover status",
	"yb-voyager get data-migration-report",
}

func shouldLock(cmd *cobra.Command) bool {
	return !slices.Contains(noLockNeededList, cmd.CommandPath())
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
	BoolVar(cmd.Flags(), &perfProfile, "profile", false,
		"profile yb-voyager for performance analysis")
	cmd.Flags().MarkHidden("profile")

	cmd.PersistentFlags().StringVarP(&exportDir, "export-dir", "e", "",
		"export directory is the workspace used to keep the exported schema, data, state, and logs")

	cmd.PersistentFlags().BoolVarP(&utils.DoNotPrompt, "yes", "y", false,
		"assume answer as yes for all questions during migration (default false)")

	BoolVar(cmd.Flags(), &callhome.SendDiagnostics, "send-diagnostics", true,
		"enable or disable the 'send-diagnostics' feature that sends analytics data to YugabyteDB.")
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
	} else {
		if exportDir == "." {
			fmt.Println("Note: Using current directory as export-dir")
		}
		var err error
		exportDir, err = filepath.Abs(exportDir)
		if err != nil {
			utils.ErrExit("Failed to get absolute path for export-dir %q: %v\n", exportDir, err)
		}
		exportDir = filepath.Clean(exportDir)
	}
}

func GetCommandID(c *cobra.Command) string {
	if c.HasParent() {
		p := GetCommandID(c.Parent())
		if p == "" {
			return c.Name()
		} else {
			return p + "-" + c.Name()
		}
	}
	return ""
}

func BoolVar(flagSet *pflag.FlagSet, p *utils.BoolStr, name string, value bool, usage string) {
	*p = utils.BoolStr(value)
	flagSet.AddFlag(&pflag.Flag{
		Name:     name,
		Usage:    usage + "; accepted values (true, false, yes, no, 0, 1)",
		Value:    p,
		DefValue: fmt.Sprintf("%t", value),
	})
}

func metaDBIsCreated(exportDir string) bool {
	return utils.FileOrFolderExists(filepath.Join(exportDir, "metainfo", "meta.db"))
}
