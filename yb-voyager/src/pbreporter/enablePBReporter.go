package pbreporter

import (
	"github.com/vbauerster/mpb/v7"
	"github.com/vbauerster/mpb/v7/decor"
)

type EnablePBReporter struct {
	bar *mpb.Bar
}

func newEnablePBReporter(progressContainer *mpb.Progress, tableName string) EnablePBReporter {
	bar := progressContainer.AddBar(int64(0),
		mpb.BarFillerClearOnComplete(),
		mpb.BarRemoveOnComplete(),
		mpb.PrependDecorators(
			decor.Name(tableName),
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
	return EnablePBReporter{bar: bar}
}

func (pbr EnablePBReporter) SetExportedRowCount(exportedRowCount int64) {
	pbr.bar.SetCurrent(exportedRowCount)
}

func (pbr EnablePBReporter) SetTotalRowCount(totalRowCount int64, triggerComplete bool) {
	pbr.bar.SetTotal(totalRowCount, triggerComplete)
}

func (pbr EnablePBReporter) IsComplete() bool {
	return pbr.bar.Completed()
}
