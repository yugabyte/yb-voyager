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
	"fmt"
	"os"
	"time"

	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	PolicyDelete = "delete"
	PolicyRetain = "retain"
)

var ValidPolicyNames = []string{PolicyDelete, PolicyRetain}

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
	FSUtilizationThreshold int
}

type SegmentCleaner struct {
	config     Config
	metaDB     *metadb.MetaDB
	stopSignal *bool
}

func NewSegmentCleaner(cfg Config, metaDB *metadb.MetaDB, stopSignal *bool) *SegmentCleaner {
	return &SegmentCleaner{
		config:     cfg,
		metaDB:     metaDB,
		stopSignal: stopSignal,
	}
}

func (sc *SegmentCleaner) isStopRequested() bool {
	return sc.stopSignal != nil && *sc.stopSignal
}

func (sc *SegmentCleaner) Run() error {
	switch sc.config.Policy {
	case PolicyDelete:
		return sc.runDeletePolicy()
	case PolicyRetain:
		return sc.runRetainPolicy()
	default:
		return fmt.Errorf("unsupported cleanup policy: %s", sc.config.Policy)
	}
}

func (sc *SegmentCleaner) getImportCount() (int, error) {
	msr, err := sc.metaDB.GetMigrationStatusRecord()
	if err != nil {
		return 0, goerrors.Errorf("get migration status record: %v", err)
	}
	if msr == nil {
		return 0, goerrors.Errorf("migration status record not found")
	}
	if msr.FallForwardEnabled {
		return 2, nil
	}
	return 1, nil
}

func (sc *SegmentCleaner) isFSUtilizationExceeded() bool {
	if sc.isStopRequested() {
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

		importCount, err := sc.getImportCount()
		if err != nil {
			return fmt.Errorf("get import count: %w", err)
		}

		segments, err := sc.metaDB.GetProcessedSegments(importCount)
		if err != nil {
			return goerrors.Errorf("get processed segments: %v", err)
		}

		if sc.isStopRequested() && len(segments) == 0 {
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
		if sc.isStopRequested() {
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

