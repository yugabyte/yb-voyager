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

package migassessment

import (
	"fmt"
	"strings"

	goerrors "github.com/go-errors/errors"

	"github.com/fatih/color"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const POSTGRESQL = "postgresql"

// pgssByNode tracks per-node pg_stat_statements availability.
// Populated by CheckAssessmentPermissionsOnAllNodes, read by GatherAssessmentMetadataFromPG.
// Keys are "primary" for the primary node and "host:port" for replicas.
var pgssByNode map[string]bool

// NodePermissionResult tracks permission check results for a single node
type NodePermissionResult struct {
	NodeName         string
	IsPrimary        bool
	MissingPerms     []string
	PgssEnabled      bool
	ConnectionFailed bool  // Explicitly tracks if connection to this node failed
	Error            error // Any error during permission check (can be connection or other errors)
}

// CheckAssessmentPermissionsOnAllNodes verifies that the source database has the required
// permissions for assess-migration command.
//
// For PostgreSQL: Checks permissions on the primary and all provided replica nodes. This includes
// verifying access to system catalogs, pg_stat_statements extension, and other metadata tables.
// Returns a map keyed by node name ("primary" / "host:port") indicating whether
// pg_stat_statements is available on each node.
//
// For other databases (Oracle, etc.): Checks permissions on the primary database only.
// Returns nil for the map since pg_stat_statements is PostgreSQL-specific.
func CheckAssessmentPermissionsOnAllNodes(source *srcdb.Source, validatedReplicas []srcdb.ReplicaEndpoint) (pgssByNode map[string]bool, err error) {
	if source.DBType != POSTGRESQL {
		return checkPermissionsForNonPostgreSQL(source)
	}
	return checkPermissionsForPostgreSQL(source, validatedReplicas)
}

// checkPermissionsForNonPostgreSQL checks permissions for non-PostgreSQL databases (Oracle, etc.)
// Returns nil for the pgssByNode map since pg_stat_statements is PostgreSQL-specific.
func checkPermissionsForNonPostgreSQL(source *srcdb.Source) (map[string]bool, error) {
	// GetMissingAssessMigrationPermissions returns (missingPerms, pgssEnabled, error)
	// We ignore pgssEnabled since it's always false for non-PostgreSQL databases
	missingPerms, _, err := source.DB().GetMissingAssessMigrationPermissions()
	if err != nil {
		return nil, fmt.Errorf("failed to get missing assess migration permissions: %w", err)
	}

	if len(missingPerms) > 0 {
		color.Red("\nPermissions missing in the source database for assess migration:\n")
		output := strings.Join(missingPerms, "\n")
		utils.PrintAndLogf("%s\n\n", output)

		link := "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-source-database"
		fmt.Println("Check the documentation to prepare the database for migration:", color.BlueString(link))

		reply := utils.AskPrompt("\nDo you want to continue anyway")
		if !reply {
			return nil, goerrors.Errorf("grant the required permissions and try again")
		}
	}

	return nil, nil
}

// checkPermissionsForPostgreSQL checks permissions on PostgreSQL primary and replica nodes.
// Returns a map keyed by node name ("primary" / "host:port") → pgss availability and
// also assigns it to the package-level pgssByNode for use by the gather phase.
func checkPermissionsForPostgreSQL(source *srcdb.Source, validatedReplicas []srcdb.ReplicaEndpoint) (map[string]bool, error) {
	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		return nil, goerrors.Errorf("source database is not PostgreSQL")
	}

	// Print appropriate message based on replica count
	if len(validatedReplicas) == 0 {
		utils.PrintAndLogfInfo("\nChecking permissions on database...")
	} else {
		utils.PrintAndLogfInfo("\nChecking permissions on all nodes (primary + %d replica(s))...", len(validatedReplicas))
	}

	var results []NodePermissionResult

	// Check primary
	primaryResult, err := checkPermissionsOnPrimaryNode(pg)
	if err != nil {
		return nil, err
	}
	results = append(results, primaryResult)

	// Check each replica
	for _, replica := range validatedReplicas {
		results = append(results, checkPermissionsOnReplicaNode(source, replica))
	}

	// Build per-node pgss map from results
	pgssMap := make(map[string]bool, len(results))
	for _, result := range results {
		pgssMap[result.NodeName] = result.PgssEnabled
	}
	pgssByNode = pgssMap

	return pgssMap, displayPermissionCheckResults(results)
}

// checkPermissionsOnPrimaryNode checks permissions on the primary PostgreSQL node
func checkPermissionsOnPrimaryNode(pg *srcdb.PostgreSQL) (NodePermissionResult, error) {
	missingPerms, pgssEnabled, err := pg.GetMissingAssessMigrationPermissions()
	if err != nil {
		return NodePermissionResult{}, fmt.Errorf("failed to check permissions on primary: %w", err)
	}
	return NodePermissionResult{
		NodeName:         "primary",
		IsPrimary:        true,
		MissingPerms:     missingPerms,
		PgssEnabled:      pgssEnabled,
		ConnectionFailed: false,
		Error:            nil,
	}, nil
}

// checkPermissionsOnReplicaNode checks permissions on a replica PostgreSQL node
func checkPermissionsOnReplicaNode(source *srcdb.Source, replica srcdb.ReplicaEndpoint) NodePermissionResult {
	// Create a new Source with replica's host/port
	replicaSource := srcdb.Source{
		DBType:         source.DBType,
		Host:           replica.Host,
		Port:           replica.Port,
		DBName:         source.DBName,
		User:           source.User,
		Password:       source.Password,
		Schemas:        source.Schemas,
		SSLMode:        source.SSLMode,
		SSLCertPath:    source.SSLCertPath,
		SSLKey:         source.SSLKey,
		SSLRootCert:    source.SSLRootCert,
		SSLCRL:         source.SSLCRL,
		NumConnections: source.NumConnections,
	}

	// Create a new PostgreSQL connection for this replica
	replicaDB := replicaSource.DB().(*srcdb.PostgreSQL)

	err := replicaDB.Connect()
	if err != nil {
		return NodePermissionResult{
			NodeName:         fmt.Sprintf("%s:%d", replica.Host, replica.Port),
			IsPrimary:        false,
			ConnectionFailed: true,
			Error:            fmt.Errorf("failed to connect: %w", err),
		}
	}

	missingPerms, pgssEnabled, err := replicaDB.GetMissingAssessMigrationPermissionsForNode(true) // isReplica=true
	replicaDB.Disconnect()

	return NodePermissionResult{
		NodeName:         fmt.Sprintf("%s:%d", replica.Host, replica.Port),
		IsPrimary:        false,
		MissingPerms:     missingPerms,
		PgssEnabled:      pgssEnabled,
		ConnectionFailed: false,
		Error:            err,
	}
}

// displayPermissionCheckResults displays the results of permission checks across all nodes
func displayPermissionCheckResults(results []NodePermissionResult) error {
	var nodesWithoutPgss []string
	var nodesMissingPerms []string

	utils.PrintAndLogfPhase("\n=== Permission Check Results ===\n")

	// Only use "Primary" / "Replica" labels if there are multiple nodes
	hasMultipleNodes := len(results) > 1

	replicaCounter := 1
	for _, result := range results {
		// Format node display name (only for multi-node scenarios)
		var displayName string
		if hasMultipleNodes {
			if result.IsPrimary {
				displayName = "Primary"
			} else {
				displayName = fmt.Sprintf("Replica %d (%s)", replicaCounter, result.NodeName)
				replicaCounter++
			}
		}

		if result.ConnectionFailed {
			if hasMultipleNodes {
				utils.PrintAndLogfError("\n%s:", displayName)
			}
			utils.PrintAndLogfError("  ✗ Connection failed: %v", result.Error)
			continue
		}

		// Handle other errors during permission checks (non-connection errors)
		if result.Error != nil {
			if hasMultipleNodes {
				utils.PrintAndLogfError("\n%s:", displayName)
			}
			utils.PrintAndLogfError("  ✗ Permission check failed: %v", result.Error)
			continue
		}

		if len(result.MissingPerms) > 0 {
			if hasMultipleNodes {
				utils.PrintAndLogf("\n%s:", displayName)
			}
			for _, perm := range result.MissingPerms {
				utils.PrintAndLogfWarning("  ⚠ %s", strings.TrimSpace(perm))
			}
			nodesMissingPerms = append(nodesMissingPerms, result.NodeName)
			// Track if pg_stat_statements is missing (already shown in permissions list above)
			if !result.PgssEnabled {
				nodesWithoutPgss = append(nodesWithoutPgss, result.NodeName)
			}
		} else {
			// No permission issues - show success
			if hasMultipleNodes {
				utils.PrintAndLogf("\n%s:", displayName)
			}
			utils.PrintAndLogfSuccess("  ✓ All required permissions present")

			// Show pg_stat_statements status separately only when there are no other permission issues
			if !result.PgssEnabled {
				utils.PrintAndLogfWarning("  ⚠ pg_stat_statements not available (query-level analysis will be limited)")
				nodesWithoutPgss = append(nodesWithoutPgss, result.NodeName)
			}
		}
	}

	// If any node has permission issues, ask user
	if len(nodesMissingPerms) > 0 {
		utils.PrintAndLogf("\n")
		link := "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-source-database"
		fmt.Println("Check the documentation to prepare the database for migration:", color.BlueString(link))

		reply := utils.AskPrompt("\nDo you want to continue anyway")
		if !reply {
			return goerrors.Errorf("grant the required permissions and try again")
		}
	}

	// If some nodes have pg_stat_statements and some don't, inform user
	if len(nodesWithoutPgss) > 0 && len(nodesWithoutPgss) < len(results) {
		utils.PrintAndLogfInfo("\nNote: Query-level analysis (Unsupported Query Constructs) will only include data from nodes with pg_stat_statements.")
	}

	return nil
}
