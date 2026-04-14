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

// AssessmentProgressTracker renders stages with a spinner and a step counter.
// Each stage has a name and tracks completed steps out of a total.
// The spinner shows: ⠋ Stage name (3/12)
// On completion:     ✓ Stage name (12/12)
type AssessmentProgressTracker struct {
	mu            sync.Mutex
	stageName     string
	subStage      string
	stepsTotal    int
	stepsDone     int
	spinnerStop   chan struct{}
	spinnerDone   chan struct{}
	headerPrinted bool
}

func NewAssessmentProgressTracker() *AssessmentProgressTracker {
	return &AssessmentProgressTracker{}
}

// spinnerLabel builds the text shown next to the spinner (includes sub-stage if set).
func (t *AssessmentProgressTracker) spinnerLabel() string {
	base := t.stageName
	if t.stepsTotal > 0 {
		base = fmt.Sprintf("%s (%d/%d)", base, t.stepsDone, t.stepsTotal)
	}
	if t.subStage != "" {
		base = fmt.Sprintf("%s %s", base, t.subStage)
	}
	return base
}

// completedLabel builds the text shown on the checkmark line (no sub-stage).
func (t *AssessmentProgressTracker) completedLabel() string {
	if t.stepsTotal > 0 {
		return fmt.Sprintf("%s (%d/%d)", t.stageName, t.stepsDone, t.stepsTotal)
	}
	return t.stageName
}

// StartStage begins a new stage with a spinner. totalSteps can be 0 if unknown upfront.
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
	t.spinnerStop = make(chan struct{})
	t.spinnerDone = make(chan struct{})
	go t.runSpinner()
}

// IncrementStep increments the step counter and updates the inner sub-stage label.
func (t *AssessmentProgressTracker) IncrementStep(subStage string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stepsDone++
	t.subStage = subStage
}

// CompleteStage stops the spinner and prints a checkmark for the current stage.
func (t *AssessmentProgressTracker) CompleteStage() {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.stopSpinnerLocked()

	if t.stepsTotal > 0 {
		t.stepsDone = t.stepsTotal
	}
	t.subStage = ""

	fmt.Printf("\r\033[K")
	checkColor.Printf("  %s %s\n", "\u2713", t.completedLabel())
}

// Finish stops any running spinner without printing a checkmark.
func (t *AssessmentProgressTracker) Finish() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.stopSpinnerLocked()
}

func (t *AssessmentProgressTracker) stopSpinnerLocked() {
	if t.spinnerStop != nil {
		close(t.spinnerStop)
		<-t.spinnerDone
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
			t.mu.Lock()
			label := t.spinnerLabel()
			t.mu.Unlock()
			frame := spinnerFrames[i%len(spinnerFrames)]
			fmt.Printf("\r\033[K")
			stageColor.Printf("  %s %s", frame, label)
			i++
			time.Sleep(80 * time.Millisecond)
		}
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
