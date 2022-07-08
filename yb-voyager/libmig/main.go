package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

func main() {
	var err error
	initLogging()
	log.Infof("Start.")
	ctx := context.Background()

	//	migstate := NewMigrationState("/Users/amit.jambure/export-dir")
	migstate := NewMigrationState("/tmp/export-dir")
	progressReporter := NewProgressReporter()

	params := &ConnectionParams{
		NumConnections: 3,
		ConnUriList: []string{
			"postgresql://yugabyte@127.0.0.1:5433/testdb",
			"postgresql://yugabyte@127.0.0.2:5433/testdb",
			"postgresql://yugabyte@127.0.0.3:5433/testdb",
		},
	}
	connPool := NewConnectionPool(params)
	tdb := NewTargetDB(connPool)

	desc1 := &DataFileDescriptor{FileType: FILE_TYPE_CSV}
	op1 := NewImportFileOp(migstate, progressReporter, tdb, "/tmp/test.txt", NewTableID("testdb", "public", "foo"), desc1)
	op1.BatchSize = 4
	err = op1.Run(ctx)
	panicOnErr(err)

	desc2 := &DataFileDescriptor{FileType: FILE_TYPE_ORA2PG}
	op2 := NewImportFileOp(migstate, progressReporter, tdb, "/tmp/category_data.sql", NewTableID("testdb", "public", "category"), desc2)
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
