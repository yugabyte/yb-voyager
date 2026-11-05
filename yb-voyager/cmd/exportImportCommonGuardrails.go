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
	"slices"
	"strings"

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// perTableFlagEntry is one '<table>:<value>' entry of a ';'-separated per-table flag such as
// --exclude-column-list or --cdc-partition-key-overrides.
type perTableFlagEntry struct {
	table string
	value string
}

// parsePerTableFlag splits flagValue into its entries, in the order given. Separators inside double
// quotes belong to the name, so quoted identifiers may contain ';', ':' or ','.
func parsePerTableFlag(flagName, flagValue, entryFormat string) ([]perTableFlagEntry, error) {
	if strings.TrimSpace(flagValue) == "" {
		return nil, nil
	}
	if strings.Count(flagValue, `"`)%2 != 0 {
		return nil, goerrors.Errorf("invalid --%s %q: unbalanced double quotes", flagName, flagValue)
	}
	var entries []perTableFlagEntry
	for _, rawEntry := range splitOutsideQuotes(flagValue, ';') {
		rawEntry = strings.TrimSpace(rawEntry)
		if rawEntry == "" {
			continue
		}
		parts := splitOutsideQuotes(rawEntry, ':')
		if len(parts) != 2 {
			return nil, goerrors.Errorf("invalid --%s entry %q: expected format %s", flagName, rawEntry, entryFormat)
		}
		entry := perTableFlagEntry{table: strings.TrimSpace(parts[0]), value: strings.TrimSpace(parts[1])}
		if entry.table == "" || entry.value == "" {
			return nil, goerrors.Errorf("invalid --%s entry %q: table and value must both be non-empty, expected format %s", flagName, rawEntry, entryFormat)
		}
		if slices.ContainsFunc(entries, func(e perTableFlagEntry) bool { return e.table == entry.table }) {
			return nil, goerrors.Errorf("invalid --%s: duplicate table %q", flagName, entry.table)
		}
		entries = append(entries, entry)
	}
	if len(entries) == 0 {
		return nil, goerrors.Errorf("invalid --%s %q: no entries, expected format %s", flagName, flagValue, entryFormat)
	}
	return entries, nil
}

// parseColumnNameList splits a ','-separated column list. normalize maps each raw name to the form
// it is compared and returned in, so that two spellings of one column count as a duplicate.
func parseColumnNameList(columnList string) ([]string, error) {
	var columns []string
	for _, rawColumn := range splitOutsideQuotes(columnList, ',') {
		rawColumn = strings.TrimSpace(rawColumn)
		if rawColumn == "" {
			return nil, goerrors.Errorf("empty column name")
		}
		if slices.Contains(columns, rawColumn) {
			return nil, goerrors.Errorf("duplicate column %q", rawColumn)
		}
		columns = append(columns, rawColumn)
	}
	return columns, nil
}

func splitOutsideQuotes(s string, sep rune) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	for _, r := range s {
		switch {
		case r == '"':
			inQuotes = !inQuotes
			current.WriteRune(r)
		case r == sep && !inQuotes:
			parts = append(parts, current.String())
			current.Reset()
		default:
			current.WriteRune(r)
		}
	}
	return append(parts, current.String())
}

// lookupPerTableFlagTables resolves each table named in a per-table flag. Instead of stopping at the
// first bad table, it returns a problem for every table that can't be found or that is the same
// table as one named earlier under another spelling, so that the user can fix them all at once.
func lookupPerTableFlagTables(tableSpecs []string, lookupTable func(string) (sqlname.NameTuple, error)) (map[string]sqlname.NameTuple, []string) {
	resolved := make(map[string]sqlname.NameTuple, len(tableSpecs))
	var problems []string
	for _, spec := range tableSpecs {
		table, err := lookupTable(spec)
		if err != nil {
			log.Infof("lookup of table %q: %v", spec, err)
			problems = append(problems, fmt.Sprintf("%s: table not found", spec))
			continue
		}
		earlierSpec, found := lo.FindKeyBy(resolved, func(_ string, t sqlname.NameTuple) bool { return t.Key() == table.Key() })
		if found {
			problems = append(problems, fmt.Sprintf("%s: table %s is already listed as %s", spec, table.ForOutput(), earlierSpec))
			continue
		}
		resolved[spec] = table
	}
	return resolved, problems
}

func perTableFlagProblemsError(flagName string, problems []string) error {
	return goerrors.Errorf("invalid --%s:\n  %s", flagName, strings.Join(problems, "\n  "))
}
