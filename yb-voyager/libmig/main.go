package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

func main() {
	initLogging()
	log.Infof("Start.")
	ctx := context.Background()

	migstate := NewMigrationState("/Users/amit.jambure/export-dir")
	progressReporter := NewProgressReporter()

	desc1 := &DataFileDescriptor{FileType: FILE_TYPE_CSV}
	op1 := NewImportFileOp(migstate, progressReporter, "test.txt", NewTableID("testdb", "public", "foo"), desc1)
	op1.BatchSize = 4
	err := op1.Run(ctx)
	panicOnErr(err)

	desc2 := &DataFileDescriptor{FileType: FILE_TYPE_ORA2PG}
	op2 := NewImportFileOp(migstate, progressReporter, "/tmp/category_data.sql", NewTableID("testdb", "public", "category"), desc2)
	op2.BatchSize = 5
	err = op2.Run(ctx)
	panicOnErr(err)

	time.Sleep(time.Second)
}

func panicOnErr(err error) {
	if err != nil {
		panic(err)
	}
}
