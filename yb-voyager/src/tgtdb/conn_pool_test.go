/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package tgtdb

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/stretchr/testify/assert"

	embeddedpostgres "github.com/fergusstrange/embedded-postgres"
)

// var postgres *embeddedpostgres.EmbeddedPostgres

func setupPostgres(t *testing.T) *embeddedpostgres.EmbeddedPostgres {
	postgres := embeddedpostgres.NewDatabase(embeddedpostgres.DefaultConfig().
		Username("postgres").
		Password("postgres").
		Database("test").
		Port(9876).
		StartTimeout(30 * time.Second))
	err := postgres.Start()
	if err != nil {
		t.Fatal(err)
	}
	return postgres
}

func shutdownPostgres(postgres *embeddedpostgres.EmbeddedPostgres, t *testing.T) {
	err := postgres.Stop()
	if err != nil {
		t.Fatal(err)
	}
}

func TestBasic(t *testing.T) {
	postgres := setupPostgres(t)
	defer shutdownPostgres(postgres, t)
	// GIVEN: a conn pool of size 10.
	size := 10

	connParams := &ConnectionParams{
		NumConnections:    size,
		NumMaxConnections: size,
		ConnUriList:       []string{fmt.Sprintf("postgresql://postgres:postgres@localhost:%d/test", 9876)},
		SessionInitScript: []string{},
	}
	pool := NewConnectionPool(connParams)
	assert.Equal(t, size, len(pool.conns))

	// WHEN: multiple goroutines acquire connection, perform some operation
	// and release connection back to pool
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go dummyProcess(pool, 1, &wg)
	}
	wg.Wait()

	// THEN: all connections are released back to pool
	assert.Equal(t, size, len(pool.conns))
}

func dummyProcess(pool *ConnectionPool, seconds int, wg *sync.WaitGroup) {
	defer wg.Done()
	_ = pool.WithConn(func(conn *pgx.Conn) (retry bool, err error) {
		time.Sleep(time.Duration(seconds) * time.Second)
		return false, nil
	})
}

func TestIncreaseConnectionsUptoMax(t *testing.T) {
	postgres := setupPostgres(t)
	defer shutdownPostgres(postgres, t)
	// GIVEN: a conn pool of size 10, with max 20 connections.
	size := 10
	maxSize := 20

	connParams := &ConnectionParams{
		NumConnections:    size,
		NumMaxConnections: maxSize,
		ConnUriList:       []string{fmt.Sprintf("postgresql://postgres:postgres@localhost:%d/test", 9876)},
		SessionInitScript: []string{},
	}
	pool := NewConnectionPool(connParams)
	assert.Equal(t, size, len(pool.conns))

	// WHEN: multiple goroutines acquire connection, perform some operation
	// and release connection back to pool
	// WHEN: we keep increasing the connnections upto the max..

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < maxSize-size; i++ {
			err := pool.UpdateNumConnections(1)
			assert.NoError(t, err)
			time.Sleep(1 * time.Second)
			assert.Equal(t, size+i+1, pool.size) // assert that size is increasing
		}
		// now that we will increase beyond maxSize, we should get an error.
		err := pool.UpdateNumConnections(1)
		assert.Error(t, err)
		wg.Done()
	}()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go dummyProcess(pool, 1, &wg)
	}
	wg.Wait()

	// THEN: we should have as many as maxSize connections in pool now.
	assert.Equal(t, maxSize, len(pool.conns))
}

func TestDecreaseConnectionsUptoMin(t *testing.T) {
	postgres := setupPostgres(t)
	defer shutdownPostgres(postgres, t)
	// GIVEN: a conn pool of size 10, with max 20 connections.
	size := 10
	maxSize := 20

	connParams := &ConnectionParams{
		NumConnections:    size,
		NumMaxConnections: maxSize,
		ConnUriList:       []string{fmt.Sprintf("postgresql://postgres:postgres@localhost:%d/test", 9876)},
		SessionInitScript: []string{},
	}
	pool := NewConnectionPool(connParams)
	assert.Equal(t, size, len(pool.conns))

	// WHEN: multiple goroutines acquire connection, perform some operation
	// and release connection back to pool
	// WHEN: we keep decrease the connnections upto the minimum(1)..

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		for i := 0; i < size-1; i++ {
			err := pool.UpdateNumConnections(-1)
			assert.NoError(t, err)
			time.Sleep(1 * time.Second)
			assert.Equal(t, size-(i+1), pool.size) // assert that size is increasing
		}
		// now that we will decrease beyond 1, we should get an error.
		err := pool.UpdateNumConnections(-1)
		assert.Error(t, err)
		wg.Done()
	}()

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go dummyProcess(pool, 1, &wg)
	}
	wg.Wait()

	// THEN: we should have as many as maxSize connections in pool now.
	assert.Equal(t, 1, len(pool.conns))
}
