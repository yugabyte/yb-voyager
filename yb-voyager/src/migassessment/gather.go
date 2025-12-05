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
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"github.com/fatih/color"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	ORACLE = "oracle"
)

// NodeProgress tracks the progress of metadata collection for a single node
type NodeProgress struct {
	NodeName string // Unique identifier for tracking (e.g., "primary", "host-5432")
	Stage    string // Current stage (e.g., "Collecting table row counts...", "Complete", "Failed")
}

// collectionNode represents a database node (primary or replica) that metadata will be collected from.
// It contains all the information needed to run the collection script for that node.
type collectionNode struct {
	nodeName      string                 // Filesystem-safe unique identifier (used for subdirectory names)
	connectionUri string                 // Database connection string
	isPrimary     bool                   // Whether this is the primary node (affects script behavior)
	replica       *srcdb.ReplicaEndpoint // Replica info (nil for primary, contains raw data for display name extraction)
}

// collectionResult tracks the result of metadata collection from a single node
type collectionResult struct {
	nodeName  string // Node identifier (used to lookup display name from tracker)
	isPrimary bool   // Determines if failure is critical (primary) or warning (replica)
	err       error  // nil = success, non-nil = failure
}

// progressTracker manages the display of progress for parallel metadata collection
type progressTracker struct {
	nodes        []string          // Ordered list of node names for consistent display
	displayNames map[string]string // nodeName -> displayName
	statuses     map[string]*NodeProgress
	mutex        sync.Mutex
	initialized  bool // Whether initial lines have been printed
	maxNameLen   int  // Maximum length of display names for alignment
}

func newProgressTracker(nodes []collectionNode) *progressTracker {
	tracker := &progressTracker{
		nodes:        make([]string, 0, len(nodes)),
		displayNames: make(map[string]string),
		statuses:     make(map[string]*NodeProgress),
		initialized:  false,
		maxNameLen:   0,
	}

	// Truncate long display names and calculate maximum length for alignment
	const maxDisplayNameLen = 80
	replicaCount := 0

	for _, node := range nodes {
		tracker.nodes = append(tracker.nodes, node.nodeName)

		// Build display name from node info
		var displayName string
		if node.isPrimary {
			displayName = "Primary"
		} else {
			replicaCount++
			// Extract name from replica object
			displayName = fmt.Sprintf("Replica %d (%s)", replicaCount, node.replica.Name)
		}

		// Truncate display name if too long (prevents line wrapping in terminal)
		if len(displayName) > maxDisplayNameLen {
			displayName = displayName[:maxDisplayNameLen-3] + "..."
		}
		tracker.displayNames[node.nodeName] = displayName

		if len(displayName) > tracker.maxNameLen {
			tracker.maxNameLen = len(displayName)
		}

		// Initialize with pending stage
		tracker.statuses[node.nodeName] = &NodeProgress{
			NodeName: node.nodeName,
			Stage:    "Pending...",
		}
	}

	return tracker
}

func (pt *progressTracker) update(progress NodeProgress) {
	pt.mutex.Lock()
	defer pt.mutex.Unlock()

	// Update only the stage, preserve the formatted & truncated display name
	if existing := pt.statuses[progress.NodeName]; existing != nil {
		existing.Stage = progress.Stage
	}

	// Print/update display
	pt.printAll()
}

func (pt *progressTracker) printAll() {
	if !pt.initialized {
		// First time: print all lines
		fmt.Println() // Blank line
		for _, nodeName := range pt.nodes {
			status := pt.statuses[nodeName]
			pt.printSingleLine(*status)
		}
		pt.initialized = true
	} else {
		// Move cursor up to the first line and reprint all
		// Move up by number of nodes
		fmt.Printf("\033[%dA", len(pt.nodes))

		// Reprint all lines
		for _, nodeName := range pt.nodes {
			status := pt.statuses[nodeName]
			pt.printSingleLine(*status)
		}
	}
}

func (pt *progressTracker) printSingleLine(progress NodeProgress) {
	// Determine icon based on stage
	var statusIcon string
	switch progress.Stage {
	case "Complete":
		statusIcon = "✓"
	case "Failed":
		statusIcon = "⚠"
	default:
		statusIcon = "⏳"
	}

	// Look up display name from the map (already truncated in newProgressTracker)
	displayName := pt.displayNames[progress.NodeName]

	// Clear line and print with fixed-width column for name (for alignment)
	// \r returns to start, \033[K clears to end of line
	// Using %-*s for left-alignment with dynamic width based on longest (truncated) name
	fmt.Printf("\r\033[K  %s %-*s %s\n", statusIcon, pt.maxNameLen+1, displayName+":", progress.Stage)
}

// GatherAssessmentMetadataFromPG collects metadata from PostgreSQL primary and replicas.
// Accepts the validated replicas to collect from, and returns the list of failed replica names
// (for reporting partial multi-node assessment).
// Collection is performed in parallel for better performance.
func GatherAssessmentMetadataFromPG(
	source *srcdb.Source,
	validatedReplicas []srcdb.ReplicaEndpoint,
	assessmentMetadataDir string,
	pgssEnabled bool,
	iopsInterval int64,
) (failedReplicaNodes []string, err error) {
	scriptPath, err := findGatherMetadataScriptPath(POSTGRESQL)
	if err != nil {
		return nil, err
	}

	// Build list of all nodes to collect from (primary + replicas)
	nodes := []collectionNode{
		{
			nodeName:      "primary",
			connectionUri: source.DB().GetConnectionUriWithoutPassword(),
			isPrimary:     true,
			replica:       nil, // nil for primary
		},
	}

	for i := range validatedReplicas {
		uniqueNodeName := fmt.Sprintf("%s-%d", validatedReplicas[i].Host, validatedReplicas[i].Port)
		nodes = append(nodes, collectionNode{
			nodeName:      uniqueNodeName,
			connectionUri: validatedReplicas[i].ConnectionUri,
			isPrimary:     false,
			replica:       &validatedReplicas[i], // Pass entire replica object
		})
	}

	totalNodes := len(nodes)
	if totalNodes == 1 {
		utils.PrintAndLogfInfo("\nCollecting metadata from 1 node...")
	} else {
		utils.PrintAndLogfInfo("\nCollecting metadata from %d nodes in parallel...", totalNodes)
	}

	// Initialize progress tracker
	tracker := newProgressTracker(nodes)

	// Channel for progress updates
	progressChan := make(chan NodeProgress, totalNodes*10) // Buffer for multiple updates per node

	// Channel to signal completion of progress display goroutine
	displayDone := make(chan struct{})

	// Start progress display goroutine
	go func() {
		defer close(displayDone)
		for progress := range progressChan {
			tracker.update(progress)
		}
	}()

	// WaitGroup for parallel collection
	var wg sync.WaitGroup

	// Channel for collection results
	resultChan := make(chan collectionResult, totalNodes)

	// Start parallel collection for all nodes
	for _, node := range nodes {
		wg.Add(1)
		go func(n collectionNode) {
			defer wg.Done()

			// Run collection with buffered output
			err := runGatherAssessmentMetadataScriptBuffered(
				scriptPath,
				[]string{fmt.Sprintf("PGPASSWORD=%s", source.Password), "PGCONNECT_TIMEOUT=10"},
				n.nodeName,
				progressChan,
				assessmentMetadataDir,
				n.connectionUri,
				source.Schema,
				assessmentMetadataDir,
				fmt.Sprintf("%t", pgssEnabled),
				fmt.Sprintf("%d", iopsInterval),
				"true",     // --yes (doesn't matter since skip_checks=true)
				n.nodeName, // source_node_name
				"true",     // skip_checks - guardrails already validated
			)

			// Send result
			resultChan <- collectionResult{
				nodeName:  n.nodeName,
				isPrimary: n.isPrimary,
				err:       err,
			}
		}(node)
	}

	// Wait for all collection goroutines to complete
	wg.Wait()
	close(progressChan)
	close(resultChan)

	// Wait for display goroutine to finish (it closes when progressChan is closed and drained)
	<-displayDone

	// Process results
	var failedReplicasList []string
	var successfulReplicaCount int
	var primaryFailed bool
	var primaryErr error

	for result := range resultChan {
		displayName := tracker.displayNames[result.nodeName]
		if result.err == nil {
			log.Infof("Successfully collected metadata from %s", displayName)
			if !result.isPrimary {
				successfulReplicaCount++
			}
		} else {
			if result.isPrimary {
				primaryFailed = true
				primaryErr = result.err
			} else {
				log.Warnf("Failed to collect metadata from replica %s: %v", displayName, result.err)
				failedReplicasList = append(failedReplicasList, displayName)
			}
		}
	}

	// If primary failed, return error immediately
	if primaryFailed {
		return nil, fmt.Errorf("metadata collection failed on primary database (critical): %w", primaryErr)
	}

	// Print summary
	if !primaryFailed && len(failedReplicasList) > 0 {
		fmt.Println() // Blank line before warnings
		color.Yellow("WARNING: Metadata collection failed on %d replica(s): [%s]", len(failedReplicasList), strings.Join(failedReplicasList, ", "))
		utils.PrintAndLogfInfo("Continuing assessment with data from primary + %d successful replica(s)", successfulReplicaCount)
		utils.PrintAndLogfWarning("Note: Sizing and metrics will reflect only the nodes that succeeded.")
	}

	fmt.Println() // Single blank line before final success message
	if len(validatedReplicas) == 0 {
		utils.PrintAndLogfSuccess("Successfully completed metadata collection from primary node")
	} else {
		utils.PrintAndLogfSuccess("Successfully completed metadata collection from %d node(s) (primary + %d replica(s))",
			1+successfulReplicaCount, successfulReplicaCount)
	}

	return failedReplicasList, nil
}

// GatherAssessmentMetadataFromOracle collects metadata from Oracle database.
func GatherAssessmentMetadataFromOracle(
	source *srcdb.Source,
	assessmentMetadataDir string,
) error {
	scriptPath, err := findGatherMetadataScriptPath(ORACLE)
	if err != nil {
		return err
	}

	tnsAdmin, err := source.GetTNSAdmin()
	if err != nil {
		return fmt.Errorf("error getting tnsAdmin: %w", err)
	}
	envVars := []string{fmt.Sprintf("ORACLE_PASSWORD=%s", source.Password),
		fmt.Sprintf("TNS_ADMIN=%s", tnsAdmin),
		fmt.Sprintf("ORACLE_HOME=%s", source.GetOracleHome()),
	}
	log.Infof("environment variables passed to oracle gather metadata script: %v", envVars)
	return runGatherAssessmentMetadataScript(scriptPath, envVars, assessmentMetadataDir,
		source.DB().GetConnectionUriWithoutPassword(), strings.ToUpper(source.Schema), assessmentMetadataDir)
}

func findGatherMetadataScriptPath(dbType string) (string, error) {
	var defaultScriptPath string
	switch dbType {
	case POSTGRESQL:
		defaultScriptPath = "/etc/yb-voyager/gather-assessment-metadata/postgresql/yb-voyager-pg-gather-assessment-metadata.sh"
	case ORACLE:
		defaultScriptPath = "/etc/yb-voyager/gather-assessment-metadata/oracle/yb-voyager-oracle-gather-assessment-metadata.sh"
	default:
		panic(fmt.Sprintf("invalid source db type %q", dbType))
	}

	homebrewVoyagerDir := fmt.Sprintf("yb-voyager@%s", utils.YB_VOYAGER_VERSION)
	possiblePathsForScript := []string{
		defaultScriptPath,
		filepath.Join("/", "opt", "homebrew", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, defaultScriptPath),
		filepath.Join("/", "usr", "local", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, defaultScriptPath),
	}

	for _, path := range possiblePathsForScript {
		if utils.FileOrFolderExists(path) {
			log.Infof("found the gather assessment metadata script at: %s", path)
			return path, nil
		}
	}

	return "", fmt.Errorf("script not found in possible paths: %v", possiblePathsForScript)
}

func runGatherAssessmentMetadataScript(scriptPath string, envVars []string, workingDir string, scriptArgs ...string) error {
	cmd := exec.Command(scriptPath, scriptArgs...)
	log.Infof("running script: %s", cmd.String())
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, envVars...)
	cmd.Dir = workingDir
	cmd.Stdin = os.Stdin

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting gather assessment metadata script: %w", err)
	}

	go func() {
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			log.Errorf("[stderr of script]: %s", scanner.Text())
			fmt.Printf("%s\n", scanner.Text())
		}
	}()

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		log.Infof("[stdout of script]: %s", scanner.Text())
		fmt.Printf("%s\n", scanner.Text())
	}

	err = cmd.Wait()
	if err != nil {
		if exiterr, ok := err.(*exec.ExitError); ok {
			if status, ok := exiterr.Sys().(syscall.WaitStatus); ok {
				if status.ExitStatus() == 2 {
					log.Infof("Exit without error as user opted not to continue in the script.")
					os.Exit(0)
				}
			}
		}
		return fmt.Errorf("error waiting for gather assessment metadata script to complete: %w", err)
	}
	return nil
}

// runGatherAssessmentMetadataScriptBuffered runs the metadata collection script with buffered output
// and sends progress updates to the provided channel. This version is safe for parallel execution.
func runGatherAssessmentMetadataScriptBuffered(
	scriptPath string,
	envVars []string,
	nodeName string,
	progressChan chan<- NodeProgress,
	workingDir string,
	scriptArgs ...string,
) error {
	cmd := exec.Command(scriptPath, scriptArgs...)
	log.Infof("[%s] running script: %s", nodeName, cmd.String())
	cmd.Env = os.Environ()
	cmd.Env = append(cmd.Env, envVars...)
	cmd.Dir = workingDir
	// Don't set stdin for parallel execution to avoid conflicts.
	// Not needed anyway since we pass skip_checks=true (no prompts).

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("error creating stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("error creating stderr pipe: %w", err)
	}

	err = cmd.Start()
	if err != nil {
		return fmt.Errorf("error starting gather assessment metadata script: %w", err)
	}

	// Report starting status
	if progressChan != nil {
		progressChan <- NodeProgress{
			NodeName: nodeName,
			Stage:    "Starting collection...",
		}
	}

	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine to read stderr
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stderr)
		for scanner.Scan() {
			line := scanner.Text()
			log.Errorf("[%s][stderr]: %s", nodeName, line)
		}
	}()

	// Goroutine to read stdout and detect stages
	go func() {
		defer wg.Done()
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			log.Infof("[%s][stdout]: %s", nodeName, line)

			// Detect stage changes from script output
			if progressChan != nil {
				stage := detectStageFromOutput(line)
				if stage != "" {
					progressChan <- NodeProgress{
						NodeName: nodeName,
						Stage:    stage,
					}
				}
			}
		}
	}()

	// Wait for output goroutines to finish
	wg.Wait()

	// Wait for command to complete
	err = cmd.Wait()
	if err != nil {
		// Send Failed status to progress channel for ANY error
		if progressChan != nil {
			progressChan <- NodeProgress{
				NodeName: nodeName,
				Stage:    "Failed",
			}
		}
		log.Errorf("[%s] Script failed with error: %v", nodeName, err)
		return fmt.Errorf("error waiting for gather assessment metadata script to complete: %w", err)
	}

	// Report completion
	if progressChan != nil {
		progressChan <- NodeProgress{
			NodeName: nodeName,
			Stage:    "Complete",
		}
	}

	return nil
}

// detectStageFromOutput parses script output to detect the current stage
// It matches the actual messages printed by print_and_log() in the shell script
func detectStageFromOutput(line string) string {
	originalLine := strings.TrimSpace(line)
	lineLower := strings.ToLower(originalLine)

	// Special case: Start of collection
	if strings.Contains(lineLower, "assessment metadata collection started") {
		return "Starting collection..."
	}

	// Special case: Completion
	if strings.Contains(lineLower, "assessment metadata collection completed") {
		return "Complete"
	}

	// Pass through any line that looks like a stage message
	// (contains keywords that indicate this is a meaningful status update)
	isStageMessage := strings.Contains(lineLower, "collecting") ||
		strings.Contains(lineLower, "skipping") ||
		strings.Contains(lineLower, "executing")

	if isStageMessage {
		// Return the original line (preserves capitalization)
		// Add "..." if not already present
		if !strings.HasSuffix(originalLine, "...") && !strings.HasSuffix(originalLine, ".") {
			return originalLine + "..."
		}
		return originalLine
	}

	return ""
}
