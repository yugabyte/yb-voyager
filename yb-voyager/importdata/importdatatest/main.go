package main

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/importdata"
	"golang.org/x/sync/semaphore"
)

func main() {
	var err error
	initLogging()
	log.Infof("Start.")
	ctx := context.Background()

	sema := semaphore.NewWeighted(2)
	//	migstate := NewMigrationState("/Users/amit.jambure/export-dir")
	migstate := importdata.NewMigrationState("/tmp/export-dir")
	progressReporter := importdata.NewProgressReporter(false)

	params := &importdata.ConnectionParams{
		NumConnections: 3,
		ConnUriList: []string{
			"postgresql://yugabyte@127.0.0.1:5433/testdb",
			"postgresql://yugabyte@127.0.0.2:5433/testdb",
			"postgresql://yugabyte@127.0.0.3:5433/testdb",
		},
	}
	connPool := importdata.NewConnectionPool(params)
	tdb := importdata.NewTargetDB(connPool)

	desc1 := &importdata.DataFileDescriptor{FileType: importdata.FILE_TYPE_CSV}
	op1 := importdata.NewImportFileOp(migstate, progressReporter, tdb, "/tmp/test.txt", importdata.NewTableID("testdb", "public", "foo"), desc1, sema)
	op1.BatchSize = 4
	err = op1.Run(ctx)
	panicOnErr(err)

	desc2 := &importdata.DataFileDescriptor{FileType: importdata.FILE_TYPE_ORA2PG}
	op2 := importdata.NewImportFileOp(migstate, progressReporter, tdb, "/tmp/category_data.sql", importdata.NewTableID("testdb", "public", "category"), desc2, sema)
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
