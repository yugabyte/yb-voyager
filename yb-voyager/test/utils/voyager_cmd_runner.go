package testutils

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	log "github.com/sirupsen/logrus"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

// testLogWriter is a line-buffered io.Writer that routes each complete line
// through t.Log so that go test -json can attribute output to the correct test.
type testLogWriter struct {
	t      *testing.T
	prefix string
	mu     sync.Mutex
	buf    []byte
}

func (w *testLogWriter) Write(p []byte) (n int, err error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.buf = append(w.buf, p...)
	for {
		idx := bytes.IndexByte(w.buf, '\n')
		if idx < 0 {
			break
		}
		line := string(w.buf[:idx])
		w.buf = w.buf[idx+1:]
		w.t.Logf("[%s] %s", w.prefix, line)
	}
	return len(p), nil
}

func (w *testLogWriter) Flush() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.buf) > 0 {
		w.t.Logf("[%s] %s", w.prefix, string(w.buf))
		w.buf = nil
	}
}

type ExitCode int

const (
	ExitCodeSuccess ExitCode = 0
	ExitCodeFailure ExitCode = 1

	separator = "=================================================================================="
)

func (ec ExitCode) String() string {
	switch ec {
	case ExitCodeSuccess:
		return "Success"
	case ExitCodeFailure:
		return "Failure"
	default:
		return fmt.Sprintf("Exitcode(%d)", int(ec))
	}
}

// wrapper for exec.Cmd to capture stdout and stderr and to extend it as per our test framework needs
type VoyagerCommandRunner struct {
	// basic command information to be provided
	container   testcontainers.TestContainer
	CmdName     string
	CmdArgs     []string
	finalArgs   []string // store final command args after preparation
	isAsync     bool
	doDuringCmd func()

	// as per test framework needs
	Cmd       *exec.Cmd
	StdoutBuf *bytes.Buffer
	StderrBuf *bytes.Buffer
	exitCode  ExitCode

	// additional environment variables for testing
	testEnvVars []string

	// testing.T for routing output through t.Log
	t         *testing.T
	logWriter *testLogWriter

	stopChan chan error
}

// WithEnv adds custom environment variables to the command.
// This is useful for testing scenarios like failpoint injection.
// Returns the VoyagerCommandRunner for method chaining.
//
// Example:
//
//	runner := NewVoyagerCommandRunner(...).WithEnv("GO_FAILPOINTS=pkg/fp1=return()")
func (v *VoyagerCommandRunner) WithEnv(envVars ...string) *VoyagerCommandRunner {
	v.testEnvVars = append(v.testEnvVars, envVars...)
	return v
}

// WithT attaches a testing.T so that subprocess output and command
// headers/footers are routed through t.Log instead of os.Stdout/os.Stderr.
// This allows go test -json to attribute output to the correct test.
func (v *VoyagerCommandRunner) WithT(t *testing.T) *VoyagerCommandRunner {
	v.t = t
	return v
}

func NewVoyagerCommandRunner(container testcontainers.TestContainer, cmdName string, cmdArgs []string, doDuringCmd func(), isAsync bool) *VoyagerCommandRunner {
	if cmdName == "" {
		log.Fatal("Command name cannot be empty")
	}

	if container == nil && (isSourceCmd(cmdName) || isTargetCmd(cmdName)) {
		log.Fatal("Container cannot be nil for source/target commands")
	}

	cmdRunner := VoyagerCommandRunner{
		container:   container,
		CmdName:     cmdName,
		CmdArgs:     cmdArgs,
		doDuringCmd: doDuringCmd,
		isAsync:     isAsync,
	}
	log.Debugf("Creating CommandRunner for command: %s with args: %s", cmdName, strings.Join(cmdArgs, " "))
	return &cmdRunner
}

// commandsSupportingSendDiagnostics lists commands that accept the --send-diagnostics flag
// (registered via registerCommonGlobalFlags). This allowlist ensures we only pass the flag to
// commands that support it -- passing it to commands like cutover, status, or get data-migration-report
// would cause Cobra to fail with "unknown flag".
var commandsSupportingSendDiagnostics = map[string]bool{
	"export schema":                    true,
	"export data":                      true,
	"export data from source":          true,
	"export data from target":          true,
	"import schema":                    true,
	"import data":                      true,
	"import data to target":            true,
	"import data to source":            true,
	"import data to source-replica":    true,
	"import data file":                 true,
	"analyze-schema":                   true,
	"assess-migration":                 true,
	"finalize-schema-post-data-import": true,
	"compare-performance":              true,
	"end migration":                    true,
	"archive changes":                  true,
	"assess-migration-bulk":            true,
}

// supportsSendDiagnosticsFlag checks if the given command supports the --send-diagnostics flag.
func supportsSendDiagnosticsFlag(cmdName string) bool {
	return commandsSupportingSendDiagnostics[cmdName]
}

func (v *VoyagerCommandRunner) Prepare() error {
	if len(v.finalArgs) != 0 {
		return nil // already prepared
	}

	var connectionArgs []string
	if isSourceCmd(v.CmdName) || isTargetCmd(v.CmdName) {
		if v.container == nil {
			return fmt.Errorf("container cannot be nil for source/target commands")
		}

		// Appending to CmdArgs based on the command type.
		host, port, err := v.container.GetHostPort()
		if err != nil {
			return fmt.Errorf("failed to get host port for container: %v", err)
		}

		config := v.container.GetConfig()
		if isSourceCmd(v.CmdName) {
			connectionArgs = []string{
				"--source-db-type", config.DBType,
				"--source-db-user", config.User,
				"--source-db-password", config.Password,
				"--source-db-host", host,
				"--source-db-port", strconv.Itoa(port),
				"--source-ssl-mode", "disable",
			}
			if !slices.Contains(v.CmdArgs, "--source-db-name") {
				connectionArgs = append(connectionArgs, "--source-db-name", config.DBName)
			}
		} else {
			connectionArgs = []string{
				"--target-db-user", config.User,
				"--target-db-password", config.Password,
				"--target-db-host", host,
				"--target-db-port", strconv.Itoa(port),
				"--target-ssl-mode", "disable",
			}
			if !slices.Contains(v.CmdArgs, "--target-db-name") {
				connectionArgs = append(connectionArgs, "--target-db-name", config.DBName)
			}
		}
	}

	// split the command name on spaces so that Cobra framework
	// can detect parent and child commands correctly.
	// e.g. CmdName == "export data" → parts == ["export", "data"]
	parts := strings.Fields(v.CmdName)

	/*
		append order is important here as the CmdArgs passed in CommmandRunner might want to override the default of ContainerConfig
		For eg: append(v.CmdArgs, connectionArgs...) the default connection args with override the ones passed to CommandRunner
	*/
	v.finalArgs = append(parts, append(connectionArgs, v.CmdArgs...)...)

	// Add --send-diagnostics=false explicitly for commands that support it.
	// This is an extra safety measure in addition to setting the YB_VOYAGER_SEND_DIAGNOSTICS
	// environment variable, ensuring diagnostics are disabled even if the env var is not
	// properly inherited by subprocesses.
	if supportsSendDiagnosticsFlag(v.CmdName) && !slices.Contains(v.CmdArgs, "--send-diagnostics") {
		v.finalArgs = append(v.finalArgs, "--send-diagnostics", "false")
	}

	return nil
}

func (v *VoyagerCommandRunner) newCmd() {
	v.StdoutBuf = &bytes.Buffer{}
	v.StderrBuf = &bytes.Buffer{}

	v.Cmd = exec.Command("yb-voyager", v.finalArgs...)

	if v.t != nil {
		v.logWriter = &testLogWriter{t: v.t, prefix: v.CmdName}
		v.Cmd.Stdout = io.MultiWriter(v.logWriter, v.StdoutBuf)
		v.Cmd.Stderr = io.MultiWriter(v.logWriter, v.StderrBuf)
	} else {
		v.Cmd.Stdout = io.MultiWriter(os.Stdout, v.StdoutBuf)
		v.Cmd.Stderr = io.MultiWriter(os.Stderr, v.StderrBuf)
	}

	v.Cmd.Env = append(os.Environ(), "YB_VOYAGER_SEND_DIAGNOSTICS=false")

	if len(v.testEnvVars) > 0 {
		v.Cmd.Env = append(v.Cmd.Env, v.testEnvVars...)
	}
}

func (v *VoyagerCommandRunner) Run() error {
	if err := v.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare command: %w", err)
	}

	v.newCmd()
	v.printCommandHeader()

	log.Debugf("running command: %s", v.Cmd.String())
	err := v.Cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	if v.doDuringCmd != nil {
		// Small delay to let the command initiate.
		time.Sleep(2 * time.Second)
		v.doDuringCmd()
	}

	if !v.isAsync {
		return v.Wait()
	}
	//In case the command is asynchronous, we need to wait for the command to finish but asynchronously
	//and prevents the zombie process from being left behind.
	//As we issue signal to stop the command, it will exit but wihout wait the process metadata is not updated so if we are checking if the
	//command stopped properly or not, we won't be able to know (e.g. in end-migration command).
	v.stopChan = make(chan error, 1)
	go func() {
		v.stopChan <- v.Wait()
	}()
	return nil
}

func (v *VoyagerCommandRunner) IsStopped() bool {
	select {
	case <-v.stopChan:
		return true
	default:
		return false
	}
}
func (v *VoyagerCommandRunner) Wait() error {
	err := v.Cmd.Wait()
	if v.logWriter != nil {
		v.logWriter.Flush()
	}
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			v.exitCode = ExitCode(ee.ExitCode())
		} else {
			v.exitCode = ExitCodeFailure
		}
		v.printCommandFooter(err)
		return fmt.Errorf("command failed: %w", err)
	}

	v.exitCode = ExitCodeSuccess
	v.printCommandFooter(nil)
	return nil
}

func (v *VoyagerCommandRunner) Kill() error {
	if v.Cmd == nil {
		return fmt.Errorf("command for %s not built yet", v.CmdName)
	}

	if v.Cmd.Process == nil {
		return fmt.Errorf("process for command %s is not available", v.CmdName)
	}

	log.Debugf("killing command: %s", v.Cmd.String())
	err := v.Cmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill command: %w", err)
	}

	v.exitCode = ExitCodeFailure // setting failure code for unsuccessful execution
	return nil
}

func (v *VoyagerCommandRunner) GracefulStop(timeoutSeconds int) error {
	if v.Cmd == nil {
		return fmt.Errorf("command for %s not built yet", v.CmdName)
	}
	if v.Cmd.Process == nil {
		return fmt.Errorf("process for command %s is not available", v.CmdName)
	}

	log.Debugf("sending SIGTERM to command: %s (pid=%d)", v.Cmd.String(), v.Cmd.Process.Pid)
	err := v.Cmd.Process.Signal(syscall.SIGTERM)
	if err != nil {
		return fmt.Errorf("failed to send SIGTERM to command: %w", err)
	}

	select {
	case err := <-v.stopChan:
		if err != nil {
			log.Debugf("command %s exited with error (expected after SIGTERM): %v", v.CmdName, err)
		}
	case <-time.After(time.Duration(timeoutSeconds) * time.Second):
		log.Debugf("command %s did not exit within %ds after SIGTERM, sending SIGKILL", v.CmdName, timeoutSeconds)
		if killErr := v.Cmd.Process.Kill(); killErr != nil && !errors.Is(killErr, os.ErrProcessDone) {
			return fmt.Errorf("failed to SIGKILL command after timeout: %w", killErr)
		}
		<-v.stopChan
	}

	v.exitCode = ExitCodeFailure
	return nil
}

func (v *VoyagerCommandRunner) ExitCode() ExitCode {
	return v.exitCode
}

func (v *VoyagerCommandRunner) Stdout() string {
	if v.StdoutBuf == nil {
		return ""
	}
	return v.StdoutBuf.String()
}

func (v *VoyagerCommandRunner) Stderr() string {
	if v.StderrBuf == nil {
		return ""
	}
	return v.StderrBuf.String()
}

func (v *VoyagerCommandRunner) SetAsync(async bool) {
	v.isAsync = async
}

func (v *VoyagerCommandRunner) printCommandHeader() {
	if v.t != nil {
		v.t.Logf("\n%s\n>>> Running: %s\n%s", separator, v.GetCmd(), separator)
	} else {
		fmt.Println()
		fmt.Println(separator)
		fmt.Printf(">>> Running: %s\n", v.GetCmd())
		fmt.Println(separator)
	}
}

func (v *VoyagerCommandRunner) printCommandFooter(err error) {
	if v.t != nil {
		if err != nil {
			v.t.Logf("%s\n>>> Command FAILED: %s (Exit Code: %s)\n%s", separator, v.CmdName, v.exitCode.String(), separator)
		} else {
			v.t.Logf("%s\n>>> Command COMPLETED: %s\n%s", separator, v.CmdName, separator)
		}
	} else {
		fmt.Println(separator)
		if err != nil {
			fmt.Printf(">>> Command FAILED: %s (Exit Code: %s)\n", v.CmdName, v.exitCode.String())
		} else {
			fmt.Printf(">>> Command COMPLETED: %s\n", v.CmdName)
		}
		fmt.Println(separator)
		fmt.Println()
	}
}

func (v *VoyagerCommandRunner) AddArgs(args ...string) {
	v.finalArgs = append(v.finalArgs, args...)
}

func (v *VoyagerCommandRunner) GetFinalArgs() []string {
	return v.finalArgs
}

func (v *VoyagerCommandRunner) GetCmd() string {
	return v.Cmd.String()
}
