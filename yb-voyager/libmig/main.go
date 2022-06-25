package main

import (
	"context"

	log "github.com/sirupsen/logrus"
)

func main() {
	log.Infof("Start.")
	ctx := context.Background()

	migstate := NewMigrationState("/Users/amit.jambure/export-dir")
	//	op := NewImportFileOp(migstate, "test.txt", NewTableID("testdb", "public", "foo"))
	op := NewImportFileOp(migstate, "/tmp/category_data.sql", NewTableID("testdb", "public", "category"))
	op.BatchSize = 7

	err := op.Run(ctx)
	panicOnErr(err)
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}
