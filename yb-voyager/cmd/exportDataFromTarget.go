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

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var transactionOrdering utils.BoolStr

var exportDataFromTargetCmd = &cobra.Command{
	Use:   "target",
	Short: "Export data from target Yugabyte DB in the fall-back/fall-forward workflows.",
	Long:  ``,

	Run: func(cmd *cobra.Command, args []string) {
		validateMetaDBCreated()
		source.DBType = YUGABYTEDB
		exportType = CHANGES_ONLY
		msr, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("get migration status record: %w", err)
		}
		if msr.FallbackEnabled {
			exporterRole = TARGET_DB_EXPORTER_FB_ROLE
		} else {
			exporterRole = TARGET_DB_EXPORTER_FF_ROLE
		}
		err = verifySSLFlags(cmd, msr)
		if err != nil {
			utils.ErrExit("failed to verify SSL flags: %w", err)
		}
		if source.SSLRootCert != "" {
			err = metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
				record.TargetDBConf.SSLRootCert = source.SSLRootCert
			})
			if err != nil {
				utils.ErrExit("error updating migration status record: %w", err)
			}
		}
		err = initSourceConfFromTargetConf(cmd)
		if err != nil {
			utils.ErrExit("failed to setup source conf from target conf in MSR: %w", err)
		}

		exportDataCmd.PreRun(cmd, args)
		exportDataCmd.Run(cmd, args)
	},
}

func init() {
	exportDataFromCmd.AddCommand(exportDataFromTargetCmd)
	registerCommonGlobalFlags(exportDataFromTargetCmd)
	registerTargetDBAsSourceConnFlags(exportDataFromTargetCmd)
	registerExportDataFlags(exportDataFromTargetCmd)
	hideExportFlagsInFallForwardOrBackCmds(exportDataFromTargetCmd)

	BoolVar(exportDataFromTargetCmd.Flags(), &transactionOrdering, "transaction-ordering", true,
		"Setting the flag to `false` disables the transaction ordering. This speeds up change data capture from target YugabyteDB. Disable transaction ordering only if the tables under migration do not have unique keys or the app does not modify/reuse the unique keys.")
}

/*
For GRPC connector, only limited SSL modes are supported (disable, require, verify-ca, verify-full).
If import-data had used the prefer/allow SSL modes, then, it will be downgraded to disable mode.
Therefore, we allow users to override the ssl mode in export-data-from-target.

Additinally, SSL root cert is required for require, verify-ca, and verify-full modes. (because of having to use the yb-admin client)
Therefore, we allow users to override the ssl root cert in export-data-from-target.

For logical-replication connecter, all SSL modes are supported.
*/
func verifySSLFlags(cmd *cobra.Command, msr *metadb.MigrationStatusRecord) error {
	allowedSSLModes := ValidSSLModesForSourceDB[YUGABYTEDB]
	// the debezium GRPC connector has some limitations because of which prefer and allow are not supported.
	allowedSSLModesGRPCConnector := []string{constants.DISABLE, constants.REQUIRE, constants.VERIFY_CA, constants.VERIFY_FULL}

	if msr.UseYBgRPCConnector {
		if !lo.Contains(allowedSSLModesGRPCConnector, source.SSLMode) {
			return goerrors.Errorf("invalid SSL mode '%s' for 'export data from target'. Please restart 'export data from target' with the --target-ssl-mode flag with one of these modes: %v", source.SSLMode, allowedSSLModesGRPCConnector)
		}
		if (lo.Contains([]string{constants.REQUIRE, constants.VERIFY_CA, constants.VERIFY_FULL}, source.SSLMode)) && source.SSLRootCert == "" {
			return goerrors.Errorf("SSL root cert is required for SSL mode '%s'. Please restart 'export data from target' with the --target-ssl-mode and --target-ssl-root-cert flags", source.SSLMode)
		}
	} else {
		if !lo.Contains(allowedSSLModes, source.SSLMode) {
			return goerrors.Errorf("invalid SSL mode '%s' for 'export data from target'. Please restart 'export data from target' with the --target-ssl-mode flag with one of these modes: %v", source.SSLMode, allowedSSLModes)
		}
	}

	return nil
}

func initSourceConfFromTargetConf(cmd *cobra.Command) error {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return goerrors.Errorf("get migration status record: %v", err)
	}
	sourceDBConf := msr.SourceDBConf
	targetConf := msr.TargetDBConf
	source.DBType = targetConf.TargetDBType
	source.Host = targetConf.Host
	source.Port = targetConf.Port
	source.User = targetConf.User
	source.DBName = targetConf.DBName

	if sourceDBConf.DBType == POSTGRESQL {
		source.Schemas = sourceDBConf.Schemas // in case of PG migration the tconf.Schema is public but in case of non-puclic or multiple schemas this needs to PG schemas
	} else {
		source.Schemas = targetConf.Schemas
	}

	if msr.UseYBgRPCConnector {
		// ssl-mode and ssl-root-cert as passed via CLI to export-data-from-target.
		// ssl-key and ssl-cert are not supported by export-data-from-target, so they are ignored.
		if (targetConf.SSLCertPath != "" || targetConf.SSLKey != "") && source.SSLMode != "disable" {
			if !utils.AskPrompt("Warning: SSL cert and key are not supported for 'export data from target' yet. Do you want to ignore these settings and continue") {
				{
					fmt.Println("Exiting...")
					return goerrors.Errorf("SSL cert and key are not supported for 'export data from target' yet")
				}
			}
		}
	} else {
		// no limitations when using logical replication connector. All values are read from target-db-conf.
		// by default, unless overriden by the user.
		// target-ssl-mode and target-ssl-root-cert are only available in the CLI because
		// of limitations of grpc connector. In future, these flags will be removed.
		if !cmd.Flags().Changed("target-ssl-mode") {
			source.SSLMode = targetConf.SSLMode
		}
		if !cmd.Flags().Changed("target-ssl-root-cert") {
			source.SSLRootCert = targetConf.SSLRootCert
		}
		source.SSLKey = targetConf.SSLKey
		source.SSLCertPath = targetConf.SSLCertPath
	}

	source.SSLCRL = targetConf.SSLCRL
	source.SSLQueryString = targetConf.SSLQueryString
	source.Uri = targetConf.Uri
	return nil
}

func packAndSendExportDataFromTargetPayload(status string, errorMsg error) {
	if !shouldSendCallhome() {
		return
	}
	payload := createCallhomePayload()
	payload.MigrationType = LIVE_MIGRATION

	targetDBDetails := callhome.TargetDBDetails{
		DBVersion: source.DBVersion,
	}
	payload.TargetDBDetails = callhome.MarshalledJsonString(targetDBDetails)

	payload.MigrationPhase = EXPORT_DATA_FROM_TARGET_PHASE
	exportDataPayload := callhome.ExportDataPhasePayload{
		PayloadVersion:   callhome.EXPORT_DATA_CALLHOME_PAYLOAD_VERSION,
		ParallelJobs:     int64(source.NumConnections),
		StartClean:       bool(startClean),
		Error:            callhome.SanitizeErrorMsg(errorMsg, anonymizer),
		ControlPlaneType: getControlPlaneType(),
	}

	exportDataPayload.Phase = exportPhase
	if exportPhase != dbzm.MODE_SNAPSHOT {
		exportDataPayload.TotalExportedEvents = totalEventCount
		exportDataPayload.EventsExportRate = throughputInLast3Min
	}
	switch exporterRole {
	case TARGET_DB_EXPORTER_FF_ROLE:
		exportDataPayload.LiveWorkflowType = FALL_FORWARD
	case TARGET_DB_EXPORTER_FB_ROLE:
		exportDataPayload.LiveWorkflowType = FALL_BACK
	}

	// Add cutover timings if applicable
	msr, err := metaDB.GetMigrationStatusRecord()
	if err == nil {
		exportDataPayload.CutoverTimings = CalculateCutoverTimingsForTarget(msr)
	} else {
		log.Infof("callhome: error getting MSR for cutover timings: %v", err)
	}

	payload.PhasePayload = callhome.MarshalledJsonString(exportDataPayload)
	payload.Status = status

	err = callhome.SendPayload(&payload)
	if err == nil && (status == COMPLETE || status == ERROR) {
		callHomeErrorOrCompletePayloadSent = true
	}

}
