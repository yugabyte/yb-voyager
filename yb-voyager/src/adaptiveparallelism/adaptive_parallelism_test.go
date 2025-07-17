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
package adaptiveparallelism

import (
	"os"
	"strconv"
	"testing"

	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
)

func init() {
	readConfig()
}

type dummyTargetYugabyteDB struct {
	size          int
	maxSize       int
	cpuUsageUser1 float64
	cpuUsageSys1  float64
	cpuUsageUser2 float64
	cpuUsageSys2  float64

	memAvailable1              int
	memTotal1                  int
	tserverRootMemConsumption1 int
	tserverRootMemSoftLimit1   int

	memAvailable2              int
	memTotal2                  int
	tserverRootMemConsumption2 int
	tserverRootMemSoftLimit2   int
}

func (d *dummyTargetYugabyteDB) IsAdaptiveParallelismSupported() bool {
	return true
}

/*
{

	     "memory_free": "2934779904",
	     "memory_total": "8054566912",
	     "cpu_usage_user": "0.010204",
	     "cpu_usage_system": "0.010204",
	     "memory_available": "7280869376",
	     "tserver_root_memory_limit": "3866192117",
	     "tserver_root_memory_soft_limit": "3286263299",
	     "tserver_root_memory_consumption": "40091648"
	}
*/
func (d *dummyTargetYugabyteDB) GetClusterMetrics() (map[string]tgtdb.NodeMetrics, error) {
	result := make(map[string]tgtdb.NodeMetrics)
	metrics1 := make(map[string]string)
	metrics1["cpu_usage_user"] = strconv.FormatFloat(d.cpuUsageUser1, 'f', -1, 64)
	metrics1["cpu_usage_system"] = strconv.FormatFloat(d.cpuUsageSys1, 'f', -1, 64)
	metrics1["memory_available"] = strconv.Itoa(d.memAvailable1)
	metrics1["memory_total"] = strconv.Itoa(d.memTotal1)
	metrics1["tserver_root_memory_consumption"] = strconv.Itoa(d.tserverRootMemConsumption1)
	metrics1["tserver_root_memory_soft_limit"] = strconv.Itoa(d.tserverRootMemSoftLimit1)

	result["node1"] = tgtdb.NodeMetrics{
		Metrics: metrics1,
		UUID:    "node1",
		Status:  "OK",
		Error:   "",
	}

	metrics2 := make(map[string]string)
	metrics2["cpu_usage_user"] = strconv.FormatFloat(d.cpuUsageUser2, 'f', -1, 64)
	metrics2["cpu_usage_system"] = strconv.FormatFloat(d.cpuUsageSys2, 'f', -1, 64)
	metrics2["memory_available"] = strconv.Itoa(d.memAvailable2)
	metrics2["memory_total"] = strconv.Itoa(d.memTotal2)
	metrics2["tserver_root_memory_consumption"] = strconv.Itoa(d.tserverRootMemConsumption2)
	metrics2["tserver_root_memory_soft_limit"] = strconv.Itoa(d.tserverRootMemSoftLimit2)

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

func TestMain(m *testing.M) {
	// set logging level to WARN
	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()

	os.Exit(exitCode)
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

	adapter := NewParallelismAdapter(yb, MIN_PARALLELISM, yb.GetNumMaxConnectionsInPool(), MAX_CPU_HARD_THRESHOLD, MAX_CPU_SOFT_THRESHOLD_HIGH, MAX_CPU_SOFT_THRESHOLD_LOW, CPU_SOFT_THRESHOLD_WINDOW, MIN_AVAILABLE_MEMORY_THRESHOLD)
	clusterMetrics, _ := yb.GetClusterMetrics()
	maxCpuUsage, err := adapter.getMaxCpuUsageInCluster(clusterMetrics)
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

		memAvailable1:              7280869376,
		memTotal1:                  8054566912,
		tserverRootMemConsumption1: 40091648,
		tserverRootMemSoftLimit1:   3286263299,

		memAvailable2:              7280869376,
		memTotal2:                  8054566912,
		tserverRootMemConsumption2: 40091648,
		tserverRootMemSoftLimit2:   3286263299,
	}

	adapter := NewParallelismAdapter(yb, MIN_PARALLELISM, yb.GetNumMaxConnectionsInPool(), MAX_CPU_HARD_THRESHOLD, MAX_CPU_SOFT_THRESHOLD_HIGH, MAX_CPU_SOFT_THRESHOLD_LOW, CPU_SOFT_THRESHOLD_WINDOW, MIN_AVAILABLE_MEMORY_THRESHOLD)
	err := adapter.FetchClusterMetricsAndUpdateParallelism()
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

	adapter := NewParallelismAdapter(yb, MIN_PARALLELISM, yb.GetNumMaxConnectionsInPool(), MAX_CPU_HARD_THRESHOLD, MAX_CPU_SOFT_THRESHOLD_HIGH, MAX_CPU_SOFT_THRESHOLD_LOW, CPU_SOFT_THRESHOLD_WINDOW, MIN_AVAILABLE_MEMORY_THRESHOLD)
	err := adapter.FetchClusterMetricsAndUpdateParallelism()
	assert.NoErrorf(t, err, "failed to fetch cluster metrics and update parallelism")
	assert.Equal(t, 2, yb.GetNumConnectionsInPool())
}

func TestDecreaseInParallelismBecauseOfLowAvailableMemory(t *testing.T) {
	yb := &dummyTargetYugabyteDB{
		size:          3,
		maxSize:       6,
		cpuUsageUser1: 0.5,
		cpuUsageSys1:  0.1,
		cpuUsageUser2: 0.5,
		cpuUsageSys2:  0.1,

		memAvailable1:              705456691, // less than 10% of memTotal1
		memTotal1:                  8054566912,
		tserverRootMemConsumption1: 40091648,
		tserverRootMemSoftLimit1:   3286263299,

		memAvailable2:              7280869376,
		memTotal2:                  8054566912,
		tserverRootMemConsumption2: 40091648,
		tserverRootMemSoftLimit2:   3286263299,
	}

	adapter := NewParallelismAdapter(yb, MIN_PARALLELISM, yb.GetNumMaxConnectionsInPool(), MAX_CPU_HARD_THRESHOLD, MAX_CPU_SOFT_THRESHOLD_HIGH, MAX_CPU_SOFT_THRESHOLD_LOW, CPU_SOFT_THRESHOLD_WINDOW, MIN_AVAILABLE_MEMORY_THRESHOLD)
	err := adapter.FetchClusterMetricsAndUpdateParallelism()
	assert.NoErrorf(t, err, "failed to fetch cluster metrics and update parallelism")
	assert.Equal(t, 2, yb.GetNumConnectionsInPool())
}

func TestDecreaseInParallelismBecauseofTserverRootMemoryConsumptionSoftLimitBreached(t *testing.T) {
	yb := &dummyTargetYugabyteDB{
		size:          3,
		maxSize:       6,
		cpuUsageUser1: 0.5,
		cpuUsageSys1:  0.1,
		cpuUsageUser2: 0.5,
		cpuUsageSys2:  0.1,

		memAvailable1:              7280869376,
		memTotal1:                  8054566912,
		tserverRootMemConsumption1: 40091648,
		tserverRootMemSoftLimit1:   3286263299,

		memAvailable2:              7280869376,
		memTotal2:                  8054566912,
		tserverRootMemConsumption2: 3286263300, // breaches tserverRootMemSoftLimit2
		tserverRootMemSoftLimit2:   3286263299,
	}

	adapter := NewParallelismAdapter(yb, MIN_PARALLELISM, yb.GetNumMaxConnectionsInPool(), MAX_CPU_HARD_THRESHOLD, MAX_CPU_SOFT_THRESHOLD_HIGH, MAX_CPU_SOFT_THRESHOLD_LOW, CPU_SOFT_THRESHOLD_WINDOW, MIN_AVAILABLE_MEMORY_THRESHOLD)
	err := adapter.FetchClusterMetricsAndUpdateParallelism()
	assert.NoErrorf(t, err, "failed to fetch cluster metrics and update parallelism")
	assert.Equal(t, 2, yb.GetNumConnectionsInPool())
}

func TestIncreaseInParallelismBeyondMax(t *testing.T) {
	yb := &dummyTargetYugabyteDB{
		size:          6, // already at max
		maxSize:       6,
		cpuUsageUser1: 0.5,
		cpuUsageSys1:  0.1,
		cpuUsageUser2: 0.5,
		cpuUsageSys2:  0.1,

		memAvailable1:              7280869376,
		memTotal1:                  8054566912,
		tserverRootMemConsumption1: 40091648,
		tserverRootMemSoftLimit1:   3286263299,

		memAvailable2:              7280869376,
		memTotal2:                  8054566912,
		tserverRootMemConsumption2: 40091648,
		tserverRootMemSoftLimit2:   3286263299,
	}

	adapter := NewParallelismAdapter(yb, MIN_PARALLELISM, yb.GetNumMaxConnectionsInPool(), MAX_CPU_HARD_THRESHOLD, MAX_CPU_SOFT_THRESHOLD_HIGH, MAX_CPU_SOFT_THRESHOLD_LOW, CPU_SOFT_THRESHOLD_WINDOW, MIN_AVAILABLE_MEMORY_THRESHOLD)
	err := adapter.FetchClusterMetricsAndUpdateParallelism()
	assert.NoErrorf(t, err, "failed to fetch cluster metrics and update parallelism")
	// assert no change in size because it would go beyond max size
	assert.Equal(t, 6, yb.GetNumConnectionsInPool())
}

func TestDecreaseInParallelismBelowMin(t *testing.T) {
	yb := &dummyTargetYugabyteDB{
		size:          1, // already at min
		maxSize:       6,
		cpuUsageUser1: 0.8, // above threshold
		cpuUsageSys1:  0.1,
		cpuUsageUser2: 0.5,
		cpuUsageSys2:  0.1,
	}

	adapter := NewParallelismAdapter(yb, MIN_PARALLELISM, yb.GetNumMaxConnectionsInPool(), MAX_CPU_HARD_THRESHOLD, MAX_CPU_SOFT_THRESHOLD_HIGH, MAX_CPU_SOFT_THRESHOLD_LOW, CPU_SOFT_THRESHOLD_WINDOW, MIN_AVAILABLE_MEMORY_THRESHOLD)
	err := adapter.FetchClusterMetricsAndUpdateParallelism()
	assert.NoErrorf(t, err, "failed to fetch cluster metrics and update parallelism")
	// assert no change in size because it would go below min size
	assert.Equal(t, 1, yb.GetNumConnectionsInPool())
}
