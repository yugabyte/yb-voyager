package tgtdb

// TODO Define a Target interface and make TargetYugabyteDB implement it.
func NewTargetDB(tconf *TargetConf) *TargetYugabyteDB {
	return newTargetYugabyteDB(tconf)
}
