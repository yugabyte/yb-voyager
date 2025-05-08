package testutils

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
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

func RemoveTempExportDir(exportDir string) {
	// Remove the temporary directory
	err := os.RemoveAll(exportDir)
	if err != nil {
		utils.ErrExit("failed to remove temp export dir: %v", err)
	}
}

func RunVoyagerCommmand(container testcontainers.TestContainer,
	cmdName string, cmdArgs []string,
	doDuringCmd func(),
) error {
	fmt.Printf("Running voyager command: %s %s\n", cmdName, strings.Join(cmdArgs, " "))

	// Gather DB connection info
	host, port, err := container.GetHostPort()
	if err != nil {
		return fmt.Errorf("failed to get host port for container: %v", err)
	}

	config := container.GetConfig()

	var connectionArgs []string
	if isSourceCmd(cmdName) {
		connectionArgs = []string{
			"--source-db-type", config.DBType,
			"--source-db-user", config.User,
			"--source-db-password", config.Password,
			"--source-db-schema", config.Schema,
			"--source-db-name", config.DBName,
			"--source-db-host", host,
			"--source-db-port", strconv.Itoa(port),
			"--source-ssl-mode", "disable",
		}
	} else if isTargetCmd(cmdName) {
		connectionArgs = []string{
			"--target-db-user", config.User,
			"--target-db-password", config.Password,
			"--target-db-schema", config.Schema,
			"--target-db-name", config.DBName,
			"--target-db-host", host,
			"--target-db-port", strconv.Itoa(port),
			"--target-ssl-mode", "disable",
		}
	}

	// Build the command to run.
	cmdArgs = append(connectionArgs, cmdArgs...)
	cmdStr := fmt.Sprintf("yb-voyager %s %s", cmdName, strings.Join(cmdArgs, " "))
	cmd := exec.Command("/bin/bash", "-c", cmdStr)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	// don't send to callhome for tests
	cmd.Env = append(os.Environ(), "YB_VOYAGER_SEND_DIAGNOSTICS=false")

	// Start the voyager command asynchronously.
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start voyager command: %w", err)
	}

	// Execute the during command function (if provided).
	if doDuringCmd != nil {
		// delay for 2 seconds to ensure the command has started.
		time.Sleep(2 * time.Second)
		doDuringCmd()
	}

	// Wait for the voyager command to finish.
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("voyager command exited with error: %w", err)
	}

	return nil
}
