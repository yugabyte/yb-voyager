//go:build integration

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
	"math/rand"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"

	testutils "github.com/yugabyte/yb-voyager/yb-voyager/test/utils"
)

func TestBasic(t *testing.T) {
	// GIVEN: a conn pool of size 10.
	size := 10

	connParams := &ConnectionParams{
		NumConnections:    size,
		NumMaxConnections: size,
		ConnUriList:       []string{testYugabyteDBTarget.GetConnectionString()},
		SessionInitScript: []string{},
	}
	pool := NewConnectionPool(connParams)
	assert.Equal(t, size, len(pool.conns))

	// WHEN: multiple goroutines acquire connection, perform some operation
	// and release connection back to pool
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go dummyProcess(pool, 1000, &wg)
	}
	wg.Wait()

	// THEN: all connections are released back to pool
	assert.Equal(t, size, len(pool.conns))
}

func dummyProcess(pool *ConnectionPool, milliseconds int, wg *sync.WaitGroup) {
	defer wg.Done()
	_ = pool.WithConn(func(conn *pgx.Conn) (retry bool, err error) {
		time.Sleep(time.Duration(milliseconds) * time.Millisecond)
		return false, nil
	})
}

func TestIncreaseConnectionsUptoMax(t *testing.T) {
	// GIVEN: a conn pool of size 10, with max 20 connections.
	size := 10
	maxSize := 20

	connParams := &ConnectionParams{
		NumConnections:    size,
		NumMaxConnections: maxSize,
		ConnUriList:       []string{testYugabyteDBTarget.GetConnectionString()},
		SessionInitScript: []string{},
	}
	pool := NewConnectionPool(connParams)
	assert.Equal(t, size, len(pool.conns))

	// WHEN: multiple goroutines acquire connection, perform some operation
	// and release connection back to pool
	// WHEN: we keep increasing the connections upto the max..

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
		go dummyProcess(pool, 1000, &wg)
	}
	wg.Wait()

	// THEN: we should have as many as maxSize connections in pool now.
	assert.Equal(t, maxSize, len(pool.conns))
}

func TestDecreaseConnectionsUptoMin(t *testing.T) {
	// GIVEN: a conn pool of size 10, with max 20 connections.
	size := 10
	maxSize := 20

	connParams := &ConnectionParams{
		NumConnections:    size,
		NumMaxConnections: maxSize,
		ConnUriList:       []string{testYugabyteDBTarget.GetConnectionString()},
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
		go dummyProcess(pool, 1000, &wg)
	}
	wg.Wait()

	// THEN: we should have as many as 1 connections in pool now.
	assert.Equal(t, 1, len(pool.conns))
}

func TestUpdateConnectionsRandom(t *testing.T) {
	// GIVEN: a conn pool of size 10, with max 20 connections.
	size := 10
	maxSize := 20

	connParams := &ConnectionParams{
		NumConnections:    size,
		NumMaxConnections: maxSize,
		ConnUriList:       []string{testYugabyteDBTarget.GetConnectionString()},
		SessionInitScript: []string{},
	}
	pool := NewConnectionPool(connParams)
	assert.Equal(t, size, len(pool.conns))

	// WHEN: multiple goroutines acquire connection, perform some operation
	// and release connection back to pool
	// WHEN: we keep increasing and decreasing the connnections randomly..

	expectedFinalSize := size
	var wg sync.WaitGroup
	wg.Add(1)
	go func(expectedFinalSize *int) {
		// 100 random updates either increase or decrease
		for i := 0; i < 1000; i++ {
			randomNumber := rand.Intn(11) - 5 // Generates a number between -5 and 5
			if pool.size+randomNumber < 1 || (pool.size+randomNumber > pool.params.NumMaxConnections) {
				continue
			}
			log.Infof("i=%d, updating by %d. New pool size expected = %d\n", i, randomNumber, *expectedFinalSize+randomNumber)
			err := pool.UpdateNumConnections(randomNumber)
			assert.NoError(t, err)
			time.Sleep(10 * time.Millisecond)
			*expectedFinalSize = *expectedFinalSize + randomNumber
			assert.Equal(t, *expectedFinalSize, pool.size) // assert that size is increasing
		}
		fmt.Printf("done updating. expectedFinalSize=%d\n", *expectedFinalSize)
		wg.Done()
	}(&expectedFinalSize)

	for i := 0; i < 1000; i++ {
		wg.Add(1)
		go dummyProcess(pool, 100, &wg)
	}
	wg.Wait()

	if pool.pendingConnsToClose > 0 {
		// some are yet to be closed. but there are no jobs running at the end of which, they will be closed.
		assert.Equal(t, expectedFinalSize, len(pool.conns)-pool.pendingConnsToClose)
	} else {
		// THEN: we should have as many as expectedFinalSize connections in pool now.
		assert.Equal(t, expectedFinalSize, len(pool.conns))
		// idle connections should have the rest.
		assert.Equal(t, maxSize-expectedFinalSize, len(pool.idleConns))
	}
}

func TestRemoveConnectionsForHosts(t *testing.T) {
	// GIVEN: a conn pool of size 10.
	size := 10

	connParams := &ConnectionParams{
		NumConnections:    size,
		NumMaxConnections: 2 * size,
		ConnUriList:       []string{testYugabyteDBTarget.GetConnectionString()},
		SessionInitScript: []string{},
	}
	pool := NewConnectionPool(connParams)
	assert.Equal(t, size, len(pool.conns))
	assert.Equal(t, size, len(pool.idleConns))

	host, _, err := testYugabyteDBTarget.GetHostPort()
	testutils.FatalIfError(t, err)

	//all nil case
	pool.RemoveConnectionsForHosts([]string{host})

	assert.Equal(t, size, len(pool.conns))
	assert.Equal(t, size, len(pool.idleConns))

	//4 conns to host in conns and 2 conns in idleconns

	var conn *pgx.Conn
	for i := 0; i < 4; i++ {
		conn, _ = <-pool.conns
		if conn == nil {
			conn, err = pool.createNewConnection()
			testutils.FatalIfError(t, err)
			pool.conns <- conn
		} else {
			pool.conns <- conn
		}
	}

	for i := 0; i < 2; i++ {
		conn, _ = <-pool.idleConns
		if conn == nil {
			conn, err = pool.createNewConnection()
			testutils.FatalIfError(t, err)
			pool.idleConns <- conn
		} else {
			pool.idleConns <- conn
		}
	}

	assert.Equal(t, size, len(pool.conns))
	assert.Equal(t, size, len(pool.idleConns))

	pool.RemoveConnectionsForHosts([]string{host})

	assert.Equal(t, size, len(pool.conns))
	assert.Equal(t, size, len(pool.idleConns))

	for i := 0; i < size; i++ {
		conn, gotIt := <-pool.conns
		assert.True(t, gotIt)
		assert.Nil(t, conn)
		pool.conns <- conn

		conn, gotIt = <-pool.idleConns
		assert.True(t, gotIt)
		assert.Nil(t, conn)
		pool.idleConns <- conn
	}
	//TODO: add test case for multiple hosts in conn pool for the RemoveConnectionsForHosts;
	//we need to enhance the test container for starting a container with different host or
	//have a configuration for multiple node yb cluster.
}
