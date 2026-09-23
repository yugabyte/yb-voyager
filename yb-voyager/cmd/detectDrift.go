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
	"strings"
	"time"

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/tebeka/atexit"

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
	Use: "detect-drift",
	Short: "Report source schema changes made during the migration, and what to do about each one " +
		"(needs --disable-schema-snapshot-capture=false on the export commands)",
	Long: `Reports how the PostgreSQL source schema changed while the migration was running, and
what to do about each change.

Voyager records a schema snapshot at each migration milestone -- export schema, export data
start, periodically during export data, and export data exit. This command diffs consecutive
snapshots, plus a final comparison against a live read of the source, and writes the result
to <export-dir>/reports/. It is read-only: it never modifies migration state and never
applies anything on the target, it only writes report files.

PREREQUISITE: those snapshots are only recorded when capture is enabled, which is currently
off by default. Pass --disable-schema-snapshot-capture=false to export schema and export
data. Capture cannot be turned on after the fact: a migration that ran without it has no
history, and this command exits 2 rather than reporting a misleading "no drift".

The report groups each change by the interval between the two captures that bracket it, and
labels the interval with what Voyager was doing at the time (for example "export data:
running"). Every change carries a severity, what the migration will do if the change is not
reconciled on the target, and the corrective step.

Exit codes: 0 = success, no drift found; 1 = success, drift found; 2 = operational error
(bad flags, unreachable source, unsupported source type, etc.).`,

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
		driftFound, err := detectDrift()
		if err != nil {
			exitDriftOperationalError("%v", err)
		}
		if driftFound {
			// Not a failure: the report is already on disk and says so. Exiting
			// here rather than inside detectDrift lets its defers unwind first.
			atexit.Exit(1)
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

	detectDriftCmd.Flags().StringVar(&driftOutputFormat, "output-format", "html,json",
		"comma-separated list of report formats to generate: ('html', 'json')")

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
		exitDriftOperationalError("schema detect-drift currently supports PostgreSQL sources only (got --source-db-type=%q)", source.DBType)
	}

	if driftTableList != "" && driftExcludeTableList != "" {
		exitDriftOperationalError("--table-list and --exclude-table-list are mutually exclusive. Use only one of them.")
	}
	if driftObjectTypeList != "" && driftExcludeObjectTypeList != "" {
		exitDriftOperationalError("--object-type-list and --exclude-object-type-list are mutually exclusive. Use only one of them.")
	}

	if err := validateDriftOutputFormat(driftOutputFormat); err != nil {
		exitDriftOperationalError("%v", err)
	}

	var err error
	if driftParsedFlags.objectTypes, err = parseDriftObjectTypeList(driftObjectTypeList); err != nil {
		exitDriftOperationalError("invalid --object-type-list: %v", err)
	}
	if driftParsedFlags.excludeObjectTypes, err = parseDriftObjectTypeList(driftExcludeObjectTypeList); err != nil {
		exitDriftOperationalError("invalid --exclude-object-type-list: %v", err)
	}
}

// validateDriftOutputFormat checks that format is a non-empty, comma-separated
// list drawn from driftValidOutputFormats with no duplicates.
func validateDriftOutputFormat(format string) error {
	if strings.TrimSpace(format) == "" {
		return goerrors.Errorf("--output-format cannot be empty; supported formats: %s", strings.Join(driftValidOutputFormats, ", "))
	}
	seen := make(map[string]bool)
	for _, f := range utils.CsvStringToSlice(format) {
		f = strings.ToLower(f)
		if !lo.Contains(driftValidOutputFormats, f) {
			return goerrors.Errorf("invalid report output format: %s. Supported formats are %v", f, driftValidOutputFormats)
		}
		if seen[f] {
			return goerrors.Errorf("duplicate report output format: %s", f)
		}
		seen[f] = true
	}
	return nil
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

// driftTableCandidate pairs a table's identity (for building
// schemasnapshot.ObjectRef / Scope entries) with the sqlname.ObjectName view of
// it (for --table-list / --exclude-table-list glob matching).
type driftTableCandidate struct {
	ref  schemasnapshot.ObjectRef
	name *sqlname.ObjectName
}

// buildDriftTableCandidates reads the live catalog and hands the union off to
// unionDriftTableCandidates. It lists tables through an error-returning call
// rather than GetAllTableNames, which exits 1 on a query failure -- the code this
// command reserves for "drift found".
func buildDriftTableCandidates(listTables func(schema string) ([]string, error), schemas []string, defaultSchema string,
	snapshotContents []*schemasnapshot.SnapshotContent, liveContent *schemasnapshot.SnapshotContent) ([]driftTableCandidate, error) {
	var liveRefs []schemasnapshot.ObjectRef
	for _, schema := range schemas {
		names, err := listTables(schema)
		if err != nil {
			return nil, fmt.Errorf("list the tables in schema %q: %w", schema, err)
		}
		for _, name := range names {
			liveRefs = append(liveRefs, schemasnapshot.ObjectRef{Schema: schema, Name: name})
		}
	}
	return unionDriftTableCandidates(source.DBType, defaultSchema, liveRefs, snapshotContents, liveContent), nil
}

// unionDriftTableCandidates builds the --table-list / --exclude-table-list matching
// universe, deduped by (schema, name), in the order live catalog -> snapshots ->
// live capture.
//
// It is the union rather than the live catalog alone because a table dropped from
// the source is gone from the catalog but must still be nameable, to see its own
// drop reported. Only the stored snapshots still know about it.
func unionDriftTableCandidates(dbType, defaultSchema string, liveRefs []schemasnapshot.ObjectRef, snapshotContents []*schemasnapshot.SnapshotContent, liveContent *schemasnapshot.SnapshotContent) []driftTableCandidate {
	seen := make(map[schemasnapshot.ObjectRef]bool)
	var candidates []driftTableCandidate

	add := func(schema, name string) {
		ref := schemasnapshot.ObjectRef{Schema: schema, Name: name}
		if seen[ref] {
			return
		}
		seen[ref] = true
		objName := sqlname.NewObjectName(dbType, defaultSchema, schema, name)
		candidates = append(candidates, driftTableCandidate{ref: ref, name: objName})
	}

	for _, r := range liveRefs {
		add(r.Schema, r.Name)
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

	return candidates
}

// resolveDriftTableRefs resolves a --table-list / --exclude-table-list glob
// pattern list against candidates. Returns (nil, nil) for an empty pattern
// list (meaning "no filter"). A pattern matching no candidate is reported as an
// unknown table name, mirroring export/import's --table-list validation.
func resolveDriftTableRefs(candidates []driftTableCandidate, patternList string, flagName string, hasDefaultSchema bool) ([]schemasnapshot.ObjectRef, error) {
	if strings.TrimSpace(patternList) == "" {
		return nil, nil
	}
	var refs []schemasnapshot.ObjectRef
	var unknown []string
	for _, pattern := range utils.CsvStringToSlice(patternList) {
		// An unqualified pattern only ever matches a candidate in the default
		// schema (see ObjectName.MatchesPattern). With no default there is nothing
		// for it to match, and it would otherwise be reported as an unknown table.
		if !hasDefaultSchema && !strings.Contains(pattern, ".") {
			return nil, goerrors.Errorf("--%s entry %q is not schema-qualified, and --source-db-schema names no "+
				"default schema (no \"public\"); write it as schema.table", flagName, pattern)
		}
		matched := false
		for _, c := range candidates {
			ok, err := c.name.MatchesPattern(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid table name pattern %q in --%s: %w", pattern, flagName, err)
			}
			if ok {
				refs = append(refs, c.ref)
				matched = true
			}
		}
		if !matched {
			unknown = append(unknown, pattern)
		}
	}
	if len(unknown) > 0 {
		return nil, goerrors.Errorf("unknown table name(s) %v in --%s", unknown, flagName)
	}
	return lo.UniqBy(refs, func(r schemasnapshot.ObjectRef) string { return r.Schema + "." + r.Name }), nil
}

// complementDriftTableRefs returns every candidate ref NOT present in exclude --
// the resolution of --exclude-table-list into the single positive allow-list the
// collapsed schemadiff.Scope expects.
//
// An EMPTY result means the user excluded the whole universe. Callers MUST
// reject it rather than forward it: Scope keeps nothing for an empty dimension,
// so the run would compare nothing and report a clean bill of health.
func complementDriftTableRefs(candidates []driftTableCandidate, exclude []schemasnapshot.ObjectRef) []schemasnapshot.ObjectRef {
	excludeSet := make(map[schemasnapshot.ObjectRef]bool, len(exclude))
	for _, r := range exclude {
		excludeSet[r] = true
	}
	var out []schemasnapshot.ObjectRef
	for _, c := range candidates {
		if !excludeSet[c.ref] {
			out = append(out, c.ref)
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
	var liveContent *schemasnapshot.SnapshotContent
	if live != nil {
		liveContent = live.Content
	}
	// noDefaultSchema is carried rather than discarded: without a default, an
	// unqualified --table-list pattern can match nothing at all, and every other
	// caller of GetDefaultPGSchema treats that as an error rather than a silent
	// no-match.
	defaultSchema, noDefaultSchema := GetDefaultPGSchema(source.Schemas)

	// Built even when nothing is filtered: besides being the set --exclude-table-list
	// subtracts from, it IS the set of tables compared, which the report states.
	candidates, err := buildDriftTableCandidates(source.DB().GetAllTableNamesRaw, schemas, defaultSchema, snapshotContents, liveContent)
	if err != nil {
		return schemadiff.Scope{}, err
	}

	var includeTables []schemasnapshot.ObjectRef
	switch {
	case driftTableList != "":
		if includeTables, err = resolveDriftTableRefs(candidates, driftTableList, "table-list", !noDefaultSchema); err != nil {
			return schemadiff.Scope{}, err
		}
	case driftExcludeTableList != "":
		var excludeTables []schemasnapshot.ObjectRef
		if excludeTables, err = resolveDriftTableRefs(candidates, driftExcludeTableList, "exclude-table-list", !noDefaultSchema); err != nil {
			return schemadiff.Scope{}, err
		}
		includeTables = complementDriftTableRefs(candidates, excludeTables)
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
		includeTables = lo.Map(candidates, func(c driftTableCandidate, _ int) schemasnapshot.ObjectRef { return c.ref })
	}
	if driftObjectTypeList == "" && driftExcludeObjectTypeList == "" {
		objectTypes = allDriftObjectTypes
	}
	return schemadiff.Scope{Schemas: schemas, Tables: includeTables, ObjectTypes: objectTypes}, nil
}

// ─── The run ─────────────────────────────────────────────────────────────────

// detectDrift runs the command and reports whether drift was found. It never
// exits: returning lets its defers unwind and leaves the exit code to the caller,
// which is also what gives the atexit handlers (callhome among them) a chance to
// run on every path. driftFound is only meaningful when err is nil, and by then
// the report is already on disk.
func detectDrift() (driftFound bool, err error) {
	// CreateMigrationProjectIfNotExists is idempotent: it's a no-op (aside from
	// mkdir -p) if this export-dir already has a migration project. detect-drift
	// only ever writes to <export-dir>/reports/ afterwards; it never touches
	// migration state (MigrationStatusRecord, table lists, etc.).
	metaDB = CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	// sqlname.SourceDBType is a package global that sqlname's quoting helpers read.
	// Unlike export/import, detect-drift has no shared setup path that sets it, so
	// set it here before any sqlname use.
	sqlname.SourceDBType = source.DBType

	if err := source.DB().Connect(); err != nil {
		return false, fmt.Errorf("failed to connect to source database: %w", err)
	}
	defer source.DB().Disconnect()

	source.FetchSourceInfo() // best-effort; populates source.DBVersion (used non-fatally elsewhere)

	allSchemas, err := source.DB().GetAllSchemaNamesIdentifiers()
	if err != nil {
		return false, fmt.Errorf("failed to fetch schema names from source: %w", err)
	}
	source.Schemas, err = namereg.SchemaNameMatcher(source.DBType, allSchemas, source.SchemaConfig)
	if err != nil {
		return false, err
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
		return false, fmt.Errorf("failed to list schema snapshots: %w", err)
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
				return false, fmt.Errorf("cannot read snapshot %q: %w", h.Name(), lerr)
			case errors.Is(lerr, schemasnapshot.ErrPlaceholderSnapshot), errors.Is(lerr, schemasnapshot.ErrSnapshotNotFound):
				utils.PrintAndLogfWarning("Note: could not load snapshot %q (%v); skipping it in the diff chain.\n", h.Name(), lerr)
			default:
				utils.PrintAndLogfWarning("Note: error loading snapshot %q (%v); skipping it in the diff chain.\n", h.Name(), lerr)
			}
		}
		snapshots = append(snapshots, schemasnapshot.SchemaSnapshot{Header: h, Content: content})
	}

	// Must also precede Scope resolution: a live capture that succeeded contributes
	// its tables to the candidate universe.
	live := captureLiveSnapshotForDrift(schemas)

	scope, err := resolveDriftScope(snapshots, live, schemas)
	if err != nil {
		return false, err
	}

	if live != nil {
		snapshots = append(snapshots, *live)
	}

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
		return false, fmt.Errorf("build the drift report: %w", err)
	}

	// An empty report reads as "no drift", so refuse to emit one when nothing was
	// examined.
	if report.Summary.ComparedIntervalCount == 0 {
		return false, nothingComparedError(report)
	}

	writtenPaths, err := writeDriftReports(report, driftOutputFormat)
	if err != nil {
		return false, err
	}

	printDriftSummary(report, writtenPaths)

	return report.Summary.ChangeCount > 0, nil
}

// captureLiveSnapshotForDrift attempts a best-effort, in-memory-only schema
// capture of the source for comparison against the historical snapshot chain.
// Unlike CaptureAndSaveSnapshot, the result is never persisted. Any failure
// (unsupported source type, capture error) is logged as a note and yields a nil
// result -- the source being briefly unreachable (or the capture racing DDL)
// must never fail the whole command, since the snapshot-only comparison is
// still useful on its own.
func captureLiveSnapshotForDrift(schemas []string) *schemasnapshot.SchemaSnapshot {
	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		utils.PrintAndLogfWarning("Note: live schema capture is only supported for PostgreSQL sources; skipping live comparison.\n")
		return nil
	}
	db := pg.GetDB()
	if db == nil {
		utils.PrintAndLogfWarning("Note: no active database handle for live schema capture; skipping live comparison.\n")
		return nil
	}
	snap, err := schemasnapshot.Capture(context.Background(), db, schemasnapshot.CaptureParams{
		DatabaseType: source.DBType,
		DBMetadata:   schemasnapshot.DBMetadata{Host: source.Host, Port: source.Port, Database: source.DBName, User: source.User},
		Schemas:      schemas,
		Label:        schemasnapshot.LabelSourceLive,
	})
	if err != nil {
		utils.PrintAndLogfWarning("Note: could not capture live schema for comparison: %v; continuing with snapshot-only comparison.\n", err)
		return nil
	}
	return snap
}

// exitDriftOperationalError prints the given error to stderr (and the log) and
// exits with code 2, the contractual exit code for detect-drift operational
// errors (bad flags, unreachable source, unsupported source type, etc.).
//
// It deliberately does not use utils.ErrExit, which exits with code 1 -- that
// would collide with detect-drift's own "success, drift found" exit code. It
// still leaves via atexit, so the handlers main.go registers (callhome, child
// cleanup, terminal restore after a password prompt) run as they do for every
// other command.
func exitDriftOperationalError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	log.Errorf("schema detect-drift: %s", msg)
	atexit.Exit(2)
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
			"Snapshots are recorded only while `export schema` and `export data` run, and only when " +
			"--disable-schema-snapshot-capture=false was passed to them")
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

// ─── Output: report files and terminal summary ───────────────────────────────

// writeDriftReports renders and writes report to <export-dir>/reports/ in each
// of the comma-separated formats in formatSpec, creating the reports directory
// if necessary. Returns the paths written, in the same order as formatSpec.
func writeDriftReports(report schemadrift.Report, formatSpec string) ([]string, error) {
	reportsDir := filepath.Join(exportDir, "reports")
	if err := os.MkdirAll(reportsDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create reports directory %q: %w", reportsDir, err)
	}

	var written []string
	for _, f := range utils.CsvStringToSlice(formatSpec) {
		f = strings.ToLower(f)
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
			fmt.Printf("\n%s already exists, overwriting it with a new generated report\n", filepath.Base(path))
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
	printDriftSummaryField("Schemas", utils.PrintAndLogf, "%s", joinOrAllDrift(report.Comparing.Schemas))
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

func joinOrAllDrift(items []string) string {
	if len(items) == 0 {
		return "all"
	}
	return strings.Join(items, ", ")
}
