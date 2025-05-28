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

package importdata

import (
	"fmt"
	"strings"
)

/*
ErrorPolicy defines what to do when an error occurs during the following tasks:
1. Read the data from the source
2. Transform the data
3. Write the data to the target
If an error occurs during any of these tasks related to data specifically, a policy will
determine what needs to be done.
*/
// type ErrorPolicy int
type ErrorPolicy string

const (
	AbortErrorPolicy            ErrorPolicy = AbortErrorPolicyName            // Abort the task and return an error
	StashAndContinueErrorPolicy ErrorPolicy = StashAndContinueErrorPolicyName // Stash the error to a file and continue with the task
)

const (
	AbortErrorPolicyName            = "abort"
	StashAndContinueErrorPolicyName = "stash-and-continue"
)

var validErrorPolicyNames = []string{
	AbortErrorPolicyName,
	StashAndContinueErrorPolicyName,
}

func (e *ErrorPolicy) Type() string {
	return "ErrorPolicy"
}

func (e *ErrorPolicy) String() string {
	return string(*e)
}

func (e *ErrorPolicy) Set(s string) error {
	policy, err := NewErrorPolicy(s)
	if err != nil {
		return err
	}
	*e = policy
	return nil
}

func NewErrorPolicy(s string) (ErrorPolicy, error) {
	s = strings.ToLower(s)
	switch s {
	case AbortErrorPolicyName:
		return AbortErrorPolicy, nil
	case StashAndContinueErrorPolicyName:
		return StashAndContinueErrorPolicy, nil
	default:
		return "", fmt.Errorf("invalid error policy: %q. Allowed error policies: %v", s, validErrorPolicyNames)
	}
}
