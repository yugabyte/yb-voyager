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
package cp

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

// StepForCommand is the step registry: the step a command claims, if any.
func StepForCommand(commandPath string, postSnapshotImport bool) (string, bool) {
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
		return STEP_EXPORT_DATA_FROM_SOURCE, true
	case "yb-voyager import data", "yb-voyager import data to target":
		return STEP_IMPORT_DATA_TO_TARGET, true
	case "yb-voyager finalize-schema-post-data-import":
		return STEP_FINALIZE_SCHEMA, true
	case "yb-voyager end migration":
		return STEP_END_MIGRATION, true
	}
	return "", false
}
