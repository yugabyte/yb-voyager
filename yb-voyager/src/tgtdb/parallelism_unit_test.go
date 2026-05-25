//go:build unit

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
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/types"
)

// Regression test for an adaptive-parallelism hang:
// when --adaptive-parallelism-max was less than the auto-computed Parallelism
// (clusterCores/4), the conn-pool init deadlocked silently. Now Parallelism
// is capped to MaxParallelism before the pool is created.
func TestReconcileAdaptiveParallelism_CapsParallelismToUserMax(t *testing.T) {
	// Mimics the post-fetch state: Parallelism would be set by fetchDefaultParallelJobs
	// to clusterCores/4 = 72 for a 288-core cluster; user passed --adaptive-parallelism-max=12.
	yb := &TargetYugabyteDB{Tconf: &TargetConf{
		Parallelism:             72,
		MaxParallelism:          12,
		AdaptiveParallelismMode: types.BalancedAdaptiveParallelismMode,
	}}
	yb.reconcileAdaptiveParallelism()

	assert.Equal(t, 12, yb.Tconf.Parallelism, "Parallelism should be capped to MaxParallelism")
	assert.Equal(t, 12, yb.Tconf.MaxParallelism, "MaxParallelism should remain user-supplied value")
}

func TestReconcileAdaptiveParallelism_NoUserMax_DefaultsTo4xParallelism(t *testing.T) {
	yb := &TargetYugabyteDB{Tconf: &TargetConf{
		Parallelism:             50,
		MaxParallelism:          0,
		AdaptiveParallelismMode: types.BalancedAdaptiveParallelismMode,
	}}
	yb.reconcileAdaptiveParallelism()
	assert.Equal(t, 50, yb.Tconf.Parallelism)
	assert.Equal(t, 200, yb.Tconf.MaxParallelism)
}

func TestReconcileAdaptiveParallelism_Disabled_MaxEqualsParallelism(t *testing.T) {
	yb := &TargetYugabyteDB{Tconf: &TargetConf{
		Parallelism:             8,
		MaxParallelism:          0,
		AdaptiveParallelismMode: types.DisabledAdaptiveParallelismMode,
	}}
	yb.reconcileAdaptiveParallelism()
	assert.Equal(t, 8, yb.Tconf.Parallelism)
	assert.Equal(t, 8, yb.Tconf.MaxParallelism)
}

func TestReconcileAdaptiveParallelism_MaxAboveParallelism_LeavesAsIs(t *testing.T) {
	yb := &TargetYugabyteDB{Tconf: &TargetConf{
		Parallelism:             10,
		MaxParallelism:          40,
		AdaptiveParallelismMode: types.BalancedAdaptiveParallelismMode,
	}}
	yb.reconcileAdaptiveParallelism()
	assert.Equal(t, 10, yb.Tconf.Parallelism)
	assert.Equal(t, 40, yb.Tconf.MaxParallelism)
}

// Regression: previously NewConnectionPool would deadlock when
// NumConnections > NumMaxConnections — the second init loop would block
// forever trying to drain from an empty idleConns channel.
func TestNewConnectionPool_RejectsInvertedSizes(t *testing.T) {
	done := make(chan struct{})
	var pool *ConnectionPool
	var err error
	go func() {
		pool, err = NewConnectionPool(&ConnectionParams{
			NumConnections:    72,
			NumMaxConnections: 12,
			ConnUriList:       []string{"postgres://stub"},
			SessionInitScript: []string{},
		})
		close(done)
	}()

	select {
	case <-done:
		assert.Error(t, err, "expected error for NumConnections > NumMaxConnections")
		assert.Nil(t, pool)
	case <-time.After(2 * time.Second):
		t.Fatal("NewConnectionPool deadlocked instead of returning an error")
	}
}

func TestNewConnectionPool_AcceptsValidSizes(t *testing.T) {
	// NumConnections == NumMaxConnections and NumConnections < NumMaxConnections both fine.
	for _, c := range []struct{ n, max int }{{10, 10}, {5, 20}, {0, 1}} {
		pool, err := NewConnectionPool(&ConnectionParams{
			NumConnections:    c.n,
			NumMaxConnections: c.max,
			ConnUriList:       []string{"postgres://stub"},
			SessionInitScript: []string{},
		})
		assert.NoError(t, err)
		assert.NotNil(t, pool)
		assert.Equal(t, c.n, pool.size)
	}
}

func TestNewConnectionPool_RejectsZeroMax(t *testing.T) {
	pool, err := NewConnectionPool(&ConnectionParams{
		NumConnections:    0,
		NumMaxConnections: 0,
	})
	assert.Error(t, err)
	assert.Nil(t, pool)
}
