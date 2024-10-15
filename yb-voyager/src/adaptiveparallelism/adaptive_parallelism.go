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
)

const (
	CPU_USAGE_USER                 = "cpu_usage_user"
	CPU_USAGE_SYSTEM               = "cpu_usage_system"
	MAX_CPU_THRESHOLD              = 70
	ADAPTIVE_PARALLELISM_FREQUENCY = 10 * time.Second
)

type TargetYugabyteDBWithConnectionPool interface {
	IsAdaptiveParallelismSupported() bool
	GetClusterMetrics() (map[string]tgtdb.NodeMetrics, error) // node_uuid:metric_name:metric_value
	GetNumConnectionsInPool() int
	GetNumMaxConnectionsInPool() int
	UpdateNumConnectionsInPool(int) error // (delta)
}

var AdaptiveParallelismNotSupportedError = fmt.Errorf("adaptive parallelism not supported in target YB database")

func AdaptParallelism(yb TargetYugabyteDBWithConnectionPool) error {
	if !yb.IsAdaptiveParallelismSupported() {
		return AdaptiveParallelismNotSupportedError
	}
	for {
		time.Sleep(ADAPTIVE_PARALLELISM_FREQUENCY)
		err := fetchClusterMetricsAndUpdateParallelism(yb)
		if err != nil {
			log.Warnf("adaptive: error updating parallelism: %v", err)
		}
	}
}

func fetchClusterMetricsAndUpdateParallelism(yb TargetYugabyteDBWithConnectionPool) error {
	clusterMetrics, err := yb.GetClusterMetrics()
	log.Infof("adaptive: clusterMetrics: %v", spew.Sdump(clusterMetrics)) // TODO: move to debug?
	if err != nil {
		return fmt.Errorf("getting cluster metrics: %w", err)
	}

	// get max CPU
	// Note that right now, voyager ingests data into the target in parallel,
	// but one table at a time. Therefore, in cases where there is a single tablet for a table,
	// either due to pre-split or colocated table, it is possible that the load on the cluster
	// will be uneven. Nevertheless, we still want to ensure that the cluster is not overloaded,
	// therefore we use the max CPU usage across all nodes in the cluster.
	maxCpuUsage, err := getMaxCpuUsageInCluster(clusterMetrics)
	if err != nil {
		return fmt.Errorf("getting max cpu usage in cluster: %w", err)
	}
	if maxCpuUsage < 0 {
		log.Warnf("adaptive: Ignoring update as max cpu usage in cluster is negative: %d", maxCpuUsage)
	}
	log.Infof("adaptive: max cpu usage in cluster = %d", maxCpuUsage)

	if maxCpuUsage > MAX_CPU_THRESHOLD {
		log.Infof("adaptive: found CPU usage = %d > %d, reducing parallelism to %d", maxCpuUsage, MAX_CPU_THRESHOLD, yb.GetNumConnectionsInPool()-1)
		err = yb.UpdateNumConnectionsInPool(-1)
		if err != nil {
			return fmt.Errorf("updating parallelism with -1: %w", err)
		}
	} else {
		log.Infof("adaptive: found CPU usage = %d <= %d, increasing parallelism to %d", maxCpuUsage, MAX_CPU_THRESHOLD, yb.GetNumConnectionsInPool()+1)
		err := yb.UpdateNumConnectionsInPool(1)
		if err != nil {
			return fmt.Errorf("updating parallelism with +1 : %w", err)
		}
	}
	return nil
}

func getMaxCpuUsageInCluster(clusterMetrics map[string]tgtdb.NodeMetrics) (int, error) {
	maxCpuPct := -1
	for _, nodeMetrics := range clusterMetrics {
		if nodeMetrics.Status != "OK" {
			continue
		}
		cpuUsageUser, err := strconv.ParseFloat(nodeMetrics.Metrics[CPU_USAGE_USER], 64)
		if err != nil {
			return -1, fmt.Errorf("parsing cpu usage user as float: %w", err)
		}
		cpuUsageSystem, err := strconv.ParseFloat(nodeMetrics.Metrics[CPU_USAGE_SYSTEM], 64)
		if err != nil {
			return -1, fmt.Errorf("parsing cpu usage system as float: %w", err)
		}

		cpuUsagePct := int((cpuUsageUser + cpuUsageSystem) * 100)
		maxCpuPct = max(maxCpuPct, cpuUsagePct)
	}
	return maxCpuPct, nil
}
