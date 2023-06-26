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
)

func main() {
	registerSignalHandlers()
	cmd.Execute()
}

func registerSignalHandlers() {
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-sigs
		utils.PrintAndLog("Received signal %s. Exiting...", sig)
		atexit.Exit(0)
	}()
}
