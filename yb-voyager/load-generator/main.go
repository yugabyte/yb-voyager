package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4"
	"golang.org/x/sync/semaphore"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

var exitVoyager atomic.Bool

func signalHandler() {
	fmt.Println("Received interrupt signal. Exiting...")
	exitVoyager.Store(true)
}

const MAX_CONN_COUNT = 3
const BATCH_SIZE = 500

func main() {
	var err error
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		<-signalChan
		signalHandler()
	}()
	fmt.Println("Starting load generator")
	// Create a new conn pool.
	connPool := tgtdb.NewConnectionPool(&tgtdb.ConnectionParams{
		NumConnections: MAX_CONN_COUNT,
		ConnUriList:    []string{"postgresql://yugabyte:password@10.9.80.235:5433/standalone_repro","postgresql://yugabyte:password@10.9.77.38:5433/standalone_repro","postgresql://yugabyte:password@10.9.85.61:5433/standalone_repro"}, // TODO: Change this to the correct value.
		SessionInitScript: []string{
			"SET client_encoding to 'UTF-8'",
			"SET session_replication_role to replica",
			"SET yb_enable_upsert_mode to true",
		},
	})
	connPool.DisableThrottling()
	wg := &sync.WaitGroup{}
	sem := semaphore.NewWeighted(int64(MAX_CONN_COUNT))
	k := int32(1)
	// var counter int32
	var tableRowsAffectedTotal sync.Map
	for !exitVoyager.Load() {
		sem.Acquire(context.Background(), 1)
		wg.Add(1)
		go func() {
			defer sem.Release(1)
			defer wg.Done()

			endK := atomic.AddInt32(&k, BATCH_SIZE)
			startK := endK - BATCH_SIZE
			for attempt := 1; !exitVoyager.Load(); attempt++ {
				err = connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
					tableRowsAffected, err2 := executeBatch(conn, startK, endK)
					if err2 == nil {
						// totalCount := atomic.AddInt32(&counter, int32(rowCount))
						// if totalCount%5000 == 0 {
						for k, v := range tableRowsAffected {
							updatedValue, ok := tableRowsAffectedTotal.Load(k)
							if !ok {
								updatedValue = 0
							}
							tableRowsAffectedTotal.Store(k, updatedValue.(int)+v)
						}
						fmt.Printf("Total rows inserted: %v\n", tableRowsAffected)
						// }
					}
					return false, err2
				})
				if err == nil {
					break
				}
				fmt.Printf("Batch [%d:%d], attempt %d: %v\n", startK, endK, attempt, err)
				time.Sleep(10 * time.Second)
			}
		}()
	}
	wg.Wait()
	tableRowsAffectedTotal.Range(func(key, value interface{}) bool {
		fmt.Printf("Total rows inserted for table %s: %v\n", key, value)
		return true
	})
}

const insertStmt = "INSERT INTO %s (k, u, v) VALUES ($1, $2, $3)"

func executeBatch(conn *pgx.Conn, startK, endK int32) (map[string]int, error) {
	tx, err := conn.Begin(context.Background())
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %s", err)
	}
	defer func() {
		err2 := tx.Rollback(context.Background())
		if err2 != nil && err2 != pgx.ErrTxClosed {
			fmt.Printf("failed to rollback transaction: %s\n", err2)
		}
	}()

	batch := &pgx.Batch{}
	tables := []string{"foo", "foo1", "foo2", "foo3", "foo4", "foo5"}
	
	//map of table and rows affected
	tableRowsAffected := make(map[string]int)
	for i := 0; i < len(tables); i++ {
		tableRowsAffected[tables[i]] = 0
	}
	var batchTables []string
	for k := startK; k < endK; k++ {
		u := uuid.New().String()
		v := uuid.New().String()
		tableName := tables[rand.Intn(len(tables))]
		batchTables = append(batchTables, tableName)
		insertStmtWithTable := fmt.Sprintf(insertStmt, tableName)
		batch.Queue(insertStmtWithTable, k, u, v)
	}
	br := tx.SendBatch(context.Background(), batch)
	var onceCloseBatchResults sync.Once
	closeBatchResult := func() {
		err2 := br.Close()
		if err2 != nil {
			fmt.Printf("failed to close batch [%d:%d]: %s\n", startK, endK, err2)
		}
	}
	defer onceCloseBatchResults.Do(closeBatchResult)
	for i := int32(0); i < endK-startK; i++ {
		res, err := br.Exec()
		if err != nil {
			return nil, fmt.Errorf("failed insert %d: %s", startK+i, err)
		}
		if res.RowsAffected() != 1 {
			return nil, fmt.Errorf("failed insert (rows affected == %d) %d: %s", startK+i, res.RowsAffected(), res)
		}
		tableName := batchTables[i]
		tableRowsAffected[tableName] += int(res.RowsAffected())
		// time.Sleep(10 * time.Millisecond)
	}
	onceCloseBatchResults.Do(closeBatchResult)
	err = tx.Commit(context.Background())
	if err != nil {
		fmt.Printf("failed to commit transaction [%d-%d]: %s\n", startK, endK, err)
		return nil, fmt.Errorf("failed to commit transaction: %s", err)
	}
	return tableRowsAffected, nil
}
