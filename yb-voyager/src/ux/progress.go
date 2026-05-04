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

package ux

import (
	"fmt"
	"sync"
	"time"
)

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// ProgressTracker renders stages with a spinner.
//
// For stages WITH sub-steps (e.g. "Gathering metadata"), each sub-step is
// printed on its own line as it completes:
//
//	✓ Collecting column statistics...
//	✓ Collecting db queries summary...
//	⠋ Collecting index to table mapping...
//
// When the stage finishes, all sub-step lines are erased and replaced by a
// single completion line:
//
//	✓ Gathering metadata (12/12)
//
// For stages WITHOUT sub-steps, a single spinner line is shown and then
// replaced by the checkmark on completion.
type ProgressTracker struct {
	mu            sync.Mutex
	stageName     string
	subStage      string
	stepsTotal    int
	stepsDone     int
	linesPrinted  int
	spinnerStop   chan struct{}
	spinnerDone   chan struct{}
	headerPrinted bool
	headerLabel   string
}

// NewProgressTracker creates a tracker with a custom section header label.
func NewProgressTracker(headerLabel string) *ProgressTracker {
	return &ProgressTracker{headerLabel: headerLabel}
}

func (t *ProgressTracker) spinnerLabel() string {
	if t.subStage != "" {
		return fmt.Sprintf("%s %s", t.stepCounter(), t.subStage)
	}
	if t.stepsTotal > 0 {
		return fmt.Sprintf("%s (%d steps)", t.stageName, t.stepsTotal)
	}
	return t.stageName
}

func (t *ProgressTracker) stepCounter() string {
	return t.stepCounterFor(t.stepsDone)
}

func (t *ProgressTracker) stepCounterFor(step int) string {
	if t.stepsTotal > 0 {
		return fmt.Sprintf("(%d/%d)", step, t.stepsTotal)
	}
	return fmt.Sprintf("(%d)", step)
}

func (t *ProgressTracker) completedLabel() string {
	if t.stepsTotal > 0 {
		return fmt.Sprintf("%s (%d/%d)", t.stageName, t.stepsDone, t.stepsTotal)
	}
	if t.stepsDone > 0 {
		return fmt.Sprintf("%s (%d steps)", t.stageName, t.stepsDone)
	}
	return t.stageName
}

// StartStage begins a new stage with a spinner. totalSteps can be 0 if unknown.
func (t *ProgressTracker) StartStage(name string, totalSteps int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopSpinnerLocked()
	t.initStageLocked(name, totalSteps)
	t.spinnerStop = make(chan struct{})
	t.spinnerDone = make(chan struct{})
	go t.runSpinner()
}

// PrepareStage sets up stage state, prints the section header and a static
// stage heading (e.g. "  Gathering metadata (12 steps)") but does NOT start a
// spinner. Use this when external code (e.g. the parallel replica tracker)
// will handle its own progress display. Call CompleteStage when done.
func (t *ProgressTracker) PrepareStage(name string, totalSteps int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopSpinnerLocked()
	t.initStageLocked(name, totalSteps)

	StageColor.Printf("  %s\n", t.spinnerLabel())
}

func (t *ProgressTracker) initStageLocked(name string, totalSteps int) {
	if !t.headerPrinted {
		fmt.Println()
		BoldColor.Println(t.headerLabel)
		t.headerPrinted = true
	}
	t.stageName = name
	t.subStage = ""
	t.stepsTotal = totalSteps
	t.stepsDone = 0
	t.linesPrinted = 0
}

// IncrementStep finalizes the previous sub-step (prints it with a checkmark
// and its step number), then starts a new spinner line for the new sub-step.
func (t *ProgressTracker) IncrementStep(subStage string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	prevStep := t.stepsDone
	t.stepsDone++
	t.stopSpinnerLocked()

	if t.subStage != "" {
		counter := t.stepCounterFor(prevStep)
		CheckColor.Printf("    ✓ %s %s\n", counter, t.subStage)
		t.linesPrinted++
	}

	t.subStage = subStage
	t.spinnerStop = make(chan struct{})
	t.spinnerDone = make(chan struct{})
	go t.runSpinner()
}

// CompleteStage finalizes the last sub-step, erases all sub-step lines, and
// prints a single checkmark line for the stage.
func (t *ProgressTracker) CompleteStage() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopSpinnerLocked()

	if t.subStage != "" {
		counter := t.stepCounter()
		CheckColor.Printf("    ✓ %s %s\n", counter, t.subStage)
		t.linesPrinted++
	}

	if t.linesPrinted > 0 {
		fmt.Printf("\033[%dA\033[J", t.linesPrinted)
	}

	if t.stepsTotal > 0 {
		t.stepsDone = t.stepsTotal
	}
	t.subStage = ""

	fmt.Printf("\r\033[K")
	CheckColor.Printf("  ✓ %s\n", t.completedLabel())
	t.linesPrinted = 0
}

// FailStage stops the spinner, erases any sub-step lines, and prints a red
// "✗ <stage>" line so the user sees a clear failure marker in the progress
// output before the error message is returned.
func (t *ProgressTracker) FailStage() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopSpinnerLocked()

	if t.linesPrinted > 0 {
		fmt.Printf("\033[%dA\033[J", t.linesPrinted)
	}

	fmt.Printf("\r\033[K")
	FailColor.Printf("  ✗ %s\n", t.stageName)
	t.linesPrinted = 0
	t.subStage = ""
}

// Finish stops any running spinner without printing a checkmark.
func (t *ProgressTracker) Finish() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopSpinnerLocked()
}

// stopSpinnerLocked signals the spinner goroutine to stop and waits for it.
// The mutex is temporarily released to avoid deadlock since the spinner
// goroutine also acquires it on each frame.
func (t *ProgressTracker) stopSpinnerLocked() {
	if t.spinnerStop != nil {
		close(t.spinnerStop)
		t.mu.Unlock()
		<-t.spinnerDone
		t.mu.Lock()
		t.spinnerStop = nil
		t.spinnerDone = nil
	}
}

func (t *ProgressTracker) runSpinner() {
	defer close(t.spinnerDone)
	i := 0
	for {
		select {
		case <-t.spinnerStop:
			// This line clears the current line in the terminal:
			fmt.Printf("\r\033[K")
			return
		default:
		}

		t.mu.Lock()
		label := t.spinnerLabel()
		indent := "  "
		if t.subStage != "" {
			indent = "    "
		}
		t.mu.Unlock()

		select {
		case <-t.spinnerStop:
			fmt.Printf("\r\033[K")
			return
		default:
		}

		frame := spinnerFrames[i%len(spinnerFrames)]
		prefixLen := len(indent) + len(frame) + 1
		truncated := TruncateToWidth(label, prefixLen)
		fmt.Printf("\r\033[K")
		StageColor.Printf("%s%s %s", indent, frame, truncated)
		i++
		time.Sleep(80 * time.Millisecond)
	}
}
