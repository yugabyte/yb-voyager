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
	"encoding/json"
	"fmt"
	"html/template"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	mat "github.com/yugabyte/yb-voyager/yb-voyager/src/mat/plugins"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var supportedPlugins = []string{"sharding", "sizer"}
var supportedMigrationReportFormats = []string{"json", "html"}

var pluginsList []string
var pluginParamsFpath string // TODO: clarity required
var assessmentReportFormat string
var metadataAndStatsDir string

var assessMigrationCmd = &cobra.Command{
	Use:   "assess-migration",
	Short: "Assess the migration from source database to YugabyteDB.",
	Long:  `Assess the migration from source database to YugabyteDB.`,

	Run: func(cmd *cobra.Command, args []string) {
		assessMigration()
	},
}

func init() {
	rootCmd.AddCommand(assessMigrationCmd)
	registerCommonGlobalFlags(assessMigrationCmd)

	assessMigrationCmd.Flags().StringSliceVar(&pluginsList, "plugins", []string{},
		fmt.Sprintf("List of plugins to be used for migration assessment (use 'all' for all supported plugins). Supported plugins are: %s.",
			strings.Join(supportedPlugins, ", ")))
	assessMigrationCmd.MarkFlagRequired("plugins")

	// TODO: implement Plugin Params flag

	assessMigrationCmd.Flags().StringVar(&assessmentReportFormat, "output-format", "json",
		fmt.Sprintf("Output format for migration assessment report. Supported formats are: %s.",
			strings.Join(supportedMigrationReportFormats, ", ")))

	// optional flag to take metadata and stats directory path in case it is not in exportDir
	assessMigrationCmd.Flags().StringVar(&metadataAndStatsDir, "metadata-and-stats-dir", "",
		"Directory path where metadata and stats are stored. Optional flag, if not provided, "+
			"it will be assumed to be present at default path inside the export directory.")

}

func assessMigration() {
	for _, pluginName := range pluginsList {
		plugin := mat.GetPlugin(pluginName)
		queryResults, err := mat.LoadQueryResults(pluginName, exportDir)
		if err != nil {
			utils.ErrExit("error loading query results for plugin '%s': %v", pluginName, err)
		}

		pluginParams, err := mat.LoadPluginParams(pluginName)
		if err != nil {
			utils.ErrExit("error loading plugin params for plugin '%s': %v", pluginName, err)
		}

		report, err := plugin.RunAssessment(queryResults, pluginParams)
		if err != nil {
			utils.ErrExit("error running assessment for plugin '%s': %v", pluginName, err)
		}

		reportOutput, err := generateReportOutput(report, plugin)
		if err != nil {
			utils.ErrExit("error generating report output for plugin '%s': %v", pluginName, err)
		}
		fmt.Printf("Migration assessment report for plugin '%s':\n%s\n", pluginName, reportOutput)
		// TODO: save the output in a file
	}
}

func generateReportOutput(report any, plugin mat.AssessmentPlugin) (string, error) {
	pluginName := plugin.GetName()
	switch assessmentReportFormat {
	case "json":
		jsonReport, err := json.MarshalIndent(report, "", " ")
		if err != nil {
			log.Errorf("error converting assessment report to JSON for plugin '%s': %v", pluginName, err)
			return "", fmt.Errorf("error converting assessment report to JSON for plugin '%s': %w", pluginName, err)
		}
		return string(jsonReport), nil
	case "html":
		// TODO: implement GetHtmlTemplate() method for plugins
		tmplFile := plugin.GetHtmlTemplate()
		tmpl, err := template.New(pluginName).Parse(tmplFile)
		if err != nil {
			log.Errorf("error parsing HTML template for plugin '%s': %v", pluginName, err)
			return "", fmt.Errorf("error parsing HTML template for plugin '%s': %w", pluginName, err)
		}
		var output strings.Builder
		err = tmpl.Execute(&output, report)
		if err != nil {
			log.Errorf("error generating HTML report for plugin '%s': %v", pluginName, err)
			return "", fmt.Errorf("error generating HTML report for plugin '%s': %w", pluginName, err)
		}
		return output.String(), nil
	default:
		log.Errorf("unsupported output format '%s' for migration assessment report", assessmentReportFormat)
		panic(fmt.Sprintf("unsupported output format '%s' for migration assessment report", assessmentReportFormat))
	}
}
