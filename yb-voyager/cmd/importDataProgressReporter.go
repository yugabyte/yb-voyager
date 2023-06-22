/*
Copyright (c) YugaByte, Inc.

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
}

func NewImportDataProgressReporter(disablePb bool) *ImportDataProgressReporter {
	pr := &ImportDataProgressReporter{
		disablePb:           disablePb,
		progress:            mpb.New(),
		progressBars:        make(map[int]*mpb.Bar),
		totalProgressAmount: make(map[int]int64),
	}
	return pr
}

func (pr *ImportDataProgressReporter) ImportFileStarted(task *ImportFileTask, totalProgressAmount int64) {
	pr.Lock()
	defer pr.Unlock()

	if pr.disablePb {
		fmt.Printf("File %s: import started\n", task.FilePath)
		return
	}
	log.Infof("Import started for file %s, total progress: %v", task.FilePath, totalProgressAmount)

	bar := pr.progress.AddBar(totalProgressAmount,
		mpb.BarFillerClearOnComplete(),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(task.TableName),
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
		utils.PrintAndLog("Table %s: import completed", task.TableName)
		return
	}
	progressBar := pr.progressBars[task.ID]
	progressBar.SetCurrent(pr.totalProgressAmount[task.ID])
}
