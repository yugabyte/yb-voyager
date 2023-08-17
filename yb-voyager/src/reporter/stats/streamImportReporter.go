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
package stats

import (
	"fmt"
	"math"
	"strconv"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"
	"github.com/gosuri/uilive"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

type StreamImportStatsReporter struct {
	migrationUUID        uuid.UUID
	totalEventsImported  int64
	latestBatchEvents    int64
	importRatesPerMinute map[float64]float64
	importRatePerMinute  float64
	importRateLast3Mins  float64
	importRateLast10Mins float64
	startTime            time.Time
}

func NewStreamImportStatsReporter() *StreamImportStatsReporter {
	return &StreamImportStatsReporter{}
}

func (s *StreamImportStatsReporter) Init(tdb tgtdb.TargetDB, migrationUUID uuid.UUID) error {
	s.migrationUUID = migrationUUID
	numInserts, numUpdates, numDeletes, err := tdb.GetImportStatsMetaInfo(migrationUUID)
	s.totalEventsImported = numInserts + numUpdates + numDeletes
	if err != nil {
		return fmt.Errorf("failed to fetch import stats meta info from target : %w", err)
	}
	s.startTime = time.Now()
	s.importRatesPerMinute = make(map[float64]float64)
	return nil
}

func (s *StreamImportStatsReporter) ReportStats() {
	var remainingEvents int64 //TODO: calculate remaining events using sqlite db table exported_event_count
	var estimatedTimeToCatchUp time.Duration
	displayTicker := time.NewTicker(30 * time.Second)
	defer displayTicker.Stop()
	table := uilive.New()
	headerRow := table.Newline()
	seperator := table.Newline()
	row1 := table.Newline()
	row2 := table.Newline()
	row3 := table.Newline()
	row4 := table.Newline()
	row5 := table.Newline()
	row6 := table.Newline()
	table.Start()

	for range displayTicker.C {
		s.CalcStats()
		fmt.Fprint(seperator, color.GreenString("| %-30s | %30s |\n", "-----------------------------", "-----------------------------"))
		fmt.Fprint(headerRow, color.GreenString("| %-30s | %30s |\n", "Metric", "Value"))
		fmt.Fprint(seperator, color.GreenString("| %-30s | %30s |\n", "-----------------------------", "-----------------------------"))
		fmt.Fprint(row1, color.GreenString("| %-30s | %30s |\n", "Total Imported events", strconv.FormatInt(s.totalEventsImported, 10)))
		fmt.Fprint(row2, color.GreenString("| %-30s | %30s |\n", "Last imported events", strconv.FormatInt(s.latestBatchEvents, 10)))
		fmt.Fprint(row3, color.GreenString("| %-30s | %30s |\n", "Ingestion Rate (last 3 mins)", fmt.Sprintf("%.0f events/sec", math.Round(s.importRateLast3Mins/3/60))))
		fmt.Fprint(row4, color.GreenString("| %-30s | %30s |\n", "Ingestion Rate (last 10 mins)", fmt.Sprintf("%.0f events/sec", math.Round(s.importRateLast10Mins/10/60))))
		fmt.Fprint(row5, color.GreenString("| %-30s | %30s |\n", "Remaining Events", strconv.FormatInt(remainingEvents, 10)))
		fmt.Fprint(row6, color.GreenString("| %-30s | %30s |\n", "Estimated Time to catch up", estimatedTimeToCatchUp.String()))
		fmt.Fprint(seperator, color.GreenString("| %-30s | %30s |\n", "-----------------------------", "-----------------------------"))
		table.Flush()
	}
}

func (s *StreamImportStatsReporter) CalcStats() {
	elapsedTime := math.Round(time.Since(s.startTime).Minutes()*100) / 100
	rate := float64(s.totalEventsImported) / elapsedTime
	s.importRatePerMinute = rate
	elapsedMinutes := int(math.Floor(elapsedTime))
	if elapsedTime == float64(elapsedMinutes) { //only store rate per minute
		s.importRatesPerMinute[elapsedTime] = rate
	}
	s.importRateLast3Mins += rate
	if elapsedTime > 3 {
		s.importRateLast3Mins -= s.importRatesPerMinute[elapsedTime-3]
	}
	s.importRateLast10Mins += rate
	if elapsedTime > 10 {
		s.importRateLast10Mins -= s.importRatesPerMinute[elapsedTime-10]
		delete(s.importRatesPerMinute, elapsedTime-10)
	}
}

func (s *StreamImportStatsReporter) BatchImported(numInserts, numUpdates, numDeletes int64) {
	s.latestBatchEvents = numInserts + numUpdates + numDeletes
	s.totalEventsImported += numInserts + numUpdates + numDeletes
}
