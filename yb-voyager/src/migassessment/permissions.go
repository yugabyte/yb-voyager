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
	log "github.com/sirupsen/logrus"

	"github.com/fatih/color"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const POSTGRESQL = "postgresql"

// NodePermissionResult tracks permission check results for a single node
type NodePermissionResult struct {
	NodeName         string
	IsPrimary        bool
	MissingPerms     []string
	ConnectionFailed bool  // Explicitly tracks if connection to this node failed
	Error            error // Any error during permission check (can be connection or other errors)
}

// CheckAssessmentPermissionsOnAllNodes verifies that the source database has the required
// permissions for assess-migration command. This is part of the guardrails checks and is
// skipped when --run-guardrails-checks=false.
//
// For PostgreSQL: Checks permissions on the primary and all provided replica nodes (access to
// system catalogs, table SELECT, track_counts, ANALYZE stats, etc.).
//
// For other databases (Oracle, etc.): Checks permissions on the primary database only.
//
// Note: pg_stat_statements availability is intentionally NOT checked here; it is detected
// separately as a mandatory check via DetectPgssAvailabilityOnAllNodes.
func CheckAssessmentPermissionsOnAllNodes(source *srcdb.Source, validatedReplicas []srcdb.ReplicaEndpoint) error {
	if source.DBType != POSTGRESQL {
		return checkPermissionsForNonPostgreSQL(source)
	}
	return checkPermissionsForPostgreSQL(source, validatedReplicas)
}

// DetectPgssAvailabilityOnAllNodes determines pg_stat_statements availability on the
// primary and each replica node. It always runs, regardless of whether guardrails permission
// checks are enabled, because the result drives whether query-level metadata (Unsupported
// Query Constructs) is collected. If it did not run, the gather step would default to "pgss
// disabled" and silently skip Unsupported Query Constructs detection even when the extension
// is fully enabled.
//
// This is a non-blocking detection check: when pg_stat_statements is unavailable it only warns
// (query-level analysis will be limited) and proceeds. It never aborts the run.
//
// The primary node reuses the already-open source connection; each replica uses a short-lived
// connection of its own. Detection failures are treated as non-fatal (the node is recorded as
// pgss-unavailable).
func DetectPgssAvailabilityOnAllNodes(source *srcdb.Source, validatedReplicas []srcdb.ReplicaEndpoint) (map[string]bool, error) {
	// pg_stat_statements is PostgreSQL-specific.
	if source.DBType != POSTGRESQL {
		return nil, nil
	}

	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		return nil, goerrors.Errorf("source database is not PostgreSQL")
	}

	pgssByNode := make(map[string]bool, len(validatedReplicas)+1)
	var nodesWithoutPgss []string

	primaryPgss, err := pg.IsPgStatStatementsAvailable()
	if err != nil {
		log.Warnf("failed to detect pg_stat_statements availability on primary: %v", err)
		primaryPgss = false
	}
	pgssByNode["primary"] = primaryPgss
	if !primaryPgss {
		nodesWithoutPgss = append(nodesWithoutPgss, "primary")
	}

	for _, replica := range validatedReplicas {
		nodeKey := fmt.Sprintf("%s:%d", replica.Host, replica.Port)
		replicaPgss, err := detectPgssOnReplicaNode(source, replica)
		if err != nil {
			log.Warnf("failed to detect pg_stat_statements availability on replica %s: %v", nodeKey, err)
			replicaPgss = false
		}
		pgssByNode[nodeKey] = replicaPgss
		if !replicaPgss {
			nodesWithoutPgss = append(nodesWithoutPgss, nodeKey)
		}
	}

	// Warn the user when pg_stat_statements is unavailable (query-level analysis will be
	// limited). This is informational only; it does not block the run.
	if len(nodesWithoutPgss) > 0 {
		hasMultipleNodes := len(pgssByNode) > 1
		for _, node := range nodesWithoutPgss {
			if hasMultipleNodes {
				utils.PrintAndLogfWarning("\n⚠ pg_stat_statements not available on %s (query-level analysis will be limited)", node)
			} else {
				utils.PrintAndLogfWarning("\n⚠ pg_stat_statements not available (query-level analysis will be limited)")
			}
		}

		// If some nodes have pg_stat_statements and some don't, inform the user.
		if len(nodesWithoutPgss) < len(pgssByNode) {
			utils.PrintAndLogfInfo("\nNote: Query-level analysis (Unsupported Query Constructs) will only include data from nodes with pg_stat_statements.")
		}
	}

	return pgssByNode, nil
}

// detectPgssOnReplicaNode opens a short-lived connection to a replica and reports
// whether pg_stat_statements is available on it.
func detectPgssOnReplicaNode(source *srcdb.Source, replica srcdb.ReplicaEndpoint) (bool, error) {
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

	replicaDB := replicaSource.DB().(*srcdb.PostgreSQL)
	if err := replicaDB.Connect(); err != nil {
		return false, fmt.Errorf("failed to connect: %w", err)
	}
	defer replicaDB.Disconnect()

	return replicaDB.IsPgStatStatementsAvailable()
}

// checkPermissionsForNonPostgreSQL checks permissions for non-PostgreSQL databases (Oracle, etc.)
func checkPermissionsForNonPostgreSQL(source *srcdb.Source) error {
	missingPerms, err := source.DB().GetMissingAssessMigrationPermissions()
	if err != nil {
		return fmt.Errorf("failed to get missing assess migration permissions: %w", err)
	}

	if len(missingPerms) > 0 {
		color.Red("\nPermissions missing in the source database for assess migration:\n")
		output := strings.Join(missingPerms, "\n")
		utils.PrintAndLogf("%s\n\n", output)

		link := "https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/#prepare-the-source-database"
		fmt.Println("Check the documentation to prepare the database for migration:", color.BlueString(link))

		reply := utils.AskPrompt("\nDo you want to continue anyway")
		if !reply {
			return goerrors.Errorf("grant the required permissions and try again")
		}
	}

	return nil
}

// checkPermissionsForPostgreSQL checks permissions on PostgreSQL primary and replica nodes.
func checkPermissionsForPostgreSQL(source *srcdb.Source, validatedReplicas []srcdb.ReplicaEndpoint) error {
	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		return goerrors.Errorf("source database is not PostgreSQL")
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
		return err
	}
	results = append(results, primaryResult)

	// Check each replica
	for _, replica := range validatedReplicas {
		results = append(results, checkPermissionsOnReplicaNode(source, replica))
	}

	return displayPermissionCheckResults(results)
}

// checkPermissionsOnPrimaryNode checks permissions on the primary PostgreSQL node
func checkPermissionsOnPrimaryNode(pg *srcdb.PostgreSQL) (NodePermissionResult, error) {
	missingPerms, err := pg.GetMissingAssessMigrationPermissions()
	if err != nil {
		return NodePermissionResult{}, fmt.Errorf("failed to check permissions on primary: %w", err)
	}
	return NodePermissionResult{
		NodeName:         "primary",
		IsPrimary:        true,
		MissingPerms:     missingPerms,
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

	missingPerms, err := replicaDB.GetMissingAssessMigrationPermissionsForNode(true) // isReplica=true
	replicaDB.Disconnect()

	return NodePermissionResult{
		NodeName:         fmt.Sprintf("%s:%d", replica.Host, replica.Port),
		IsPrimary:        false,
		MissingPerms:     missingPerms,
		ConnectionFailed: false,
		Error:            err,
	}
}

// displayPermissionCheckResults displays the results of permission checks across all nodes
func displayPermissionCheckResults(results []NodePermissionResult) error {
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
		} else {
			// No permission issues - show success
			if hasMultipleNodes {
				utils.PrintAndLogf("\n%s:", displayName)
			}
			utils.PrintAndLogfSuccess("  ✓ All required permissions present")
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

	return nil
}
