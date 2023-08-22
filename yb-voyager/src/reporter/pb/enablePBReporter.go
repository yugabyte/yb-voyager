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
package pbreporter

import (
	"github.com/vbauerster/mpb/v8"
	"github.com/vbauerster/mpb/v8/decor"
)

type EnablePBReporter struct {
	bar *mpb.Bar
}

func newEnablePBReporter(progressContainer *mpb.Progress, tableName string) *EnablePBReporter {
	bar := progressContainer.AddBar(int64(0), // mandatory to set total with 0 while AddBar to achieve dynamic total behaviour
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
	return &EnablePBReporter{bar: bar}
}

func (pbr *EnablePBReporter) SetExportedRowCount(exportedRowCount int64) {
	pbr.bar.SetCurrent(exportedRowCount)
}

func (pbr *EnablePBReporter) SetTotalRowCount(totalRowCount int64, triggerComplete bool) {
	pbr.bar.SetTotal(totalRowCount, triggerComplete)
}

func (pbr *EnablePBReporter) IsComplete() bool {
	return pbr.bar.Completed()
}
