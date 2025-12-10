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

	goerrors "github.com/go-errors/errors"

	"github.com/fatih/color"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// HandleReplicaDiscoveryAndValidation discovers and validates PostgreSQL read replicas.
// Takes the source database, replica endpoints flag, and primary-only flag as parameters
// and returns the validated replicas to include in assessment.
// Returns nil replicas if no replicas are to be included (single-node assessment) or if source is not PostgreSQL.
func HandleReplicaDiscoveryAndValidation(source *srcdb.Source, replicaEndpointsFlag string, primaryOnly bool) ([]srcdb.ReplicaEndpoint, error) {
	// Only PostgreSQL supports read replica assessment
	pg, ok := source.DB().(*srcdb.PostgreSQL)
	if !ok {
		return nil, nil // No replicas for non-PostgreSQL databases
	}

	// If primary-only flag is set, skip replica discovery
	if primaryOnly {
		utils.PrintAndLogfInfo("\nPrimary-only assessment requested. Skipping read replica discovery.")
		return nil, nil
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
		return processDiscoveredReplicasWhenNoEndpointsProvided(pg, discoveredReplicas)
	}

	// Case 3 & 4: Endpoints provided (with or without discovery)
	if len(providedEndpoints) > 0 {
		return processProvidedEndpoints(pg, discoveredReplicas, providedEndpoints)
	}

	return nil, nil
}

// displayDiscoveredReplicas displays information about discovered replicas
func displayDiscoveredReplicas(replicas []srcdb.ReplicaInfo) {
	for _, replica := range replicas {
		endpoint := lo.Ternary(replica.ClientAddr == "", "(no client_addr)", fmt.Sprintf("%s:5432", replica.ClientAddr))
		utils.PrintAndLogf("  - endpoint=%s, state=%s, sync_state=%s",
			endpoint, replica.State, replica.SyncState)
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
			endpoint := lo.Ternary(replica.ApplicationName == "", "(unnamed)", replica.ApplicationName)
			connectionFailures = append(connectionFailures, fmt.Sprintf("%s (no client_addr)", endpoint))
			continue
		}

		// Generate replica name: always use host:port format
		name := fmt.Sprintf("%s:5432", replica.ClientAddr)

		// Try to connect to client_addr on port 5432
		err := pg.ValidateReplicaConnection(replica.ClientAddr, 5432)
		if err == nil {
			connectableReplicas = append(connectableReplicas, srcdb.ReplicaEndpoint{
				Host:          replica.ClientAddr,
				Port:          5432,
				Name:          name,
				ConnectionUri: pg.GetReplicaConnectionUri(replica.ClientAddr, 5432),
			})
			utils.PrintAndLogfSuccess("  ✓ Successfully connected to %s", name)
		} else {
			// Check if this is a "not a replica" error vs connection error.
			// Note: For auto-discovered replicas, "not a replica" errors should rarely occur since
			// the DiscoverReplicas SQL query filters out logical replicas. However, we keep this
			// check as defense-in-depth for edge cases if any. This error path is primarily designed for user-provided endpoints where
			// the user might accidentally specify a logical subscriber or the primary itself.
			if errors.Is(err, srcdb.ErrNotAReplica) {
				notReplicaEndpoints = append(notReplicaEndpoints, name)
				log.Infof("Endpoint %s is not a physical replica: %v", name, err)
			} else {
				connectionFailures = append(connectionFailures, name)
				log.Infof("Failed to connect to replica %s: %v", name, err)
			}
		}
	}

	return connectableReplicas, notReplicaEndpoints, connectionFailures
}

// promptToIncludeAllDiscoveredReplicas handles the scenario where all discovered replicas are connectable.
// Prompts the user to include them in the assessment.
// Returns the replicas to include in assessment. If user declines, exits cleanly with guidance.
func promptToIncludeAllDiscoveredReplicas(connectableReplicas []srcdb.ReplicaEndpoint) ([]srcdb.ReplicaEndpoint, error) {
	utils.PrintAndLogfSuccess("\nSuccessfully validated all %d replica(s) for connection.", len(connectableReplicas))
	if utils.AskPrompt("\nDo you want to include these replicas in this assessment") {
		utils.PrintAndLogfInfo("Proceeding with multi-node assessment using discovered replicas.")
		return connectableReplicas, nil
	}

	// User declined - exit cleanly with guidance (not an error, user's choice)
	utils.PrintAndLogfInfo("\nTo proceed with primary-only assessment, please rerun with --primary-only flag")
	utils.ErrExit("Assessment cancelled by user. Please rerun with the appropriate flag.")
	return nil, nil // unreachable, but required for compilation
}

// promptForPartialDiscoveredReplicas handles the scenario where some discovered replicas are connectable
// but others are not. Prompts the user to proceed with validated replicas or exit.
// Returns the replicas to include in assessment. If user declines, exits cleanly with guidance.
func promptForPartialDiscoveredReplicas(connectableReplicas []srcdb.ReplicaEndpoint, failedReplicas []string) ([]srcdb.ReplicaEndpoint, error) {
	utils.PrintAndLogfWarning("\nPartial validation result:")
	utils.PrintAndLogfSuccess("  ✓ %d replica(s) successfully validated", len(connectableReplicas))
	utils.PrintAndLogfWarning("  ✗ %d replica(s) could not be validated:", len(failedReplicas))
	for _, failed := range failedReplicas {
		utils.PrintAndLogf("      - %s", failed)
	}

	if utils.AskPrompt(fmt.Sprintf("\nDo you want to proceed with the %d validated replica(s)", len(connectableReplicas))) {
		utils.PrintAndLogfInfo("Proceeding with multi-node assessment using %d validated replica(s).", len(connectableReplicas))
		return connectableReplicas, nil
	}

	// User declined - exit cleanly with guidance (not an error, user's choice)
	utils.PrintAndLogfInfo("\nTo proceed, you can either:")
	utils.PrintAndLogfInfo("  1. Rerun with --primary-only for primary-only assessment")
	utils.PrintAndLogfInfo("  2. Rerun with --source-read-replica-endpoints to specify all replica endpoints")
	utils.ErrExit("Assessment cancelled by user. Please rerun with appropriate flags.")
	return nil, nil
}

// promptWhenNoDiscoveredReplicasConnectable handles the scenario where no discovered replicas could be validated.
// Explains the issue and exits cleanly with guidance to use appropriate flags.
// Exits the program (does not return an error since it's not an error condition).
func promptWhenNoDiscoveredReplicasConnectable(notReplicaEndpoints []string, connectionFailures []string) ([]srcdb.ReplicaEndpoint, error) {
	if len(notReplicaEndpoints) > 0 && len(connectionFailures) == 0 {
		// All endpoints are not physical replicas.
		// Note: For auto-discovered replicas, this should rarely happen since DiscoverReplicas()
		// filters out logical replicas via SQL. This code is just added for completeness and defense-in-depth.
		utils.PrintAndLogfWarning("\nThe discovered endpoint(s) are not physical replicas.")
		utils.PrintAndLogfInfo("This typically indicates logical replication connections or non-replica endpoints.")
		utils.PrintAndLogfInfo("Discovered endpoint(s):")
		for _, endpoint := range notReplicaEndpoints {
			utils.PrintAndLogfInfo("  - %s", endpoint)
		}
	} else if len(connectionFailures) > 0 && len(notReplicaEndpoints) == 0 {
		// All discovered endpoints had connection failures
		utils.PrintAndLogfWarning("\nIt is possible that the addresses shown in pg_stat_replication are not directly connectable from this environment.\nThis is common in cloud environments (RDS, Aurora), Docker, Kubernetes, or proxy setups.")
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

	utils.PrintAndLogfInfo("\nTo proceed, you can either:")
	utils.PrintAndLogfInfo("  1. Rerun with --primary-only for primary-only assessment")
	utils.PrintAndLogfInfo("  2. Rerun with --source-read-replica-endpoints to specify replica endpoints manually")

	utils.ErrExit("No replicas could be validated. Please rerun with appropriate flags.")
	return nil, nil
}

// resolveDiscoveredReplicasOutcome dispatches to the appropriate prompt based on
// validation results (all successful, partial success, or all failed).
// Returns the replicas to include in assessment based on user's choice.
func resolveDiscoveredReplicasOutcome(connectableReplicas []srcdb.ReplicaEndpoint, notReplicaEndpoints []string, connectionFailures []string) ([]srcdb.ReplicaEndpoint, error) {
	totalFailures := len(notReplicaEndpoints) + len(connectionFailures)

	// All replicas validated successfully
	if totalFailures == 0 {
		return promptToIncludeAllDiscoveredReplicas(connectableReplicas)
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
		return promptForPartialDiscoveredReplicas(connectableReplicas, allFailures)
	}

	// All validation attempts failed
	return promptWhenNoDiscoveredReplicasConnectable(notReplicaEndpoints, connectionFailures)
}

// processDiscoveredReplicasWhenNoEndpointsProvided handles scenarios where replicas are discovered via
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
func processDiscoveredReplicasWhenNoEndpointsProvided(pg *srcdb.PostgreSQL, discoveredReplicas []srcdb.ReplicaInfo) ([]srcdb.ReplicaEndpoint, error) {
	utils.PrintAndLogfInfo("\nDiscovered %d replica(s) via pg_stat_replication:", len(discoveredReplicas))
	displayDiscoveredReplicas(discoveredReplicas)

	utils.PrintAndLogfInfo("\nAttempting to validate discovered replica addresses...")
	connectableReplicas, notReplicaEndpoints, connectionFailures := validateDiscoveredReplicas(pg, discoveredReplicas)

	return resolveDiscoveredReplicasOutcome(connectableReplicas, notReplicaEndpoints, connectionFailures)
}

// warnIfProvidedEndpointsMismatch displays discovery status and warns if the count of discovered
// replicas doesn't match the provided endpoints. Returns error if user aborts.
func warnIfProvidedEndpointsMismatch(discoveredReplicas []srcdb.ReplicaInfo, providedEndpoints []srcdb.ReplicaEndpoint) error {
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
		return goerrors.Errorf("please update --source-read-replica-endpoints to match the topology")
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
			utils.PrintAndLogfSuccess("  ✓ Validated replica: %s (%s:%d)", endpoint.Name, endpoint.Host, endpoint.Port)
		}
	}

	return validEndpoints, failedEndpoints
}

// reportProvidedEndpointsValidationFailures displays validation failures and returns an error.
// Used when user-provided endpoints fail validation - user must fix the configuration.
func reportProvidedEndpointsValidationFailures(failedEndpoints []string) error {
	color.Red("\nFailed to validate the following replica endpoint(s):")
	for _, failed := range failedEndpoints {
		color.Red("  ✗ %s", failed)
	}
	return goerrors.Errorf("replica validation failed for %d endpoint(s)", len(failedEndpoints))
}

// processProvidedEndpoints handles scenarios where the user provided explicit replica
// endpoints via the --source-read-replica-endpoints flag.
//
// This function handles multiple scenarios:
//
// 1. Endpoints provided and replica count matches discovery - validates all endpoints
// 2. Endpoints provided but count differs from discovery - warns user about mismatch and asks for confirmation
// 3. Endpoints provided but no replicas discovered - proceeds with validation anyway
//
// Validates each endpoint by connecting and checking pg_is_in_recovery(), and fails if
// any endpoint is invalid or unreachable.
//
// Returns the validated replica endpoints to include in assessment.
func processProvidedEndpoints(pg *srcdb.PostgreSQL, discoveredReplicas []srcdb.ReplicaInfo, providedEndpoints []srcdb.ReplicaEndpoint) ([]srcdb.ReplicaEndpoint, error) {
	// Check for discovery mismatch and get user confirmation if needed
	if err := warnIfProvidedEndpointsMismatch(discoveredReplicas, providedEndpoints); err != nil {
		return nil, err
	}

	// Validate all provided endpoints
	validEndpoints, failedEndpoints := validateProvidedEndpoints(pg, providedEndpoints)

	// Handle failures - user must fix all endpoints
	if len(failedEndpoints) > 0 {
		return nil, reportProvidedEndpointsValidationFailures(failedEndpoints)
	}

	// All endpoints validated successfully
	utils.PrintAndLogfInfo("\nProceeding with multi-node assessment...")
	return validEndpoints, nil
}
