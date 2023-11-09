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

	"github.com/tebeka/atexit"
	"github.com/yugabyte/yb-voyager/yb-voyager/cmd"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/term"
)

var originalTermState *term.State

func main() {
	// Capture the original terminal state
	state, err := term.GetState(int(syscall.Stdin))
	if err != nil {
		utils.ErrExit("error capturing terminal state: %v\n", err)
	}
	originalTermState = state

	registerSignalHandlers()
	cmd.Execute()
}

func registerSignalHandlers() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM, syscall.SIGUSR2)
	go func() {
		sig := <-sigs
		switch sig {
		case syscall.SIGINT, syscall.SIGTERM:
			utils.PrintAndLog("\nReceived signal %s. Exiting...", sig)
		case syscall.SIGUSR2:
			utils.PrintAndLog("\nReceived signal to terminate due to end migration command. Exiting...")
		}
		// Ensure we restore the terminal even if everything goes well
		restoreTerminal()
		atexit.Exit(0)
	}()
}

func restoreTerminal() {
	// Restore the terminal to its original state
	if originalTermState != nil {
		if err := term.Restore(0, originalTermState); err != nil {
			utils.ErrExit("error restoring terminal: %v\n", err)
		}
	}
}
