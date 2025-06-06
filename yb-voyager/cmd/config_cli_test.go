//go:build unit
// +build unit

package cmd

import (
	"strings"
	"testing"

	mapset "github.com/deckarep/golang-set/v2"
	"github.com/spf13/pflag"
	"github.com/stretchr/testify/assert"
)

// Test that the set of CLI flags for the import-data command matches the allowedImportDataConfigKeys set.
func TestImportDataFlagsMatchAllowedConfigKeys(t *testing.T) {
	cmd := importDataCmd // Replace with actual function to get the import-data command
	flagNames := mapset.NewThreadUnsafeSet[string]()
	sourceFlags := mapset.NewThreadUnsafeSet[string]()
	sourceReplicaFlags := mapset.NewThreadUnsafeSet[string]()
	targetFlags := mapset.NewThreadUnsafeSet[string]()
	globalFlags := mapset.NewThreadUnsafeSet[string]()

	excludedFlags := mapset.NewThreadUnsafeSet[string]("start-clean")
	globalLevelFlags := mapset.NewThreadUnsafeSet[string]("send-diagnostics", "run-guardrails-checks")

	forceIncludeHiddenFlags := mapset.NewThreadUnsafeSet[string]("max-retries", "error-policy-snapshot")
	cmd.Flags().VisitAll(func(f *pflag.Flag) {
		if f.Hidden && !forceIncludeHiddenFlags.Contains(f.Name) {
			return // skip hidden flags except for exceptions
		}
		if excludedFlags.Contains(f.Name) {
			return // skip flags that are never present in config
		}
		if globalLevelFlags.Contains(f.Name) {
			globalFlags.Add(f.Name)
			return
		}
		if f.Name == "target-endpoints" {
			flagNames.Add(f.Name) // treat as command-specific, not target config
			return
		}
		if strings.HasPrefix(f.Name, SourceDBFlagPrefix) {
			sourceFlags.Add(strings.TrimPrefix(f.Name, SourceDBFlagPrefix))
		} else if strings.HasPrefix(f.Name, "source-replica-") {
			sourceReplicaFlags.Add(strings.TrimPrefix(f.Name, "source-replica-"))
		} else if strings.HasPrefix(f.Name, TargetDBFlagPrefix) {
			targetFlags.Add(strings.TrimPrefix(f.Name, TargetDBFlagPrefix))
		} else {
			flagNames.Add(f.Name)
		}
	})

	// allowedImportDataConfigKeys is already a mapset.Set[string]
	envVarOnlyConfigKeys := mapset.NewThreadUnsafeSet[string](
		"num-event-channels", "max-cpu-threshold", "max-events-per-batch", "adaptive-parallelism-frequency-seconds",
		"min-available-memory-threshold", "event-channel-size", "max-batch-size-bytes", "max-interval-between-batches",
		"ybvoyager-max-colocated-batches-in-progress", "ybvoyager-use-task-picker-for-import",
	)

	missingInConfig := flagNames.Difference(allowedImportDataConfigKeys.Difference(envVarOnlyConfigKeys))
	missingInFlags := allowedImportDataConfigKeys.Difference(flagNames.Union(envVarOnlyConfigKeys))

	assert.True(t, missingInConfig.Cardinality() == 0, "Flags missing in allowedImportDataConfigKeys (excluding env-var-only keys): %v", missingInConfig.ToSlice())
	assert.True(t, missingInFlags.Cardinality() == 0, "Config keys missing in CLI flags (excluding env-var-only keys): %v", missingInFlags.ToSlice())

	missingSource := sourceFlags.Difference(allowedSourceConfigKeys)
	missingSourceReplica := sourceReplicaFlags.Difference(allowedSourceReplicaConfigKeys)
	missingTarget := targetFlags.Difference(allowedTargetConfigKeys)
	missingGlobal := globalFlags.Difference(allowedGlobalConfigKeys)

	assert.True(t, missingSource.Cardinality() == 0, "source- flags missing in allowedSourceConfigKeys: %v", missingSource.ToSlice())
	assert.True(t, missingSourceReplica.Cardinality() == 0, "source-replica- flags missing in allowedSourceReplicaConfigKeys: %v", missingSourceReplica.ToSlice())
	assert.True(t, missingTarget.Cardinality() == 0, "target- flags missing in allowedTargetConfigKeys: %v", missingTarget.ToSlice())
	assert.True(t, missingGlobal.Cardinality() == 0, "global-level flags missing in allowedGlobalConfigKeys: %v", missingGlobal.ToSlice())
}

// You may need to implement or import getImportDataCommand() depending on your codebase.
// This is a placeholder for the actual function that returns the *cobra.Command for import-data.
