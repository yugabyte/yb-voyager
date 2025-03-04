package testutils

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

func CreateTempExportDir() string {
	// Create a temporary directory for export inside /tmp
	exportDir, err := os.MkdirTemp("", "yb-voyager-export")
	if err != nil {
		utils.ErrExit("failed to create temp export dir for testing", err)
	}

	return exportDir
}

func RemoveTempExportDir(exportDir string) {
	// Remove the temporary directory
	err := os.RemoveAll(exportDir)
	if err != nil {
		utils.ErrExit("failed to remove temp export dir", err)
	}
}

func RunVoyagerCommmand(container testcontainers.TestContainer,
	cmdName string, cmdArgs []string,
	doDuringCmd func(),
) error {
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

	// 1) Build the command to run.
	cmdArgs = append(connectionArgs, cmdArgs...)
	cmdStr := fmt.Sprintf("yb-voyager %s %s", cmdName, strings.Join(cmdArgs, " "))
	cmd := exec.Command("/bin/bash", "-c", cmdStr)

	// 2) Get the stdout and stderr pipes.
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	// 3) Start the voyager command asynchronously.
	if err = cmd.Start(); err != nil {
		return fmt.Errorf("failed to start voyager command: %w", err)
	}

	// 4) Launch goroutines to stream output live to console.
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		// Copy stdout to os.Stdout.
		if _, err := io.Copy(os.Stdout, stdoutPipe); err != nil {
			log.Printf("Error copying stdout: %v", err)
		}
	}()

	go func() {
		defer wg.Done()
		// Copy stderr to os.Stderr.
		if _, err := io.Copy(os.Stderr, stderrPipe); err != nil {
			log.Printf("Error copying stderr: %v", err)
		}
	}()

	// 5) Execute the during-command function (if provided).
	if doDuringCmd != nil {
		// delay for 2 seconds to ensure the command has started.
		time.Sleep(2 * time.Second)
		doDuringCmd()
	}

	// 6) Wait for the voyager command to finish.
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("voyager command exited with error: %w", err)
	}

	// 7) Wait for the output goroutines to complete.
	wg.Wait()

	return nil
}
