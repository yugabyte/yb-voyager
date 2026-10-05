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
package errs

import "fmt"

const (
	// steps
	SCHEMA_DRIFT_STEP_SETUP               = "setup"
	SCHEMA_DRIFT_STEP_CONNECT_TO_SOURCE   = "connect_to_source"
	SCHEMA_DRIFT_STEP_RESOLVE_SCHEMAS     = "resolve_schemas"
	SCHEMA_DRIFT_STEP_LOAD_SNAPSHOTS      = "load_snapshots"
	SCHEMA_DRIFT_STEP_CAPTURE_LIVE_SCHEMA = "capture_live_schema"
	SCHEMA_DRIFT_STEP_RESOLVE_SCOPE       = "resolve_scope"
	SCHEMA_DRIFT_STEP_BUILD_REPORT        = "build_report"
	SCHEMA_DRIFT_STEP_NOTHING_COMPARED    = "nothing_compared"
	SCHEMA_DRIFT_STEP_WRITE_REPORTS       = "write_reports"
)

type SchemaDriftError struct {
	step string
	err  error
}

func (e SchemaDriftError) Step() string {
	return e.step
}

func (e SchemaDriftError) Error() string {
	return fmt.Sprintf("schema drift: step=%s: %s", e.step, e.err.Error())
}

func (e SchemaDriftError) Unwrap() error {
	return e.err
}

func NewSchemaDriftError(step string, err error) SchemaDriftError {
	return SchemaDriftError{
		step: step,
		err:  err,
	}
}
