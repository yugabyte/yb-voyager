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

	"github.com/golang-collections/collections/stack"
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
	currentFlow          string       // The current flow which is finally throwing the error (e.g., "get_initial_table_list")
	callExecutionHistory *stack.Stack // Stack of all the calls completed so far in the current flow
	failedStep           string       // The step that failed in the last call where the error occurred
	err                  error        // The underlying error
}

/*
e.g.
error in get_initial_table_list at step 'extract_table_list_from_exclude_list' in call 'apply_table_list_flags_on_full_list'
Execution history

	-> get_initial_table_list
	-> fetch_tables_names_from_source
	-> apply_table_list_flags_on_full_list

Error:
Unknown table names in the exclude list: [abc]
*/
func (e *ExportDataError) Error() string {
	errMsg := fmt.Sprintf("error in %s at step '%s'", e.currentFlow, e.failedStep)

	// Add call execution history
	if e.callExecutionHistory.Len() > 0 {
		trace, lastCall := e.buildStackTrace()
		errMsg += fmt.Sprintf(" in call '%s'\n", lastCall)
		errMsg += trace
	}
	errMsg += fmt.Sprintf("Error: %s\n", e.err.Error())
	return strings.TrimRight(errMsg, "\n")
}

/*
e.g.
Execution history

	-> get_initial_table_list
	-> fetch_tables_names_from_source
	-> apply_table_list_flags_on_full_list
*/
func (e *ExportDataError) buildStackTrace() (string, string) {
	stackTrace := "Execution history\n"
	stackTrace += fmt.Sprintf("    -> %s\n", e.currentFlow)
	//copy of execution history to avoid emptying the original stack
	tempStack := stack.New()
	lastCall := ""

	for e.callExecutionHistory.Len() > 0 {
		call := e.callExecutionHistory.Pop().(string)
		tempStack.Push(call)
		lastCall = call
		stackTrace += fmt.Sprintf("    -> %s\n", call)
	}
	// Restore the original stack
	for tempStack.Len() > 0 {
		e.callExecutionHistory.Push(tempStack.Pop())
	}
	return stackTrace, lastCall
}

func (e *ExportDataError) CurrentFlow() string {
	return e.currentFlow
}

func (e *ExportDataError) CallExecutionHistory() *stack.Stack {
	return e.callExecutionHistory
}

func (e *ExportDataError) AddCall(call string) {
	e.callExecutionHistory.Push(call)
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
		currentFlow:          flow,
		failedStep:           failedStep,
		callExecutionHistory: stack.New(),
		err:                  err,
	}
}

// NewExportDataErrorWithCompletedFlows creates a new error with flow context and completed flows
func NewExportDataErrorWithCompletedCalls(currentFlow string, callExecutionHistory *stack.Stack, failedStep string, err error) *ExportDataError {
	return &ExportDataError{
		currentFlow:          currentFlow,
		callExecutionHistory: callExecutionHistory,
		failedStep:           failedStep,
		err:                  err,
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
