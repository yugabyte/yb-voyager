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
	"strings"
	"testing"
	"time"
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

// WaitForFailpointMarker polls for a failpoint marker file to appear at the given
// path. Returns (true, nil) if the file appears and contains "hit" before the
// timeout, (false, nil) if the timeout elapses without the marker, or (false, err)
// if the final read attempt fails.
func WaitForFailpointMarker(path string, timeout, pollInterval time.Duration) (bool, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		data, err := os.ReadFile(path)
		if err == nil && strings.Contains(string(data), "hit") {
			return true, nil
		}
		time.Sleep(pollInterval)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}
	return strings.Contains(string(data), "hit"), nil
}

// WaitForProcessExitOrKill waits for a VoyagerCommandRunner to exit naturally.
// If it doesn't exit within the timeout, it sends SIGKILL and returns (true, nil).
// On natural exit it returns (false, err) where err is the process exit error.
func WaitForProcessExitOrKill(runner *VoyagerCommandRunner, timeout time.Duration) (bool, error) {
	errCh := make(chan error, 1)
	go func() {
		errCh <- runner.Wait()
	}()

	select {
	case err := <-errCh:
		return false, err
	case <-time.After(timeout):
		_ = runner.Kill()
		return true, nil
	}
}

// WaitForFailpointAndProcessCrash waits for a failpoint marker file to appear,
// then waits for the given process to exit with an error. Returns an error if
// the marker doesn't appear (failpoints not enabled) or if the process exits
// cleanly instead of crashing.
func WaitForFailpointAndProcessCrash(t *testing.T, runner *VoyagerCommandRunner, markerPath string, markerTimeout, exitTimeout time.Duration) error {
	t.Logf("Waiting for failpoint marker: %s", markerPath)

	matched, err := WaitForFailpointMarker(markerPath, markerTimeout, 2*time.Second)
	if err != nil {
		return fmt.Errorf("error reading failpoint marker %s: %w", markerPath, err)
	}
	if !matched {
		_ = runner.Kill()
		return fmt.Errorf("failpoint marker %s did not trigger — ensure `failpoint-ctl enable` "+
			"was run before `go test -tags=failpoint`", markerPath)
	}

	t.Log("Failpoint marker detected; waiting for process to exit with error...")
	_, waitErr := WaitForProcessExitOrKill(runner, exitTimeout)
	if waitErr == nil {
		return fmt.Errorf("process exited without error after failpoint %s — expected a failure", markerPath)
	}
	return nil
}
