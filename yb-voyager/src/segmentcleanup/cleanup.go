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
package segmentcleanup

import (
	"os"
	"path/filepath"
	"time"

	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	PolicyDelete  = "delete"
	PolicyRetain  = "retain"
	PolicyArchive = "archive"
)

var ValidPolicyNames = []string{PolicyDelete, PolicyRetain, PolicyArchive}

func IsValidPolicy(policy string) bool {
	for _, v := range ValidPolicyNames {
		if policy == v {
			return true
		}
	}
	return false
}

type Config struct {
	Policy                 string
	ExportDir              string
	ArchiveDir             string
	FSUtilizationThreshold int
}

type SegmentCleaner struct {
	config Config
	metaDB *metadb.MetaDB
	stop   bool
}

func NewSegmentCleaner(cfg Config, metaDB *metadb.MetaDB) *SegmentCleaner {
	return &SegmentCleaner{
		config: cfg,
		metaDB: metaDB,
	}
}

// SignalStop tells the cleaner to drain remaining work and exit.
func (sc *SegmentCleaner) SignalStop() {
	sc.stop = true
}

func (sc *SegmentCleaner) Run() error {
	switch sc.config.Policy {
	case PolicyDelete:
		return sc.runDeletePolicy()
	case PolicyRetain:
		return sc.runRetainPolicy()
	case PolicyArchive:
		return sc.runArchivePolicy()
	default:
		return goerrors.Errorf("unsupported cleanup policy: %s", sc.config.Policy)
	}
}

func (sc *SegmentCleaner) isFSUtilizationExceeded() bool {
	if sc.stop {
		return true
	}
	fsUtil, err := utils.GetFSUtilizationPercentage(sc.config.ExportDir)
	if err != nil {
		utils.ErrExit("failed to get fs utilization: %v", err)
	}
	return fsUtil > sc.config.FSUtilizationThreshold
}

// --- delete policy ---

func (sc *SegmentCleaner) runDeletePolicy() error {
	utils.PrintAndLogf("segment cleanup running with policy=delete, fs-utilization-threshold=%d%%\n",
		sc.config.FSUtilizationThreshold)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if !sc.isFSUtilizationExceeded() {
			continue
		}

		segments, err := sc.metaDB.GetProcessedQueueSegments()
		if err != nil {
			return goerrors.Errorf("get processed segments: %v", err)
		}

		if sc.stop && len(segments) == 0 {
			log.Infof("all processed segments deleted, cleanup complete")
			return nil
		}

		for _, seg := range segments {
			if err := sc.deleteSegment(seg); err != nil {
				return goerrors.Errorf("delete segment %s: %v", seg.FilePath, err)
			}
			if !sc.isFSUtilizationExceeded() {
				break
			}
		}
	}
	return nil
}

func (sc *SegmentCleaner) deleteSegment(seg utils.Segment) error {
	if utils.FileOrFolderExists(seg.FilePath) {
		if err := os.Remove(seg.FilePath); err != nil {
			return goerrors.Errorf("remove segment file %s: %v", seg.FilePath, err)
		}
	}
	if err := sc.metaDB.MarkSegmentDeletedAndArchived(seg.Num); err != nil {
		return goerrors.Errorf("mark segment %d as deleted and archived: %v", seg.Num, err)
	}
	utils.PrintAndLogf("segment file %s deleted", seg.FilePath)
	return nil
}

// --- retain policy ---

func (sc *SegmentCleaner) runRetainPolicy() error {
	utils.PrintAndLogf("segment cleanup running with policy=retain — processed segments will not be deleted\n")

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		if sc.stop {
			log.Infof("retain policy: stop signal received, exiting")
			return nil
		}
		fsUtil, err := utils.GetFSUtilizationPercentage(sc.config.ExportDir)
		if err != nil {
			return goerrors.Errorf("get fs utilization: %v", err)
		}
		if fsUtil > sc.config.FSUtilizationThreshold {
			utils.PrintAndLogf("WARNING: fs utilization at %d%% (threshold %d%%) — segments are being retained per policy\n",
				fsUtil, sc.config.FSUtilizationThreshold)
		}
	}
	return nil
}

// --- archive policy ---

func (sc *SegmentCleaner) runArchivePolicy() error {
	if sc.config.ArchiveDir == "" {
		return goerrors.Errorf("archive policy requires --archive-dir to be specified")
	}
	utils.PrintAndLogf("segment cleanup running with policy=archive, archive-dir=%s\n",
		sc.config.ArchiveDir)

	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for range ticker.C {
		segments, err := sc.metaDB.GetProcessedQueueSegments()
		if err != nil {
			return goerrors.Errorf("get processed segments: %v", err)
		}

		if sc.stop && len(segments) == 0 {
			log.Infof("all processed segments archived and deleted, cleanup complete")
			return nil
		}

		for _, seg := range segments {
			if err := sc.archiveAndDeleteSegment(seg); err != nil {
				return goerrors.Errorf("archive segment %s: %v", seg.FilePath, err)
			}
		}
	}
	return nil
}

func (sc *SegmentCleaner) archiveAndDeleteSegment(seg utils.Segment) error {
	archivePath := filepath.Join(sc.config.ArchiveDir, filepath.Base(seg.FilePath))

	if utils.FileOrFolderExists(seg.FilePath) {
		if utils.FileOrFolderExists(archivePath) {
			if err := os.Remove(archivePath); err != nil {
				return goerrors.Errorf("remove existing archive file %s: %v", archivePath, err)
			}
			log.Infof("removed existing archive file %s before re-archiving", archivePath)
		}
		if err := utils.CopyFile(seg.FilePath, archivePath); err != nil {
			return goerrors.Errorf("copy segment to archive: %v", err)
		}
		if err := os.Remove(seg.FilePath); err != nil {
			return goerrors.Errorf("remove segment file %s after archiving: %v", seg.FilePath, err)
		}
	}

	if err := sc.metaDB.UpdateSegmentArchiveLocation(seg.Num, archivePath); err != nil {
		return goerrors.Errorf("update archive location for segment %d: %v", seg.Num, err)
	}
	if err := sc.metaDB.MarkSegmentDeleted(seg.Num); err != nil {
		return goerrors.Errorf("mark segment %d as deleted: %v", seg.Num, err)
	}
	utils.PrintAndLogf("segment file %s archived to %s and deleted", seg.FilePath, archivePath)
	return nil
}
