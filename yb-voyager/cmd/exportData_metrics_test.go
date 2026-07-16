//go:build unit

package cmd

import (
	"errors"
	"testing"

	"github.com/golang-collections/collections/stack"
	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/errs"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metrics"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestHandleGetInitialTableListError(t *testing.T) {
	utils.MonkeyPatchUtilsErrExitToIgnore()
	defer utils.RestoreUtilsErrExit()

	rec := metrics.NewRecordingRecorder()
	prev := metrics.Get()
	defer metrics.SetRecorder(prev)
	metrics.SetRecorder(rec)

	err := errs.NewExportDataError(errs.GET_INITIAL_TABLE_LIST_OPERATION, "some_step", errors.New("boom"))
	// Simulate propagation through a couple of call levels, as getInitialTableList does.
	propagated := errs.NewExportDataErrorWithCompletedCalls(errs.FETCH_TABLES_NAMES_FROM_SOURCE, stack.New(), err.FailedStep(), err.Unwrap())
	propagated = errs.NewExportDataErrorWithCompletedCalls(errs.APPLY_TABLE_LIST_FLAGS_ON_SUBSEQUENT_RUN, propagated.CallExecutionHistory(), propagated.FailedStep(), propagated.Unwrap())

	handleGetInitialTableListError(propagated)

	assert.Equal(t, int64(1), rec.ExportErrors[errs.APPLY_TABLE_LIST_FLAGS_ON_SUBSEQUENT_RUN],
		"expected the export error to be recorded exactly once, at the terminal handling site")
	assert.Equal(t, int64(0), rec.ExportErrors[errs.GET_INITIAL_TABLE_LIST_OPERATION],
		"intermediate flows must not be recorded")
	assert.Equal(t, int64(0), rec.ExportErrors[errs.FETCH_TABLES_NAMES_FROM_SOURCE],
		"intermediate flows must not be recorded")
}
