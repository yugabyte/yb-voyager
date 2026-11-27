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

import "strings"

const (
	OPERATION_ID_ENV_VAR = "VOYAGER_OPERATION_ID"
	// STEP_EXECUTION_ID_ENV_VAR is internal: Voyager sets it for its own child
	// process, which joins the parent's step execution instead of claiming one.
	STEP_EXECUTION_ID_ENV_VAR = "VOYAGER_STEP_EXECUTION_ID"
)

// WithoutStepClaimingEnv returns environ without the step claiming variables, so
// a child Voyager process never claims a step of its own, whatever its control plane.
func WithoutStepClaimingEnv(environ []string) []string {
	env := make([]string, 0, len(environ)+1)
	for _, entry := range environ {
		if strings.HasPrefix(entry, OPERATION_ID_ENV_VAR+"=") || strings.HasPrefix(entry, STEP_EXECUTION_ID_ENV_VAR+"=") {
			continue
		}
		env = append(env, entry)
	}
	return env
}
