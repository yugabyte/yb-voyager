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
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// IsSupported reports whether the new exportdata package can handle the given
// combination of export parameters. The cmd layer uses this to decide whether
// to route to the new code or fall back to the old path.
func IsSupported(exportType, dbType, exporterRole string, useDebezium bool) bool {
	if exportType == utils.SNAPSHOT_ONLY && !useDebezium {
		switch dbType {
		case constants.POSTGRESQL, constants.YUGABYTEDB:
			return true
		}
	}
	return false
}

func newSnapshotOnlyExporter(ectx *ExportContext) (SnapshotOnlyExporter, error) {
	switch {
	case !ectx.UseDebezium && (ectx.Source.DBType == constants.POSTGRESQL || ectx.Source.DBType == constants.YUGABYTEDB):
		return NewPGDumpExporter(ectx), nil
	default:
		return nil, fmt.Errorf(
			"no snapshot-only exporter for dbType=%s useDebezium=%t",
			ectx.Source.DBType, ectx.UseDebezium)
	}
}

func newSnapshotAndChangesExporter(ectx *ExportContext) (SnapshotAndChangesExporter, error) {
	switch ectx.Source.DBType {
	default:
		return nil, fmt.Errorf(
			"no snapshot-and-changes exporter for dbType=%s",
			ectx.Source.DBType)
	}
}

func newChangesOnlyExporter(ectx *ExportContext) (ChangesOnlyExporter, error) {
	switch {
	default:
		return nil, fmt.Errorf(
			"no changes-only exporter for dbType=%s role=%s",
			ectx.Source.DBType, ectx.ExporterRole)
	}
}
