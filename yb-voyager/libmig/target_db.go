package main

import (
	"bufio"
	"context"
	"io"
	"time"
)

type TargetDB struct {
}

func NewTargetDB() *TargetDB {
	return &TargetDB{}
}

func (tdb *TargetDB) Connect() error {
	return nil
}

func (tdb *TargetDB) Copy(ctx context.Context, copyCommand string, r io.Reader) (int, error) {
	//	fmt.Printf("%s\n", copyCommand)
	scanner := bufio.NewScanner(r)
	count := 0
	for scanner.Scan() {
		//		fmt.Println(scanner.Text())
		count++
	}
	time.Sleep(500 * time.Millisecond)
	return count, scanner.Err()
}
