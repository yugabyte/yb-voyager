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
	"os"
	"path/filepath"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	mat "github.com/yugabyte/yb-voyager/yb-voyager/src/mat/plugins"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

var supportedPlugins = []string{"sharding"}
var supportedMigrationReportFormats = []string{"json", "html"}

var pluginsList []string
var userInputFpath string
var assessmentReportFormat string
var metadataAndStatsDir string

var assessMigrationCmd = &cobra.Command{
	Use:   "assess-migration",
	Short: "Assess the migration from source database to YugabyteDB.",
	Long:  `Assess the migration from source database to YugabyteDB.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validatePluginsList()
	},

	Run: func(cmd *cobra.Command, args []string) {
		assessMigration()
	},
}

func validatePluginsList() {
	if len(pluginsList) == 1 && pluginsList[0] == "all" {
		pluginsList = supportedPlugins
		return
	}
	for _, pluginName := range pluginsList {
		if !slices.Contains(supportedPlugins, pluginName) {
			utils.ErrExit("unsupported plugin '%s' specified for migration assessment", pluginName)
		}
	}
}

func init() {
	rootCmd.AddCommand(assessMigrationCmd)
	registerCommonGlobalFlags(assessMigrationCmd)

	assessMigrationCmd.Flags().StringSliceVar(&pluginsList, "plugins", []string{},
		fmt.Sprintf("List of plugins to be used for migration assessment (use 'all' for all supported plugins). Supported plugins are: %s.",
			strings.Join(supportedPlugins, ", ")))
	assessMigrationCmd.MarkFlagRequired("plugins")

	// TODO: clarity on whether this flag should be a mandatory or not
	assessMigrationCmd.Flags().StringVar(&userInputFpath, "user-input", "",
		"File path for user input to the plugins. This file should contain the user input for the plugins in TOML format.")

	assessMigrationCmd.Flags().StringVar(&assessmentReportFormat, "report-format", "json",
		fmt.Sprintf("Output format for migration assessment report. Supported formats are: %s.",
			strings.Join(supportedMigrationReportFormats, ", ")))

	// optional flag to take metadata and stats directory path in case it is not in exportDir
	assessMigrationCmd.Flags().StringVar(&metadataAndStatsDir, "metadata-and-stats-dir", "",
		"Directory path where metadata and stats are stored. Optional flag, if not provided, "+
			"it will be assumed to be present at default path inside the export directory.")

}

func assessMigration() {
	assessmentDirPath := filepath.Join(exportDir, "assessment", "reports")
	for _, pluginName := range pluginsList {
		plugin := mat.GetPlugin(pluginName)
		queryResults, err := mat.LoadQueryResults(pluginName, exportDir)
		if err != nil {
			utils.ErrExit("error loading query results for plugin '%s': %v", pluginName, err)
		}

		pluginParams, err := mat.LoadUserInput(pluginName, userInputFpath)
		if err != nil {
			utils.ErrExit("error loading user input for plugin '%s': %v", pluginName, err)
		}

		report, err := plugin.RunAssessment(queryResults, pluginParams)
		if err != nil {
			utils.ErrExit("error running assessment for plugin '%s': %v", pluginName, err)
		}

		reportOutput, err := generateReportOutput(report, plugin)
		if err != nil {
			utils.ErrExit("error generating report output for plugin '%s': %v", pluginName, err)
		}

		reportOutputFpath := filepath.Join(assessmentDirPath, fmt.Sprintf("%s.%s", pluginName, assessmentReportFormat))
		err = os.WriteFile(reportOutputFpath, []byte(reportOutput), 0644)
		if err != nil {
			utils.ErrExit("error writing migration assessment report for plugin '%s' to file: %v", pluginName, err)
		}
		utils.PrintAndLog("Migration assessment report for plugin '%s' written to file: %s", pluginName, reportOutputFpath)
	}

	if assessmentReportFormat == "html" {
		err := generateIndexHtmlFile(assessmentDirPath)
		if err != nil {
			utils.ErrExit("error generating index.html file for migration assessment reports: %v", err)
		}
	}
}

func generateReportOutput(report any, plugin mat.AssessmentPlugin) (string, error) {
	pluginName := plugin.GetName()
	switch assessmentReportFormat {
	case "json":
		jsonReport, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			log.Errorf("error converting assessment report to JSON for plugin '%s': %v", pluginName, err)
			return "", fmt.Errorf("error converting assessment report to JSON for plugin '%s': %w", pluginName, err)
		}
		return string(jsonReport), nil
	case "html":
		// TODO: implement GetHtmlTemplate() method for plugins
		tmplHtmlFile := plugin.GetHtmlTemplate()
		tmpl, err := template.New(pluginName).Funcs(tmplFuncs).Parse(tmplHtmlFile)
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

// Define a custom template function to split a string by a delimiter
var tmplFuncs = template.FuncMap{
	"split": func(s, sep string) []string {
		return strings.Split(s, sep)
	},
}

func generateIndexHtmlFile(assessmentDirPath string) error {
	// Open or create the index.html file
	indexFilePath := filepath.Join(assessmentDirPath, "index.html")
	indexFile, err := os.Create(indexFilePath)
	if err != nil {
		return err
	}
	defer indexFile.Close()

	htmlContent := `
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>Migration Assessment Reports</title>
<style>
    body {
        font-family: Arial, sans-serif;
        margin: 0;
        padding: 0;
        background-color: #f4f4f4;
    }
    .container {
        max-width: 800px;
        margin: 20px auto;
        padding: 20px;
        background-color: #fff;
        border-radius: 5px;
        box-shadow: 0 0 10px rgba(0, 0, 0, 0.1);
    }
    h1 {
        text-align: center;
        color: #333;
        margin-bottom: 20px;
    }
    h2 {
        font-size: 1.2em;
        margin-bottom: 10px;
        text-align: center;
    }
    ul {
        list-style: none;
        padding: 0;
        margin: 0;
    }
    li {
        font-size: 1.2em; /* Adjust font size for plugin reports */
        margin-bottom: 10px;
    }
    a {
        text-decoration: none;
        color: #007bff;
    }
</style>
</head>
<body>
<div class="container">
    <h1>Migration Assessment Reports</h1>
    <ul>
`

	for _, pluginName := range pluginsList {
		// using relative path for plugins report, as the whole report directory can be shared or moved to different locations
		pluginReportFileRelpath := fmt.Sprintf("%s.%s", pluginName, assessmentReportFormat)
		htmlContent += fmt.Sprintf("<li>&#8226; %s Report (<a href=\"%s\">&#x2197;</a>)</li>\n",
			cases.Title(language.English).String(pluginName), pluginReportFileRelpath)
	}
	// Close the <ul> and <div> tags
	htmlContent += "</ul></div></body></html>"

	if _, err := indexFile.WriteString(htmlContent); err != nil {
		return err
	}

	utils.PrintAndLog("Index HTML file for migration assessment reports written to: %s", indexFilePath)
	return nil
}
