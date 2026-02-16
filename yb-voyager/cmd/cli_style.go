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
	"os"
	"path/filepath"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// ruleWidth is the length of the thin rule under section titles.
const ruleWidth = 50

// Lipgloss styles used across init and start-migration commands.
var (
	titleStyle = lipgloss.NewStyle().
			Bold(true)

	ruleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("240"))

	dimStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("242"))

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("2")) // green

	warnStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("3")) // yellow

	cmdStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("6")) // cyan

	nextStepLabelStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("6")) // cyan, matching cmdStyle
)

// printSection prints a titled section: bold title, thin rule, then body lines.
// Each line in body is indented by 2 spaces.
func printSection(title string, bodyLines ...string) {
	fmt.Println()
	fmt.Println("  " + titleStyle.Render(title))
	fmt.Println("  " + ruleStyle.Render(strings.Repeat("─", ruleWidth)))
	for _, line := range bodyLines {
		if line == "" {
			fmt.Println()
		} else {
			fmt.Println("  " + line)
		}
	}
}

// displayPath returns a relative path (prefixed with ./) when the path is under
// the current working directory. Falls back to the absolute path otherwise.
func displayPath(absPath string) string {
	cwd, err := os.Getwd()
	if err != nil {
		return absPath
	}
	rel, err := filepath.Rel(cwd, absPath)
	if err != nil || strings.HasPrefix(rel, "..") {
		return absPath
	}
	return "./" + rel
}

// successLine returns a green "✓ text" string.
func successLine(text string) string {
	return successStyle.Render("✓") + " " + text
}

// formatKeyValue formats a "Key:  Value" pair with the key right-padded to a fixed width.
// It uses lipgloss.Width to measure the visible width so ANSI-styled keys align correctly.
func formatKeyValue(key, value string, keyWidth int) string {
	pad := keyWidth - lipgloss.Width(key)
	if pad < 0 {
		pad = 0
	}
	return key + strings.Repeat(" ", pad) + " " + value
}

// Phase progress markers for migration status display.
func phaseDoneMarker() string {
	return successStyle.Render("✓")
}

func phaseActiveMarker() string {
	return warnStyle.Render(">")
}

func phasePendingMarker() string {
	return dimStyle.Render("·")
}
