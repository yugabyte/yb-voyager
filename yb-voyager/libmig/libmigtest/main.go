package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/libmig"
	"golang.org/x/sync/semaphore"
)

func main() {
	var err error
	initLogging()
	log.Infof("Start.")
	ctx := context.Background()

	sema := semaphore.NewWeighted(2)
	//	migstate := NewMigrationState("/Users/amit.jambure/export-dir")
	migstate := libmig.NewMigrationState("/tmp/export-dir")
	progressReporter := libmig.NewProgressReporter(false)

	params := &libmig.ConnectionParams{
		NumConnections: 3,
		ConnUriList: []string{
			"postgresql://yugabyte@127.0.0.1:5433/testdb",
			"postgresql://yugabyte@127.0.0.2:5433/testdb",
			"postgresql://yugabyte@127.0.0.3:5433/testdb",
		},
	}
	connPool := libmig.NewConnectionPool(params)
	tdb := libmig.NewTargetDB(connPool)

	desc1 := &libmig.DataFileDescriptor{FileType: libmig.FILE_TYPE_CSV}
	op1 := libmig.NewImportFileOp(migstate, progressReporter, tdb, "/tmp/test.txt", libmig.NewTableID("testdb", "public", "foo"), desc1, sema)
	op1.BatchSize = 4
	err = op1.Run(ctx)
	panicOnErr(err)

	desc2 := &libmig.DataFileDescriptor{FileType: libmig.FILE_TYPE_ORA2PG}
	op2 := libmig.NewImportFileOp(migstate, progressReporter, tdb, "/tmp/category_data.sql", libmig.NewTableID("testdb", "public", "category"), desc2, sema)
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
