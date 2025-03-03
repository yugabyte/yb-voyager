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
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/goccy/go-json"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	QUEUE_DIR_NAME               = "queue"
	QUEUE_SEGMENT_FILE_NAME      = "segment"
	QUEUE_SEGMENT_FILE_EXTENSION = "ndjson"
)

type EventQueue struct {
	QueueDirPath       string
	SegmentNumToStream int64
	EndOfQueue         bool
}

func NewEventQueue(exportDir string) *EventQueue {
	return &EventQueue{
		QueueDirPath:       filepath.Join(exportDir, "data", QUEUE_DIR_NAME),
		SegmentNumToStream: -1,
		EndOfQueue:         false,
	}
}

// GetNextSegment returns the next segment to process
func (eq *EventQueue) GetLastProcessedSegment() (*EventQueueSegment, error) {
	var err error
	segmentsExporterRole := ""
	if importerRole == SOURCE_DB_IMPORTER_ROLE {
		// in case of fall-back import, restrict to only segments exported from target db.
		segmentsExporterRole = TARGET_DB_EXPORTER_FB_ROLE

	}
	eq.SegmentNumToStream, err = metaDB.GetMaxSegmentExportedByAndImportedBy(importerRole, segmentsExporterRole)
	segmentFileName := fmt.Sprintf("%s.%d.%s", QUEUE_SEGMENT_FILE_NAME, eq.SegmentNumToStream, QUEUE_SEGMENT_FILE_EXTENSION)
	segmentFilePath := filepath.Join(eq.QueueDirPath, segmentFileName)
	_, err = os.Stat(segmentFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get next segment file path: %w", err)
	}
	segment := NewEventQueueSegment(segmentFilePath, eq.SegmentNumToStream)
	return segment, nil
}

// GetNextSegment returns the next segment to process
func (eq *EventQueue) GetNextSegment() (*EventQueueSegment, error) {
	var err error
	if eq.SegmentNumToStream == -1 {
		// called for the first time

		err = eq.resolveSegmentToResumeFrom()
		if err != nil {
			return nil, fmt.Errorf("failed to get segment num to resume: %w", err)
		}
		log.Info("segment num to resume: ", eq.SegmentNumToStream)
	}
	segmentFileName := fmt.Sprintf("%s.%d.%s", QUEUE_SEGMENT_FILE_NAME, eq.SegmentNumToStream, QUEUE_SEGMENT_FILE_EXTENSION)
	segmentFilePath := filepath.Join(eq.QueueDirPath, segmentFileName)
	_, err = os.Stat(segmentFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get next segment file path: %w", err)
	}

	segment := NewEventQueueSegment(segmentFilePath, eq.SegmentNumToStream)
	eq.SegmentNumToStream++
	return segment, nil
}

func (eq *EventQueue) resolveSegmentToResumeFrom() error {
	var err error
	segmentsExporterRole := ""
	if importerRole == SOURCE_DB_IMPORTER_ROLE {
		// in case of fall-back import, restrict to only segments exported from target db.
		segmentsExporterRole = TARGET_DB_EXPORTER_FB_ROLE

	}

	for {
		eq.SegmentNumToStream, err = metaDB.GetMinSegmentExportedByAndNotImportedBy(importerRole, segmentsExporterRole)
		if err == nil {
			break
		} else if errors.Is(err, metadb.ErrNoQueueSegmentsFound) {
			time.Sleep(2 * time.Second)
			log.Infof("Did not find any segments to start streaming from. Retrying..")
			continue
		} else {
			return err
		}
	}

	if err != nil {
		return err
	}
	return nil
}

type EventQueueSegment struct {
	FilePath   string
	SegmentNum int64 // 0-based
	processed  bool
	file       *os.File
	reader     *bufio.Reader
}

var EOFMarker = []byte(`\.`)

func NewEventQueueSegment(filePath string, segmentNum int64) *EventQueueSegment {
	return &EventQueueSegment{
		FilePath:   filePath,
		SegmentNum: segmentNum,
		processed:  false,
	}
}

func (eqs *EventQueueSegment) Open() error {
	file, err := os.OpenFile(eqs.FilePath, os.O_RDONLY, 0640)
	if err != nil {
		return fmt.Errorf("failed to open segment file %s: %w", eqs.FilePath, err)
	}
	eqs.file = file

	fn := func() (int64, error) {
		return metaDB.GetLastValidOffsetInSegmentFile(eqs.SegmentNum)
	}
	eqs.reader = bufio.NewReaderSize(utils.NewTailReader(file, fn), 100*MB)
	return nil
}

func (eqs *EventQueueSegment) Close() error {
	return eqs.file.Close()
}

// ReadEvent reads an event from the segment file.
// Waits until an event is available.
func (eqs *EventQueueSegment) NextEvent() (*tgtdb.Event, error) {
	var event tgtdb.Event
	var err error
	var isPrefix = true
	var line, currLine []byte

	for isPrefix && err == nil {
		currLine, isPrefix, err = eqs.reader.ReadLine()
		if isPrefix || len(line) != 0 {
			line = append(line, currLine...)
		} else {
			line = currLine
		}
	}
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read line from %s: %w", eqs.FilePath, err)
	}

	if bytes.Equal(line, EOFMarker) {
		log.Infof("reached EOF marker in segment %s", eqs.FilePath)
		eqs.MarkProcessed()
		return nil, nil
	}
	err = json.Unmarshal(line, &event)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal json event %s: %w", string(line), err)
	}
	return &event, nil
}

func (eqs *EventQueueSegment) IsProcessed() bool {
	return eqs.processed
}

func (eqs *EventQueueSegment) MarkProcessed() {
	eqs.processed = true
}

func (eqs *EventQueueSegment) getLastEvent() (*tgtdb.Event, error) {
	var lastEvent *tgtdb.Event
	for {
		event, err := eqs.NextEvent()
		if err != nil {
			return nil, fmt.Errorf("error getting next event of last processed segment: %v", err)
		}
		if event == nil {
			break
		}

		lastEvent = event
	}

	return lastEvent, nil

}
