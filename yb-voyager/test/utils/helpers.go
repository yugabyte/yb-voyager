package testutils


func isSourceCmd(cmdName string) bool {
	return cmdName == "assess-migration" || cmdName == "export schema" || cmdName == "export data"
}

func isTargetCmd(cmdName string) bool {
	return cmdName == "import schema" || cmdName == "import data" || cmdName == "import data file"
}