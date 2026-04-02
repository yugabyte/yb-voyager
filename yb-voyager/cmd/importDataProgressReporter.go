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
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type ImportDataProgressReporter struct {
	sync.Mutex
	disablePb           bool
	progress            *mpb.Progress
	progressBars        map[int]*mpb.Bar
	totalProgressAmount map[int]int64
	prependDecorators   map[int]*PrependDecorator
}

func NewImportDataProgressReporter(disablePb bool) *ImportDataProgressReporter {
	pr := &ImportDataProgressReporter{
		disablePb:           disablePb,
		progress:            mpb.New(),
		progressBars:        make(map[int]*mpb.Bar),
		totalProgressAmount: make(map[int]int64),
		prependDecorators:   make(map[int]*PrependDecorator),
	}
	return pr
}

// Custom prepend decorator
// This decorator is used to prepend the table name to the progress bar
// It is also used to display the resume message to the user
// Implementing the decor.Decorator interface for the customization
type PrependDecorator struct {
	content   string
	conf      decor.WC
	mu        sync.RWMutex
	tableName string
	resumeMsg string
}

func NewPrependDecorator(initialContent string) *PrependDecorator {
	return &PrependDecorator{
		content:   initialContent,
		tableName: initialContent,
		conf:      decor.WC{W: len(initialContent)},
	}
}

func (pd *PrependDecorator) SetContent(content string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.content = content
	pd.conf.W = len(content)
}

func (pd *PrependDecorator) SetResumeMessage(msg string) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.resumeMsg = msg
	pd.updateContent()
}

func (pd *PrependDecorator) ClearResumeMessage() {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.resumeMsg = ""
	pd.updateContent()
}

func (pd *PrependDecorator) updateContent() {
	if pd.resumeMsg != "" {
		pd.content = "(" + pd.resumeMsg + ") " + pd.tableName
	} else {
		pd.content = pd.tableName
	}
	pd.conf.W = len(pd.content)
}

// This is the function which gets run whenever there is an update in decorator to re-render the bar UI
// so returning the content which is updated content with  resume msg in case of resumption
func (pd *PrependDecorator) Decor(s decor.Statistics) string {
	pd.mu.RLock()
	defer pd.mu.RUnlock()

	return pd.content
}

// these below functions are just for implementing the Decorator interface and no change to these
func (pd *PrependDecorator) GetConf() decor.WC {
	pd.mu.RLock()
	defer pd.mu.RUnlock()
	return pd.conf
}

func (pd *PrependDecorator) SetConf(conf decor.WC) {
	pd.mu.Lock()
	defer pd.mu.Unlock()
	pd.conf = conf
}

func (pd *PrependDecorator) Sync() (chan int, bool) {
	// No-op for this decorator
	return nil, false
}
func (pr *ImportDataProgressReporter) DisplayInformation(info string) {
	pr.Lock()
	defer pr.Unlock()
	_, err := pr.progress.Write([]byte(info))
	if err != nil {
		log.Infof("error writing the information (%s) on the console via progress container: %v", info, err)
	}
}

func (pr *ImportDataProgressReporter) ImportFileStarted(task *ImportFileTask, totalProgressAmount int64) {
	pr.Lock()
	defer pr.Unlock()

	if pr.disablePb {
		//displaying the table as well in the print to cover the same file to multiple table scenario
		fmt.Printf("File %s: import to table: %s started\n", task.FilePath, task.TableNameTup.ForOutput())
		return
	}
	log.Infof("Import started for file %s, total progress: %v", task.FilePath, totalProgressAmount)

	// Create prepend decorator to display the table name and some progress for resumption
	prependDecorator := NewPrependDecorator(task.TableNameTup.ForOutput())
	pr.prependDecorators[task.ID] = prependDecorator

	bar := pr.progress.AddBar(totalProgressAmount,
		mpb.BarFillerClearOnComplete(),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			prependDecorator,
		),
		mpb.AppendDecorators(
			decor.OnComplete(
				decor.NewPercentage("%.2f", decor.WCSyncSpaceR), "completed",
			),
			decor.OnComplete(
				decor.AverageETA(decor.ET_STYLE_GO), "",
			),
		),
	)
	pr.progressBars[task.ID] = bar
	pr.totalProgressAmount[task.ID] = totalProgressAmount
}

func (pr *ImportDataProgressReporter) AddResumeInformation(task *ImportFileTask, msg string) {
	pr.Lock()
	defer pr.Unlock()

	prependDecorator, ok := pr.prependDecorators[task.ID]
	if !ok {
		return
	}

	prependDecorator.SetResumeMessage(msg)
}

func (pr *ImportDataProgressReporter) RemoveResumeInformation(task *ImportFileTask) {
	pr.Lock()
	defer pr.Unlock()

	prependDecorator, ok := pr.prependDecorators[task.ID]
	if !ok {
		return
	}

	prependDecorator.ClearResumeMessage()
}

func (pr *ImportDataProgressReporter) AddProgressAmount(task *ImportFileTask, progressAmount int64) {
	pr.Lock()
	defer pr.Unlock()

	if pr.disablePb {
		return
	}
	progressBar := pr.progressBars[task.ID]
	progressBar.IncrInt64(progressAmount)
}

func (pr *ImportDataProgressReporter) FileImportDone(task *ImportFileTask) {
	pr.Lock()
	defer pr.Unlock()
	if pr.disablePb {
		utils.PrintAndLogf("Table %s: import completed", task.TableNameTup.ForOutput())
		return
	}
	progressBar := pr.progressBars[task.ID]
	progressBar.SetCurrent(pr.totalProgressAmount[task.ID])
}
