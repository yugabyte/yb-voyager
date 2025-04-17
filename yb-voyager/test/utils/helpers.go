package testutils

import "github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"

func isSourceCmd(cmdName string) bool {
	return cmdName == "assess-migration" || cmdName == "export schema" || cmdName == "export data"
}

func isTargetCmd(cmdName string) bool {
	return cmdName == "import schema" || cmdName == "import data" || cmdName == "import data file"
}

// check if sqlname tuple slice contains a tuple
func SliceContainsTuple(tuples []sqlname.NameTuple, tuple sqlname.NameTuple) bool {
	for _, t := range tuples {
		if t.Equals(tuple) {
			return true
		}
	}
	return false
}
