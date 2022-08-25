package importdata

import (
	"fmt"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type ProgressReporter struct {
	sync.Mutex
	disablePb    bool
	progress     *mpb.Progress
	progressBars map[string]*mpb.Bar
}

func NewProgressReporter(disablePb bool) *ProgressReporter {
	pr := &ProgressReporter{
		disablePb:    disablePb,
		progress:     mpb.New(),
		progressBars: make(map[string]*mpb.Bar),
	}
	return pr
}

func (pr *ProgressReporter) ImportFileStarted(tableID *TableID, totalProgressAmount int64) {
	pr.Lock()
	defer pr.Unlock()

	if pr.disablePb {
		fmt.Printf("Table %s: import started\n", tableID)
		return
	}
	log.Infof("Import started for table %s, total progress: %v", tableID, totalProgressAmount)

	name := fmt.Sprintf("%s:%s", tableID.SchemaName, tableID.TableName)
	bar := pr.progress.AddBar(totalProgressAmount,
		mpb.BarFillerClearOnComplete(),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(name),
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
	pr.progressBars[tableID.String()] = bar
}

func (pr *ProgressReporter) AddProgressAmount(tableID *TableID, progressAmount int64) {
	pr.Lock()
	defer pr.Unlock()

	log.Infof("Add %v progress to table %s", progressAmount, tableID)
	if pr.disablePb {
		return
	}
	progressBar := pr.progressBars[tableID.String()]
	progressBar.IncrInt64(progressAmount)
}

func (pr *ProgressReporter) TableImportDone(tableID *TableID) {
	pr.Lock()
	defer pr.Unlock()
	if pr.disablePb {
		utils.PrintAndLog("Table %s: import completed", tableID)
		return
	}
	progressBar := pr.progressBars[tableID.String()]
	progressBar.SetTotal(-1, true)
}
