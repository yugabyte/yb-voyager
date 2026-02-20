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
package testutils

import (
	"os"
	"testing"

	"github.com/fatih/color"
)

func init() {
	// Force colors even when go test output isn't a TTY.
	color.NoColor = false
	color.Output = os.Stdout
}

const testLogPrefix = "[TEST] "

func LogTest(t *testing.T, msg string) {
	t.Helper()
	t.Log(color.HiCyanString("%s%s", testLogPrefix, msg))
}

func LogTestf(t *testing.T, format string, args ...any) {
	t.Helper()
	t.Log(color.HiCyanString(testLogPrefix+format, args...))
}
