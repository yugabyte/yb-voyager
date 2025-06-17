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

func (this *VoyagerCommandRunner) Prepare() error {
	if len(this.finalArgs) != 0 {
		return nil // already prepared
	}

	// Appending to CmdArgs based on the command type.
	host, port, err := this.container.GetHostPort()
	if err != nil {
		return fmt.Errorf("failed to get host port for container: %v", err)
	}

	config := this.container.GetConfig()
	var connectionArgs []string
	if isSourceCmd(this.CmdName) {
		connectionArgs = []string{
			"--source-db-type", config.DBType,
			"--source-db-user", config.User,
			"--source-db-password", config.Password,
			"--source-db-name", config.DBName,
			"--source-db-host", host,
			"--source-db-port", strconv.Itoa(port),
			"--source-ssl-mode", "disable",
		}
	} else if isTargetCmd(this.CmdName) {
		connectionArgs = []string{
			"--target-db-user", config.User,
			"--target-db-password", config.Password,
			"--target-db-name", config.DBName,
			"--target-db-host", host,
			"--target-db-port", strconv.Itoa(port),
			"--target-ssl-mode", "disable",
		}
	}

	// split the command name on spaces so that Cobra framework
	// can detect parent and child commands correctly.
	// e.g. CmdName == "export data" → parts == ["export", "data"]
	parts := strings.Fields(this.CmdName)

	/*
		append order is important here as the CmdArgs passed in CommmandRunner might want to override the default of ContainerConfig
		For eg: append(this.CmdArgs, connectionArgs...) the default connection args with override the ones passed to CommandRunner
	*/
	this.finalArgs = append(parts, append(connectionArgs, this.CmdArgs...)...)
	return nil
}

func (this *VoyagerCommandRunner) newCmd() {
	this.StdoutBuf = &bytes.Buffer{}
	this.StderrBuf = &bytes.Buffer{}

	this.Cmd = exec.Command("yb-voyager", this.finalArgs...)
	this.Cmd.Stdout = io.MultiWriter(os.Stdout, this.StdoutBuf)
	this.Cmd.Stderr = io.MultiWriter(os.Stderr, this.StderrBuf)
	// disable callhome diagnostics during tests
	this.Cmd.Env = append(os.Environ(), "YB_VOYAGER_SEND_DIAGNOSTICS=false")
}

func (this *VoyagerCommandRunner) Run() error {
	if err := this.Prepare(); err != nil {
		return fmt.Errorf("failed to prepare command: %w", err)
	}

	this.newCmd()

	log.Debugf("running command: %s", this.Cmd.String())
	err := this.Cmd.Start()
	if err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	if this.doDuringCmd != nil {
		// Small delay to let the command initiate.
		time.Sleep(2 * time.Second)
		this.doDuringCmd()
	}

	if !this.isAsync {
		return this.Wait()
	}
	return nil
}

func (this *VoyagerCommandRunner) Wait() error {
	err := this.Cmd.Wait()
	if err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			this.exitCode = ExitCode(ee.ExitCode())
		} else {
			this.exitCode = ExitCodeFailure
		}
		return fmt.Errorf("command failed: %w", err)
	} else {
		this.exitCode = ExitCodeSuccess
	}
	return nil
}

func (this *VoyagerCommandRunner) Kill() error {
	if this.Cmd == nil {
		return fmt.Errorf("command for %s not built yet", this.CmdName)
	}

	if this.Cmd.Process == nil {
		return fmt.Errorf("process for command %s is not available", this.CmdName)
	}

	log.Debugf("killing command: %s", this.Cmd.String())
	err := this.Cmd.Process.Kill()
	if err != nil {
		return fmt.Errorf("failed to kill command: %w", err)
	}

	this.exitCode = ExitCodeFailure // setting failure code for unsuccessful execution
	return nil
}

func (this *VoyagerCommandRunner) ExitCode() ExitCode {
	return this.exitCode
}

func (this *VoyagerCommandRunner) Stdout() string {
	if this.StdoutBuf == nil {
		return ""
	}
	return this.StdoutBuf.String()
}

func (this *VoyagerCommandRunner) Stderr() string {
	if this.StderrBuf == nil {
		return ""
	}
	return this.StderrBuf.String()
}
