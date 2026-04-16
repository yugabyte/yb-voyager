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

package cmd

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
)

var (
	bannerColor    = color.New(color.FgCyan, color.Bold)
	stageColor     = color.New(color.FgCyan)
	checkColor     = color.New(color.FgGreen)
	skipColor      = color.New(color.FgYellow)
	failColor      = color.New(color.FgRed)
	separatorColor = color.New(color.FgHiBlack)
	dimColor       = color.New(color.FgHiBlack)
	boldColor      = color.New(color.Bold)
	pathColor      = color.New(color.Underline)
)

const boxWidth = 55

func printBanner(targetVersion string) {
	top := "+" + strings.Repeat("-", boxWidth-2) + "+"
	bot := "+" + strings.Repeat("-", boxWidth-2) + "+"

	title := "YugabyteDB Voyager -- Migration Assessment"
	subtitle := fmt.Sprintf("Target: YugabyteDB %s", targetVersion)

	bannerColor.Println(top)
	bannerColor.Printf("| %-*s|\n", boxWidth-3, title)
	bannerColor.Printf("| %-*s|\n", boxWidth-3, subtitle)
	bannerColor.Println(bot)
	fmt.Println()
}

func printSeparator() {
	separatorColor.Println(strings.Repeat("-", boxWidth))
}

func printPreflightHeader() {
	boldColor.Println("Preflight Checks")
}

func printPreflightCheck(label string) {
	checkColor.Printf("  %s %s\n", "\u2713", label)
}

func printPreflightSkip(label string) {
	skipColor.Printf("  %s %s\n", "-", label)
}

func printPreflightFail(label string) {
	failColor.Printf("  %s %s\n", "\u2717", label)
}

// AssessmentProgressTracker renders stages with a spinner.
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
type AssessmentProgressTracker struct {
	mu            sync.Mutex
	stageName     string
	subStage      string
	stepsTotal    int
	stepsDone     int
	linesPrinted  int // finalized sub-step lines currently visible on screen
	spinnerStop   chan struct{}
	spinnerDone   chan struct{}
	headerPrinted bool
}

func NewAssessmentProgressTracker() *AssessmentProgressTracker {
	return &AssessmentProgressTracker{}
}

// spinnerLabel returns the text rendered next to the spinner frame.
// When a sub-step is active it shows the sub-step with a step counter;
// otherwise just the stage name.
func (t *AssessmentProgressTracker) spinnerLabel() string {
	if t.subStage != "" {
		return fmt.Sprintf("%s %s", t.stepCounter(), t.subStage)
	}
	return t.stageName
}

// stepCounter returns a formatted counter for the current step like "(3/12)" or "(3)".
func (t *AssessmentProgressTracker) stepCounter() string {
	return t.stepCounterFor(t.stepsDone)
}

// stepCounterFor returns a formatted counter for a given step number.
func (t *AssessmentProgressTracker) stepCounterFor(step int) string {
	if t.stepsTotal > 0 {
		return fmt.Sprintf("(%d/%d)", step, t.stepsTotal)
	}
	return fmt.Sprintf("(%d)", step)
}

// completedLabel builds the text shown on the final checkmark line.
func (t *AssessmentProgressTracker) completedLabel() string {
	if t.stepsTotal > 0 {
		return fmt.Sprintf("%s (%d/%d)", t.stageName, t.stepsDone, t.stepsTotal)
	}
	if t.stepsDone > 0 {
		return fmt.Sprintf("%s (%d steps)", t.stageName, t.stepsDone)
	}
	return t.stageName
}

// StartStage begins a new stage with a spinner. totalSteps can be 0 if unknown.
func (t *AssessmentProgressTracker) StartStage(name string, totalSteps int) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopSpinnerLocked()

	if !t.headerPrinted {
		fmt.Println()
		boldColor.Println("Running Assessment")
		t.headerPrinted = true
	}

	t.stageName = name
	t.subStage = ""
	t.stepsTotal = totalSteps
	t.stepsDone = 0
	t.linesPrinted = 0
	t.spinnerStop = make(chan struct{})
	t.spinnerDone = make(chan struct{})
	go t.runSpinner()
}

// IncrementStep finalizes the previous sub-step (prints it with a checkmark
// and its step number), then starts a new spinner line for the new sub-step.
func (t *AssessmentProgressTracker) IncrementStep(subStage string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	prevStep := t.stepsDone
	t.stepsDone++
	t.stopSpinnerLocked()

	if t.subStage != "" {
		counter := t.stepCounterFor(prevStep)
		checkColor.Printf("    %s %s %s\n", "\u2713", counter, t.subStage)
		t.linesPrinted++
	}

	t.subStage = subStage
	t.spinnerStop = make(chan struct{})
	t.spinnerDone = make(chan struct{})
	go t.runSpinner()
}

// CompleteStage finalizes the last sub-step, erases all sub-step lines, and
// prints a single checkmark line for the stage.
func (t *AssessmentProgressTracker) CompleteStage() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopSpinnerLocked()

	if t.subStage != "" {
		counter := t.stepCounter()
		checkColor.Printf("    %s %s %s\n", "\u2713", counter, t.subStage)
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
	checkColor.Printf("  %s %s\n", "\u2713", t.completedLabel())
	t.linesPrinted = 0
}

// Finish stops any running spinner without printing a checkmark.
func (t *AssessmentProgressTracker) Finish() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopSpinnerLocked()
}

// stopSpinnerLocked signals the spinner goroutine to stop and waits for it.
// Because the spinner acquires mu on each frame, we must temporarily release mu
// here to avoid a deadlock (caller holds mu → spinner blocks on mu → caller
// blocks on spinnerDone).
func (t *AssessmentProgressTracker) stopSpinnerLocked() {
	if t.spinnerStop != nil {
		close(t.spinnerStop)
		t.mu.Unlock()
		<-t.spinnerDone
		t.mu.Lock()
		t.spinnerStop = nil
		t.spinnerDone = nil
	}
}

var spinnerFrames = []string{"\u280b", "\u2819", "\u2839", "\u2838", "\u283c", "\u2834", "\u2826", "\u2827", "\u2807", "\u280f"}

func (t *AssessmentProgressTracker) runSpinner() {
	defer close(t.spinnerDone)
	i := 0
	for {
		select {
		case <-t.spinnerStop:
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

		// Re-check stop after releasing the mutex so we exit promptly
		// when stopSpinnerLocked is called.
		select {
		case <-t.spinnerStop:
			fmt.Printf("\r\033[K")
			return
		default:
		}

		frame := spinnerFrames[i%len(spinnerFrames)]
		fmt.Printf("\r\033[K")
		stageColor.Printf("%s%s %s", indent, frame, label)
		i++
		time.Sleep(80 * time.Millisecond)
	}
}

// printAssessmentSummary prints the final summary block with issue count and report paths.
func printAssessmentSummary(issueCount int, jsonPath, htmlPath string) {
	fmt.Println()
	printSeparator()
	fmt.Println()
	checkColor.Printf("%s Assessment completed successfully\n", "\u2713")
	fmt.Println()
	dimColor.Printf("  Issues detected: ")
	if issueCount > 0 {
		color.New(color.FgYellow).Printf("%d\n", issueCount)
	} else {
		checkColor.Printf("%d\n", issueCount)
	}
	dimColor.Print("  JSON report: ")
	pathColor.Println(jsonPath)
	dimColor.Print("  HTML report: ")
	pathColor.Println(htmlPath)
	fmt.Println()
}

func printAssessmentFailure(errMsg string) {
	fmt.Println()
	printSeparator()
	fmt.Println()
	failColor.Printf("%s Assessment failed: %s\n", "\u2717", errMsg)
	fmt.Println()
}
