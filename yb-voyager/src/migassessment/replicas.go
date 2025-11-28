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
	"errors"
	"fmt"

	"github.com/fatih/color"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// HandleReplicaDiscoveryAndValidation discovers and validates PostgreSQL read replicas.
// Takes the source database and replica endpoints flag as parameters and returns
// the validated replicas to include in assessment.
// Returns nil replicas if no replicas are to be included (single-node assessment) or if source is not PostgreSQL.
func HandleReplicaDiscoveryAndValidation(source *srcdb.Source, replicaEndpointsFlag string) ([]srcdb.ReplicaEndpoint, error) {
	// Only PostgreSQL supports read replica assessment
	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		return nil, nil // No replicas for non-PostgreSQL databases
	}
	// Step 1: Discover replicas from pg_stat_replication
	utils.PrintAndLogf("Checking for read replicas...")
	discoveredReplicas, err := pg.DiscoverReplicas()
	if err != nil {
		return nil, fmt.Errorf("failed to discover replicas: %w", err)
	}

	// Step 2: Parse user-provided replica endpoints if any
	var providedEndpoints []srcdb.ReplicaEndpoint
	if replicaEndpointsFlag != "" {
		providedEndpoints, err = pg.ParseReplicaEndpoints(replicaEndpointsFlag)
		if err != nil {
			return nil, fmt.Errorf("failed to parse replica endpoints: %w", err)
		}
	}

	// Case 1: No replicas discovered and none provided
	if len(discoveredReplicas) == 0 && len(providedEndpoints) == 0 {
		utils.PrintAndLogfInfo("No read replicas detected. Proceeding with single-node assessment.")
		return nil, nil
	}

	// Case 2: Replicas discovered but none provided - best-effort validation
	if len(discoveredReplicas) > 0 && len(providedEndpoints) == 0 {
		return handleDiscoveredReplicasWithoutEndpoints(pg, discoveredReplicas)
	}

	// Case 3 & 4: Endpoints provided (with or without discovery)
	if len(providedEndpoints) > 0 {
		return handleProvidedReplicaEndpoints(pg, discoveredReplicas, providedEndpoints)
	}

	return nil, nil
}

// displayDiscoveredReplicas displays information about discovered replicas
func displayDiscoveredReplicas(replicas []srcdb.ReplicaInfo) {
	for _, replica := range replicas {
		appName := lo.Ternary(replica.ApplicationName == "", "(unnamed)", replica.ApplicationName)
		utils.PrintAndLogf("  - application_name=%s, state=%s, sync_state=%s",
			appName, replica.State, replica.SyncState)
	}
}

// validateDiscoveredReplicas attempts to validate discovered replica addresses by connecting
// to them. Returns three lists: successfully validated replicas, non-replica endpoints, and connection failures.
func validateDiscoveredReplicas(pg *srcdb.PostgreSQL, discoveredReplicas []srcdb.ReplicaInfo) ([]srcdb.ReplicaEndpoint, []string, []string) {
	var connectableReplicas []srcdb.ReplicaEndpoint
	var notReplicaEndpoints []string
	var connectionFailures []string

	for _, replica := range discoveredReplicas {
		if replica.ClientAddr == "" {
			appName := lo.Ternary(replica.ApplicationName == "", "(unnamed)", replica.ApplicationName)
			connectionFailures = append(connectionFailures, fmt.Sprintf("%s (no client_addr)", appName))
			continue
		}

		// Try to connect to client_addr on port 5432
		err := pg.ValidateReplicaConnection(replica.ClientAddr, 5432)
		if err == nil {
			name := lo.Ternary(replica.ApplicationName == "", fmt.Sprintf("%s:5432", replica.ClientAddr), replica.ApplicationName)
			connectableReplicas = append(connectableReplicas, srcdb.ReplicaEndpoint{
				Host:          replica.ClientAddr,
				Port:          5432,
				Name:          name,
				ConnectionUri: pg.GetReplicaConnectionUri(replica.ClientAddr, 5432),
			})
			utils.PrintAndLogfSuccess("  ✓ Successfully connected to %s (%s)", name, replica.ClientAddr)
		} else {
			appName := lo.Ternary(replica.ApplicationName == "", replica.ClientAddr, replica.ApplicationName)
			// Check if this is a "not a replica" error vs connection error.
			// Note: For auto-discovered replicas, "not a replica" errors should rarely occur since
			// the DiscoverReplicas SQL query filters out logical replicas. However, we keep this
			// check as defense-in-depth for edge cases if any. This error path is primarily designed for user-provided endpoints where
			// the user might accidentally specify a logical subscriber or the primary itself.
			if errors.Is(err, srcdb.ErrNotAReplica) {
				notReplicaEndpoints = append(notReplicaEndpoints, fmt.Sprintf("%s (%s)", appName, replica.ClientAddr))
				log.Infof("Endpoint %s (%s) is not a physical replica: %v", appName, replica.ClientAddr, err)
			} else {
				connectionFailures = append(connectionFailures, fmt.Sprintf("%s (%s)", appName, replica.ClientAddr))
				log.Infof("Failed to connect to replica %s (%s): %v", appName, replica.ClientAddr, err)
			}
		}
	}

	return connectableReplicas, notReplicaEndpoints, connectionFailures
}

// handleAllReplicasValidated handles the scenario where all discovered replicas are connectable.
// Prompts the user to include them in the assessment.
// Returns the replicas to include in assessment (nil if user declines).
func handleAllReplicasValidated(connectableReplicas []srcdb.ReplicaEndpoint) ([]srcdb.ReplicaEndpoint, error) {
	utils.PrintAndLogfSuccess("\nSuccessfully validated all %d replica(s) for connection.", len(connectableReplicas))
	if utils.AskPrompt("\nDo you want to include these replicas in this assessment") {
		utils.PrintAndLogfInfo("Proceeding with multi-node assessment using discovered replicas.")
		return connectableReplicas, nil
	}
	utils.PrintAndLogfInfo("Continuing with primary-only assessment.")
	return nil, nil
}

// handlePartialReplicaValidation handles the scenario where some replicas are connectable
// but others are not. Prompts the user to choose between proceeding with partial set,
// providing explicit endpoints, or continuing with primary-only assessment.
// Returns the replicas to include in assessment (nil if user declines or aborts).
func handlePartialReplicaValidation(connectableReplicas []srcdb.ReplicaEndpoint, failedReplicas []string) ([]srcdb.ReplicaEndpoint, error) {
	utils.PrintAndLogfWarning("\nPartial validation result:")
	utils.PrintAndLogfSuccess("  ✓ %d replica(s) successfully validated", len(connectableReplicas))
	utils.PrintAndLogfWarning("  ✗ %d replica(s) could not be validated:", len(failedReplicas))
	for _, failed := range failedReplicas {
		utils.PrintAndLogf("      - %s", failed)
	}
	utils.PrintAndLogfInfo("\nYou can either:")
	utils.PrintAndLogfInfo("  1. Proceed with the %d validated replica(s)", len(connectableReplicas))
	utils.PrintAndLogfInfo("  2. Exit and provide all replica endpoints via --source-read-replica-endpoints")
	utils.PrintAndLogfInfo("  3. Continue with primary-only assessment")

	if utils.AskPrompt(fmt.Sprintf("\nDo you want to proceed with the %d validated replica(s)", len(connectableReplicas))) {
		utils.PrintAndLogfInfo("Proceeding with multi-node assessment using %d validated replica(s).", len(connectableReplicas))
		return connectableReplicas, nil
	}

	if utils.AskPrompt("Do you want to exit and provide replica endpoints") {
		return nil, fmt.Errorf("Aborting, please rerun with --source-read-replica-endpoints flag")
	}

	utils.PrintAndLogfInfo("Continuing with primary-only assessment.")
	return nil, nil
}

// handleNoReplicasValidated handles the scenario where no replicas could be validated.
// Explains the issue and offers options to provide explicit endpoints or continue with primary-only assessment.
// Returns nil replicas if user chooses to continue with primary-only, or error if user aborts.
func handleNoReplicasValidated(notReplicaEndpoints []string, connectionFailures []string) ([]srcdb.ReplicaEndpoint, error) {
	if len(notReplicaEndpoints) > 0 && len(connectionFailures) == 0 {
		// All endpoints are not physical replicas.
		// Note: For auto-discovered replicas, this should rarely happen since DiscoverReplicas()
		// filters out logical replicas via SQL. This path is more relevant for user-provided
		// endpoints where they might accidentally specify logical subscribers or the primary.
		utils.PrintAndLogfWarning("\nThe discovered endpoint(s) are not physical replicas.")
		utils.PrintAndLogfInfo("This typically indicates logical replication connections or non-replica endpoints.")
		utils.PrintAndLogfInfo("Discovered endpoint(s):")
		for _, endpoint := range notReplicaEndpoints {
			utils.PrintAndLogfInfo("  - %s", endpoint)
		}
	} else if len(connectionFailures) > 0 && len(notReplicaEndpoints) == 0 {
		// All discovered endpoints had connection failures
		utils.PrintAndLogfWarning("\nThe addresses shown in pg_stat_replication are not directly connectable from this environment.")
		utils.PrintAndLogfInfo("This is common in cloud environments (RDS, Aurora), Docker, Kubernetes, or proxy setups.")
	} else {
		// Mix of both types of failures
		utils.PrintAndLogfWarning("\nUnable to validate any discovered endpoint as a physical replica.")
		if len(notReplicaEndpoints) > 0 {
			utils.PrintAndLogfInfo("Not physical replicas:")
			for _, endpoint := range notReplicaEndpoints {
				utils.PrintAndLogfInfo("  - %s", endpoint)
			}
		}
		if len(connectionFailures) > 0 {
			utils.PrintAndLogfInfo("Connection failures:")
			for _, endpoint := range connectionFailures {
				utils.PrintAndLogfInfo("  - %s", endpoint)
			}
		}
	}

	utils.PrintAndLogfInfo("\nTo include replicas in the assessment, you can:")
	utils.PrintAndLogfInfo("  1. Exit and rerun with --source-read-replica-endpoints flag")
	utils.PrintAndLogfInfo("  2. Continue with primary-only assessment")

	if utils.AskPrompt("\nDo you want to exit and provide replica endpoints") {
		return nil, fmt.Errorf("Aborting, please rerun with --source-read-replica-endpoints flag")
	}

	utils.PrintAndLogf("Continuing with primary-only assessment.")
	return nil, nil
}

// handleBestEffortValidationOutcome dispatches to the appropriate handler based on
// validation results (all successful, partial success, or all failed).
// Returns the replicas to include in assessment based on user's choice.
func handleBestEffortValidationOutcome(connectableReplicas []srcdb.ReplicaEndpoint, notReplicaEndpoints []string, connectionFailures []string) ([]srcdb.ReplicaEndpoint, error) {
	totalFailures := len(notReplicaEndpoints) + len(connectionFailures)

	// All replicas validated successfully
	if totalFailures == 0 {
		return handleAllReplicasValidated(connectableReplicas)
	}

	// Some replicas validated, some failed
	if len(connectableReplicas) > 0 {
		// Combine failures for display
		var allFailures []string
		for _, endpoint := range notReplicaEndpoints {
			allFailures = append(allFailures, fmt.Sprintf("%s (not a replica)", endpoint))
		}
		for _, endpoint := range connectionFailures {
			allFailures = append(allFailures, fmt.Sprintf("%s (connection failed)", endpoint))
		}
		return handlePartialReplicaValidation(connectableReplicas, allFailures)
	}

	// All validation attempts failed
	return handleNoReplicasValidated(notReplicaEndpoints, connectionFailures)
}

// handleDiscoveredReplicasWithoutEndpoints handles scenarios where replicas are discovered via
// pg_stat_replication but the user did not provide explicit replica endpoints.
//
// This function attempts best-effort validation by trying to connect to the discovered
// client_addr on port 5432. It handles three possible outcomes:
//
// 1. All discovered replicas are connectable - prompts user to include them in assessment
// 2. Some replicas are connectable, others are not - prompts user to proceed with partial set
// 3. No replicas are connectable - explains the issue and offers options to provide endpoints or continue primary-only
//
// Note: The client_addr from pg_stat_replication is often not directly connectable in cloud
// environments (RDS, Aurora), container setups (Docker, K8s), or behind proxies.
//
// Returns the validated replicas to include in assessment based on user's choice.
func handleDiscoveredReplicasWithoutEndpoints(pg *srcdb.PostgreSQL, discoveredReplicas []srcdb.ReplicaInfo) ([]srcdb.ReplicaEndpoint, error) {
	utils.PrintAndLogfInfo("\nDiscovered %d replica(s) via pg_stat_replication:", len(discoveredReplicas))
	displayDiscoveredReplicas(discoveredReplicas)

	utils.PrintAndLogfInfo("\nAttempting to validate discovered replica addresses...")
	connectableReplicas, notReplicaEndpoints, connectionFailures := validateDiscoveredReplicas(pg, discoveredReplicas)

	return handleBestEffortValidationOutcome(connectableReplicas, notReplicaEndpoints, connectionFailures)
}

// enrichReplicaEndpoints matches provided endpoints with discovered replicas
// and uses application_name as the replica name when available.
func enrichReplicaEndpoints(endpoints []srcdb.ReplicaEndpoint, discoveredReplicas []srcdb.ReplicaInfo) []srcdb.ReplicaEndpoint {
	enriched := make([]srcdb.ReplicaEndpoint, len(endpoints))
	copy(enriched, endpoints)

	// Create a map of discovered replicas by client_addr for quick lookup
	replicaByAddr := make(map[string]srcdb.ReplicaInfo)
	for _, replica := range discoveredReplicas {
		if replica.ClientAddr != "" {
			replicaByAddr[replica.ClientAddr] = replica
		}
	}

	// Match each endpoint with discovered replicas by address
	for i := range enriched {
		if replica, found := replicaByAddr[enriched[i].Host]; found {
			if replica.ApplicationName != "" {
				enriched[i].Name = replica.ApplicationName
			}
		}
	}

	return enriched
}

// checkDiscoveryMismatch displays discovery status and warns if the count of discovered
// replicas doesn't match the provided endpoints. Returns error if user aborts.
func checkDiscoveryMismatch(discoveredReplicas []srcdb.ReplicaInfo, providedEndpoints []srcdb.ReplicaEndpoint) error {
	// Early return: No replicas discovered
	if len(discoveredReplicas) == 0 {
		utils.PrintAndLogf("No replicas discovered via pg_stat_replication")
		return nil
	}

	utils.PrintAndLogfInfo("Discovered %d replica(s) via pg_stat_replication", len(discoveredReplicas))

	// Early return: Counts match, no mismatch warning needed
	if len(discoveredReplicas) == len(providedEndpoints) {
		return nil
	}

	// Count mismatch - warn user and get confirmation
	utils.PrintAndLogfWarning("\nWarning: Replica count mismatch detected")
	utils.PrintAndLogfWarning("  Discovered on primary: %d replica(s)", len(discoveredReplicas))
	utils.PrintAndLogfWarning("  Provided via flag:     %d endpoint(s)", len(providedEndpoints))

	if len(discoveredReplicas) > len(providedEndpoints) {
		utils.PrintAndLogfWarning("\nNote: Some replicas discovered on the primary are not included in your endpoint list.")
	} else {
		utils.PrintAndLogfWarning("\nNote: You provided more endpoints than replicas discovered on the primary.")
	}

	utils.PrintAndLogfInfo("\nDiscovered replicas:")
	displayDiscoveredReplicas(discoveredReplicas)

	if !utils.AskPrompt("\nDo you want to proceed with the provided endpoints") {
		return fmt.Errorf("please update --source-read-replica-endpoints to match the topology")
	}

	return nil
}

// validateProvidedEndpoints validates each provided endpoint by connecting and checking
// pg_is_in_recovery(). Returns lists of valid and failed endpoints.
// Note: This is where we commonly catch ErrNotAReplica errors when users accidentally provide
// logical replication subscribers, the primary endpoint, or other non-physical-replica endpoints.
func validateProvidedEndpoints(pg *srcdb.PostgreSQL, endpoints []srcdb.ReplicaEndpoint) ([]srcdb.ReplicaEndpoint, []string) {
	utils.PrintAndLogfInfo("\nValidating %d replica endpoint(s)...", len(endpoints))
	var validEndpoints []srcdb.ReplicaEndpoint
	var failedEndpoints []string

	for _, endpoint := range endpoints {
		err := pg.ValidateReplicaConnection(endpoint.Host, endpoint.Port)
		if err != nil {
			failedEndpoints = append(failedEndpoints, fmt.Sprintf("%s:%d (%s)", endpoint.Host, endpoint.Port, err.Error()))
			log.Errorf("Failed to validate replica %s:%d: %v", endpoint.Host, endpoint.Port, err)
		} else {
			validEndpoints = append(validEndpoints, endpoint)
			utils.PrintAndLogfSuccess("  ✓ Validated replica: %s", endpoint.Name)
		}
	}

	return validEndpoints, failedEndpoints
}

// reportValidationFailures displays validation failures and returns an error.
// Used when user-provided endpoints fail validation - user must fix the configuration.
func reportValidationFailures(failedEndpoints []string) error {
	color.Red("\nFailed to validate the following replica endpoint(s):")
	for _, failed := range failedEndpoints {
		color.Red("  ✗ %s", failed)
	}
	return fmt.Errorf("replica validation failed for %d endpoint(s)", len(failedEndpoints))
}

// handleProvidedReplicaEndpoints handles scenarios where the user provided explicit replica
// endpoints via the --source-read-replica-endpoints flag.
//
// This function handles multiple scenarios:
//
// 1. Endpoints provided and replica count matches discovery - validates all endpoints
// 2. Endpoints provided but count differs from discovery - warns user about mismatch and asks for confirmation
// 3. Endpoints provided but no replicas discovered - proceeds with validation anyway
//
// The function enriches endpoint names with application_name from discovery when possible,
// validates each endpoint by connecting and checking pg_is_in_recovery(), and fails if
// any endpoint is invalid or unreachable.
//
// Returns the validated replica endpoints to include in assessment.
func handleProvidedReplicaEndpoints(pg *srcdb.PostgreSQL, discoveredReplicas []srcdb.ReplicaInfo, providedEndpoints []srcdb.ReplicaEndpoint) ([]srcdb.ReplicaEndpoint, error) {
	// Enrich endpoints with application_name from discovered replicas if possible
	enrichedEndpoints := enrichReplicaEndpoints(providedEndpoints, discoveredReplicas)

	// Check for discovery mismatch and get user confirmation if needed
	if err := checkDiscoveryMismatch(discoveredReplicas, providedEndpoints); err != nil {
		return nil, err
	}

	// Validate all provided endpoints
	validEndpoints, failedEndpoints := validateProvidedEndpoints(pg, enrichedEndpoints)

	// Handle failures - user must fix all endpoints
	if len(failedEndpoints) > 0 {
		return nil, reportValidationFailures(failedEndpoints)
	}

	// All endpoints validated successfully
	utils.PrintAndLogfInfo("\nProceeding with multi-node assessment...")
	return validEndpoints, nil
}
