package testutils

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"

	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

type ExitCode int

const (
	ExitCodeSuccess ExitCode = 0
	ExitCodeFailure ExitCode = 1
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
				"--source-db-name", config.DBName,
				"--source-db-host", host,
				"--source-db-port", strconv.Itoa(port),
				"--source-ssl-mode", "disable",
			}
		} else {
			connectionArgs = []string{
				"--target-db-user", config.User,
				"--target-db-password", config.Password,
				"--target-db-name", config.DBName,
				"--target-db-host", host,
				"--target-db-port", strconv.Itoa(port),
				"--target-ssl-mode", "disable",
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
	return nil
}

func (v *VoyagerCommandRunner) newCmd() {
	v.StdoutBuf = &bytes.Buffer{}
	v.StderrBuf = &bytes.Buffer{}

	v.Cmd = exec.Command("yb-voyager", v.finalArgs...)
	v.Cmd.Stdout = io.MultiWriter(os.Stdout, v.StdoutBuf)
	v.Cmd.Stderr = io.MultiWriter(os.Stderr, v.StderrBuf)
	// disable callhome diagnostics during tests
	v.Cmd.Env = append(os.Environ(), "YB_VOYAGER_SEND_DIAGNOSTICS=false")
}

func (v *VoyagerCommandRunner) Run() error {
	if err := v.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare command: %w", err)
	}

	v.newCmd()

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
	return nil
}

func (v *VoyagerCommandRunner) Wait() error {
	err := v.Cmd.Wait()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			v.exitCode = ExitCode(ee.ExitCode())
		} else {
			v.exitCode = ExitCodeFailure
		}
		return fmt.Errorf("command failed: %w", err)
	} else {
		v.exitCode = ExitCodeSuccess
	}
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
