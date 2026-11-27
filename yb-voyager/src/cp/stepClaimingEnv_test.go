//go:build unit

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

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWithoutStepClaimingEnv(t *testing.T) {
	environ := []string{"A=1", "VOYAGER_OPERATION_ID=x", "VOYAGER_STEP_EXECUTION_ID=7", "VOYAGER_OPERATION_ID_EXTRA=y"}
	assert.Equal(t, []string{"A=1", "VOYAGER_OPERATION_ID_EXTRA=y"}, WithoutStepClaimingEnv(environ))
}
