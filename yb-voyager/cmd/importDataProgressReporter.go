package cmd

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type ImportDataProgressReporter struct {
	sync.Mutex
	disablePb    bool
	progress     *mpb.Progress
	progressBars map[int]*mpb.Bar
}

func NewImportDataProgressReporter(disablePb bool) *ImportDataProgressReporter {
	pr := &ImportDataProgressReporter{
		disablePb:    disablePb,
		progress:     mpb.New(),
		progressBars: make(map[int]*mpb.Bar),
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
}

func (pr *ImportDataProgressReporter) AddProgressAmount(task *ImportFileTask, progressAmount int64) {
	pr.Lock()
	defer pr.Unlock()

	log.Infof("Add %v progress to table %s", progressAmount, task.TableName)
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
	progressBar.SetTotal(-1, true)
}
