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

import controlPlane "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"

// The step each migration event type reports. Its done event closes the step execution.
var stepForEventType = map[string]string{
	"ASSESS MIGRATION": controlPlane.STEP_ASSESS_MIGRATION,
	"EXPORT SCHEMA":    controlPlane.STEP_EXPORT_SCHEMA,
	"ANALYZE SCHEMA":   controlPlane.STEP_ANALYZE_SCHEMA,
	"EXPORT DATA":      controlPlane.STEP_EXPORT_DATA_FROM_SOURCE,
	"IMPORT SCHEMA":    controlPlane.STEP_IMPORT_SCHEMA,
	"IMPORT DATA":      controlPlane.STEP_IMPORT_DATA_TO_TARGET,
}
