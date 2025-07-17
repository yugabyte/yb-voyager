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
	MEMORY_FREE_METRIC                             = "memory_free"
	MEMORY_TOTAL_METRIC                            = "memory_total"
	MEMORY_AVAILABLE_METRIC                        = "memory_available"
	MIN_PARALLELISM                                = 1
	DEFAULT_MAX_CPU_HARD_THRESHOLD                 = 90
	DEFAULT_MAX_CPU_SOFT_THRESHOLD                 = 70
	DEFAULT_CPU_SOFT_THRESHOLD_WINDOW              = 3
	DEFAULT_ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS = 10
	DEFAULT_MIN_AVAILABLE_MEMORY_THRESHOLD         = 10
	CPU_USAGE_UNKNOWN                              = -1
	CPU_LOAD_HIGH                                  = "HIGH"
	CPU_LOAD_OK                                    = "OK"
	CPU_LOAD_LOW                                   = "LOW"
)

var MAX_CPU_HARD_THRESHOLD int
var MAX_CPU_SOFT_THRESHOLD int
var CPU_SOFT_THRESHOLD_WINDOW int
var ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS int
var MIN_AVAILABLE_MEMORY_THRESHOLD int

func readConfig() {
	MAX_CPU_HARD_THRESHOLD = utils.GetEnvAsInt("MAX_CPU_HARD_THRESHOLD", DEFAULT_MAX_CPU_HARD_THRESHOLD)
	if MAX_CPU_HARD_THRESHOLD != DEFAULT_MAX_CPU_HARD_THRESHOLD {
		utils.PrintAndLog("Using MAX_CPU_HARD_THRESHOLD: %d", MAX_CPU_HARD_THRESHOLD)
	}
	MAX_CPU_SOFT_THRESHOLD = utils.GetEnvAsInt("MAX_CPU_SOFT_THRESHOLD", DEFAULT_MAX_CPU_SOFT_THRESHOLD)
	if MAX_CPU_SOFT_THRESHOLD != DEFAULT_MAX_CPU_SOFT_THRESHOLD {
		utils.PrintAndLog("Using MAX_CPU_SOFT_THRESHOLD: %d", MAX_CPU_SOFT_THRESHOLD)
	}
	CPU_SOFT_THRESHOLD_WINDOW = utils.GetEnvAsInt("CPU_SOFT_THRESHOLD_WINDOW", DEFAULT_CPU_SOFT_THRESHOLD_WINDOW)
	if CPU_SOFT_THRESHOLD_WINDOW != DEFAULT_CPU_SOFT_THRESHOLD_WINDOW {
		utils.PrintAndLog("Using CPU_SOFT_THRESHOLD_WINDOW: %d", CPU_SOFT_THRESHOLD_WINDOW)
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

type ParallelismAdapter struct {
	yb                          TargetYugabyteDBWithConnectionPool
	minParallelism              int
	maxParallelism              int
	maxCpuHardThreshold         int
	maxCpuSoftThreshold         int
	cpuSoftThresholdWindow      int
	minAvailableMemoryThreshold int
	cpuHistory                  map[string][]int // node_uuid -> []cpu_percentages
}

func NewParallelismAdapter(yb TargetYugabyteDBWithConnectionPool, minParallelism, maxParallelism, maxCpuHardThreshold, maxCpuSoftThreshold, cpuSoftThresholdWindow, minAvailableMemoryThreshold int) *ParallelismAdapter {
	return &ParallelismAdapter{
		yb:                          yb,
		minParallelism:              minParallelism,
		maxParallelism:              maxParallelism,
		maxCpuHardThreshold:         maxCpuHardThreshold,
		maxCpuSoftThreshold:         maxCpuSoftThreshold,
		cpuSoftThresholdWindow:      cpuSoftThresholdWindow,
		minAvailableMemoryThreshold: minAvailableMemoryThreshold,
		cpuHistory:                  make(map[string][]int),
	}
}

var ErrAdaptiveParallelismNotSupported = fmt.Errorf("adaptive parallelism not supported in target YB database")

func AdaptParallelism(yb TargetYugabyteDBWithConnectionPool) error {
	if !yb.IsAdaptiveParallelismSupported() {
		return ErrAdaptiveParallelismNotSupported
	}
	readConfig()
	adapter := NewParallelismAdapter(yb, MIN_PARALLELISM, yb.GetNumMaxConnectionsInPool(), MAX_CPU_HARD_THRESHOLD, MAX_CPU_SOFT_THRESHOLD, CPU_SOFT_THRESHOLD_WINDOW, MIN_AVAILABLE_MEMORY_THRESHOLD)
	for {
		time.Sleep(time.Duration(ADAPTIVE_PARALLELISM_FREQUENCY_SECONDS) * time.Second)
		err := adapter.FetchClusterMetricsAndUpdateParallelism()
		if err != nil {
			log.Warnf("adaptive: error updating parallelism: %v", err)
		}
	}
}

func (pa *ParallelismAdapter) FetchClusterMetricsAndUpdateParallelism() error {
	clusterMetrics, err := pa.yb.GetClusterMetrics()
	log.Infof("adaptive: clusterMetrics: %v", spew.Sdump(clusterMetrics)) // TODO: move to debug?
	if err != nil {
		return fmt.Errorf("getting cluster metrics: %w", err)
	}

	pa.updateCpuHistory(clusterMetrics)

	cpuLoadState, err := pa.getCpuLoadState(clusterMetrics)
	if err != nil {
		return fmt.Errorf("checking cpu load state: %w", err)
	}
	memLoadHigh, err := pa.isMemoryLoadHigh(clusterMetrics)
	if err != nil {
		return fmt.Errorf("checking if memory load is high: %w", err)
	}

	if cpuLoadState == CPU_LOAD_HIGH || memLoadHigh {
		deltaParallelism := -1
		if (pa.yb.GetNumConnectionsInPool() + deltaParallelism) < pa.minParallelism {
			log.Infof("adaptive: cpuLoadState=%s, memLoadHigh=%t, not reducing parallelism to %d as it will become less than minParallelism %d",
				cpuLoadState, memLoadHigh, pa.yb.GetNumConnectionsInPool()+deltaParallelism, pa.minParallelism)
			return nil
		}

		log.Infof("adaptive: cpuLoadState=%s, memLoadHigh=%t, reducing parallelism to %d",
			cpuLoadState, memLoadHigh, pa.yb.GetNumConnectionsInPool()+deltaParallelism)
		err = pa.yb.UpdateNumConnectionsInPool(deltaParallelism)
		if err != nil {
			return fmt.Errorf("updating parallelism with -1: %w", err)
		}
	} else if cpuLoadState == CPU_LOAD_LOW {
		deltaParallelism := 1
		if (pa.yb.GetNumConnectionsInPool() + deltaParallelism) > pa.maxParallelism {
			log.Infof("adaptive: cpuLoadState=%s, memLoadHigh=%t, not increasing parallelism to %d as it will become more than maxParallelism %d",
				cpuLoadState, memLoadHigh, pa.yb.GetNumConnectionsInPool()+deltaParallelism, pa.maxParallelism)
			return nil
		}

		log.Infof("adaptive: cpuLoadState=%s, memLoadHigh=%t, increasing parallelism to %d",
			cpuLoadState, memLoadHigh, pa.yb.GetNumConnectionsInPool()+deltaParallelism)
		err := pa.yb.UpdateNumConnectionsInPool(deltaParallelism)
		if err != nil {
			return fmt.Errorf("updating parallelism with +1 : %w", err)
		}
	} else {
		// CPU_LOAD_OK - maintain current parallelism
		log.Infof("adaptive: cpuLoadState=%s, memLoadHigh=%t, maintaining current parallelism %d",
			cpuLoadState, memLoadHigh, pa.yb.GetNumConnectionsInPool())
	}
	return nil
}

func (pa *ParallelismAdapter) getCpuLoadState(clusterMetrics map[string]tgtdb.NodeMetrics) (string, error) {
	// Check hard threshold first (immediate response)
	hardThresholdBreached, err := pa.checkHardCpuBreach(clusterMetrics)
	if err != nil {
		return "", fmt.Errorf("checking hard cpu threshold: %w", err)
	}
	if hardThresholdBreached {
		return CPU_LOAD_HIGH, nil
	}

	// Check soft threshold (trend-based response)
	softThresholdBreached, err := pa.checkSoftCpuBreach(clusterMetrics)
	if err != nil {
		return "", fmt.Errorf("checking soft cpu threshold: %w", err)
	}
	if softThresholdBreached {
		return CPU_LOAD_HIGH, nil
	}

	// Check if CPU load is consistently low
	cpuLoadLow, err := pa.checkCpuLoadLow(clusterMetrics)
	if err != nil {
		return "", fmt.Errorf("checking cpu load low: %w", err)
	}
	if cpuLoadLow {
		return CPU_LOAD_LOW, nil
	}

	return CPU_LOAD_OK, nil
}

func (pa *ParallelismAdapter) checkHardCpuBreach(clusterMetrics map[string]tgtdb.NodeMetrics) (bool, error) {
	// get max CPU
	// Note that right now, voyager ingests data into the target in parallel,
	// but one table at a time. Therefore, in cases where there is a single tablet for a table,
	// either due to pre-split or colocated table, it is possible that the load on the cluster
	// will be uneven. Nevertheless, we still want to ensure that the cluster is not overloaded,
	// therefore we use the max CPU usage across all nodes in the cluster.
	maxCpuUsagePct, err := pa.getMaxCpuUsageInCluster(clusterMetrics)
	if err != nil {
		return false, fmt.Errorf("getting max cpu usage in cluster: %w", err)
	}
	log.Infof("adaptive: max cpu usage in cluster = %d, max cpu hard threshold = %d", maxCpuUsagePct, pa.maxCpuHardThreshold)
	return maxCpuUsagePct > pa.maxCpuHardThreshold, nil
}

func (pa *ParallelismAdapter) checkSoftCpuBreach(clusterMetrics map[string]tgtdb.NodeMetrics) (bool, error) {
	for nodeUUID, history := range pa.cpuHistory {
		if len(history) < pa.cpuSoftThresholdWindow {
			// Not enough data for this node
			continue
		}

		allAbove := true
		for _, cpu := range history {
			if cpu == CPU_USAGE_UNKNOWN {
				allAbove = false
				break
			}
			if cpu <= pa.maxCpuSoftThreshold {
				allAbove = false
				break
			}
		}

		if allAbove {
			log.Infof("adaptive: node %s soft cpu threshold breached: last %d readings = %v, soft threshold = %d", nodeUUID, pa.cpuSoftThresholdWindow, history, pa.maxCpuSoftThreshold)
			return true, nil
		}
	}
	return false, nil
}

func (pa *ParallelismAdapter) checkCpuLoadLow(clusterMetrics map[string]tgtdb.NodeMetrics) (bool, error) {
	for nodeUUID, history := range pa.cpuHistory {
		if len(history) < pa.cpuSoftThresholdWindow {
			// Not enough data for this node
			continue
		}

		allBelow := true
		for _, cpu := range history {
			if cpu == CPU_USAGE_UNKNOWN {
				allBelow = false
				break
			}
			if cpu >= pa.maxCpuSoftThreshold {
				allBelow = false
				break
			}
		}

		if allBelow {
			log.Infof("adaptive: node %s cpu load consistently low: last %d readings = %v, soft threshold = %d", nodeUUID, pa.cpuSoftThresholdWindow, history, pa.maxCpuSoftThreshold)
			return true, nil
		}
	}
	return false, nil
}

func (pa *ParallelismAdapter) updateCpuHistory(clusterMetrics map[string]tgtdb.NodeMetrics) {
	// Clean up history for nodes no longer in cluster
	for nodeUUID := range pa.cpuHistory {
		if _, exists := clusterMetrics[nodeUUID]; !exists {
			delete(pa.cpuHistory, nodeUUID)
		}
	}

	// Update history for current nodes
	for nodeUUID, nodeMetrics := range clusterMetrics {
		var cpuUsagePct int

		if nodeMetrics.Status != "OK" {
			cpuUsagePct = CPU_USAGE_UNKNOWN
		} else {
			cpuUsagePctFloat, err := nodeMetrics.GetCPUPercent()
			if err != nil {
				log.Warnf("adaptive: error getting cpu usage for node %s: %v", nodeUUID, err)
				cpuUsagePct = CPU_USAGE_UNKNOWN
			} else {
				cpuUsagePct = int(cpuUsagePctFloat)
			}
		}

		// Add current CPU reading to history
		pa.cpuHistory[nodeUUID] = append(pa.cpuHistory[nodeUUID], cpuUsagePct)

		// Limit history to window size
		if len(pa.cpuHistory[nodeUUID]) > pa.cpuSoftThresholdWindow {
			pa.cpuHistory[nodeUUID] = pa.cpuHistory[nodeUUID][len(pa.cpuHistory[nodeUUID])-pa.cpuSoftThresholdWindow:]
		}
	}
}

func (pa *ParallelismAdapter) getMaxCpuUsageInCluster(clusterMetrics map[string]tgtdb.NodeMetrics) (int, error) {
	maxCpuPct := -1
	for _, nodeMetrics := range clusterMetrics {
		if nodeMetrics.Status != "OK" {
			continue
		}
		cpuUsagePct, err := nodeMetrics.GetCPUPercent()
		if err != nil {
			return -1, fmt.Errorf("getting cpu usage for node %s: %w", nodeMetrics.UUID, err)
		}
		maxCpuPct = max(maxCpuPct, int(cpuUsagePct))
	}
	return maxCpuPct, nil
}

/*
Memory load is considered to be high in the following scenarios
- Available memory of any node is less than 10% (MIN_AVAILABLE_MEMORY_THRESHOLD) of it's total memory
- tserver root memory consumption of any node has breached it's soft limit.
*/
func (pa *ParallelismAdapter) isMemoryLoadHigh(clusterMetrics map[string]tgtdb.NodeMetrics) (bool, error) {
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
		if minMemoryAvailablePct < pa.minAvailableMemoryThreshold {
			break
		}
	}

	return minMemoryAvailablePct < pa.minAvailableMemoryThreshold || isTserverRootMemorySoftLimitBreached, nil
}

/*
This PR improves the adaptive parallelism logic to be more aggressive when CPU usage is consistently low while being less sensitive to random CPU spikes. The current single-threshold approach is too conservative and misses opportunities to increase parallelism when the cluster has spare capacity.
Changes Made

Before (Binary Logic)
Single CPU threshold (80%)
Binary state: CPU load either "high" or "low"
Immediate throttling if any node exceeds threshold
Conservative approach that could miss optimization opportunities

After (Trend-based decision + Three-State Logic)
Three load states: HIGH (reduce parallelism), OK (maintain), LOW (increase parallelism)
HIGH state:
Two CPU thresholds: Hard threshold (90%) for immediate response, soft threshold (70%) for trend-based decisions
	Hard Threshold (90%): Immediate response - reduces parallelism if any node breaches
	Soft Threshold (70%): Trend-based response - reduces parallelism if any node's last 3 readings consistently exceed threshold.
			This will help avoid responses to temporary spikes and sustained load

Low State: Increases parallelism when any node's last 3 readings are consistently below 70%

OK State: Maintains current parallelism when CPU is in moderate range, preventing oscillation

*/
