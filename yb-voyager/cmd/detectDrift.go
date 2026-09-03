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

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemadiff/driftreport"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ux"
)

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

// driftValidOutputFormats are the report formats detect-drift can render.
var driftValidOutputFormats = []string{"html", "json"}

// driftObjectTypesByName maps the --object-type-list vocabulary onto
// schemadiff.ObjectType. Deliberately smaller than export/analyze-schema's
// --object-type-list: the engine only emits TABLE and COLUMN findings in v1.
// COLUMN is its own selector, not swept in under TABLE.
var driftObjectTypesByName = map[string]schemadiff.ObjectType{
	"TABLE":  schemadiff.ObjectTypeTable,
	"COLUMN": schemadiff.ObjectTypeColumn,
}

var detectDriftCmd = &cobra.Command{
	Use: "detect-drift",
	Short: "Report source schema changes made during the migration, and what to do about each one " +
		"(needs --suppress-schema-snapshot-capture=false on the export commands)",
	Long: `Reports how the PostgreSQL source schema changed while the migration was running, and
what to do about each change.

Voyager records a schema snapshot at each migration milestone -- export schema, export data
start, periodically during export data, and export data exit. This command diffs consecutive
snapshots, plus a final comparison against a live read of the source, and writes the result
to <export-dir>/reports/. It is read-only: it never modifies migration state and never
applies anything on the target, it only writes report files.

PREREQUISITE: those snapshots are only recorded when capture is enabled, which is currently
off by default. Pass --suppress-schema-snapshot-capture=false to export schema and export
data. Without it this command has no history to compare against and reports no drift.

The report groups each change by the interval between the two captures that bracket it, and
labels the interval with what Voyager was doing at the time (for example "export data:
running"). Every change carries a severity, what the migration will do if the change is not
reconciled on the target, and the corrective step.

Exit codes: 0 = success, no drift found; 1 = success, drift found; 2 = operational error
(bad flags, unreachable source, unsupported source type, etc.).`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateDetectDriftFlags(cmd)
		// Resolve the source password from the --source-db-password flag, the
		// SOURCE_DB_PASSWORD env var, or an interactive prompt -- exactly as the
		// export commands do. Without this, source.Password stays empty and the
		// connect in detectDrift() fails SASL auth against any password-protected
		// source.
		getAndStoreSourceDBPasswordInSourceConf(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		detectDrift()
	},
}

func init() {
	schemaCmd.AddCommand(detectDriftCmd)
	registerCommonGlobalFlags(detectDriftCmd)
	// PostgreSQL-only: registerOracleFlags=false, includeOracleCDBFlags=false so no
	// Oracle-specific flags (SID/home/TNS/CDB) are registered on this command.
	registerSourceDBConnFlags(detectDriftCmd, false, false)
	detectDriftCmd.MarkFlagRequired("source-db-user")
	detectDriftCmd.MarkFlagRequired("source-db-name")
	detectDriftCmd.MarkFlagRequired("source-db-schema")
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

// exitDriftOperationalError prints the given error to stderr (and the log) and
// exits with code 2, the contractual exit code for detect-drift operational
// errors (bad flags, unreachable source, unsupported source type, etc.).
// It deliberately does not use utils.ErrExit, which exits with code 1 -- that
// would collide with detect-drift's own "success, drift found" exit code.
func exitDriftOperationalError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	fmt.Fprintf(os.Stderr, "Error: %s\n", msg)
	log.Errorf("schema detect-drift: %s", msg)
	os.Exit(2)
}

// driftDetectionHint returns the example invocation of `schema detect-drift`
// printed as part of the guidance footers below.
func driftDetectionHint() string {
	return fmt.Sprintf("\t%s --export-dir %s (with your source connection flags)", detectDriftCmd.CommandPath(), exportDir)
}

// printSchemaDriftErrorFooter prints a guidance footer nudging the user
// towards `schema detect-drift` after an export/import data command has
// exited with an error. firstLine is the caller-specific lead-in sentence
// (e.g. "export data exited with an error."); the rest of the message and
// the example invocation are identical across call sites.
func printSchemaDriftErrorFooter(firstLine string) {
	utils.PrintAndLog(fmt.Sprintf("%s If the source schema may have changed since export began, review schema drift before retrying or cutting over:\n%s",
		firstLine, driftDetectionHint()))
}

// printCutoverSchemaDriftRecommendation prints a one-time, non-error
// recommendation to review schema drift on the source before proceeding
// with cutover -- cutover is the last point at which the source schema is
// captured, so drift after that point won't be reflected in the migration.
func printCutoverSchemaDriftRecommendation() {
	utils.PrintAndLog(fmt.Sprintf("Recommendation: cutover is the last point at which the source schema is captured. Consider reviewing schema drift on the source before proceeding:\n%s",
		driftDetectionHint()))
}

// validateDetectDriftFlags runs all flag-only validation (no DB connection
// required): source DB type, mutually-exclusive flag pairs, and output-format
// values. Table-list validation (whether a pattern actually matches a table)
// needs the source's table list and so happens later, inside detectDrift().
func validateDetectDriftFlags(cmd *cobra.Command) {
	if source.DBType == "" {
		source.DBType = POSTGRESQL
	}
	if source.DBType != POSTGRESQL {
		exitDriftOperationalError("schema detect-drift currently supports PostgreSQL sources only (got --source-db-type=%q)", source.DBType)
	}
	setSourceDefaultPort()
	setDefaultSSLMode()

	if driftTableList != "" && driftExcludeTableList != "" {
		exitDriftOperationalError("--table-list and --exclude-table-list are mutually exclusive. Use only one of them.")
	}
	if driftObjectTypeList != "" && driftExcludeObjectTypeList != "" {
		exitDriftOperationalError("--object-type-list and --exclude-object-type-list are mutually exclusive. Use only one of them.")
	}

	if err := validateDriftOutputFormat(driftOutputFormat); err != nil {
		exitDriftOperationalError("%v", err)
	}
	if _, err := parseDriftObjectTypeList(driftObjectTypeList); err != nil {
		exitDriftOperationalError("invalid --object-type-list: %v", err)
	}
	if _, err := parseDriftObjectTypeList(driftExcludeObjectTypeList); err != nil {
		exitDriftOperationalError("invalid --exclude-object-type-list: %v", err)
	}
}

// validateDriftOutputFormat checks that format is a non-empty, comma-separated
// list drawn from driftValidOutputFormats with no duplicates.
func validateDriftOutputFormat(format string) error {
	if strings.TrimSpace(format) == "" {
		return fmt.Errorf("--output-format cannot be empty; supported formats: %s", strings.Join(driftValidOutputFormats, ", "))
	}
	seen := make(map[string]bool)
	for _, f := range utils.CsvStringToSlice(format) {
		f = strings.ToLower(f)
		if !lo.Contains(driftValidOutputFormats, f) {
			return fmt.Errorf("invalid report output format: %s. Supported formats are %v", f, driftValidOutputFormats)
		}
		if seen[f] {
			return fmt.Errorf("duplicate report output format: %s", f)
		}
		seen[f] = true
	}
	return nil
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
		return nil, fmt.Errorf("unknown object type(s) %v; supported types: TABLE, COLUMN", invalid)
	}
	return out, nil
}

// driftTableCandidate pairs a table's identity (for building
// schemasnapshot.ObjectRef / Scope entries) with the sqlname.ObjectName view of
// it (for --table-list / --exclude-table-list glob matching).
type driftTableCandidate struct {
	ref  schemasnapshot.ObjectRef
	name *sqlname.ObjectName
}

// buildDriftTableCandidates builds the --table-list / --exclude-table-list
// matching universe as the UNION of: the live source catalog, every table
// appearing in each successfully-loaded historical snapshot (snapshotContents),
// and the best-effort live capture (liveContent, nil if it failed or was
// skipped). This union -- not the live catalog alone -- matters because a table
// that has since been DROPPED from the source is absent from the live catalog
// but may still need to be named in --table-list (to see its drop reported) or
// matched by --exclude-table-list; only the historical snapshots know about it.
// Entries are de-duplicated by (schema, name). Must be called after
// source.Schemas has been resolved (it drives which schemas GetAllTableNames()
// queries). It reads the package globals (source) and delegates the actual
// union/dedup to the pure unionDriftTableCandidates.
func buildDriftTableCandidates(snapshotContents []*schemasnapshot.SnapshotContent, liveContent *schemasnapshot.SnapshotContent) []driftTableCandidate {
	defaultSchema, _ := GetDefaultPGSchema(source.Schemas)
	var liveRefs []schemasnapshot.ObjectRef
	for _, n := range source.DB().GetAllTableNames() {
		liveRefs = append(liveRefs, schemasnapshot.ObjectRef{Schema: n.SchemaName.Unquoted, Name: n.ObjectName.Unquoted})
	}
	return unionDriftTableCandidates(source.DBType, defaultSchema, liveRefs, snapshotContents, liveContent)
}

// unionDriftTableCandidates builds the deduped --table-list / --exclude-table-list
// matching universe as the UNION of: the live source catalog (liveRefs), every table
// appearing in each successfully-loaded historical snapshot (snapshotContents), and the
// best-effort live capture (liveContent, nil if it failed or was skipped). This union --
// not the live catalog alone -- matters because a table that has since been DROPPED from
// the source is absent from the live catalog but may still need to be named in
// --table-list (to see its drop reported) or matched by --exclude-table-list; only the
// historical snapshots know about it. Entries are de-duplicated by (schema, name), in the
// order live catalog -> snapshots (in order) -> live capture. It is pure (reads no package
// globals) so the universe-union behavior is unit-testable.
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

	// 1. The live source catalog.
	for _, r := range liveRefs {
		add(r.Schema, r.Name)
	}
	// 2. Every table in each successfully-loaded historical snapshot.
	for _, c := range snapshotContents {
		if c == nil {
			continue // placeholder / failed-to-load snapshot; nothing to contribute.
		}
		for _, t := range c.Tables {
			add(t.Schema, t.Name)
		}
	}
	// 3. The best-effort live capture, if it succeeded.
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
func resolveDriftTableRefs(candidates []driftTableCandidate, patternList string, flagName string) ([]schemasnapshot.ObjectRef, error) {
	if strings.TrimSpace(patternList) == "" {
		return nil, nil
	}
	var refs []schemasnapshot.ObjectRef
	var unknown []string
	for _, pattern := range utils.CsvStringToSlice(patternList) {
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
		return nil, fmt.Errorf("unknown table name(s) %v in --%s", unknown, flagName)
	}
	return lo.UniqBy(refs, func(r schemasnapshot.ObjectRef) string { return r.Schema + "." + r.Name }), nil
}

// complementDriftTableRefs returns every candidate ref NOT present in exclude --
// the resolution of --exclude-table-list into the single positive allow-list the
// collapsed schemadiff.Scope expects.
//
// An EMPTY result means "select nothing" and callers MUST reject it rather than
// forward it: schemadiff.Scope reads an empty list as "all", so passing it on
// would invert the exclusion into comparing everything.
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

// allDriftObjectTypes is the full v1 object-type universe, used to resolve
// --exclude-object-type-list into its complement (see complementDriftObjectTypes).
var allDriftObjectTypes = []schemadiff.ObjectType{schemadiff.ObjectTypeTable, schemadiff.ObjectTypeColumn}

// complementDriftObjectTypes returns every type in allDriftObjectTypes NOT
// present in exclude -- the resolution of --exclude-object-type-list into the
// single positive allow-list the collapsed schemadiff.Scope expects.
//
// As with complementDriftTableRefs, an EMPTY result means "select nothing" and
// must be rejected by the caller, not forwarded to Scope (where empty = all).
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

// detectDrift is the full RunE body of `schema detect-drift`. Every operational
// failure exits via exitDriftOperationalError (code 2). It returns normally
// (exit 0) when the report was generated with zero diffs, and calls os.Exit(1)
// itself when diffs were found -- both after the report has already been
// written to disk.
func detectDrift() {
	// CreateMigrationProjectIfNotExists is idempotent: it's a no-op (aside from
	// mkdir -p) if this export-dir already has a migration project. detect-drift
	// only ever writes to <export-dir>/reports/ afterwards; it never touches
	// migration state (MigrationStatusRecord, table lists, etc.).
	metaDB = CreateMigrationProjectIfNotExists(source.DBType, exportDir)

	// sqlname.SourceDBType is a package global that sqlname's quoting/matching
	// helpers read (e.g. via source.DB().GetAllTableNames() -> NewSourceName).
	// Unlike export/import, detect-drift has no shared setup path that sets it,
	// so set it here before any sqlname use; otherwise GetAllTableNames panics
	// with "invalid source db type" on the --table-list/--exclude-table-list path.
	sqlname.SourceDBType = source.DBType

	if err := source.DB().Connect(); err != nil {
		exitDriftOperationalError("failed to connect to source database: %v", err)
	}
	defer source.DB().Disconnect()

	source.FetchSourceInfo() // best-effort; populates source.DBVersion (used non-fatally elsewhere)

	allSchemas, err := source.DB().GetAllSchemaNamesIdentifiers()
	if err != nil {
		exitDriftOperationalError("failed to fetch schema names from source: %v", err)
	}
	source.Schemas, err = namereg.SchemaNameMatcher(source.DBType, allSchemas, source.SchemaConfig)
	if err != nil {
		exitDriftOperationalError("%v", err)
	}
	// Raw (unquoted) names: compared against catalog values, never interpolated into
	// SQL. The quoted form matches nothing -- see srcdb.Source.GetSchemaListUnquoted.
	schemas := source.GetSchemaListUnquoted()

	// ─── Load stored snapshots (oldest-first) ───────────────────────────────────
	// Moved ahead of Scope resolution: the candidate table universe (below) needs
	// each snapshot's Content to include tables since dropped from the live
	// catalog.
	headers, err := schemasnapshot.ListSnapshots(metaDB)
	if err != nil {
		exitDriftOperationalError("failed to list schema snapshots: %v", err)
	}
	switch len(headers) {
	case 0:
		utils.PrintAndLogfWarning("Note: no historical schema snapshots found in this export directory's metadata; " +
			"the report will only reflect the live source read (if available), with no drift against history.\n")
	case 1:
		utils.PrintAndLogfWarning("Note: only one historical schema snapshot found; drift can only be reported for the " +
			"single interval between it and the live read (if available).\n")
	}

	snapshotInputs := make([]driftreport.SnapshotInput, 0, len(headers))
	for _, h := range headers {
		var content *schemasnapshot.SnapshotContent
		if h.IsPlaceholder {
			utils.PrintAndLogfWarning("Note: snapshot %q is a placeholder (its capture failed at the time); skipping it in the diff chain.\n", h.Name())
		} else {
			c, lerr := schemasnapshot.LoadSnapshotByName(metaDB, h.Name())
			switch {
			case lerr == nil:
				content = c
			case errors.Is(lerr, schemasnapshot.ErrPlaceholderSnapshot), errors.Is(lerr, schemasnapshot.ErrSnapshotNotFound):
				utils.PrintAndLogfWarning("Note: could not load snapshot %q (%v); skipping it in the diff chain.\n", h.Name(), lerr)
			default:
				utils.PrintAndLogfWarning("Note: error loading snapshot %q (%v); skipping it in the diff chain.\n", h.Name(), lerr)
			}
		}
		snapshotInputs = append(snapshotInputs, driftreport.SnapshotInput{Header: h, Content: content, Series: h.Label})
	}

	// ─── Best-effort live read of the source ────────────────────────────────────
	// Also moved ahead of Scope resolution, for the same reason: its Content (if
	// the capture succeeded) contributes to the candidate table universe too.
	live := captureLiveSnapshotForDrift(schemas)

	// ─── Resolve the collapsed schemadiff.Scope from --table-list/
	// --exclude-table-list and --object-type-list/--exclude-object-type-list.
	// validateDetectDriftFlags already enforced that at most one flag per pair is
	// set, so each dimension resolves to exactly one positive allow-list: either
	// the directly-resolved include patterns, or the complement of the resolved
	// exclude patterns against the full universe (all candidate tables / all v1
	// object types). Neither flag set => nil ("all"). ──────────────────────────
	// Built unconditionally: besides being the set to subtract --exclude-table-list
	// from, it IS the set of tables compared, which the report states. Costs no I/O.
	snapshotContents := make([]*schemasnapshot.SnapshotContent, 0, len(snapshotInputs))
	for _, si := range snapshotInputs {
		snapshotContents = append(snapshotContents, si.Content)
	}
	var liveContent *schemasnapshot.SnapshotContent
	if live != nil {
		liveContent = live.Content
	}
	candidates := buildDriftTableCandidates(snapshotContents, liveContent)

	var includeTables []schemasnapshot.ObjectRef
	switch {
	case driftTableList != "":
		includeTables, err = resolveDriftTableRefs(candidates, driftTableList, "table-list")
		if err != nil {
			exitDriftOperationalError("%v", err)
		}
	case driftExcludeTableList != "":
		excludeTables, err := resolveDriftTableRefs(candidates, driftExcludeTableList, "exclude-table-list")
		if err != nil {
			exitDriftOperationalError("%v", err)
		}
		includeTables = complementDriftTableRefs(candidates, excludeTables)
		// Empty means "all" inside Scope, so excluding everything would invert into
		// comparing everything. Selecting nothing is an operational error instead.
		if len(includeTables) == 0 {
			exitDriftOperationalError("--exclude-table-list %q excludes every table in the comparison; nothing left to compare", driftExcludeTableList)
		}
	}

	var objectTypes []schemadiff.ObjectType
	switch {
	case driftObjectTypeList != "":
		// Already flag-format-validated in validateDetectDriftFlags; error can't happen.
		objectTypes, _ = parseDriftObjectTypeList(driftObjectTypeList)
	case driftExcludeObjectTypeList != "":
		// Already flag-format-validated in validateDetectDriftFlags; error can't happen.
		excludeObjectTypes, _ := parseDriftObjectTypeList(driftExcludeObjectTypeList)
		objectTypes = complementDriftObjectTypes(excludeObjectTypes)
		// Same trap as --exclude-table-list above.
		if len(objectTypes) == 0 {
			supported := lo.Map(allDriftObjectTypes, func(t schemadiff.ObjectType, _ int) string { return string(t) })
			exitDriftOperationalError("--exclude-object-type-list %q excludes every supported object type (%s); nothing left to compare",
				driftExcludeObjectTypeList, strings.Join(supported, ", "))
		}
	}

	// Scope takes one positive allow-list per dimension (empty = all); resolving the
	// --exclude-* forms into keep-sets is this layer's job. See schemadiff.Scope.
	scope := schemadiff.Scope{
		Tables:      includeTables,
		ObjectTypes: objectTypes,
	}

	// Both lists are ALWAYS populated, so "Comparing" can state what was actually
	// compared: the whole candidate universe when unfiltered, else the resolved
	// keep-set the engine received (including an --exclude-* complement).
	tablesFiltered := driftTableList != "" || driftExcludeTableList != ""
	effectiveTables := includeTables
	if !tablesFiltered {
		effectiveTables = lo.Map(candidates, func(c driftTableCandidate, _ int) schemasnapshot.ObjectRef { return c.ref })
	}
	displayTables := lo.Map(effectiveTables, func(r schemasnapshot.ObjectRef, _ int) string {
		return r.ForDisplay(source.DBType)
	})

	objectTypesFiltered := driftObjectTypeList != "" || driftExcludeObjectTypeList != ""
	effectiveObjectTypes := objectTypes
	if !objectTypesFiltered {
		effectiveObjectTypes = allDriftObjectTypes
	}
	displayObjectTypes := lo.Map(effectiveObjectTypes, func(t schemadiff.ObjectType, _ int) string { return string(t) })

	report := driftreport.BuildReport(driftreport.BuildParams{
		Source: driftreport.Source{
			DatabaseType:    source.DBType,
			Host:            source.Host,
			Port:            source.Port,
			Database:        source.DBName,
			DatabaseVersion: source.DBVersion,
		},
		Schemas:             schemas,
		Snapshots:           snapshotInputs,
		Live:                live,
		Scope:               scope,
		Tables:              displayTables,
		TablesFiltered:      tablesFiltered,
		ObjectTypes:         displayObjectTypes,
		ObjectTypesFiltered: objectTypesFiltered,
		GeneratedAt:         time.Now().UTC(),
	})

	writtenPaths, err := writeDriftReports(report, driftOutputFormat)
	if err != nil {
		exitDriftOperationalError("%v", err)
	}

	printDriftSummary(report, writtenPaths)

	if report.Summary.ChangeCount > 0 {
		os.Exit(1)
	}
	// No drift found: return normally (exit code 0).
}

// captureLiveSnapshotForDrift attempts a best-effort, in-memory-only schema
// capture of the source for comparison against the historical snapshot chain.
// Unlike CaptureAndSaveSnapshot, the result is never persisted. Any failure
// (unsupported source type, capture error) is logged as a note and yields a nil
// result -- the source being briefly unreachable (or the capture racing DDL)
// must never fail the whole command, since the snapshot-only comparison is
// still useful on its own.
func captureLiveSnapshotForDrift(schemas []string) *driftreport.SnapshotInput {
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
		Label:        schemasnapshot.LabelDetectDrift,
	})
	if err != nil {
		utils.PrintAndLogfWarning("Note: could not capture live schema for comparison: %v; continuing with snapshot-only comparison.\n", err)
		return nil
	}
	return &driftreport.SnapshotInput{Header: snap.Header, Content: snap.Content, Series: driftreport.SeriesSourceLive}
}

// writeDriftReports renders and writes report to <export-dir>/reports/ in each
// of the comma-separated formats in formatSpec, creating the reports directory
// if necessary. Returns the paths written, in the same order as formatSpec.
func writeDriftReports(report driftreport.Report, formatSpec string) ([]string, error) {
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
			data, err = driftreport.RenderJSON(report)
		case "html":
			data, err = driftreport.RenderHTML(report)
		default:
			// Unreachable: already validated in validateDriftOutputFormat.
			return nil, fmt.Errorf("unsupported output format %q", f)
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
func printDriftSummary(report driftreport.Report, writtenPaths []string) {
	utils.PrintAndLogfPhase("\nSchema drift summary")

	printDriftSummaryField("Comparison window", utils.PrintAndLogf,
		"%s -> %s", formatDriftTimestamp(report.Window.From), formatDriftTimestamp(report.Window.To))
	printDriftSummaryField("Snapshots compared", utils.PrintAndLogf,
		"%d (live source comparison: %t)", report.Summary.CaptureCount, report.Summary.LiveCompared)
	printDriftSummaryField("Schemas", utils.PrintAndLogf, "%s", joinOrAllDrift(report.Comparing.Schemas))
	printDriftSummaryField("Tables", utils.PrintAndLogf, "%s",
		driftScopeLine(report.Comparing.Tables, report.Comparing.TablesFiltered))

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
// for the longest label ("Snapshots compared").
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

// driftScopeLine renders the scope line: a count saying whether this is everything
// or a filter's result, then the names while they fit.
func driftScopeLine(names []string, filtered bool) string {
	if len(names) == 0 {
		return "none"
	}
	head := fmt.Sprintf("all %d", len(names))
	if filtered {
		head = fmt.Sprintf("%d (filtered)", len(names))
	}
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

// joinOrAllDrift joins items with ", ", or returns "all" when items is empty.
func joinOrAllDrift(items []string) string {
	if len(items) == 0 {
		return "all"
	}
	return strings.Join(items, ", ")
}
