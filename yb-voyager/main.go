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
package main

import (
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/tebeka/atexit"
	"golang.org/x/term"

	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var originalTermState *term.State

func main() {
	captureTerminalState()

	registerSignalHandlers()
	atexit.Register(cmd.PackAndSendCallhomePayloadOnExit)
	atexit.Register(cmd.CleanupChildProcesses)
	atexit.Register(restoreTerminalState) // ensure terminal is always restored
	cmd.Execute()
	cmd.PrintElapsedDuration()
	if cmd.ProcessShutdownRequested {
		utils.PrintAndLogf("waiting for exit handlers to complete the cleanup")
		time.Sleep(time.Second * 120) // using here larger value than what we have for debezium(100sec)
	}
}

func registerSignalHandlers() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2, syscall.SIGUSR1)
	go func() {
		sig := <-sigs
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			utils.PrintAndLogf("\nReceived signal %s. Exiting...", sig)
			cmd.ProcessShutdownRequested = true
		case syscall.SIGUSR2:
			utils.PrintAndLogf("\nReceived signal to terminate due to end migration command. Exiting...")
			cmd.ProcessShutdownRequested = true
		case syscall.SIGUSR1:
			cmd.StopArchiverSignal = true
			return
		}
		// Ensure we restore the terminal even if everything goes well
		restoreTerminalState()
		atexit.Exit(1)
	}()
}

func captureTerminalState() {
	if !term.IsTerminal(int(syscall.Stdin)) {
		return
	}

	// Capture the original terminal state
	state, err := term.GetState(int(syscall.Stdin))
	if err != nil {
		utils.ErrExit("error capturing terminal state: %v\n", err)
	}
	originalTermState = state
}

func restoreTerminalState() {
	if !term.IsTerminal(int(syscall.Stdin)) {
		return
	}

	// Restore the terminal to its original state
	if originalTermState != nil {
		if err := term.Restore(0, originalTermState); err != nil {
			log.Errorf("error restoring terminal: %v\n", err)
		}
	}
}
