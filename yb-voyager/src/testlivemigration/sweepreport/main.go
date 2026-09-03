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

// Command sweepreport turns a datatype-sweep `go test` log into machine-readable
// results, joins those results with the generated probe catalog to produce the published
// report's row data, and diffs two result sets.
//
//	go run ./src/testlivemigration/sweepreport collect -log run.log -out results/run.csv
//	go run ./src/testlivemigration/sweepreport collect -log a.log -log b.log -logs 'more/*.log' -out results/run.csv
//	go run ./src/testlivemigration/sweepreport report  -results results/run.csv \
//	        -catalog results/probe-catalog.json -out results/report-rows.json
//	go run ./src/testlivemigration/sweepreport diff    -old results/v1.csv -new results/v2.csv
//
// collect's -log flag is repeatable and combines with -logs (a glob); each log is parsed
// as its own run and the results are merged, so multiple sweep runs no longer need to be
// `cat`-ed together before collecting (see cmdCollect for why that matters).
//
// It imports nothing from voyager and needs no build tag, so it runs anywhere.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// stringSlice implements flag.Value so a flag can be repeated on the command line, each
// occurrence appending rather than overwriting. Used by collect's -log flag: before this,
// -log accepted exactly one path, which forced callers to `cat` several go-test logs
// together before handing them to `collect` - and concatenating logs destroys the run
// boundary ParseLog relies on for its control gate (see cmdCollect).
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ",") }

func (s *stringSlice) Set(v string) error {
	*s = append(*s, v)
	return nil
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}
	var err error
	switch os.Args[1] {
	case "collect":
		err = cmdCollect(os.Args[2:])
	case "report":
		err = cmdReport(os.Args[2:])
	case "diff":
		err = cmdDiff(os.Args[2:])
	case "-h", "--help", "help":
		usage()
		return
	default:
		usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "sweepreport: %v\n", err)
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `sweepreport <command> [flags]

  collect  parse a datatype-sweep go-test log into a results CSV
  report   join a results CSV with the probe catalog into the published report rows
  diff     compare two results CSVs and print regressions and improvements

Run "sweepreport <command> -h" for the flags of one command.
`)
}

// ============================================================
// collect
// ============================================================

func cmdCollect(args []string) error {
	fs := flag.NewFlagSet("collect", flag.ExitOnError)
	var logPaths stringSlice
	fs.Var(&logPaths, "log", "go test log to parse ('-' for stdin); repeat to merge several runs")
	logsGlob := fs.String("logs", "", "glob matching several go test logs to parse (e.g. 'results/*.log'); merged with -log")
	out := fs.String("out", "", "results CSV to write (default results/datatype-sweep-<timestamp>.csv)")
	outDir := fs.String("dir", "results", "directory for the default -out path")
	jsonOut := fs.String("json", "", "also write the rows as JSON to this path")
	catalogPath := fs.String("catalog", "", "probe catalog JSON, used as the authoritative probe->group mapping")
	commit := fs.String("commit", "", "voyager commit the run was built from")
	pgVersion := fs.String("pg-version", "", "source PostgreSQL version under test")
	ybVersion := fs.String("yb-version", "", "target YugabyteDB version under test")
	stamp := fs.String("timestamp", "", "run timestamp (default: now, UTC, RFC3339)")
	failOnInvalid := fs.Bool("fail-on-invalid", false, "exit non-zero if any run failed its control gate")
	if err := fs.Parse(args); err != nil {
		return err
	}

	meta := RunMeta{
		Timestamp:     *stamp,
		VoyagerCommit: *commit,
		PGVersion:     *pgVersion,
		YBVersion:     *ybVersion,
	}
	if meta.Timestamp == "" {
		meta.Timestamp = time.Now().UTC().Format(time.RFC3339)
	}

	var categoryFor func(string) string
	if *catalogPath != "" {
		cat, err := ReadCatalog(*catalogPath)
		if err != nil {
			return err
		}
		groups := map[string]string{}
		for _, e := range cat.Entries {
			groups[e.ProbeID] = e.Group
		}
		categoryFor = func(id string) string { return groups[id] }
	}

	// Build the full list of logs to parse: explicit -log occurrences first, then -logs
	// glob matches, sorted so a rebuilt result set is byte-stable. When neither is given,
	// fall back to a single "-" (stdin), exactly the old default.
	paths := append([]string{}, logPaths...)
	if *logsGlob != "" {
		matches, err := filepath.Glob(*logsGlob)
		if err != nil {
			return fmt.Errorf("bad -logs glob %q: %w", *logsGlob, err)
		}
		if len(matches) == 0 {
			return fmt.Errorf("-logs glob %q matched no files", *logsGlob)
		}
		sort.Strings(matches)
		paths = append(paths, matches...)
	}
	if len(paths) == 0 {
		paths = []string{"-"}
	}

	// Each log is ONE test run and is parsed by its OWN ParseLog call, independently of
	// every other log. This is the fix for the reason multi-log support exists at all:
	// ParseLog tracks the control gate per run (see the ctrlBad / runStatus bookkeeping in
	// results.go), so feeding it the concatenation of several `cat`-ed-together logs lets a
	// single PROBE-RUN-INVALID line - or a single failed control - taint every row parsed
	// after it, including rows from runs that have nothing to do with the failure. Parsing
	// file-by-file keeps that contamination inside the file it belongs to; only the merge
	// below (dedupe, via its trust-ranked preference) lets a good run's row win over a bad
	// run's row for the same probe.
	var allRows []Row
	for _, p := range paths {
		rc, err := openLog(p)
		if err != nil {
			return fmt.Errorf("opening %s: %w", p, err)
		}
		fileRows, err := ParseLog(rc, meta, categoryFor)
		rc.Close()
		if err != nil {
			return fmt.Errorf("parsing %s: %w", p, err)
		}
		allRows = append(allRows, fileRows...)
	}
	rows := dedupe(allRows)
	if len(rows) == 0 {
		return fmt.Errorf("no PROBE-RESULT lines found in %s; did the run produce any output?",
			strings.Join(paths, ", "))
	}

	path := *out
	if path == "" {
		path = DefaultResultsPath(*outDir, meta)
	}
	if err := WriteCSV(path, rows); err != nil {
		return err
	}
	if *jsonOut != "" {
		if err := WriteJSON(*jsonOut, rows); err != nil {
			return err
		}
	}

	bad := 0
	counts := map[string]int{}
	for _, r := range rows {
		counts[r.Verdict]++
		// ATTRIBUTED counts as published-quality here, same as OK: it is the carve-out
		// that pins a control failure on the one probe that could have caused it (see
		// statusAttributed), not a broken measurement. POISON is deliberately NOT included
		// - a poison-isolation run still trips this warning today, and that is unrelated to
		// this fix, so it is left alone.
		if r.RunStatus != statusOK && r.RunStatus != statusAttributed {
			bad++
		}
	}
	fmt.Printf("wrote %d rows to %s (from %d log(s))\n", len(rows), path, len(paths))
	fmt.Printf("verdicts: %s\n", renderCounts(counts))
	if bad > 0 {
		fmt.Printf("WARNING: %d rows come from runs that did not pass their control gate "+
			"(run_status != OK/ATTRIBUTED) and must not be published\n", bad)
		if *failOnInvalid {
			return fmt.Errorf("%d rows from invalid runs", bad)
		}
	}
	return nil
}

// openLog opens one collect input, treating "-" as stdin. Returned for a uniform
// io.ReadCloser so cmdCollect's per-file loop doesn't special-case stdin's lack of a Close
// worth calling.
func openLog(path string) (io.ReadCloser, error) {
	if path == "-" {
		return io.NopCloser(os.Stdin), nil
	}
	return os.Open(path)
}

func renderCounts(counts map[string]int) string {
	order := []string{
		"WORKS", "EXCLUDED_TOLD", "BLOCKS", "STUCK", "EXPORTER_CRASHES",
		"QUIET_DROP", "SILENT_WRONG", "SILENT_LOSS", "SKIPPED", "INCONCLUSIVE",
	}
	var parts []string
	seen := map[string]bool{}
	for _, v := range order {
		seen[v] = true
		if counts[v] > 0 {
			parts = append(parts, fmt.Sprintf("%s=%d", v, counts[v]))
		}
	}
	for v, n := range counts {
		if !seen[v] {
			parts = append(parts, fmt.Sprintf("%s=%d", v, n))
		}
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, " ")
}

// ============================================================
// report
// ============================================================

func cmdReport(args []string) error {
	fs := flag.NewFlagSet("report", flag.ExitOnError)
	resultsPath := fs.String("results", "", "results CSV produced by `collect` (repeatable with commas)")
	catalogPath := fs.String("catalog", "", "probe catalog JSON produced by TestDatatypeSweepCatalog")
	out := fs.String("out", "results/report-rows.json", "report row data (JSON) to write")
	csvOut := fs.String("csv", "", "also write the rows as a flat CSV to this path")
	required := fs.String("require-modes", "", "comma-separated modes that every probe must have a measurement for (e.g. OFFLINE,LIVE)")
	strict := fs.Bool("strict", false, "exit non-zero if the join reports any problem")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *resultsPath == "" || *catalogPath == "" {
		return fmt.Errorf("both -results and -catalog are required")
	}

	cat, err := ReadCatalog(*catalogPath)
	if err != nil {
		return err
	}

	var rows []Row
	for _, p := range strings.Split(*resultsPath, ",") {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		part, err := ReadCSV(p)
		if err != nil {
			return err
		}
		rows = append(rows, part...)
	}
	rows = dedupe(rows)

	var requiredModes []string
	for _, m := range strings.Split(*required, ",") {
		if m = strings.TrimSpace(strings.ToUpper(m)); m != "" {
			requiredModes = append(requiredModes, m)
		}
	}

	doc, problems := BuildReport(cat, rows, requiredModes)
	if err := WriteJSON(*out, doc); err != nil {
		return err
	}
	if *csvOut != "" {
		if err := WriteReportCSV(*csvOut, doc); err != nil {
			return err
		}
	}
	fmt.Printf("wrote %d report rows to %s (from %d catalog entries, %d measurements)\n",
		len(doc.Rows), *out, len(cat.Entries), len(rows))
	for _, p := range problems {
		fmt.Fprintf(os.Stderr, "  problem: %s\n", p)
	}
	if len(problems) > 0 && *strict {
		return fmt.Errorf("%d problems joining results with the probe catalog", len(problems))
	}
	return nil
}

// ============================================================
// diff
// ============================================================

func cmdDiff(args []string) error {
	fs := flag.NewFlagSet("diff", flag.ExitOnError)
	oldPath := fs.String("old", "", "baseline results CSV")
	newPath := fs.String("new", "", "results CSV to compare against the baseline")
	failOnRegression := fs.Bool("fail-on-regression", false, "exit non-zero when there is a regression or lost coverage")
	jsonOut := fs.String("json", "", "write the diff as JSON to this path")
	if err := fs.Parse(args); err != nil {
		return err
	}
	if *oldPath == "" || *newPath == "" {
		return fmt.Errorf("both -old and -new are required")
	}

	oldRows, err := ReadCSV(*oldPath)
	if err != nil {
		return err
	}
	newRows, err := ReadCSV(*newPath)
	if err != nil {
		return err
	}

	d := Diff(oldRows, newRows)
	PrintDiff(os.Stdout, d, *oldPath, *newPath)
	if *jsonOut != "" {
		if err := WriteJSON(*jsonOut, d); err != nil {
			return err
		}
	}
	if *failOnRegression && d.HasRegressions() {
		return fmt.Errorf("%d regressions, %d rows of lost coverage", len(d.Regressions), len(d.CoverageLoss))
	}
	return nil
}
