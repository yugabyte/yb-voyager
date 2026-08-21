package testutils

import (
	"fmt"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

var originalErrExitPreLog func(formatString string, args ...interface{})

func MonkeyPatchUtilsErrExitPreLogWithPanic() {
	MonkeyPatchUtilsErrExitPreLog(func(formatString string, args ...interface{}) {
		panic("utils.ErrExitPreLog was called with: " + fmt.Sprintf(formatString, args...))
	})
}

// MonkeyPatchUtilsErrExitPreLog allows monkey patching of the utils.ErrExitPreLog function for testing purposes.
// It replaces the original function with a new one provided by the caller.
func MonkeyPatchUtilsErrExitPreLog(newErrExitPreLog func(formatString string, args ...interface{})) {
	originalErrExitPreLog = utils.ErrExitPreLog
	utils.ErrExitPreLog = newErrExitPreLog
}

// RestoreUtilsErrExitPreLog restores the original utils.ErrExitPreLog function after monkey patching.
func RestoreUtilsErrExitPreLog() {
	if originalErrExitPreLog != nil {
		utils.ErrExitPreLog = originalErrExitPreLog
	}
}
