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
	"time"

	"github.com/google/uuid"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

type StreamImportStatsReporter struct {
	migrationUUID       uuid.UUID
	totalEventsImported int64
	latestBatchEvents    int64
	importRatesPerMinute map[float64]float64
	importRatePerMinute float64
	importRateLast3Mins float64
	importRateLast10Mins float64
	startTime 		 time.Time
}



func NewStreamImportStatsReporter() *StreamImportStatsReporter {
	return &StreamImportStatsReporter{}
}

func(s *StreamImportStatsReporter) Init(tdb tgtdb.TargetDB, migrationUUID uuid.UUID) error {
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


func(s *StreamImportStatsReporter) ReportStats() {
	var remainingEvents int64 //TODO: calculate remaining events
	var estimatedTimeToCatchUp time.Duration
	displayTicker := time.NewTicker(30 * time.Second)
	defer displayTicker.Stop()
	
	for {
		select {
		case <-displayTicker.C:
			fmt.Printf("\rTotal Imported events: %d\n", s.totalEventsImported)
			fmt.Printf("\rLast imported events: %d\n", s.latestBatchEvents)
			fmt.Printf("\rIngestion Rate (last 3 mins): %.2f events/min\n", s.importRateLast3Mins / 3)
			fmt.Printf("\rIngestion Rate (last 10 mins): %.2f events/min\n", s.importRateLast10Mins / 10)
			fmt.Printf("\rRemaining Events: %d\n", remainingEvents)
			fmt.Printf("\rEstimated Time to catch up: %v\n", estimatedTimeToCatchUp)
		}
	}
}

func (s *StreamImportStatsReporter) CalcStats() {
	calcTicket := time.NewTicker(1* time.Minute)	
	defer calcTicket.Stop()

	for {
		select {
		case <-calcTicket.C:
			elapsedTime := time.Since(s.startTime).Minutes()
			rate := float64(s.totalEventsImported) / elapsedTime
			s.importRatesPerMinute[elapsedTime] = rate
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
	}
}

func(s *StreamImportStatsReporter) BatchImported(numInserts, numUpdates, numDeletes int64) {
	s.latestBatchEvents = numInserts + numUpdates + numDeletes
	s.totalEventsImported += numInserts + numUpdates + numDeletes
}