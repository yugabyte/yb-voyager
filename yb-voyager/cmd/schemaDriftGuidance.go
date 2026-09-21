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

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/schemasnapshot"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// Lead-in sentences for the failure footers. Named because the export branch has to
// use one directly: routing it through schemaDriftErrorFooterLeadIn would make
// exportDataCmd's initializer depend on a function that names exportDataCmd, which Go
// rejects as an initialization cycle.
const (
	exportDataDriftFooterLeadIn = "export data exited with an error."
	importDataDriftFooterLeadIn = "import data exited with an error."
)

// driftDetectionHint returns the example invocation of `schema detect-drift`
// printed as part of the guidance footers below.
func driftDetectionHint() string {
	return fmt.Sprintf("\t%s --export-dir %q (with your source connection flags)", detectDriftCmd.CommandPath(), exportDir)
}

// schemaDriftGuidanceIsUseful reports whether the export dir already holds a snapshot
// that `schema detect-drift` could report on. Capture is off by default, so without
// this gate every footer below would send users to a command that exits 2 with
// "holds no schema snapshots". Placeholders are failed-capture markers carrying no
// schema, so they do not count.
//
// A listing failure is only logged, not returned: these footers are advisory, and the
// call site that matters most is an exit path with nothing left to fail.
func schemaDriftGuidanceIsUseful() bool {
	if metaDB == nil {
		return false
	}
	headers, err := schemasnapshot.ListSnapshots(metaDB)
	if err != nil {
		log.Warnf("schema-drift guidance: could not list schema snapshots: %v", err)
		return false
	}
	return lo.SomeBy(headers, func(h schemasnapshot.SnapshotHeader) bool { return !h.IsPlaceholder })
}

// printSchemaDriftErrorFooter prints a guidance footer nudging the user
// towards `schema detect-drift` after an export/import data command has
// exited with an error. firstLine is the caller-specific lead-in sentence
// (e.g. "export data exited with an error."); the rest of the message and
// the example invocation are identical across call sites.
func printSchemaDriftErrorFooter(firstLine string) {
	if !schemaDriftGuidanceIsUseful() {
		return
	}
	utils.PrintAndLog(fmt.Sprintf("%s If the source schema may have changed since export began, review schema drift before retrying or cutting over:\n%s",
		firstLine, driftDetectionHint()))
}

// printCutoverSchemaDriftRecommendation nudges the user to review source drift
// before they confirm cutover to target. The source exporter's exit capture is the
// last source-schema snapshot the migration records -- `export data from target`
// takes none -- and it is not written until after this prompt, so detect-drift's
// live source read is the only thing covering the window up to cutover.
func printCutoverSchemaDriftRecommendation() {
	if !schemaDriftGuidanceIsUseful() {
		return
	}
	utils.PrintAndLog(fmt.Sprintf("Recommendation: cutover to target ends schema capture on the source. Consider reviewing schema drift on the source before proceeding:\n%s",
		driftDetectionHint()))
}

// schemaDriftErrorFooterLeadIn returns the lead-in sentence for a failing command,
// and false when that command gets no footer at all.
//
// Only the forward path qualifies. `import data to source`, `import data to
// source-replica` and `export data from target` all run after cutover to target,
// past the last source capture, which the design spec puts out of scope for v1.
func schemaDriftErrorFooterLeadIn(commandPath string) (string, bool) {
	switch commandPath {
	case importDataCmd.CommandPath(), importDataToTargetCmd.CommandPath():
		return importDataDriftFooterLeadIn, true
	case exportDataCmd.CommandPath(), exportDataFromSrcCmd.CommandPath():
		if exporterRole != SOURCE_DB_EXPORTER_ROLE {
			return "", false
		}
		return exportDataDriftFooterLeadIn, true
	default:
		return "", false
	}
}

// printSchemaDriftErrorFooterOnExit prints the footer from the process exit path,
// the only place that sees the export/import data failures which go through
// utils.ErrExit instead of a return value -- exportData.go alone has 95 of them.
func printSchemaDriftErrorFooterOnExit(commandPath string) {
	leadIn, ok := schemaDriftErrorFooterLeadIn(commandPath)
	if !ok {
		return
	}
	printSchemaDriftErrorFooter(leadIn)
}
