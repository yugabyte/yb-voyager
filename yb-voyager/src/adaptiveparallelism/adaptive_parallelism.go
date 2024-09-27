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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	CPU_USAGE_USER    = "cpu_usage_user"
	CPU_USAGE_SYSTEM  = "cpu_usage_system"
	MAX_CPU_THRESHOLD = 70
)

type TargetYugabyteDBWithConnectionPool interface {
	IsAdaptiveParallelismSupported() bool
	GetClusterMetrics() (map[string]map[string]string, error) // node_uuid:metric_name:metric_value
	GetNumConnectionsInPool() int
	GetNumMaxConnectionsInPool() int
	UpdateNumConnectionsInPool(int) error
}

func AdaptParallelism(yb TargetYugabyteDBWithConnectionPool) error {
	if !yb.IsAdaptiveParallelismSupported() {
		return fmt.Errorf("adaptive parallelism not supported in target YB database.")
	}
	for {
		utils.PrintAndLog("--------------------------------------------------------")
		clusterMetrics, err := yb.GetClusterMetrics()
		if err != nil {
			utils.PrintAndLog("error getting cluster metrics: %v", err)
		}
		// utils.PrintAndLog("PARALLELISM: cluster metrics: %v", clusterMetrics)

		// max cpu
		maxCpuUsage, err := getMaxCpuUsageInCluster(clusterMetrics)
		if err != nil {
			return fmt.Errorf("getting max cpu usage in cluster: %w", err)
		}
		utils.PrintAndLog("PARALLELISM: max cpu usage in cluster = %d", maxCpuUsage)

		if maxCpuUsage > MAX_CPU_THRESHOLD {
			utils.PrintAndLog("PARALLELISM: found CPU usage = %d > %d, reducing parallelism to %d", maxCpuUsage, MAX_CPU_THRESHOLD, yb.GetNumConnectionsInPool()-1)
			err = yb.UpdateNumConnectionsInPool(-1)
			if err != nil {
				utils.PrintAndLog("PARALLELISM: error updating parallelism: %v", err)
			}
		} else {
			utils.PrintAndLog("PARALLELISM: found CPU usage = %d <= %d, increasing parallelism to %d", maxCpuUsage, MAX_CPU_THRESHOLD, yb.GetNumConnectionsInPool()+1)
			err := yb.UpdateNumConnectionsInPool(1)
			if err != nil {
				utils.PrintAndLog("PARALLELISM: error updating parallelism: %v", err)
			}
		}
		time.Sleep(10 * time.Second)
	}
	return nil
}

func getMaxCpuUsageInCluster(clusterMetrics map[string]map[string]string) (int, error) {
	var maxCpuPct int
	for _, nodeMetrics := range clusterMetrics {
		cpuUsageUser, err := strconv.ParseFloat(nodeMetrics[CPU_USAGE_USER], 64)
		if err != nil {
			return -1, fmt.Errorf("parsing cpu usage user as float: %w", err)
		}
		cpuUsageSystem, err := strconv.ParseFloat(nodeMetrics[CPU_USAGE_SYSTEM], 64)
		if err != nil {
			return -1, fmt.Errorf("parsing cpu usage system as float: %w", err)
		}

		cpuUsagePct := int((cpuUsageUser + cpuUsageSystem) * 100)
		if cpuUsagePct > maxCpuPct {
			maxCpuPct = cpuUsagePct
		}
	}
	return maxCpuPct, nil
}
