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
	"fmt"
	"os"
	"path/filepath"

	"github.com/goccy/go-json"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	QUEUE_DIR_NAME               = "queue"
	QUEUE_SEGMENT_FILE_NAME      = "segment"
	QUEUE_SEGMENT_FILE_EXTENSION = "ndjson"
)

type SourceEventQueue struct {
	QueueDirPath       string
	SegmentNumToStream int64
}

func NewSourceEventQueue(exportDir string) *SourceEventQueue {
	return &SourceEventQueue{
		QueueDirPath:       filepath.Join(exportDir, "data", QUEUE_DIR_NAME),
		SegmentNumToStream: 0,
	}
}

// GetNextSegment returns the next segment to process
func (eq *SourceEventQueue) GetNextSegment() (*SourceEventQueueSegment, error) {
	segmentFileName := fmt.Sprintf("%s.%d.%s", QUEUE_SEGMENT_FILE_NAME, eq.SegmentNumToStream, QUEUE_SEGMENT_FILE_EXTENSION)
	segmentFilePath := filepath.Join(eq.QueueDirPath, segmentFileName)
	_, err := os.Stat(segmentFilePath)
	if err != nil {
		return nil, fmt.Errorf("failed to get next segment file path: %w", err)
	}

	segment := NewSourceEventQueueSegment(segmentFilePath, eq.SegmentNumToStream)
	eq.SegmentNumToStream++
	return segment, nil
}

type SourceEventQueueSegment struct {
	FilePath   string
	SegmentNum int64 // 0-based
	processed  bool
	file       *os.File
	scanner    *bufio.Scanner
	buffer     []byte // buffer for scanning from file
}

var EOFMarker = `\.`

func NewSourceEventQueueSegment(filePath string, segmentNum int64) *SourceEventQueueSegment {
	return &SourceEventQueueSegment{
		FilePath:   filePath,
		SegmentNum: segmentNum,
		processed:  false,
	}
}

func (eqs *SourceEventQueueSegment) Open() error {
	file, err := os.OpenFile(eqs.FilePath, os.O_RDONLY, 0640)
	if err != nil {
		return fmt.Errorf("failed to open segment file %s: %w", eqs.FilePath, err)
	}
	eqs.file = file
	eqs.scanner = bufio.NewScanner(utils.NewTailReader(file))

	// providing buffer to scanner for scanning
	eqs.buffer = make([]byte, 0, 100*KB)
	eqs.scanner.Buffer(eqs.buffer, cap(eqs.buffer))
	return nil
}

func (eqs *SourceEventQueueSegment) Close() error {
	return eqs.file.Close()
}

// ReadEvent reads an event from the segment file.
// Waits until an event is available.
func (eqs *SourceEventQueueSegment) NextEvent() (*tgtdb.Event, error) {
	var event tgtdb.Event

	// Scan() return false in case of error but it is handled below by Err()
	_ = eqs.scanner.Scan()
	// scanner.Err() returns nil when EOF error
	line, err := eqs.scanner.Bytes(), eqs.scanner.Err()
	if err != nil {
		return nil, fmt.Errorf("failed to read line from %s: %w", eqs.FilePath, err)
	}

	if string(line) == EOFMarker {
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

func (eqs *SourceEventQueueSegment) IsProcessed() bool {
	return eqs.processed
}
