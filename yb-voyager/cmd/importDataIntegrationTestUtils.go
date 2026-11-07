//go:build integration || integration_voyager_command

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
	"testing"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

var testYugabyteDBTarget *testutils.TestTargetDB

// setupYugabyteTestDb starts a YugabyteDB test container and points the
// package-level tdb at it. The container/TargetDB setup itself lives in
// test/utils so it can be shared with packages outside cmd.
func setupYugabyteTestDb(t *testing.T) {
	testYugabyteDBTarget = testutils.SetupYugabyteTestDb(t)
	tdb = testYugabyteDBTarget.TargetDB
}
