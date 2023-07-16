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

package tgtdb

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	QUEUE_DIR_NAME               = "queue"
	QUEUE_SEGMENT_FILE_NAME      = "segment"
	QUEUE_SEGMENT_FILE_EXTENSION = "ndjson"
)

type EventQueue struct {
	QueueDirPath string
}

func NewEventQueue(exportDir string) *EventQueue {
	return &EventQueue{
		QueueDirPath: filepath.Join(exportDir, "data", QUEUE_DIR_NAME),
	}
}

// GetNextSegments returns the next segments to process, in order.
func (eq *EventQueue) GetNextSegments() ([]*EventQueueSegment, error) {
	log.Infof("Getting next segments to process from %s", eq.QueueDirPath)
	reSegmentName := regexp.MustCompile(fmt.Sprintf("%s.[0-9]+.%s$", QUEUE_SEGMENT_FILE_NAME, QUEUE_SEGMENT_FILE_EXTENSION))
	dirEntries, err := os.ReadDir(eq.QueueDirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read dir %s: %w", eq.QueueDirPath, err)
	}
	var segmentPaths []string
	for _, dirEntry := range dirEntries {
		if !dirEntry.IsDir() && reSegmentName.MatchString(dirEntry.Name()) {
			segmentPaths = append(segmentPaths, filepath.Join(eq.QueueDirPath, dirEntry.Name()))
		}
	}

	segments := make([]*EventQueueSegment, 0, len(segmentPaths))
	for _, segmentPath := range segmentPaths {
		segmentFileName := filepath.Base(segmentPath)
		var segmentNum int64
		_, err := fmt.Sscanf(segmentFileName, QUEUE_SEGMENT_FILE_NAME+".%d."+QUEUE_SEGMENT_FILE_EXTENSION, &segmentNum)
		if err != nil {
			return nil, fmt.Errorf("failed to parse segment number from %s: %w", segmentFileName, err)
		}
		segments = append(segments, NewEventQueueSegment(segmentPath, segmentNum))
	}

	sort.Slice(segments, func(i, j int) bool {
		return segments[i].SegmentNum < segments[j].SegmentNum
	})

	return segments, nil
}

type EventQueueSegment struct {
	FilePath   string
	SegmentNum int64 // 0-based
	processed  bool
	file       *os.File
	reader     *utils.TailReader
}

var EOFMarker = `\.`

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
	eqs.reader = utils.NewTailReader(file)
	return nil
}

func (eqs *EventQueueSegment) Close() error {
	return eqs.file.Close()
}

// ReadEvent reads an event from the segment file.
// Waits until an event is available.
func (eqs *EventQueueSegment) NextEvent() (*Event, error) {
	var event Event
	line, err := eqs.reader.ReadLine()
	if err != nil && err != io.EOF {
		return nil, fmt.Errorf("failed to read line from %s: %w", eqs.FilePath, err)
	}

	if string(line) == EOFMarker && err == io.EOF {
		log.Infof("reached EOF marker in segment %s", eqs.FilePath)
		eqs.processed = true
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

func (eqs *EventQueueSegment) MarkProcessed() error {
	if !eqs.processed {
		return fmt.Errorf("cannot mark segment %s as processed before reading all events", eqs.FilePath)
	}

	processedFilePath := fmt.Sprintf("%s.processed", eqs.FilePath)
	err := os.Rename(eqs.FilePath, processedFilePath)
	if err != nil {
		return fmt.Errorf("failed to rename %s to %s: %w", eqs.FilePath, processedFilePath, err)
	}
	return nil
}
