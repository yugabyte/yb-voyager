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
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var prepareForFallBack utils.BoolStr
var useYBgRPCConnector utils.BoolStr

var cutoverToTargetCmd = &cobra.Command{
	Use:   "target",
	Short: "Initiate cutover to target DB",
	Long:  `Initiate cutover to target DB`,

	Run: func(cmd *cobra.Command, args []string) {
		var err error
		metaDB, err = metadb.NewMetaDB(exportDir)
		if err != nil {
			utils.ErrExit("Failed to initialize meta db: %s", err)
		}
		msr, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("get migration status record: %v", err)
		}
		if msr == nil {
			utils.ErrExit("migration status record not found")
		}
		if !msr.FallForwardEnabled {
			// --prepare-for-fall-back is mandatory in this case.
			prepareForFallBackSpecified := cmd.Flags().Changed("prepare-for-fall-back")
			if !prepareForFallBackSpecified {
				utils.ErrExit(`missing required flag "--prepare-for-fall-back [yes|no]"`)
			}
		}
		if prepareForFallBack {
			if msr.FallForwardEnabled {
				utils.ErrExit("Live migration with Fall-forward workflow is already started on this export-dir. So --prepare-for-fall-back is not applicable.")
			}
		}
		isFallForwardOrFallBackEnabled := bool(prepareForFallBack) || msr.FallForwardEnabled
		if isFallForwardOrFallBackEnabled {
			if prepareForFallBack {
				log.Infof("Migration workflow opted is live migration with fall-back.")
			} else if msr.FallForwardEnabled {
				log.Infof("Migration workflow opted is live migration with fall-forward.")
			}
			// --use-yb-grpc-connector is mandatory in this case.
			useYBgRPCConnectorSpecified := cmd.Flags().Changed("use-yb-grpc-connector")
			if !useYBgRPCConnectorSpecified {
				utils.ErrExit(`missing required flag "--use-yb-grpc-connector [true|false]"`)
			}
			if useYBgRPCConnector {
				utils.PrintAndLog("Using YB gRPC connector for export data from target")
			} else {
				utils.PrintAndLog("Using YB Logical Replication connector for export data from target")
			}
		} else {
			log.Infof("Migration workflow opted is normal live migration.")
		}
		err = InitiateCutover("target", bool(prepareForFallBack), bool(useYBgRPCConnector))
		if err != nil {
			utils.ErrExit("failed to initiate cutover: %v", err)
		}
	},
}

func init() {
	cutoverToCmd.AddCommand(cutoverToTargetCmd)
	registerExportDirFlag(cutoverToTargetCmd)
	registerConfigFileFlag(cutoverToTargetCmd)
	BoolVar(cutoverToTargetCmd.Flags(), &prepareForFallBack, "prepare-for-fall-back", false,
		"prepare for fallback by streaming changes from target DB back to source DB. Not applicable for fall-forward workflow.")
	//Keeping the default as false here as the default value as false, 0 etc.. is not added to the usage msg by cobra and as the flag is mandatory there is no default value.
	BoolVar(cutoverToTargetCmd.Flags(), &useYBgRPCConnector, "use-yb-grpc-connector", BOOL_FLAG_ZERO_VALUE,
		`Applicable to Fall-forward/fall-back workflows where it is required to export changes from YugabyteDB during 'export data from target'. YugabyteDB provides two types of CDC (Change Data Capture) connectors:
gRPC Connector: Requires direct access to the cluster's internal ports—specifically, TServer (9100) and Master (7100). This connector is suitable for deployments where these ports are accessible.
Logical Connector: It does not require access to internal ports. It is recommended for deployments where the gRPC connector cannot be used—such as with YBAeon or other restricted environments.
If set to true, workflow will use the gRPC connector. Otherwise, the logical connector (supported in YugabyteDB versions 2024.1.1+) is used.`)
	cutoverToCmd.PersistentFlags().StringVarP(&cfgFile, "config-file", "c", "",
		"path of the config file which is used to set the various parameters for yb-voyager commands")
}
