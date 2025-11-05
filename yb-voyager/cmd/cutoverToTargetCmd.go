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
	"regexp"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

var prepareForFallBack utils.BoolStr
var useYBgRPCConnector utils.BoolStr

// validateYBVersionForLogicalConnector checks if the YugabyteDB version is greater than minSupportedLogicalConnectorVersion¯
// Logical connector is only supported in YugabyteDB versions greater than minSupportedLogicalConnectorVersion
func validateYBVersionForLogicalConnector(tconf *tgtdb.TargetConf) error {
	if tconf == nil {
		return fmt.Errorf("target database configuration is not available")
	}

	// Version should already be stored in metadata from previous import commands
	if tconf.DBVersion == "" {
		return fmt.Errorf("YugabyteDB version not found in metadata.")
	}
	versionStr := tconf.DBVersion

	// Extract YB version from PostgreSQL version string
	// Format: PostgreSQL x.x-YB-version-bxxx or version
	ybVersion, err := extractYBVersion(versionStr)
	if err != nil {
		return fmt.Errorf("failed to extract YugabyteDB version from '%s': %w", versionStr, err)
	}

	// Parse the version
	currentVersion, err := ybversion.NewYBVersion(ybVersion)
	if err != nil {
		return fmt.Errorf("failed to parse current YugabyteDB version '%s': %w.", ybVersion, err)
	}

	var minSupportedLogicalConnectorVersion *ybversion.YBVersion
	// We dont support old format of YugabyteDB stable versions for logical connector i.e. 2.20 etc.
	// So, we only support preview versions >= 2.29.0.0 and stable versions >= 2024.2.4.0
	if currentVersion.ReleaseType() == ybversion.PREVIEW {
		minSupportedLogicalConnectorVersion = ybversion.V2_29_0_0
	} else {
		minSupportedLogicalConnectorVersion = ybversion.V2024_2_4_0
	}

	// Check if version is greater or equal to 2024.1.1.0 or 2.25.0.0
	if !currentVersion.GreaterThanOrEqual(minSupportedLogicalConnectorVersion) {
		return fmt.Errorf("YugabyteDB logical replication connector is only supported in versions greater than or equal to %s. Current version: %s. Please use --use-yb-grpc-connector=true", minSupportedLogicalConnectorVersion.String(), ybVersion)
	}

	return nil
}

// extractYBVersion extracts YugabyteDB version from PostgreSQL server_version string
// Examples: "14.6-YB-2.18.1.0-b89" or "14.6-YB-2024.1.1.0"
func extractYBVersion(versionStr string) (string, error) {
	// Try to match pattern like "14.6-YB-2.18.1.0-b89"
	re := regexp.MustCompile(`YB-([0-9.]+)`)
	match := re.FindStringSubmatch(versionStr)
	if len(match) >= 2 {
		version := match[1]
		// Remove build suffix if present (e.g., "-b89")
		if idx := strings.Index(version, "-"); idx != -1 {
			version = version[:idx]
		}
		return version, nil
	}

	return "", fmt.Errorf("unable to extract YugabyteDB version from PostgreSQL server_version string: %s. Expected format 'PostgreSQL_VERSION-YB-YUGABYTEDB_VERSION'", versionStr)
}

var cutoverToTargetCmd = &cobra.Command{
	Use:   "target",
	Short: "Initiate cutover to target DB",
	Long:  `Initiate cutover to target DB`,

	Run: func(cmd *cobra.Command, args []string) {
		var err error
		metaDB, err = metadb.NewMetaDB(exportDir)
		if err != nil {
			utils.ErrExit("Failed to initialize meta db: %w", err)
		}
		msr, err := metaDB.GetMigrationStatusRecord()
		if err != nil {
			utils.ErrExit("get migration status record: %w", err)
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

			// Validate YugabyteDB version for logical connector
			if !useYBgRPCConnector {
				err = validateYBVersionForLogicalConnector(msr.TargetDBConf)
				if err != nil {
					utils.ErrExit("%w", err)
				}
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
			utils.ErrExit("failed to initiate cutover: %w", err)
		}
	},
}

func init() {
	cutoverToCmd.AddCommand(cutoverToTargetCmd)
	registerExportDirFlag(cutoverToTargetCmd)
	registerConfigFileFlag(cutoverToTargetCmd)
	BoolVar(cutoverToTargetCmd.Flags(), &prepareForFallBack, "prepare-for-fall-back", false,
		"prepare for fallback by streaming changes from target DB back to source DB. Not applicable for fall-forward workflow.")
	BoolVar(cutoverToTargetCmd.Flags(), &useYBgRPCConnector, "use-yb-grpc-connector", false,
		`Applicable to Fall-forward/fall-back workflows where it is required to export changes from YugabyteDB during 'export data from target'. YugabyteDB provides two types of CDC (Change Data Capture) connectors:
gRPC Connector: Requires direct access to the cluster's internal ports—specifically, TServer (9100) and Master (7100). This connector is suitable for deployments where these ports are accessible.
Logical Connector: It does not require access to internal ports. It is recommended for deployments where the gRPC connector cannot be used—such as with YBAeon or other restricted environments.
If set to true, workflow will use the gRPC connector. Otherwise, the logical connector (supported in YugabyteDB versions 2024.1.1+) is used.`)
	cutoverToCmd.PersistentFlags().StringVarP(&cfgFile, "config-file", "c", "",
		"path of the config file which is used to set the various parameters for yb-voyager commands")
}
