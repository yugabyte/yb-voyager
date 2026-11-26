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
package yugabytedb

import "github.com/yugabyte/yb-voyager/yb-voyager/src/constants"

// See design-specs/cp-step-claiming.md.

const (
	STEP_ASSESS_MIGRATION        = "assess-migration"
	STEP_EXPORT_SCHEMA           = "export-schema"
	STEP_ANALYZE_SCHEMA          = "analyze-schema"
	STEP_IMPORT_SCHEMA           = "import-schema"
	STEP_EXPORT_DATA_FROM_SOURCE = "export-data-from-source"
	STEP_IMPORT_DATA_TO_TARGET   = "import-data-to-target"
	STEP_FINALIZE_SCHEMA         = "finalize-schema"
	STEP_END_MIGRATION           = "end-migration"
)

// The step each migration event type reports. Its done event closes the attempt.
var phaseSteps = map[string]string{
	"ASSESS MIGRATION": STEP_ASSESS_MIGRATION,
	"EXPORT SCHEMA":    STEP_EXPORT_SCHEMA,
	"ANALYZE SCHEMA":   STEP_ANALYZE_SCHEMA,
	"EXPORT DATA":      STEP_EXPORT_DATA_FROM_SOURCE,
	"IMPORT SCHEMA":    STEP_IMPORT_SCHEMA,
	"IMPORT DATA":      STEP_IMPORT_DATA_TO_TARGET,
}

// StepForCommand is the step registry: the step a command claims, if any.
func StepForCommand(commandPath, exporterRole, importerRole string, postSnapshotImport bool) (string, bool) {
	switch commandPath {
	case "yb-voyager assess-migration":
		return STEP_ASSESS_MIGRATION, true
	case "yb-voyager export schema":
		return STEP_EXPORT_SCHEMA, true
	case "yb-voyager analyze-schema":
		return STEP_ANALYZE_SCHEMA, true
	case "yb-voyager import schema":
		if postSnapshotImport {
			return STEP_FINALIZE_SCHEMA, true
		}
		return STEP_IMPORT_SCHEMA, true
	case "yb-voyager export data", "yb-voyager export data from source":
		return STEP_EXPORT_DATA_FROM_SOURCE, exporterRole == constants.SOURCE_DB_EXPORTER_ROLE
	case "yb-voyager import data", "yb-voyager import data to target":
		return STEP_IMPORT_DATA_TO_TARGET, importerRole == constants.TARGET_DB_IMPORTER_ROLE
	case "yb-voyager finalize-schema-post-data-import":
		return STEP_FINALIZE_SCHEMA, true
	case "yb-voyager end migration":
		return STEP_END_MIGRATION, true
	}
	return "", false
}
