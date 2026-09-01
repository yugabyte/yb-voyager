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
//	go run ./src/testlivemigration/sweepreport report  -results results/run.csv \
//	        -catalog results/probe-catalog.json -out results/report-rows.json
//	go run ./src/testlivemigration/sweepreport diff    -old results/v1.csv -new results/v2.csv
//
// It imports nothing from voyager and needs no build tag, so it runs anywhere.
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
	"time"
)

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
	logPath := fs.String("log", "-", "go test log to parse ('-' for stdin)")
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

	in := os.Stdin
	if *logPath != "-" {
		f, err := os.Open(*logPath)
		if err != nil {
			return err
		}
		defer f.Close()
		in = f
	}

	rows, err := ParseLog(in, meta, categoryFor)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return fmt.Errorf("no PROBE-RESULT lines found in %s; did the run produce any output?", *logPath)
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
		if r.RunStatus != statusOK {
			bad++
		}
	}
	fmt.Printf("wrote %d rows to %s\n", len(rows), path)
	fmt.Printf("verdicts: %s\n", renderCounts(counts))
	if bad > 0 {
		fmt.Printf("WARNING: %d rows come from runs that did not pass their control gate "+
			"(run_status != OK) and must not be published\n", bad)
		if *failOnInvalid {
			return fmt.Errorf("%d rows from invalid runs", bad)
		}
	}
	return nil
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
