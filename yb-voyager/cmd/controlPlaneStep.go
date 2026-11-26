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
	"fmt"

	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp/yugabytedb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// See design-specs/cp-step-claiming.md.

// controlPlaneStepFor is the step registry: the step a command claims, if any.
func controlPlaneStepFor(commandPath, exporterRole, importerRole string, postSnapshotImport bool) (string, bool) {
	switch commandPath {
	case "yb-voyager assess-migration":
		return yugabytedb.STEP_ASSESS_MIGRATION, true
	case "yb-voyager export schema":
		return yugabytedb.STEP_EXPORT_SCHEMA, true
	case "yb-voyager analyze-schema":
		return yugabytedb.STEP_ANALYZE_SCHEMA, true
	case "yb-voyager import schema":
		if postSnapshotImport {
			return yugabytedb.STEP_FINALIZE_SCHEMA, true
		}
		return yugabytedb.STEP_IMPORT_SCHEMA, true
	case "yb-voyager export data", "yb-voyager export data from source":
		return yugabytedb.STEP_EXPORT_DATA_FROM_SOURCE, exporterRole == SOURCE_DB_EXPORTER_ROLE
	case "yb-voyager import data", "yb-voyager import data to target":
		return yugabytedb.STEP_IMPORT_DATA_TO_TARGET, importerRole == TARGET_DB_IMPORTER_ROLE
	case "yb-voyager finalize-schema-post-data-import":
		return yugabytedb.STEP_FINALIZE_SCHEMA, true
	case "yb-voyager end migration":
		return yugabytedb.STEP_END_MIGRATION, true
	}
	return "", false
}

func stepClaimingControlPlane() (*yugabytedb.YugabyteDB, bool) {
	ybdb, ok := controlPlane.(*yugabytedb.YugabyteDB)
	return ybdb, ok && ybdb.StepClaimingEnabled()
}

// claimControlPlaneStep claims the current command's step. It does nothing
// unless the yugabytedb control plane has step claiming on.
func claimControlPlaneStep() error {
	ybdb, ok := stepClaimingControlPlane()
	if !ok {
		return nil
	}
	step, ok := controlPlaneStepFor(currentCommand, exporterRole, importerRole, bool(flagPostSnapshotImport))
	if !ok {
		return nil
	}
	err := ybdb.ClaimStep(migrationUUID, step, func(err error) {
		utils.ErrExit("control plane step lost: %w", err)
	})
	if err != nil {
		return fmt.Errorf("claim step %s: %w", step, err)
	}
	return nil
}

// closeControlPlaneStep closes the held attempt: succeeded when runErr is nil,
// failed otherwise. Safe to call more than once.
func closeControlPlaneStep(runErr error) {
	ybdb, ok := stepClaimingControlPlane()
	if !ok {
		return
	}
	if err := ybdb.CloseStep(runErr); err != nil {
		log.Errorf("Could not close the control plane step attempt: %v", err)
	}
}

// closeControlPlaneStepOnExit is closeControlPlaneStep for the exit handler,
// which runs only when the command did not reach PersistentPostRun.
func closeControlPlaneStepOnExit() {
	runErr := utils.ErrExitErr
	if runErr == nil {
		runErr = fmt.Errorf("command was interrupted before it completed")
	}
	closeControlPlaneStep(runErr)
}
