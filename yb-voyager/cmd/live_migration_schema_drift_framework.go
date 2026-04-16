//go:build integration_live_migration

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
	goerrors "github.com/go-errors/errors"
	"time"
)

// SchemaDriftWorkaroundMode controls when optional target DDL runs relative to
// source drift and replication waits.
type SchemaDriftWorkaroundMode int

const (
	// SchemaDriftNoWorkaround does not run TargetWorkaroundSQL.
	SchemaDriftNoWorkaround SchemaDriftWorkaroundMode = iota
	// SchemaDriftWorkaroundBeforeSourceDML runs TargetWorkaroundSQL on the target after
	// SourceDDL and before SourceDML (operator aligned catalogs before new rows).
	SchemaDriftWorkaroundBeforeSourceDML
	// SchemaDriftWorkaroundAfterFirstWaitFailure runs TargetWorkaroundSQL only after the
	// first replication wait returns an error, then runs a full exit+resume cycle and retries
	// that same wait once.
	SchemaDriftWorkaroundAfterFirstWaitFailure
)

// SchemaDriftPhaseConfig describes source DDL/DML after streaming mode plus ordered
// replication checks. After every replication wait, both `import data` and `export data`
// are stopped then resumed (full cycle) so tests mirror a hard restart of both sides; see
// MigrationStreamingCycleResult on SchemaDriftPhaseResult for stop/resume errors.
type SchemaDriftPhaseConfig struct {
	SourceDDL []string
	SourceDML []string

	TargetWorkaroundSQL []string
	WorkaroundMode      SchemaDriftWorkaroundMode

	// ImportExtraArgsForStreamingCycles is passed to ResumeImportData on each exit+resume
	// cycle (same key/value shape as StartImportData extra args). Optional.
	ImportExtraArgsForStreamingCycles map[string]string

	ReplicationWaits []SchemaDriftReplicationWait
}

// SchemaDriftReplicationWait is one logical check against the data-migration report.
type SchemaDriftReplicationWait struct {
	Name           string
	Expected       map[string]ChangesCount
	Timeout        time.Duration
	Poll           time.Duration
	RequireSuccess bool
}

// SchemaDriftWaitAttempt records one WaitForForwardStreamingComplete call.
type SchemaDriftWaitAttempt struct {
	StepName string
	Label    string
	Err      error
}

// SchemaDriftPhaseResult aggregates outcomes for assertions and logging.
type SchemaDriftPhaseResult struct {
	Attempts          []SchemaDriftWaitAttempt
	StreamingCycles   []MigrationStreamingCycleResult
	WorkaroundApplied bool
}

// AllStreamingCycleErrors returns every non-nil stop/resume error from all cycles, in order.
func (r *SchemaDriftPhaseResult) AllStreamingCycleErrors() []error {
	var out []error
	for i := range r.StreamingCycles {
		out = append(out, r.StreamingCycles[i].Errors()...)
	}
	return out
}

// LastAttemptError returns the most recent error for waits targeting stepName (empty = any).
func (r *SchemaDriftPhaseResult) LastAttemptError(stepName string) error {
	for i := len(r.Attempts) - 1; i >= 0; i-- {
		a := r.Attempts[i]
		if stepName != "" && a.StepName != stepName {
			continue
		}
		if a.Err != nil {
			return a.Err
		}
	}
	return nil
}

func (lm *LiveMigrationTest) exitAndResumeAndRecord(result *SchemaDriftPhaseResult, importExtra map[string]string) {
	c := lm.ExitAndResumeMigrationStreaming(true, importExtra)
	if c != nil {
		result.StreamingCycles = append(result.StreamingCycles, *c)
	}
}

// RunSchemaDriftPhase runs source drift DDL/DML and replication waits. After each wait it
// always stops then resumes both export and import (see ExitAndResumeMigrationStreaming).
// Stop/resume errors are only recorded on SchemaDriftPhaseResult.StreamingCycles — inspect
// AllStreamingCycleErrors() when a test expects or tolerates migration command failures.
// TargetWorkaroundSQL is optional (leave mode SchemaDriftNoWorkaround and empty slice).
//
// Returns a non-nil error only for SQL/setup failures or any replication wait with
// RequireSuccess that still fails (after optional workaround retry for
// SchemaDriftWorkaroundAfterFirstWaitFailure).
func (lm *LiveMigrationTest) RunSchemaDriftPhase(cfg SchemaDriftPhaseConfig) (*SchemaDriftPhaseResult, error) {
	result := &SchemaDriftPhaseResult{}

	if err := lm.ExecuteOnSource(cfg.SourceDDL...); err != nil {
		return result, err
	}

	if cfg.WorkaroundMode == SchemaDriftWorkaroundBeforeSourceDML && len(cfg.TargetWorkaroundSQL) > 0 {
		if err := lm.ExecuteOnTarget(cfg.TargetWorkaroundSQL...); err != nil {
			return result, err
		}
		result.WorkaroundApplied = true
	}

	if err := lm.ExecuteOnSource(cfg.SourceDML...); err != nil {
		return result, err
	}

	workaroundAfterFailureDone := false

	for _, w := range cfg.ReplicationWaits {
		err := lm.WaitForForwardStreamingComplete(w.Expected, w.Timeout, w.Poll)
		result.Attempts = append(result.Attempts, SchemaDriftWaitAttempt{StepName: w.Name, Label: "initial", Err: err})

		lm.exitAndResumeAndRecord(result, cfg.ImportExtraArgsForStreamingCycles)

		if err != nil && cfg.WorkaroundMode == SchemaDriftWorkaroundAfterFirstWaitFailure &&
			len(cfg.TargetWorkaroundSQL) > 0 && !workaroundAfterFailureDone {
			workaroundAfterFailureDone = true
			result.WorkaroundApplied = true
			if errT := lm.ExecuteOnTarget(cfg.TargetWorkaroundSQL...); errT != nil {
				return result, errT
			}
			lm.exitAndResumeAndRecord(result, cfg.ImportExtraArgsForStreamingCycles)
			err = lm.WaitForForwardStreamingComplete(w.Expected, w.Timeout, w.Poll)
			result.Attempts = append(result.Attempts, SchemaDriftWaitAttempt{StepName: w.Name, Label: "after_workaround", Err: err})
			lm.exitAndResumeAndRecord(result, cfg.ImportExtraArgsForStreamingCycles)
		}

		if w.RequireSuccess && err != nil {
			return result, goerrors.Errorf("replication wait %q: %w", w.Name, err)
		}
	}

	return result, nil
}

// SchemaDriftStderrTail returns a short diagnostic string with the ends of export/import stderr.
func SchemaDriftStderrTail(lm *LiveMigrationTest, maxEach int) string {
	const sep = " | "
	ex := tailCaptureString(lm.GetExportCommandStderr(), maxEach)
	im := tailCaptureString(lm.GetImportCommandStderr(), maxEach)
	return "export tail=" + ex + sep + "import tail=" + im
}

func tailCaptureString(s string, max int) string {
	if max <= 0 || len(s) <= max {
		return s
	}
	return s[len(s)-max:]
}
