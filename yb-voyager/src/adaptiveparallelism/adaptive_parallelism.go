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
	"fmt"
	"strconv"
	"time"

	"github.com/davecgh/go-spew/spew"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	CPU_USAGE_USER_METRIC                          = "cpu_usage_user"
	CPU_USAGE_SYSTEM_METRIC                        = "cpu_usage_system"
	TSERVER_ROOT_MEMORY_CONSUMPTION_METRIC         = "tserver_root_memory_consumption"
	TSERVER_ROOT_MEMORY_SOFT_LIMIT_METRIC          = "tserver_root_memory_soft_limit"
	MEMORY_TOTAL_METRIC                            = "memory_total"
	MEMORY_AVAILABLE_METRIC                        = "memory_available"
	MIN_PARALLELISM                                = 1
	DEFAULT_MAX_CPU_THRESHOLD                      = 70
	DEFAULT_ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS = 10
	DEFAULT_MIN_AVAILABLE_MEMORY_THRESHOLD         = 10
)

var MAX_CPU_THRESHOLD int
var ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS int
var MIN_AVAILABLE_MEMORY_THRESHOLD int

func readConfig() {
	MAX_CPU_THRESHOLD = utils.GetEnvAsInt("MAX_CPU_THRESHOLD", DEFAULT_MAX_CPU_THRESHOLD)
	if MAX_CPU_THRESHOLD != DEFAULT_MAX_CPU_THRESHOLD {
		utils.PrintAndLog("Using MAX_CPU_THRESHOLD: %d", MAX_CPU_THRESHOLD)
	}
	ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS = utils.GetEnvAsInt("ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS", DEFAULT_ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS)
	if ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS != DEFAULT_ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS {
		utils.PrintAndLog("Using ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS: %d", ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS)
	}
	MIN_AVAILABLE_MEMORY_THRESHOLD = utils.GetEnvAsInt("MIN_AVAILABLE_MEMORY_THRESHOLD", DEFAULT_MIN_AVAILABLE_MEMORY_THRESHOLD)
	if MIN_AVAILABLE_MEMORY_THRESHOLD != DEFAULT_MIN_AVAILABLE_MEMORY_THRESHOLD {
		utils.PrintAndLog("Using MIN_AVAILABLE_MEMORY_THRESHOLD: %d", MIN_AVAILABLE_MEMORY_THRESHOLD)
	}
}

type TargetYugabyteDBWithConnectionPool interface {
	IsAdaptiveParallelismSupported() bool
	GetClusterMetrics() (map[string]tgtdb.NodeMetrics, error) // node_uuid:metric_name:metric_value
	GetNumConnectionsInPool() int
	GetNumMaxConnectionsInPool() int
	UpdateNumConnectionsInPool(int) error // (delta)
}

var ErrAdaptiveParallelismNotSupported = fmt.Errorf("adaptive parallelism not supported in target YB database")

func AdaptParallelism(yb TargetYugabyteDBWithConnectionPool) error {
	if !yb.IsAdaptiveParallelismSupported() {
		return ErrAdaptiveParallelismNotSupported
	}
	readConfig()
	for {
		time.Sleep(time.Duration(ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS) * time.Second)
		err := fetchClusterMetricsAndUpdateParallelism(yb, MIN_PARALLELISM, yb.GetNumMaxConnectionsInPool())
		if err != nil {
			log.Warnf("adaptive: error updating parallelism: %v", err)
		}
	}
}

func fetchClusterMetricsAndUpdateParallelism(yb TargetYugabyteDBWithConnectionPool, minParallelism int, maxParallelism int) error {
	clusterMetrics, err := yb.GetClusterMetrics()
	log.Infof("adaptive: clusterMetrics: %v", spew.Sdump(clusterMetrics)) // TODO: move to debug?
	if err != nil {
		return fmt.Errorf("getting cluster metrics: %w", err)
	}

	cpuLoadHigh, err := isCpuLoadHigh(clusterMetrics)
	if err != nil {
		return fmt.Errorf("checking if cpu load is high: %w", err)
	}
	memLoadHigh, err := isMemoryLoadHigh(clusterMetrics)
	if err != nil {
		return fmt.Errorf("checking if memory load is high: %w", err)
	}

	if cpuLoadHigh || memLoadHigh {
		deltaParallelism := -1
		if (yb.GetNumConnectionsInPool() + deltaParallelism) < minParallelism {
			log.Infof("adaptive: cpuLoadHigh=%t, memLoadHigh=%t, not reducing parallelism to %d as it will become less than minParallelism %d",
				cpuLoadHigh, memLoadHigh, yb.GetNumConnectionsInPool()+deltaParallelism, minParallelism)
			return nil
		}

		log.Infof("adaptive: cpuLoadHigh=%t, memLoadHigh=%t, reducing parallelism to %d",
			cpuLoadHigh, memLoadHigh, yb.GetNumConnectionsInPool()+deltaParallelism)
		err = yb.UpdateNumConnectionsInPool(deltaParallelism)
		if err != nil {
			return fmt.Errorf("updating parallelism with -1: %w", err)
		}
	} else {
		deltaParallelism := 1
		if (yb.GetNumConnectionsInPool() + deltaParallelism) > maxParallelism {
			log.Infof("adaptive: cpuLoadHigh=%t, memLoadHigh=%t, not increasing parallelism to %d as it will become more than maxParallelism %d",
				cpuLoadHigh, memLoadHigh, yb.GetNumConnectionsInPool()+deltaParallelism, maxParallelism)
			return nil
		}

		log.Infof("adaptive: cpuLoadHigh=%t, memLoadHigh=%t, increasing parallelism to %d",
			cpuLoadHigh, memLoadHigh, yb.GetNumConnectionsInPool()+deltaParallelism)
		err := yb.UpdateNumConnectionsInPool(deltaParallelism)
		if err != nil {
			return fmt.Errorf("updating parallelism with +1 : %w", err)
		}
	}
	return nil
}

func isCpuLoadHigh(clusterMetrics map[string]tgtdb.NodeMetrics) (bool, error) {
	// get max CPU
	// Note that right now, voyager ingests data into the target in parallel,
	// but one table at a time. Therefore, in cases where there is a single tablet for a table,
	// either due to pre-split or colocated table, it is possible that the load on the cluster
	// will be uneven. Nevertheless, we still want to ensure that the cluster is not overloaded,
	// therefore we use the max CPU usage across all nodes in the cluster.
	maxCpuUsagePct, err := getMaxCpuUsageInCluster(clusterMetrics)
	if err != nil {
		return false, fmt.Errorf("getting max cpu usage in cluster: %w", err)
	}
	log.Infof("adaptive: max cpu usage in cluster = %d, max cpu threhsold = %d", maxCpuUsagePct, MAX_CPU_THRESHOLD)
	return maxCpuUsagePct > MAX_CPU_THRESHOLD, nil
}

func getMaxCpuUsageInCluster(clusterMetrics map[string]tgtdb.NodeMetrics) (int, error) {
	maxCpuPct := -1
	for _, nodeMetrics := range clusterMetrics {
		if nodeMetrics.Status != "OK" {
			continue
		}
		cpuUsageUser, err := strconv.ParseFloat(nodeMetrics.Metrics[CPU_USAGE_USER_METRIC], 64)
		if err != nil {
			return -1, fmt.Errorf("parsing cpu usage user as float: %w", err)
		}
		cpuUsageSystem, err := strconv.ParseFloat(nodeMetrics.Metrics[CPU_USAGE_SYSTEM_METRIC], 64)
		if err != nil {
			return -1, fmt.Errorf("parsing cpu usage system as float: %w", err)
		}

		cpuUsagePct := int((cpuUsageUser + cpuUsageSystem) * 100)
		maxCpuPct = max(maxCpuPct, cpuUsagePct)
	}
	return maxCpuPct, nil
}

/*
Memory load is considered to be high in the following scenarios
- Available memory of any node is less than 10% (MIN_AVAILABLE_MEMORY_THRESHOLD) of it's total memory
- tserver root memory consumption of any node has breached it's soft limit.
*/
func isMemoryLoadHigh(clusterMetrics map[string]tgtdb.NodeMetrics) (bool, error) {
	minMemoryAvailablePct := 100
	isTserverRootMemorySoftLimitBreached := false
	for _, nodeMetrics := range clusterMetrics {
		if nodeMetrics.Status != "OK" {
			continue
		}
		// check if tserver root memory soft limit is breached
		tserverRootMemoryConsumption, err := strconv.ParseInt(nodeMetrics.Metrics[TSERVER_ROOT_MEMORY_CONSUMPTION_METRIC], 10, 64)
		if err != nil {
			return false, fmt.Errorf("parsing tserver root memory consumption as int: %w", err)
		}
		tserverRootMemorySoftLimit, err := strconv.ParseInt(nodeMetrics.Metrics[TSERVER_ROOT_MEMORY_SOFT_LIMIT_METRIC], 10, 64)
		if err != nil {
			return false, fmt.Errorf("parsing tserver root memory soft limit as int: %w", err)
		}
		if tserverRootMemoryConsumption > tserverRootMemorySoftLimit {
			isTserverRootMemorySoftLimitBreached = true
			break
		}

		// check if memory available is low
		memoryAvailable, err := strconv.ParseInt(nodeMetrics.Metrics[MEMORY_AVAILABLE_METRIC], 10, 64)
		if err != nil {
			return false, fmt.Errorf("parsing memory available as int: %w", err)
		}
		memoryTotal, err := strconv.ParseInt(nodeMetrics.Metrics[MEMORY_TOTAL_METRIC], 10, 64)
		if err != nil {
			return false, fmt.Errorf("parsing memory total as int: %w", err)
		}
		if memoryAvailable == 0 || memoryTotal == 0 {
			// invalid values
			// on macos memory available is not available
			continue
		}
		memoryAvailablePct := int((memoryAvailable * 100) / memoryTotal)
		minMemoryAvailablePct = min(minMemoryAvailablePct, memoryAvailablePct)
		if minMemoryAvailablePct < MIN_AVAILABLE_MEMORY_THRESHOLD {
			break
		}
	}

	return minMemoryAvailablePct < MIN_AVAILABLE_MEMORY_THRESHOLD || isTserverRootMemorySoftLimitBreached, nil
}
