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
	"fmt"
	"os"
	"path/filepath"
)

// GetFailpointEnvVar formats the GO_FAILPOINTS environment variable for multiple failpoints.
// Multiple failpoints are separated by semicolons.
//
// Example:
//
//	GetFailpointEnvVar("pkg/fp1=return()", "pkg/fp2=return(\"error\")")
//	Returns: "GO_FAILPOINTS=pkg/fp1=return();pkg/fp2=return(\"error\")"
func GetFailpointEnvVar(failpoints ...string) string {
	if len(failpoints) == 0 {
		return ""
	}

	var result string
	for i, fp := range failpoints {
		if i > 0 {
			result += ";"
		}
		result += fp
	}
	return fmt.Sprintf("GO_FAILPOINTS=%s", result)
}

// DefaultCopyRetryCountFailpoint sets COPY_MAX_RETRY_COUNT to 1 for faster test failure.
const DefaultCopyRetryCountFailpoint = "github.com/yugabyte/yb-voyager/yb-voyager/cmd/setCopyRetryCount=return(1)"

// GetFailpointEnvVarWithDefaults includes the default retry count failpoint (COPY_MAX_RETRY_COUNT=1)
// along with any custom failpoints provided. Use this for import data failpoint tests.
//
// Example:
//
//	GetFailpointEnvVarWithDefaults("pkg/fp1=return()")
//	Returns: "GO_FAILPOINTS=github.com/.../setCopyRetryCount=return(1);pkg/fp1=return()"
func GetFailpointEnvVarWithDefaults(failpoints ...string) string {
	allFailpoints := append([]string{DefaultCopyRetryCountFailpoint}, failpoints...)
	return GetFailpointEnvVar(allFailpoints...)
}

// WriteBytemanScript creates a Byteman rules file (.btm) for Java fault injection testing.
// This is used for testing Java components like Debezium Server.
//
// Parameters:
//   - exportDir: The export directory where the script will be created
//   - rules: The Byteman rules content
//
// Returns: Path to the created script file
func WriteBytemanScript(exportDir, rules string) (string, error) {
	scriptPath := filepath.Join(exportDir, "byteman-rules.btm")
	err := os.WriteFile(scriptPath, []byte(rules), 0644)
	if err != nil {
		return "", fmt.Errorf("failed to write Byteman script: %w", err)
	}
	return scriptPath, nil
}

// GetBytemanEnvVars returns the environment variables needed to enable Byteman for Java processes.
// Returns an empty slice if BYTEMAN_HOME is not set (Byteman not available).
//
// Parameters:
//   - scriptPath: Path to the Byteman rules script
//
// Returns: Slice of environment variable strings in "KEY=VALUE" format
func GetBytemanEnvVars(scriptPath string) []string {
	bytemanHome := os.Getenv("BYTEMAN_HOME")
	if bytemanHome == "" {
		return []string{} // Byteman not available
	}

	return []string{
		fmt.Sprintf("BYTEMAN_HOME=%s", bytemanHome),
		fmt.Sprintf("BYTEMAN_SCRIPT=%s", scriptPath),
		"VOYAGER_USE_TESTING_SCRIPT=true",
	}
}

// SkipIfBytemanNotAvailable skips the test if Byteman is not installed/configured.
// This is a helper for Java failpoint tests that require Byteman.
//
// Usage:
//
//	func TestWithByteman(t *testing.T) {
//	    testutils.SkipIfBytemanNotAvailable(t)
//	    // ... test code ...
//	}
func SkipIfBytemanNotAvailable(t interface{ Skip(args ...interface{}) }) {
	if os.Getenv("BYTEMAN_HOME") == "" {
		t.Skip("BYTEMAN_HOME not set, skipping Byteman test")
	}
}

// RequireBytemanAvailable fails the test if Byteman is not installed/configured.
// This is used for tests where Byteman is a required dependency.
// It checks for the BYTEMAN_JAR environment variable which should point to byteman.jar.
//
// Usage:
//
//	func TestWithRequiredByteman(t *testing.T) {
//	    testutils.RequireBytemanAvailable(t)
//	    // ... test code ...
//	}
func RequireBytemanAvailable(t interface {
	Fatalf(format string, args ...interface{})
}) {
	if os.Getenv("BYTEMAN_JAR") == "" {
		t.Fatalf("BYTEMAN_JAR environment variable not set - Byteman is required for this test. " +
			"Please install Byteman and set BYTEMAN_JAR to the path of byteman.jar")
	}
}
