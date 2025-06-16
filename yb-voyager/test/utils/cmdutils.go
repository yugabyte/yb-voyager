package testutils

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

func CreateTempExportDir() string {
	// Create a temporary directory for export inside /tmp
	exportDir, err := os.MkdirTemp("", "yb-voyager-export")
	if err != nil {
		utils.ErrExit("failed to create temp export dir for testing: %v", err)
	}

	return exportDir
}

func CreateBackupDir(t *testing.T) string {
	backupDir, err := os.MkdirTemp("", "backup-export-dir-*")
	FatalIfError(t, err, "Failed to create backup directory")
	t.Cleanup(func() {
		if err := os.RemoveAll(backupDir); err != nil {
			t.Fatalf("Failed to remove backup directory: %v", err)
		}
	})
}

func RemoveTempExportDir(exportDir string) {
	// Remove the temporary directory
	err := os.RemoveAll(exportDir)
	if err != nil {
		utils.ErrExit("failed to remove temp export dir: %v", err)
	}
}

func RunVoyagerCommand(container testcontainers.TestContainer,
	cmdName string, cmdArgs []string, doDuringCmd func(), async bool) (*exec.Cmd, error) {

	fmt.Printf("Running voyager command: %s %s\n", cmdName, strings.Join(cmdArgs, " "))
	// Gather DB connection info.
	host, port, err := container.GetHostPort()
	if err != nil {
		return nil, fmt.Errorf("failed to get host port for container: %v", err)
	}

	config := container.GetConfig()
	var connectionArgs []string
	if isSourceCmd(cmdName) {
		connectionArgs = []string{
			"--source-db-type", config.DBType,
			"--source-db-user", config.User,
			"--source-db-password", config.Password,
			"--source-db-name", config.DBName,
			"--source-db-host", host,
			"--source-db-port", strconv.Itoa(port),
			"--source-ssl-mode", "disable",
		}
	} else if isTargetCmd(cmdName) {
		connectionArgs = []string{
			"--target-db-user", config.User,
			"--target-db-password", config.Password,
			"--target-db-name", config.DBName,
			"--target-db-host", host,
			"--target-db-port", strconv.Itoa(port),
			"--target-ssl-mode", "disable",
		}
	}

	// Append connection arguments to provided command arguments.
	cmdArgs = append(connectionArgs, cmdArgs...)
	cmdStr := fmt.Sprintf("yb-voyager %s %s", cmdName, strings.Join(cmdArgs, " "))
	cmd := exec.Command("/bin/bash", "-c", cmdStr)
	fmt.Printf("Running command: %s\n", cmdStr)

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	// Do not send callhome diagnostics during tests.
	cmd.Env = append(os.Environ(), "YB_VOYAGER_SEND_DIAGNOSTICS=false")

	// Start the Voyager command asynchronously.
	if err = cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start voyager command: %w", err)
	}

	// Execute the during-command callback if provided.
	if doDuringCmd != nil {
		// Small delay to let the command initiate.
		time.Sleep(2 * time.Second)
		doDuringCmd()
	}

	// If we want synchronous behavior, wait for the command to finish.
	if !async {
		if err := cmd.Wait(); err != nil {
			return nil, fmt.Errorf("voyager command exited with error: %w", err)
		}
	}

	// Return the command handle so that for async use cases,
	// the caller can later inspect or kill the process.
	return cmd, nil
}

// KillVoyagerCommand kills the voyager command process by sending a SIGKILL signal.
func KillVoyagerCommand(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return fmt.Errorf("command process is not available")
	}

	return cmd.Process.Kill()
}
