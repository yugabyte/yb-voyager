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
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schema/schemadrift"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ux"
)

// ─── Command definition ──────────────────────────────────────────────────────

// DRIFT_REPORT_FILE_NAME is the basename (without extension) of the report
// files written under <export-dir>/reports/ by `schema detect-drift`.
const DRIFT_REPORT_FILE_NAME = "drift_analysis_report"

var (
	driftOutputFormat          string
	driftTableList             string
	driftExcludeTableList      string
	driftObjectTypeList        string
	driftExcludeObjectTypeList string
)

// driftParsedFlags holds what validateDetectDriftFlags resolved from the raw flag
// strings, so the scope step reads a value instead of re-parsing and discarding
// the error on the strength of "validation already ran".
var driftParsedFlags struct {
	objectTypes        []schemadiff.ObjectType
	excludeObjectTypes []schemadiff.ObjectType
}

var driftValidOutputFormats = []string{"html", "json"}

// driftObjectTypesByName maps the --object-type-list vocabulary onto
// schemadiff.ObjectType. Deliberately smaller than export/analyze-schema's
// --object-type-list: the engine only emits TABLE and COLUMN findings in v1.
// COLUMN is its own selector, not swept in under TABLE.
var driftObjectTypesByName = map[string]schemadiff.ObjectType{
	"TABLE":  schemadiff.ObjectTypeTable,
	"COLUMN": schemadiff.ObjectTypeColumn,
}

// allDriftObjectTypes is the full v1 object-type universe, used to resolve
// --exclude-object-type-list into its complement (see complementDriftObjectTypes).
var allDriftObjectTypes = []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn}

var detectDriftCmd = &cobra.Command{
	Use:   "detect-drift",
	Short: "[TECH PREVIEW] Report source schema changes made during the migration, and what to do about each one",
	Long: `[TECH PREVIEW] Reports how the PostgreSQL source schema changed while the migration was running, and
what to do about each change.

Voyager records a schema snapshot at each migration milestone -- export schema, export data
start, periodically during export data, and export data exit. This command diffs consecutive
snapshots, plus a final comparison against a live read of the source, and writes the result
to <export-dir>/reports/. It is read-only: it never modifies migration state and never
applies anything on the target, it only writes report files.

Snapshots are recorded by default. A migration whose export commands ran with
--disable-schema-snapshot-capture=true has no history, and this command fails rather than
reporting a misleading "no drift".

The report groups each change by the interval between the two captures that bracket it, and
labels the interval with what Voyager was doing at the time (for example "export data:
running"). Every change carries a severity, what the migration will do if the change is not
reconciled on the target, and the corrective step.

Exit codes: 0 = the report was written, whether or not it found drift (a script reads
summary.change_count in the JSON report); 1 = error (bad flags, unreachable source,
unsupported source type, etc.).`,

	PreRun: func(cmd *cobra.Command, args []string) {
		resolveDetectDriftFlagDefaults()
		validateDetectDriftFlags()
		// Resolve the source password from the --source-db-password flag, the
		// SOURCE_DB_PASSWORD env var, or an interactive prompt -- exactly as the
		// export commands do. Without this, source.Password stays empty and the
		// connect in detectDrift() fails SASL auth against any password-protected
		// source.
		getAndStoreSourceDBPasswordInSourceConf(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		if err := detectDrift(); err != nil {
			utils.ErrExit("%w", err)
		}
	},
}

func init() {
	schemaCmd.AddCommand(detectDriftCmd)
	registerCommonGlobalFlags(detectDriftCmd)
	// PostgreSQL-only: registerOracleFlags=false, includeOracleCDBFlags=false so no
	// Oracle-specific flags (SID/home/TNS/CDB) are registered on this command.
	registerSourceDBConnFlags(detectDriftCmd, false, false)
	mustMarkFlagRequired(detectDriftCmd, "source-db-user")
	mustMarkFlagRequired(detectDriftCmd, "source-db-name")
	mustMarkFlagRequired(detectDriftCmd, "source-db-schema")
	// registerSourceDBConnFlags's help text is shared with multi-engine commands;
	// override it here since detect-drift is PostgreSQL-only.
	if f := detectDriftCmd.Flags().Lookup("source-db-type"); f != nil {
		f.Usage = "source database type: (postgresql). Defaults to postgresql; detect-drift does not support other source types yet."
	}
	if f := detectDriftCmd.Flags().Lookup("source-db-port"); f != nil {
		f.Usage = "source database server port number. Default: PostgreSQL(5432)"
	}

	detectDriftCmd.Flags().StringVar(&driftOutputFormat, "output-format", "",
		"format in which the report is generated: ('html', 'json'). If not provided, reports are generated in both 'html' and 'json' formats.")

	// KNOWN GAP: for a table renamed inside the compared window, neither name returns
	// its full history; an unfiltered run shows both findings. See the KNOWN GAP note
	// on schemadiff.FilterByScope.
	detectDriftCmd.Flags().StringVar(&driftTableList, "table-list", "",
		"comma-separated list of the tables to compare (glob patterns allowed). Only one of --table-list and --exclude-table-list can be specified.")
	detectDriftCmd.Flags().StringVar(&driftExcludeTableList, "exclude-table-list", "",
		"comma-separated list of the tables to exclude from comparison (glob patterns allowed). Only one of --table-list and --exclude-table-list can be specified.")

	detectDriftCmd.Flags().StringVar(&driftObjectTypeList, "object-type-list", "",
		"comma-separated list of object types to compare: (TABLE, COLUMN). Only one of --object-type-list and --exclude-object-type-list can be specified.")
	detectDriftCmd.Flags().StringVar(&driftExcludeObjectTypeList, "exclude-object-type-list", "",
		"comma-separated list of object types to exclude from comparison: (TABLE, COLUMN). Only one of --object-type-list and --exclude-object-type-list can be specified.")
}

// ─── Flags: defaults, validation, parsing ────────────────────────────────────

// resolveDetectDriftFlagDefaults fills in what the user did not pass and rewrites
// the list flags into their canonical form. It is the only place that WRITES flag
// state; validateDetectDriftFlags below only reads.
//
// A list flag that is empty once trimmed ("  ", ",") is the user passing nothing.
// Left as-is it would count as set and resolve to an empty keep-set, which Scope
// reads as "keep nothing": every finding dropped, reported as no drift.
func resolveDetectDriftFlagDefaults() {
	if source.DBType == "" {
		source.DBType = POSTGRESQL
	}
	setSourceDefaultPort()
	setDefaultSSLMode()

	source.SchemaConfig = normalizeDriftListFlag(source.SchemaConfig)
	driftTableList = normalizeDriftListFlag(driftTableList)
	driftExcludeTableList = normalizeDriftListFlag(driftExcludeTableList)
	driftObjectTypeList = normalizeDriftListFlag(driftObjectTypeList)
	driftExcludeObjectTypeList = normalizeDriftListFlag(driftExcludeObjectTypeList)
}

// validateDetectDriftFlags runs all flag-only validation (no DB connection
// required) and stores the parsed object-type lists for detectDrift to use, so
// the parse is not repeated later with its error discarded.
//
// Table-list validation (whether a pattern actually matches a table) needs the
// source's table list and so happens later, inside resolveDriftScope.
func validateDetectDriftFlags() {
	if source.DBType != POSTGRESQL {
		utils.ErrExit("schema detect-drift currently supports PostgreSQL sources only (got --source-db-type=%q)", source.DBType)
	}

	if driftTableList != "" && driftExcludeTableList != "" {
		utils.ErrExit("--table-list and --exclude-table-list are mutually exclusive. Use only one of them.")
	}
	if driftObjectTypeList != "" && driftExcludeObjectTypeList != "" {
		utils.ErrExit("--object-type-list and --exclude-object-type-list are mutually exclusive. Use only one of them.")
	}

	if err := validateDriftOutputFormat(driftOutputFormat); err != nil {
		utils.ErrExit("%w", err)
	}

	var err error
	if driftParsedFlags.objectTypes, err = parseDriftObjectTypeList(driftObjectTypeList); err != nil {
		utils.ErrExit("invalid --object-type-list: %v", err)
	}
	if driftParsedFlags.excludeObjectTypes, err = parseDriftObjectTypeList(driftExcludeObjectTypeList); err != nil {
		utils.ErrExit("invalid --exclude-object-type-list: %v", err)
	}
}

// validateDriftOutputFormat accepts "" (both formats) or exactly one of
// driftValidOutputFormats.
func validateDriftOutputFormat(format string) error {
	if format == "" || lo.Contains(driftValidOutputFormats, strings.ToLower(format)) {
		return nil
	}
	return goerrors.Errorf("invalid report output format: %s. Supported formats are %v", format, driftValidOutputFormats)
}

// driftReportFormats returns the formats to write for an --output-format value
// already accepted by validateDriftOutputFormat.
func driftReportFormats(format string) []string {
	if format == "" {
		return driftValidOutputFormats
	}
	return []string{strings.ToLower(format)}
}

// normalizeDriftListFlag returns raw with its entries trimmed, or "" when nothing
// is left -- so a whitespace-only or comma-only value is indistinguishable from an
// unset flag everywhere downstream.
func normalizeDriftListFlag(raw string) string {
	return strings.Join(utils.CsvStringToSlice(raw), ",")
}

// parseDriftObjectTypeList parses a comma-separated --object-type-list /
// --exclude-object-type-list value into schemadiff.ObjectType values. An empty
// string is not an error: it returns (nil, nil), meaning "no filter".
func parseDriftObjectTypeList(raw string) ([]schemadiff.ObjectType, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	var out []schemadiff.ObjectType
	var invalid []string
	for _, p := range utils.CsvStringToSlice(raw) {
		ot, ok := driftObjectTypesByName[strings.ToUpper(p)]
		if !ok {
			invalid = append(invalid, p)
			continue
		}
		out = append(out, ot)
	}
	if len(invalid) > 0 {
		return nil, goerrors.Errorf("unknown object type(s) %v; supported types: TABLE, COLUMN", invalid)
	}
	return out, nil
}

// ─── Scope resolution ────────────────────────────────────────────────────────

// driftTableUniverse builds the --table-list / --exclude-table-list matching
// universe, deduped per schema, in the order live catalog -> snapshots -> live
// capture.
//
// It is the union rather than the live catalog alone because a table dropped from
// the source is gone from the catalog but must still be nameable, to see its own
// drop reported. Only the stored snapshots still know about it.
func driftTableUniverse(listTables func(schema string) ([]string, error), schemas []string,
	snapshotContents []*schemasnapshot.SnapshotContent, liveContent *schemasnapshot.SnapshotContent) (map[string][]string, error) {
	seen := make(map[string]map[string]bool)
	universe := make(map[string][]string)
	add := func(schema, name string) {
		if seen[schema] == nil {
			seen[schema] = make(map[string]bool)
		}
		if seen[schema][name] {
			return
		}
		seen[schema][name] = true
		universe[schema] = append(universe[schema], name)
	}

	for _, schema := range schemas {
		names, err := listTables(schema)
		if err != nil {
			return nil, fmt.Errorf("list the tables in schema %q: %w", schema, err)
		}
		for _, name := range names {
			add(schema, name)
		}
	}
	for _, c := range snapshotContents {
		if c == nil {
			continue // placeholder / failed-to-load snapshot; nothing to contribute.
		}
		for _, t := range c.Tables {
			add(t.Schema, t.Name)
		}
	}
	if liveContent != nil {
		for _, t := range liveContent.Tables {
			add(t.Schema, t.Name)
		}
	}

	return universe, nil
}

// driftObjectRefs takes the unquoted source-side names, which is how schemadiff.Scope
// and the snapshots identify a table.
func driftObjectRefs(tuples []sqlname.NameTuple) []schemasnapshot.ObjectRef {
	return lo.Map(tuples, func(t sqlname.NameTuple, _ int) schemasnapshot.ObjectRef {
		return schemasnapshot.ObjectRef{Schema: t.SourceName.SchemaName.Unquoted, Name: t.SourceName.Unqualified.Unquoted}
	})
}

// requireDriftTableListQualified rejects a --table-list / --exclude-table-list pattern
// that is not schema-qualified when the registry has no default schema. Without this,
// an unqualified pattern would silently match nothing (see ObjectName.MatchesPattern)
// and get reported as an unknown table, rather than telling the user why.
func requireDriftTableListQualified(patternList, flagName string, hasDefaultSchema bool) error {
	if hasDefaultSchema {
		return nil
	}
	for _, pattern := range utils.CsvStringToSlice(patternList) {
		if !strings.Contains(pattern, ".") {
			return goerrors.Errorf("--%s entry %q is not schema-qualified, and --source-db-schema names no "+
				"default schema (no \"public\"); write it as schema.table", flagName, pattern)
		}
	}
	return nil
}

// driftPartitionChildren maps a partitioned table to every partition child recorded
// against it in any content, live or historical. It is a union across sources: a
// partition dropped since a given snapshot must still expand, so its parent's
// children list from every content is merged and deduped.
func driftPartitionChildren(snapshotContents []*schemasnapshot.SnapshotContent, liveContent *schemasnapshot.SnapshotContent) map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef {
	children := make(map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef)
	seen := make(map[schemasnapshot.ObjectRef]map[schemasnapshot.ObjectRef]bool)
	add := func(c *schemasnapshot.SnapshotContent) {
		if c == nil {
			return
		}
		for _, t := range c.Tables {
			for _, child := range t.PartitionChildren {
				if seen[t.ObjectRef] == nil {
					seen[t.ObjectRef] = make(map[schemasnapshot.ObjectRef]bool)
				}
				if seen[t.ObjectRef][child] {
					continue
				}
				seen[t.ObjectRef][child] = true
				children[t.ObjectRef] = append(children[t.ObjectRef], child)
			}
		}
	}
	for _, c := range snapshotContents {
		add(c)
	}
	add(liveContent)
	return children
}

// expandDriftPartitions expands each ref matched by --table-list / --exclude-table-list
// into itself plus every partition descendant at every level, depth-first, deduped via
// seen so a cycle in the recorded hierarchy cannot recurse forever.
func expandDriftPartitions(refs []schemasnapshot.ObjectRef, children map[schemasnapshot.ObjectRef][]schemasnapshot.ObjectRef) []schemasnapshot.ObjectRef {
	var out []schemasnapshot.ObjectRef
	seen := make(map[schemasnapshot.ObjectRef]bool)
	var visit func(ref schemasnapshot.ObjectRef)
	visit = func(ref schemasnapshot.ObjectRef) {
		if seen[ref] {
			return
		}
		seen[ref] = true
		out = append(out, ref)
		for _, child := range children[ref] {
			visit(child)
		}
	}
	for _, ref := range refs {
		visit(ref)
	}
	return out
}

// complementDriftTableRefs returns every universe ref NOT present in exclude -- the
// resolution of --exclude-table-list into the single positive allow-list the
// collapsed schemadiff.Scope expects.
//
// An EMPTY result means the user excluded the whole universe. Callers MUST
// reject it rather than forward it: Scope keeps nothing for an empty dimension,
// so the run would compare nothing and report a clean bill of health.
func complementDriftTableRefs(universe []schemasnapshot.ObjectRef, exclude []schemasnapshot.ObjectRef) []schemasnapshot.ObjectRef {
	excludeSet := make(map[schemasnapshot.ObjectRef]bool, len(exclude))
	for _, r := range exclude {
		excludeSet[r] = true
	}
	var out []schemasnapshot.ObjectRef
	for _, r := range universe {
		if !excludeSet[r] {
			out = append(out, r)
		}
	}
	return out
}

// complementDriftObjectTypes returns every type in allDriftObjectTypes NOT
// present in exclude -- the resolution of --exclude-object-type-list into the
// single positive allow-list the collapsed schemadiff.Scope expects.
//
// As with complementDriftTableRefs, an EMPTY result must be rejected by the
// caller, not forwarded to Scope, which would then keep nothing.
func complementDriftObjectTypes(exclude []schemadiff.ObjectType) []schemadiff.ObjectType {
	excludeSet := make(map[schemadiff.ObjectType]bool, len(exclude))
	for _, t := range exclude {
		excludeSet[t] = true
	}
	var out []schemadiff.ObjectType
	for _, t := range allDriftObjectTypes {
		if !excludeSet[t] {
			out = append(out, t)
		}
	}
	return out
}

// resolveDriftScope turns the flags into the single positive allow-list per
// dimension that schemadiff.Scope takes. validateDetectDriftFlags has already
// rejected passing both flags of a pair, so each dimension is either the
// resolved include patterns, the complement of the resolved exclude patterns, or
// -- when neither flag was passed -- the whole universe, spelled out rather than
// left empty, because Scope keeps nothing for an empty dimension.
func resolveDriftScope(snapshots []schemasnapshot.SchemaSnapshot, live *schemasnapshot.SchemaSnapshot, schemas []string) (schemadiff.Scope, error) {
	snapshotContents := make([]*schemasnapshot.SnapshotContent, 0, len(snapshots))
	for _, si := range snapshots {
		snapshotContents = append(snapshotContents, si.Content)
	}

	// Built even when nothing is filtered: besides being the set --exclude-table-list
	// subtracts from, it IS the set of tables compared, which the report states.
	universe, err := driftTableUniverse(source.DB().GetAllTableNamesRaw, schemas, snapshotContents, live.Content)
	if err != nil {
		return schemadiff.Scope{}, err
	}
	reg, err := namereg.NewInMemorySourceNameRegistry(source.DBType, schemas, universe)
	if err != nil {
		return schemadiff.Scope{}, err
	}
	allTuples, err := reg.GetRegisteredTableList(false)
	if err != nil {
		return schemadiff.Scope{}, err
	}
	// Map iteration inside the registry makes the list order random; sort it so
	// every downstream ref list (the unfiltered Scope, the report's "Tables" line)
	// is deterministic across runs.
	sort.Slice(allTuples, func(i, j int) bool { return allTuples[i].ForKey() < allTuples[j].ForKey() })
	allRefs := driftObjectRefs(allTuples)
	hasDefaultSchema := reg.DefaultSourceDBSchemaName != ""
	partitionChildren := driftPartitionChildren(snapshotContents, live.Content)

	var includeTables []schemasnapshot.ObjectRef
	switch {
	case driftTableList != "":
		if err := requireDriftTableListQualified(driftTableList, "table-list", hasDefaultSchema); err != nil {
			return schemadiff.Scope{}, err
		}
		includeTuples, err := extractTableListFromString(allTuples, driftTableList, "include")
		if err != nil {
			return schemadiff.Scope{}, err
		}
		includeTables = expandDriftPartitions(driftObjectRefs(includeTuples), partitionChildren)
	case driftExcludeTableList != "":
		if err := requireDriftTableListQualified(driftExcludeTableList, "exclude-table-list", hasDefaultSchema); err != nil {
			return schemadiff.Scope{}, err
		}
		excludeTuples, err := extractTableListFromString(allTuples, driftExcludeTableList, "exclude")
		if err != nil {
			return schemadiff.Scope{}, err
		}
		excludeTables := expandDriftPartitions(driftObjectRefs(excludeTuples), partitionChildren)
		includeTables = complementDriftTableRefs(allRefs, excludeTables)
		if len(includeTables) == 0 {
			return schemadiff.Scope{}, goerrors.Errorf(
				"--exclude-table-list %q excludes every table in the comparison; nothing left to compare", driftExcludeTableList)
		}
	}

	objectTypes := driftParsedFlags.objectTypes
	if driftExcludeObjectTypeList != "" {
		objectTypes = complementDriftObjectTypes(driftParsedFlags.excludeObjectTypes)
		if len(objectTypes) == 0 {
			supported := lo.Map(allDriftObjectTypes, func(t schemadiff.ObjectType, _ int) string { return string(t) })
			return schemadiff.Scope{}, goerrors.Errorf(
				"--exclude-object-type-list %q excludes every supported object type (%s); nothing left to compare",
				driftExcludeObjectTypeList, strings.Join(supported, ", "))
		}
	}

	if driftTableList == "" && driftExcludeTableList == "" {
		includeTables = allRefs
	}
	if driftObjectTypeList == "" && driftExcludeObjectTypeList == "" {
		objectTypes = allDriftObjectTypes
	}
	return schemadiff.Scope{Schemas: schemas, Tables: includeTables, ObjectTypes: objectTypes}, nil
}

// ─── The run ─────────────────────────────────────────────────────────────────

// detectDrift runs the command. It returns rather than exits, so its defers
// unwind before the caller exits.
func detectDrift() error {
	// sqlname.SourceDBType is a package global that sqlname's quoting helpers read.
	// Unlike export/import, detect-drift has no shared setup path that sets it, so
	// set it here before any sqlname use.
	sqlname.SourceDBType = source.DBType

	if err := source.DB().Connect(); err != nil {
		return fmt.Errorf("failed to connect to source database: %w", err)
	}
	defer source.DB().Disconnect()

	source.FetchSourceInfo()

	allSchemas, err := source.DB().GetAllSchemaNamesIdentifiers()
	if err != nil {
		return fmt.Errorf("failed to fetch schema names from source: %w", err)
	}
	source.Schemas, err = namereg.SchemaNameMatcher(source.DBType, allSchemas, source.SchemaConfig)
	if err != nil {
		return err
	}
	// Raw (unquoted) names: compared against catalog values, never interpolated into
	// SQL. The quoted form matches nothing -- see srcdb.Source.GetSchemaListUnquoted.
	schemas := source.GetSchemaListUnquoted()

	// ─── Load stored snapshots (oldest-first) ───────────────────────────────────
	// Must precede Scope resolution: the candidate table universe is built from
	// each snapshot's Content, which is the only place a table already dropped
	// from the live catalog still appears.
	headers, err := schemasnapshot.ListSnapshots(metaDB)
	if err != nil {
		return fmt.Errorf("failed to list schema snapshots: %w", err)
	}
	// Only the one-snapshot case gets a warning. With none at all the run cannot
	// form an interval however the live read goes, so it always ends at
	// nothingComparedError, which says so accurately; warning first that a report
	// is coming would contradict it.
	if len(headers) == 1 {
		utils.PrintAndLogfWarning("Note: only one historical schema snapshot found; drift can only be reported for the " +
			"single interval between it and the live read (if available).\n")
	}

	snapshots := make([]schemasnapshot.SchemaSnapshot, 0, len(headers))
	for _, h := range headers {
		var content *schemasnapshot.SnapshotContent
		if h.IsPlaceholder {
			utils.PrintAndLogfWarning("Note: snapshot %q is a placeholder (its capture failed at the time); skipping it in the diff chain.\n", h.Name())
		} else {
			c, lerr := schemasnapshot.LoadSnapshotByName(metaDB, h.Name())
			switch {
			case lerr == nil:
				content = c
			case errors.Is(lerr, schemasnapshot.ErrSnapshotVersionUnsupported):
				return fmt.Errorf("cannot read snapshot %q: %w", h.Name(), lerr)
			case errors.Is(lerr, schemasnapshot.ErrPlaceholderSnapshot), errors.Is(lerr, schemasnapshot.ErrSnapshotNotFound):
				utils.PrintAndLogfWarning("Note: could not load snapshot %q (%v); skipping it in the diff chain.\n", h.Name(), lerr)
			default:
				utils.PrintAndLogfWarning("Note: error loading snapshot %q (%v); skipping it in the diff chain.\n", h.Name(), lerr)
			}
		}
		snapshots = append(snapshots, schemasnapshot.SchemaSnapshot{Header: h, Content: content})
	}

	// Must also precede Scope resolution: the live capture contributes its tables
	// to the candidate universe.
	live, err := captureLiveSnapshotForDrift(schemas)
	if err != nil {
		return err
	}

	scope, err := resolveDriftScope(snapshots, live, schemas)
	if err != nil {
		return err
	}
	snapshots = append(snapshots, *live)

	report, err := schemadrift.BuildReport(schemadrift.DetectionConfig{
		Source: schemadrift.Source{
			DatabaseType:    source.DBType,
			Host:            source.Host,
			Port:            source.Port,
			Database:        source.DBName,
			DatabaseVersion: source.DBVersion,
		},
		Snapshots: snapshots,
		Scope:     scope,
	})
	if err != nil {
		return fmt.Errorf("build the drift report: %w", err)
	}

	// An empty report reads as "no drift", so refuse to emit one when nothing was
	// examined.
	if report.Summary.ComparedIntervalCount == 0 {
		nothingCompared := nothingComparedError(report)
		// Sent from here rather than left to the atexit handler, which has no report
		// to pass: how many captures existed and why none were usable is the whole
		// signal on this path.
		packAndSendSchemaDriftPayload(ERROR, nothingCompared, &report)
		return nothingCompared
	}

	writtenPaths, err := writeDriftReports(report, driftReportFormats(driftOutputFormat))
	if err != nil {
		packAndSendSchemaDriftPayload(ERROR, err, &report)
		return err
	}

	printDriftSummary(report, writtenPaths)
	packAndSendSchemaDriftPayload(COMPLETE, nil, &report)

	return nil
}

// captureLiveSnapshotForDrift captures the source schema in memory for comparison
// against the stored snapshots. Unlike CaptureAndSaveSnapshot, the result is never
// persisted.
func captureLiveSnapshotForDrift(schemas []string) (*schemasnapshot.SchemaSnapshot, error) {
	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		return nil, goerrors.Errorf("live schema capture: source is %T, expected *srcdb.PostgreSQL", source.DB())
	}
	db := pg.GetDB()
	if db == nil {
		return nil, goerrors.Errorf("live schema capture: no database handle after a successful connect")
	}
	snap, err := schemasnapshot.Capture(context.Background(), db, schemasnapshot.CaptureParams{
		DatabaseType: source.DBType,
		DBMetadata:   schemasnapshot.DBMetadata{Host: source.Host, Port: source.Port, Database: source.DBName, User: source.User},
		Schemas:      schemas,
		Label:        schemasnapshot.LabelSourceLive,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to capture the live source schema for comparison: %w", err)
	}
	return snap, nil
}

// nothingComparedError explains why the run examined no interval, reading the
// case off the report rather than guessing at one. The three causes want
// different things said, and none of them is fixable by re-running the export:
// capture happens while the export commands run, so a migration that ran without
// it cannot be given a history after the fact.
func nothingComparedError(r schemadrift.Report) error {
	var reasons []string
	usable := 0
	for _, c := range r.CapturePoints {
		switch {
		case c.Excluded == "":
			usable++
		case !lo.Contains(reasons, c.Excluded):
			reasons = append(reasons, c.Excluded)
		}
	}

	switch {
	case len(r.CapturePoints) == 0:
		return goerrors.Errorf("this export directory holds no schema snapshots, so there is nothing to compare. " +
			"Snapshots are recorded while `export schema` and `export data` run, unless they ran with " +
			"--disable-schema-snapshot-capture=true or on a voyager version that did not capture by default")
	case len(reasons) == 0:
		return goerrors.Errorf("only %d of %d captures is usable, and a single capture forms no interval, "+
			"so there was nothing to compare", usable, len(r.CapturePoints))
	default:
		return goerrors.Errorf("no two comparable schema snapshots in this export directory, so there was "+
			"nothing to compare: %d of %d captures usable; the rest were skipped because: %s. "+
			"If the schemas named there are not the ones you expected, check --source-db-schema",
			usable, len(r.CapturePoints), strings.Join(reasons, "; "))
	}
}

// ─── Telemetry ───────────────────────────────────────────────────────────────

// report is nil when the run failed before one was built, which is itself worth
// recording: it is the population that could not use the feature at all.
func packAndSendSchemaDriftPayload(status string, errorMsg error, report *schemadrift.Report) {
	if !shouldSendCallhome() {
		return
	}
	// Unlike the other commands that send from here, detect-drift is not on
	// exportDirInitialisedCheckNeededList, so a flag that fails validation reaches
	// the exit handler before detectDrift() opens metaDB. retrieveMigrationUUID
	// dereferences metaDB, and the anonymizer is initialised alongside it.
	if metaDB == nil {
		return
	}
	if err := retrieveMigrationUUID(); err != nil {
		log.Infof("callhome: could not retrieve migration UUID: %v", err)
		return
	}

	payload := createCallhomePayload(migrationUUID)
	payload.MigrationPhase = SCHEMA_DETECT_DRIFT_PHASE
	payload.Status = status
	if msr, err := metaDB.GetMigrationStatusRecord(); err != nil {
		log.Infof("callhome: could not read migration status record: %v", err)
	} else if msr != nil {
		payload.MigrationType = driftMigrationType(msr.ExportTypeFromSource)
	}
	payload.SourceDBDetails = callhome.MarshalledJsonString(anonymizeSourceDBDetails(&source))
	payload.PhasePayload = callhome.MarshalledJsonString(buildSchemaDriftPayload(errorMsg, report))

	if err := callhome.SendPayload(&payload); err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}
}

// driftMigrationType returns "" when the export type is unknown, which is not the
// same as offline: detect-drift may run before `export data` ever sets it (`export
// schema` takes a snapshot too), and start-clean resets it to "". checkStreamingMode
// is unusable here for exactly that reason -- it reads "" as offline.
func driftMigrationType(exportTypeFromSource string) string {
	switch {
	case exportTypeFromSource == "":
		return ""
	case changeStreamingIsEnabled(exportTypeFromSource):
		return LIVE_MIGRATION
	default:
		return OFFLINE
	}
}

func buildSchemaDriftPayload(errorMsg error, report *schemadrift.Report) callhome.SchemaDriftPhasePayload {
	driftPayload := callhome.SchemaDriftPhasePayload{
		PayloadVersion:   callhome.SCHEMA_DRIFT_CALLHOME_PAYLOAD_VERSION,
		OutputFormats:    utils.CsvStringToSlice(driftOutputFormat),
		Error:            callhome.SanitizeErrorMsg(errorMsg, anonymizer),
		ControlPlaneType: getControlPlaneType(),
	}
	if report == nil {
		return driftPayload
	}

	driftPayload.ChangeCount = report.Summary.ChangeCount
	driftPayload.ComparedIntervalCount = report.Summary.ComparedIntervalCount
	driftPayload.StoredCaptureCount = report.Summary.StoredCaptureCount
	driftPayload.LiveCompared = report.Summary.LiveCompared
	driftPayload.SchemaCount = len(report.Comparing.Schemas)
	driftPayload.TablesFiltered = report.Comparing.TablesFiltered
	driftPayload.ObjectTypesFiltered = report.Comparing.ObjectTypesFiltered
	driftPayload.DriftsByType = countDriftsBy(report.Drifts, func(d schemadrift.DriftEntry) string {
		return string(d.Type)
	})
	driftPayload.DriftsBySeverity = countDriftsBy(report.Drifts, func(d schemadrift.DriftEntry) string {
		return string(d.Severity)
	})
	return driftPayload
}

func countDriftsBy(drifts []schemadrift.DriftEntry, key func(schemadrift.DriftEntry) string) map[string]int {
	if len(drifts) == 0 {
		return nil
	}
	counts := make(map[string]int)
	for _, d := range drifts {
		counts[key(d)]++
	}
	return counts
}

// ─── Output: report files and terminal summary ───────────────────────────────

// writeDriftReports renders and writes report to <export-dir>/reports/ in each
// of formats, creating the reports directory if necessary. Returns the paths
// written, in the same order as formats.
func writeDriftReports(report schemadrift.Report, formats []string) ([]string, error) {
	reportsDir := filepath.Join(exportDir, "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create reports directory %q: %w", reportsDir, err)
	}

	var written []string
	for _, f := range formats {
		var data []byte
		var err error
		switch f {
		case "json":
			data, err = schemadrift.RenderJSON(report)
		case "html":
			data, err = schemadrift.RenderHTML(report)
		default:
			// Unreachable: already validated in validateDriftOutputFormat.
			return nil, goerrors.Errorf("unsupported output format %q", f)
		}
		if err != nil {
			return nil, fmt.Errorf("failed to render %s drift report: %w", f, err)
		}

		path := filepath.Join(reportsDir, fmt.Sprintf("%s.%s", DRIFT_REPORT_FILE_NAME, f))
		if utils.FileOrFolderExists(path) {
			utils.PrintAndLogf("\n%s already exists, overwriting it with a new generated report\n", filepath.Base(path))
		}
		if err := os.WriteFile(path, data, 0644); err != nil {
			return nil, fmt.Errorf("failed to write %s drift report to %q: %w", f, path, err)
		}
		written = append(written, path)
	}
	return written, nil
}

// printDriftSummary prints the terminal summary for a completed detect-drift
// run: the comparison window, how many captures were compared (and whether the
// live source was among them), the schemas in scope, the total change count,
// and the paths of the report files just written.
// Colouring follows src/utils/logging.go, as export schema and cutover status do.
// Console only; the log file records the plain message.
func printDriftSummary(report schemadrift.Report, writtenPaths []string) {
	utils.PrintAndLogfPhase("\nSchema drift summary")

	printDriftSummaryField("Comparison window", utils.PrintAndLogf,
		"%s -> %s", formatDriftTimestamp(report.Window.From), formatDriftTimestamp(report.Window.To))
	// Two numbers, not one: a stored capture that failed or did not cover the
	// requested schemas is bridged, so the count of captures on disk overstates
	// what was actually examined.
	printDriftSummaryField("Captures stored", utils.PrintAndLogf, "%d", report.Summary.StoredCaptureCount)
	printDriftSummaryField("Intervals compared", utils.PrintAndLogf,
		"%d (live source comparison: %t)", report.Summary.ComparedIntervalCount, report.Summary.LiveCompared)
	printDriftSummaryField("Schemas", utils.PrintAndLogf, "%s", driftScopeLine(report.Comparing.Schemas))
	printDriftSummaryField("Tables", utils.PrintAndLogf, "%s",
		driftScopeLine(report.Comparing.Tables))

	// The headline number carries the verdict, so colour it like one: green when
	// the source still matches what was captured, yellow when it does not.
	if report.Summary.ChangeCount == 0 {
		printDriftSummaryField("Changes detected", utils.PrintAndLogfSuccess, "0 (no schema drift)")
	} else {
		printDriftSummaryField("Changes detected", utils.PrintAndLogfWarning, "%d", report.Summary.ChangeCount)
	}

	// Paths get their own indented lines: one unbreakable token starting 20 columns in
	// would wrap on a standard terminal.
	if len(writtenPaths) > 0 {
		utils.PrintAndLogf("Reports:\n")
		for _, p := range writtenPaths {
			utils.PrintAndLogf("  %s\n", utils.Path.Sprint(p))
		}
	}
}

// driftSummaryLabelWidth is the column the summary's values start in, wide enough
// for the longest label ("Intervals compared").
const driftSummaryLabelWidth = 18

// printDriftSummaryField prints one "label : value" row, wrapping a long value with a
// HANGING INDENT to the value column so the block keeps its alignment.
//
// printFn colours the whole row, so the value stays plain text: colouring it would
// make its escape bytes count towards the wrap width.
func printDriftSummaryField(label string, printFn func(string, ...interface{}), format string, args ...interface{}) {
	indent := strings.Repeat(" ", driftSummaryLabelWidth+2) // label column + ": "
	lines := wrapDriftValue(fmt.Sprintf(format, args...), ux.GetTerminalWidth()-len(indent))
	if len(lines) == 0 {
		lines = []string{""}
	}
	printFn("%-*s: %s\n", driftSummaryLabelWidth, label, lines[0])
	for _, l := range lines[1:] {
		printFn("%s%s\n", indent, l)
	}
}

// wrapDriftValue greedily wraps on whitespace. An over-long word is emitted whole:
// breaking a table name or path mid-token would make it uncopyable.
func wrapDriftValue(s string, width int) []string {
	words := strings.Fields(s)
	if len(words) == 0 {
		return nil
	}
	if width <= 0 {
		return []string{strings.Join(words, " ")}
	}
	var lines []string
	cur := words[0]
	for _, w := range words[1:] {
		if len(cur)+1+len(w) <= width {
			cur += " " + w
			continue
		}
		lines = append(lines, cur)
		cur = w
	}
	return append(lines, cur)
}

// maxDriftScopeNamesInSummary caps the names the summary spells out; the full list
// lives in the report.
const maxDriftScopeNamesInSummary = 5

// driftScopeLine renders the scope line: a count, then the names while they fit.
func driftScopeLine(names []string) string {
	if len(names) == 0 {
		return "none"
	}
	head := fmt.Sprintf("%d", len(names))
	if len(names) <= maxDriftScopeNamesInSummary {
		return fmt.Sprintf("%s — %s", head, strings.Join(names, ", "))
	}
	return fmt.Sprintf("%s — %s, ... (+%d more; see the report)",
		head, strings.Join(names[:maxDriftScopeNamesInSummary], ", "),
		len(names)-maxDriftScopeNamesInSummary)
}

// formatDriftTimestamp renders t for the terminal summary, matching the "-" for
// a zero time.Time convention used by the HTML report itself (see render.go).
func formatDriftTimestamp(t time.Time) string {
	if t.IsZero() {
		return "-"
	}
	return t.Format(time.RFC3339)
}
