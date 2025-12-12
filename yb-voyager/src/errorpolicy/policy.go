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

package errorpolicy

import (
	goerrors "github.com/go-errors/errors"
)

/*
ErrorPolicy defines what to do when an error occurs during a task.
Currently, it is used in the context of import-data, where we do the following tasks:
1. Read the data from the source
2. Transform the data
3. Write the data to the target
If an error occurs during any of these tasks related to data specifically, a policy will
determine what needs to be done.
*/
type ErrorPolicy int

const (
	AbortErrorPolicy            ErrorPolicy = iota // Abort the task and return an error
	StashAndContinueErrorPolicy                    // Stash the error to a file and continue with the task
)

const (
	AbortErrorPolicyName            = "Abort"
	StashAndContinueErrorPolicyName = "StashAndContinue"
)

var errorPolicyNames = map[ErrorPolicy]string{
	AbortErrorPolicy:            AbortErrorPolicyName,
	StashAndContinueErrorPolicy: StashAndContinueErrorPolicyName,
}

func (e ErrorPolicy) String() string {
	return errorPolicyNames[e]
}

func NewErrorPolicy(s string) (ErrorPolicy, error) {
	switch s {
	case AbortErrorPolicyName:
		return AbortErrorPolicy, nil
	case StashAndContinueErrorPolicyName:
		return StashAndContinueErrorPolicy, nil
	default:
		return 0, goerrors.Errorf("invalid error policy: %s", s)
	}
}
