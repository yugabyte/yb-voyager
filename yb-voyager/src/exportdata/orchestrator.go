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
package exportdata

import (
	"context"
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// ---------------------------------------------------------------------------
// Exporter interfaces
// ---------------------------------------------------------------------------

type SnapshotOnlyExporter interface {
	Setup() error
	ExportSnapshot(ctx context.Context) error
	PostProcessing() error
}

type SnapshotAndChangesExporter interface {
	Setup() error
	ExportSnapshot(ctx context.Context) error
	StartExportingChanges(ctx context.Context) error
	PostProcessing() error
	SupportsConcurrentSnapshotAndChanges() bool
}

type ChangesOnlyExporter interface {
	Setup() error
	StartExportingChanges(ctx context.Context) error
	PostProcessing() error
}

// ---------------------------------------------------------------------------
// Orchestrator
// ---------------------------------------------------------------------------

// Run is the main entry point for the new export-data flow.
// All shared setup (connect, schema resolution, table list, etc.) is done
// by the cmd layer before calling Run(). The orchestrator creates the
// appropriate exporter and drives its lifecycle.
func Run(ctx context.Context, ectx *ExportContext) error {
	switch ectx.ExportType {
	case utils.SNAPSHOT_ONLY:
		return runSnapshotOnly(ctx, ectx)
	case utils.SNAPSHOT_AND_CHANGES:
		return runSnapshotAndChanges(ctx, ectx)
	case utils.CHANGES_ONLY:
		return runChangesOnly(ctx, ectx)
	default:
		return fmt.Errorf("unsupported export type: %s", ectx.ExportType)
	}
}

func runSnapshotOnly(ctx context.Context, ectx *ExportContext) error {
	exporter, err := newSnapshotOnlyExporter(ectx)
	if err != nil {
		return fmt.Errorf("create snapshot-only exporter: %w", err)
	}
	if err := exporter.Setup(); err != nil {
		return fmt.Errorf("snapshot-only setup: %w", err)
	}
	if err := exporter.ExportSnapshot(ctx); err != nil {
		return fmt.Errorf("snapshot-only export: %w", err)
	}
	if err := exporter.PostProcessing(); err != nil {
		return fmt.Errorf("snapshot-only post-processing: %w", err)
	}
	return nil
}

func runSnapshotAndChanges(ctx context.Context, ectx *ExportContext) error {
	exporter, err := newSnapshotAndChangesExporter(ectx)
	if err != nil {
		return fmt.Errorf("create snapshot-and-changes exporter: %w", err)
	}
	if err := exporter.Setup(); err != nil {
		return fmt.Errorf("snapshot-and-changes setup: %w", err)
	}
	if err := exporter.ExportSnapshot(ctx); err != nil {
		return fmt.Errorf("snapshot-and-changes snapshot export: %w", err)
	}
	if err := exporter.StartExportingChanges(ctx); err != nil {
		return fmt.Errorf("snapshot-and-changes start exporting changes: %w", err)
	}
	if err := exporter.PostProcessing(); err != nil {
		return fmt.Errorf("snapshot-and-changes post-processing: %w", err)
	}
	return nil
}

func runChangesOnly(ctx context.Context, ectx *ExportContext) error {
	exporter, err := newChangesOnlyExporter(ectx)
	if err != nil {
		return fmt.Errorf("create changes-only exporter: %w", err)
	}
	if err := exporter.Setup(); err != nil {
		return fmt.Errorf("changes-only setup: %w", err)
	}
	if err := exporter.StartExportingChanges(ctx); err != nil {
		return fmt.Errorf("changes-only start exporting changes: %w", err)
	}
	if err := exporter.PostProcessing(); err != nil {
		return fmt.Errorf("changes-only post-processing: %w", err)
	}
	return nil
}
