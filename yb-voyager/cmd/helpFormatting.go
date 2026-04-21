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
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// subcommandGroup defines a named group of subcommands for help display.
type subcommandGroup struct {
	Title    string
	Commands []string
}

// parentHelpGroups maps parent command names to their subcommand groupings.
// Commands not listed in any group are appended at the end.
var parentHelpGroups = map[string][]subcommandGroup{
	"data": {
		{Title: "Migrate Data to Target", Commands: []string{"export", "import", "import-file"}},
		{Title: "Fall-back & Fall-forward", Commands: []string{"export-from-target", "import-to-source", "import-to-replica"}},
		{Title: "Utility", Commands: []string{"archive-changes", "status"}},
	},
	"schema": {
		{Title: "Available Commands", Commands: []string{"export", "analyze", "import", "finalize-post-data-import"}},
	},
	"cutover": {
		{Title: "Available Commands", Commands: []string{"prepare-target", "prepare-source", "prepare-replica", "status"}},
	},
}

// flagGroup defines a named group of flags for leaf command help display.
type flagGroup struct {
	Title   string
	Matcher func(f *pflag.Flag) bool
}

// flagGroupOrder defines the display order and grouping rules for flags.
// Flags are matched top-to-bottom; the first matching group wins.
var flagGroupOrder = []flagGroup{
	{
		Title: "Migration",
		Matcher: func(f *pflag.Flag) bool {
			return f.Name == "migration-name" || f.Name == "export-dir" || f.Name == "config-file"
		},
	},
	{
		Title: "Source Connection",
		Matcher: func(f *pflag.Flag) bool {
			return strings.HasPrefix(f.Name, "source-db-") || f.Name == "source-db-type"
		},
	},
	{
		Title: "Source SSL",
		Matcher: func(f *pflag.Flag) bool {
			return strings.HasPrefix(f.Name, "source-ssl-")
		},
	},
	{
		Title: "Target Connection",
		Matcher: func(f *pflag.Flag) bool {
			return strings.HasPrefix(f.Name, "target-db-") ||
				f.Name == "target-endpoints" || f.Name == "use-public-ip"
		},
	},
	{
		Title: "Target SSL",
		Matcher: func(f *pflag.Flag) bool {
			return strings.HasPrefix(f.Name, "target-ssl-")
		},
	},
	{
		Title: "Source Replica",
		Matcher: func(f *pflag.Flag) bool {
			return strings.HasPrefix(f.Name, "source-replica-")
		},
	},
	{
		Title: "Oracle",
		Matcher: func(f *pflag.Flag) bool {
			return strings.HasPrefix(f.Name, "oracle-")
		},
	},
	{
		Title: "Table Filtering",
		Matcher: func(f *pflag.Flag) bool {
			return strings.Contains(f.Name, "table-list") ||
				strings.Contains(f.Name, "object-type-list") ||
				strings.Contains(f.Name, "exclude-object-type")
		},
	},
	{
		Title: "Performance",
		Matcher: func(f *pflag.Flag) bool {
			return f.Name == "parallel-jobs" || f.Name == "batch-size" ||
				strings.HasPrefix(f.Name, "adaptive-parallelism")
		},
	},
	{
		Title: "General",
		Matcher: func(f *pflag.Flag) bool {
			return f.Name == "log-level" || f.Name == "yes" ||
				f.Name == "send-diagnostics" || f.Name == "start-clean" ||
				f.Name == "disable-pb"
		},
	},
}

// buildParentHelp renders styled help for commands that have subcommands.
func buildParentHelp(cmd *cobra.Command) string {
	rule := ruleStyle.Render(strings.Repeat("─", ruleWidth))
	var b strings.Builder

	b.WriteString(titleStyle.Render(cmd.Short) + "\n\n")
	b.WriteString("Usage:\n")
	b.WriteString(fmt.Sprintf("  %s [command]\n\n", cmd.CommandPath()))

	cmdMap := make(map[string]*cobra.Command)
	for _, c := range cmd.Commands() {
		if c.IsAvailableCommand() {
			cmdMap[c.Name()] = c
		}
	}

	groups, hasGroups := parentHelpGroups[cmd.Name()]
	if hasGroups {
		for i, group := range groups {
			b.WriteString(titleStyle.Render(group.Title) + "\n")
			b.WriteString(rule + "\n")
			for _, name := range group.Commands {
				if c, ok := cmdMap[name]; ok {
					short := firstLine(c.Short)
					b.WriteString(fmt.Sprintf("  %s %s\n", cmdStyle.Render(fmt.Sprintf("%-24s", c.Name())), short))
					delete(cmdMap, name)
				}
			}
			if i < len(groups)-1 {
				b.WriteString("\n")
			}
		}
		// Any remaining commands not covered by groups.
		remaining := make([]*cobra.Command, 0)
		for _, c := range cmd.Commands() {
			if _, ok := cmdMap[c.Name()]; ok {
				remaining = append(remaining, c)
			}
		}
		if len(remaining) > 0 {
			b.WriteString("\n" + titleStyle.Render("Other") + "\n")
			b.WriteString(rule + "\n")
			for _, c := range remaining {
				short := firstLine(c.Short)
				b.WriteString(fmt.Sprintf("  %s %s\n", cmdStyle.Render(fmt.Sprintf("%-24s", c.Name())), short))
			}
		}
	} else {
		b.WriteString(titleStyle.Render("Available Commands") + "\n")
		b.WriteString(rule + "\n")
		for _, c := range cmd.Commands() {
			if c.IsAvailableCommand() {
				short := firstLine(c.Short)
				b.WriteString(fmt.Sprintf("  %s %s\n", cmdStyle.Render(fmt.Sprintf("%-24s", c.Name())), short))
			}
		}
	}

	if cmd.HasAvailableLocalFlags() {
		b.WriteString(fmt.Sprintf("\nFlags:\n%s", cmd.LocalFlags().FlagUsages()))
	}
	b.WriteString(fmt.Sprintf("\nUse \"%s [command] --help\" for more information about a command.\n", cmd.CommandPath()))
	return b.String()
}

// buildLeafHelp renders styled help for leaf commands, with flags organized
// into named sections.
func buildLeafHelp(cmd *cobra.Command) string {
	rule := ruleStyle.Render(strings.Repeat("─", ruleWidth))
	var b strings.Builder

	if cmd.Long != "" {
		b.WriteString(cmd.Long + "\n\n")
	} else if cmd.Short != "" {
		b.WriteString(cmd.Short + "\n\n")
	}

	b.WriteString("Usage:\n")
	b.WriteString(fmt.Sprintf("  %s [flags]\n", cmd.CommandPath()))

	allFlags := cmd.Flags()
	if allFlags == nil || !allFlags.HasFlags() {
		return b.String()
	}

	// Classify each flag into a group.
	type classifiedFlag struct {
		flag  *pflag.Flag
		group int // index into flagGroupOrder, or len(flagGroupOrder) for "Options"
	}
	var classified []classifiedFlag

	allFlags.VisitAll(func(f *pflag.Flag) {
		if f.Hidden {
			return
		}
		groupIdx := len(flagGroupOrder) // default: "Options" catchall
		for i, g := range flagGroupOrder {
			if g.Matcher(f) {
				groupIdx = i
				break
			}
		}
		classified = append(classified, classifiedFlag{flag: f, group: groupIdx})
	})

	// Build a FlagSet per group.
	groupFlags := make(map[int]*pflag.FlagSet)
	for _, cf := range classified {
		if _, ok := groupFlags[cf.group]; !ok {
			groupFlags[cf.group] = pflag.NewFlagSet("", pflag.ContinueOnError)
		}
		groupFlags[cf.group].AddFlag(cf.flag)
	}

	// Render groups in order.
	for i, g := range flagGroupOrder {
		fs, ok := groupFlags[i]
		if !ok {
			continue
		}
		b.WriteString("\n" + titleStyle.Render(g.Title) + "\n")
		b.WriteString(rule + "\n")
		b.WriteString(fs.FlagUsages())
		delete(groupFlags, i)
	}

	// Render catchall "Options" group for any flags not matched above.
	catchallIdx := len(flagGroupOrder)
	if fs, ok := groupFlags[catchallIdx]; ok {
		b.WriteString("\n" + titleStyle.Render("Options") + "\n")
		b.WriteString(rule + "\n")
		b.WriteString(fs.FlagUsages())
	}

	return b.String()
}

// firstLine returns the first line of a potentially multi-line string.
func firstLine(s string) string {
	if i := strings.Index(s, "\n"); i >= 0 {
		return s[:i]
	}
	return s
}
