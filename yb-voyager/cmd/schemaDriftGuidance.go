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

// Named because the export branch uses one directly: routing it through
// schemaDriftErrorHintLeadIn would make exportDataCmd's initializer depend on a
// function naming exportDataCmd, which Go rejects as an initialization cycle.
const (
	exportDataDriftHintLeadIn = "export data exited with an error."
	importDataDriftHintLeadIn = "import data exited with an error."
)

func driftDetectionHint() string {
	return fmt.Sprintf("\t%s --export-dir %q (with your source connection flags)", detectDriftCmd.CommandPath(), exportDir)
}

// Capture is off by default, so without this gate every hint below would send users to
// a command that exits 2 with "holds no schema snapshots". Placeholders are
// failed-capture markers carrying no schema, so they do not count.
//
// A listing failure is only logged, not returned: these hints are advisory, and the
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

func printSchemaDriftErrorHint(firstLine string) {
	if !schemaDriftGuidanceIsUseful() {
		return
	}
	utils.PrintAndLog(fmt.Sprintf("%s If the source schema may have changed since export began, review schema drift before retrying or cutting over:\n%s",
		firstLine, driftDetectionHint()))
}

// The source exporter's exit capture is the last source-schema snapshot the migration
// records -- `export data from target` takes none -- and it is not written until after
// this prompt, so detect-drift's live source read is the only thing covering the
// window up to cutover.
func printCutoverSchemaDriftRecommendation() {
	if !schemaDriftGuidanceIsUseful() {
		return
	}
	utils.PrintAndLog(fmt.Sprintf("Recommendation: cutover to target ends schema capture on the source. Consider reviewing schema drift on the source before proceeding:\n%s",
		driftDetectionHint()))
}

// Only the forward path qualifies. `import data to source`, `import data to
// source-replica` and `export data from target` all run after cutover to target, past
// the last source capture, which the design spec puts out of scope for v1.
func schemaDriftErrorHintLeadIn(commandPath string) (string, bool) {
	switch commandPath {
	case importDataCmd.CommandPath(), importDataToTargetCmd.CommandPath():
		return importDataDriftHintLeadIn, true
	case exportDataCmd.CommandPath(), exportDataFromSrcCmd.CommandPath():
		if exporterRole != SOURCE_DB_EXPORTER_ROLE {
			return "", false
		}
		return exportDataDriftHintLeadIn, true
	default:
		return "", false
	}
}

// Called from the process exit path, the only place that sees the export and import
// data failures which go through utils.ErrExit instead of a return value.
func printSchemaDriftErrorHintOnExit(commandPath string) {
	leadIn, ok := schemaDriftErrorHintLeadIn(commandPath)
	if !ok {
		return
	}
	printSchemaDriftErrorHint(leadIn)
}
