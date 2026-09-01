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
	"errors"
	"testing"
)

// TestHasExitedIsRepeatable pins the property a wait loop depends on: liveness may be
// asked on a timer.
//
// IsStopped used to answer by RECEIVING from the single-buffered stop channel, so the
// answer was destructive - the second caller saw a finished command as still running.
// Polling it every two seconds would therefore have reported a dead importer alive from
// the second poll onwards, which is precisely the bug the poll exists to catch.
func TestHasExitedIsRepeatable(t *testing.T) {
	v := &VoyagerCommandRunner{CmdName: "import data", stopChan: make(chan error, 1)}

	for i := 0; i < 3; i++ {
		if exited, _ := v.HasExited(); exited {
			t.Fatalf("poll %d: HasExited before Wait = true, want false", i)
		}
		if v.IsStopped() {
			t.Fatalf("poll %d: IsStopped before Wait = true, want false", i)
		}
	}

	want := errors.New("command failed: exit status 1")
	v.recordExit(want)

	for i := 0; i < 3; i++ {
		exited, err := v.HasExited()
		if !exited {
			t.Fatalf("poll %d: HasExited after the command finished = false, want true", i)
		}
		if !errors.Is(err, want) {
			t.Fatalf("poll %d: HasExited error = %v, want %v", i, err, want)
		}
		if !v.IsStopped() {
			t.Fatalf("poll %d: IsStopped after the command finished = false, want true", i)
		}
	}
}

// TestIsStoppedDoesNotConsumeTheStopChannel guards the other half of that bug. The async
// stop channel carries exactly one value, and GracefulStop and WaitForAsyncCompletion both
// receive from it. A liveness check that consumed it left them waiting on a channel
// nothing would ever send to again - and after a cutover two handles point at the SAME
// runner, so two liveness checks on one command is the normal case, not a corner one.
func TestIsStoppedDoesNotConsumeTheStopChannel(t *testing.T) {
	v := &VoyagerCommandRunner{CmdName: "export data", stopChan: make(chan error, 1)}
	v.stopChan <- nil // what the async Run goroutine sends when Wait returns
	v.recordExit(nil)

	for i := 0; i < 5; i++ {
		if !v.IsStopped() {
			t.Fatalf("poll %d: IsStopped = false on a finished command", i)
		}
	}

	select {
	case <-v.stopChan:
	default:
		t.Fatal("IsStopped consumed the stop channel; GracefulStop and WaitForAsyncCompletion " +
			"would block on it forever")
	}
}

// TestRecordExitKeepsTheFirstOutcome: a command exits once. Whatever recorded that exit
// first is the truth, and a later call must not overwrite it with something tidier.
func TestRecordExitKeepsTheFirstOutcome(t *testing.T) {
	v := &VoyagerCommandRunner{CmdName: "import data"}
	first := errors.New("command failed: exit status 1")
	v.recordExit(first)
	v.recordExit(nil)

	exited, err := v.HasExited()
	if !exited || !errors.Is(err, first) {
		t.Fatalf("HasExited = (%v, %v), want (true, %v)", exited, err, first)
	}
}
