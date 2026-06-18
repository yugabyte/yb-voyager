// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemasnapshot

import (
	"fmt"
	"slices"
)

// The four valid snapshot capture labels.
const (
	LabelExportSchema                 = "export_schema"
	LabelExportDataFromSourceStart    = "export_data_from_source_start"
	LabelExportDataFromSourcePeriodic = "export_data_from_source_periodic"
	LabelExportDataFromSourceExit     = "export_data_from_source_exit"
)

// The valid capture reasons, grouped by the label that carries them.
const (
	// Reasons for LabelExportDataFromSourceStart.
	ReasonInitial      = "initial"
	ReasonResume       = "resume"
	ReasonCleanRestart = "clean_restart"

	// Reasons for LabelExportDataFromSourceExit.
	ReasonCutover   = "cutover"
	ReasonComplete  = "complete"
	ReasonInterrupt = "interrupt"
	ReasonError     = "error"
)

// labelReasons maps each label to its allowed reason vocabulary.
// A nil slice means no reason is permitted (empty reason required).
var labelReasons = map[string][]string{
	LabelExportSchema:                 nil,
	LabelExportDataFromSourceStart:    {ReasonInitial, ReasonResume, ReasonCleanRestart},
	LabelExportDataFromSourcePeriodic: nil,
	LabelExportDataFromSourceExit:     {ReasonCutover, ReasonComplete, ReasonInterrupt, ReasonError},
}

// ValidateLabelReason checks that the (label, reason) pair is legal.
//   - Unknown label → error.
//   - A reason given where the vocabulary is empty → error.
//   - A reason not in the vocabulary → error.
//   - An empty reason where a non-empty vocabulary is required → error.
func ValidateLabelReason(label, reason string) error {
	vocab, ok := labelReasons[label]
	if !ok {
		return fmt.Errorf("unknown snapshot label %q", label)
	}
	if vocab == nil {
		// No reason is allowed for this label.
		if reason != "" {
			return fmt.Errorf("label %q does not accept a reason, got %q", label, reason)
		}
		return nil
	}
	// Non-empty vocabulary: an empty reason is not allowed.
	if reason == "" {
		return fmt.Errorf("label %q requires a reason; valid values: %v", label, vocab)
	}
	if slices.Contains(vocab, reason) {
		return nil
	}
	return fmt.Errorf("label %q does not accept reason %q; valid values: %v", label, reason, vocab)
}
