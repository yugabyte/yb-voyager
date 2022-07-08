package main

import (
	"context"
	"io"
	"strings"

	"github.com/jackc/pgx/v4"
	log "github.com/sirupsen/logrus"
)

type TargetDB struct {
	connPool *ConnectionPool
}

func NewTargetDB(connPool *ConnectionPool) *TargetDB {
	return &TargetDB{connPool: connPool}
}

func (tdb *TargetDB) Copy(ctx context.Context, copyCommand string, r io.Reader) (int64, error) {
	// //	fmt.Printf("%s\n", copyCommand)
	// scanner := bufio.NewScanner(r)
	// count := 0
	// for scanner.Scan() {
	// 	//		fmt.Println(scanner.Text())
	// 	count++
	// }
	// time.Sleep(500 * time.Millisecond)
	// return count, scanner.Err()
	var rowsAffected int64
	err := tdb.connPool.WithConn(func(c *pgx.Conn) error {
		res, err := c.PgConn().CopyFrom(context.Background(), r, copyCommand)
		rowsAffected = res.RowsAffected()
		if strings.Contains(err.Error(), "duplicate key value violates unique constraint") {
			err = nil
			log.Infof("%q => rowsAffected: %v", copyCommand, rowsAffected)
		}
		return err
	})

	return rowsAffected, err
}
