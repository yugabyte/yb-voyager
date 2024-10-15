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
package adaptiveparallelism

import (
	"strconv"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

type dummyTargetYugabyteDB struct {
	size          int
	maxSize       int
	cpuUsageUser1 float64
	cpuUsageSys1  float64
	cpuUsageUser2 float64
	cpuUsageSys2  float64
}

func (d *dummyTargetYugabyteDB) IsAdaptiveParallelismSupported() bool {
	return true
}

func (d *dummyTargetYugabyteDB) GetClusterMetrics() (map[string]tgtdb.NodeMetrics, error) {
	result := make(map[string]tgtdb.NodeMetrics)
	metrics1 := make(map[string]string)
	metrics1["cpu_usage_user"] = strconv.FormatFloat(d.cpuUsageUser1, 'f', -1, 64)
	metrics1["cpu_usage_system"] = strconv.FormatFloat(d.cpuUsageSys1, 'f', -1, 64)

	result["node1"] = tgtdb.NodeMetrics{
		Metrics: metrics1,
		UUID:    "node1",
		Status:  "OK",
		Error:   "",
	}

	metrics2 := make(map[string]string)
	metrics2["cpu_usage_user"] = strconv.FormatFloat(d.cpuUsageUser2, 'f', -1, 64)
	metrics2["cpu_usage_system"] = strconv.FormatFloat(d.cpuUsageSys2, 'f', -1, 64)

	result["node2"] = tgtdb.NodeMetrics{
		Metrics: metrics2,
		UUID:    "node2",
		Status:  "OK",
		Error:   "",
	}
	return result, nil
}

func (d *dummyTargetYugabyteDB) GetNumConnectionsInPool() int {
	return d.size
}

func (d *dummyTargetYugabyteDB) GetNumMaxConnectionsInPool() int {
	return d.maxSize
}

func (d *dummyTargetYugabyteDB) UpdateNumConnectionsInPool(delta int) error {
	d.size += delta
	return nil
}

func TestMaxCpuUsage(t *testing.T) {
	yb := &dummyTargetYugabyteDB{
		size:          3,
		maxSize:       6,
		cpuUsageUser1: 0.5,
		cpuUsageSys1:  0.1,
		cpuUsageUser2: 0.1,
		cpuUsageSys2:  0.1,
	}

	clusterMetrics, _ := yb.GetClusterMetrics()
	maxCpuUsage, err := getMaxCpuUsageInCluster(clusterMetrics)
	assert.NoError(t, err)
	assert.Equal(t, 60, maxCpuUsage)
}

func TestIncreaseParallelism(t *testing.T) {
	yb := &dummyTargetYugabyteDB{
		size:          3,
		maxSize:       6,
		cpuUsageUser1: 0.5,
		cpuUsageSys1:  0.1,
		cpuUsageUser2: 0.5,
		cpuUsageSys2:  0.1,
	}

	err := fetchClusterMetricsAndUpdateParallelism(yb)
	assert.NoErrorf(t, err, "failed to fetch cluster metrics and update parallelism")
	assert.Equal(t, 4, yb.GetNumConnectionsInPool())
}

func TestDecreaseParallelismBasedOnCpu(t *testing.T) {
	yb := &dummyTargetYugabyteDB{
		size:          3,
		maxSize:       6,
		cpuUsageUser1: 0.8, // above threshold
		cpuUsageSys1:  0.1,
		cpuUsageUser2: 0.5,
		cpuUsageSys2:  0.1,
	}

	err := fetchClusterMetricsAndUpdateParallelism(yb)
	assert.NoErrorf(t, err, "failed to fetch cluster metrics and update parallelism")
	assert.Equal(t, 2, yb.GetNumConnectionsInPool())
}
