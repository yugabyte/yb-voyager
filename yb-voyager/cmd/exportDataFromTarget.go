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
	"encoding/json"
	"fmt"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
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
			utils.ErrExit("get migration status record: %v", err)
		}
		if msr.FallbackEnabled {
			exporterRole = TARGET_DB_EXPORTER_FB_ROLE
		} else {
			exporterRole = TARGET_DB_EXPORTER_FF_ROLE
		}
		err = verifySSLFlags()
		if err != nil {
			utils.ErrExit("failed to verify SSL flags: %v", err)
		}
		err = initSourceConfFromTargetConf()
		if err != nil {
			utils.ErrExit("failed to setup source conf from target conf in MSR: %v", err)
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

func verifySSLFlags() error {
	if !lo.Contains([]string{"disable", "require", "verify-ca", "verify-full"}, source.SSLMode) {
		return fmt.Errorf("invalid SSL mode '%s' for 'export data from target'. Please restart 'export data from target' with the --target-ssl-mode flag with one of these modes: 'disable', 'require', 'verify-ca', 'verify-full'", source.SSLMode)
	}
	if (lo.Contains([]string{"require", "verify-ca", "verify-full"}, source.SSLMode)) && source.SSLRootCert == "" {
		return fmt.Errorf("SSL root cert is required for SSL mode '%s'. Please restart 'export data from target' with the --target-ssl-mode and --target-ssl-root-cert flags", source.SSLMode)
	}
	return nil
}

func initSourceConfFromTargetConf() error {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		return fmt.Errorf("get migration status record: %v", err)
	}
	sourceDBConf := msr.SourceDBConf
	targetConf := msr.TargetDBConf
	source.DBType = targetConf.TargetDBType
	source.Host = targetConf.Host
	source.Port = targetConf.Port
	source.User = targetConf.User
	source.DBName = targetConf.DBName
	if sourceDBConf.DBType == POSTGRESQL {
		source.Schema = sourceDBConf.Schema // in case of PG migration the tconf.Schema is public but in case of non-puclic or multiple schemas this needs to PG schemas
	} else {
		source.Schema = targetConf.Schema
	}
	if (targetConf.SSLCertPath != "" || targetConf.SSLKey != "") && source.SSLMode != "disable" {
		if !utils.AskPrompt("Warning: SSL cert and key are not supported for 'export data from target' yet. Do you want to ignore these settings and continue") {
			{
				fmt.Println("Exiting...")
				return fmt.Errorf("SSL cert and key are not supported for 'export data from target' yet")
			}
		}
	}
	source.SSLCRL = targetConf.SSLCRL
	source.SSLQueryString = targetConf.SSLQueryString
	source.Uri = targetConf.Uri
	return nil
}

func packAndSendExportDataFromTargetPayload(status string) {
	if !callhome.SendDiagnostics {
		return
	}
	payload := createCallhomePayload()
	payload.MigrationType = LIVE_MIGRATION

	targetDBDetails := callhome.TargetDBDetails{
		Host:      source.Host,
		DBVersion: source.DBVersion,
	}
	targetDBBytes, err := json.Marshal(targetDBDetails)
	if err != nil {
		log.Errorf("callhome: error parsing targetDB Details: %v", err)
	}
	payload.SourceDBDetails = string(targetDBBytes)

	payload.MigrationPhase = EXPORT_DATA_PHASE
	exportDataPayload := callhome.ExportDataPhasePayload{
		ParallelJobs: int64(source.NumConnections),
		StartClean:   bool(startClean),
	}

	updateExportSnapshotDataStatsInPayload(&exportDataPayload)

	if changeStreamingIsEnabled(exportType) {
		exportDataPayload.ExportDataMechanism = "" //unsetting this as not required
		if cutoverToSourceByExport {
			exportDataPayload.LiveMigrationPhase = CUTOVER_TO_SOURCE
		} else if cutoverToSourceReplicaByExport {
			exportDataPayload.LiveMigrationPhase = CUTOVER_TO_SOURCE_REPLICA
		} else {
			exportDataPayload.LiveMigrationPhase = dbzm.MODE_STREAMING
		}
		exportDataPayload.TotalExportedEvents = callhomeTotalExportEvents
		exportDataPayload.EventsExportRate = callhomeEventsExportRate
	}

	exportDataPayloadBytes, err := json.Marshal(exportDataPayload)
	if err != nil {
		log.Errorf("callhome: error in parsing the export data payload: %v", err)
	}
	payload.PhasePayload = string(exportDataPayloadBytes)
	payload.Status = status

	callhome.SendPayload(&payload)
	callHomePayloadSent = true
}
