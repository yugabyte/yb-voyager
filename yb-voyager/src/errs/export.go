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

package errs

import (
	"fmt"
	"strings"
)

const (
	//operation names
	GET_INITIAL_TABLE_LIST_OPERATION                            = "get_initial_table_list"
	RETRIEVE_FIRST_RUN_TABLE_LIST_OPERATION                     = "retrieve_first_run_table_list"
	FETCH_TABLES_NAMES_FROM_SOURCE                              = "fetch_tables_names_from_source"
	APPLY_TABLE_LIST_FLAGS_ON_FULL_LIST                         = "apply_table_list_flags_on_full_list"
	DETECT_AND_REPORT_NEW_LEAF_PARTITIONS_ON_PARTITIONED_TABLES = "detect_and_report_new_leaf_partitions_on_partitioned_tables"
	APPLY_TABLE_LIST_FLAGS_ON_SUBSEQUENT_RUN                    = "apply_table_list_flags_on_subsequent_run"
)

type ExportDataError struct {
	flow       string   // The main operation flow (e.g., "get_initial_table_list")
	steps      []string // All steps completed so far in the flow
	failedStep string   // The step that failed
	err        error    // The underlying error
}

func (e *ExportDataError) Error() string {
	// Reverse the steps array to show the flow in chronological order
	reversedSteps := make([]string, len(e.steps))
	for i, step := range e.steps {
		reversedSteps[len(e.steps)-1-i] = step
	}
	completedSteps := strings.Join(reversedSteps, ", ")
	if len(e.steps) > 0 {
		return fmt.Sprintf("error in %s at step '%s', after steps - (%s): %s",
			e.flow, e.failedStep, completedSteps, e.err.Error())
	}
	return fmt.Sprintf("error in %s at step '%s': %s",
		e.flow, e.failedStep, e.err.Error())
}

func (e *ExportDataError) Flow() string {
	return e.flow
}

func (e *ExportDataError) Steps() []string {
	return e.steps
}

func (e *ExportDataError) AddStep(step string) {
	e.steps = append(e.steps, step)
}

func (e *ExportDataError) FailedStep() string {
	return e.failedStep
}

func (e *ExportDataError) Unwrap() error {
	return e.err
}

// NewExportDataError creates a new error with flow context
func NewExportDataError(flow string, failedStep string, err error) *ExportDataError {
	return &ExportDataError{
		flow:       flow,
		failedStep: failedStep,
		err:        err,
	}
}

// NewExportDataErrorWithSteps creates a new error with flow context and completed steps
func NewExportDataErrorWithSteps(flow string, steps []string, failedStep string, err error) *ExportDataError {
	return &ExportDataError{
		flow:       flow,
		steps:      steps,
		failedStep: failedStep,
		err:        err,
	}
}



type UnknownTableErr struct {
	typeOfList      string
	unknownTables   []string
	validTableNames []string
}

func (e *UnknownTableErr) Error() string {
	return fmt.Sprintf("\nUnknown table names in the %s list: %v\nValid table names are: %v", e.typeOfList, e.unknownTables, e.validTableNames)
}

func NewUnknownTableErr(typeOfList string, unknownTables []string, validTableNames []string) *UnknownTableErr {
	return &UnknownTableErr{
		typeOfList:      typeOfList,
		unknownTables:   unknownTables,
		validTableNames: validTableNames,
	}
}