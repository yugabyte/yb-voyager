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
package monitor

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/fatih/color"
	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	MONITOR_HEALTH_FREQUENCY_SECONDS = 15
	METRIC_FREE_DISK_SPACE           = "free_disk_space"
	METRIC_TOTAL_DISK_SPACE          = "total_disk_space"
	FREE_DISK_SPACE_THREASHOLD       = 10
	REPLICATION_GUARDRAIL_ALERT_MSG  = "Replication is enabled on the cluster, enable it only once the import-data is done ingesting data to use the disk only for the data import operation and keep the cluster stable."
)

type TargetDBForMonitorHealth interface {
	GetYBServers() (bool, []*tgtdb.TargetConf, error)
	RemoveConnectionsForHosts([]string) error
	NumOfLogicalReplicationSlots() (int64, error)
	GetClusterMetrics() (map[string]tgtdb.NodeMetrics, error) // node_uuid:metric_name:metric_value
}

// bool variable is for indicating if a node is down or up
var nodeStatuses map[string]bool

func MonitorTargetHealth(yb TargetDBForMonitorHealth, metaDB *metadb.MetaDB, skipDiskUsageHealthChecks utils.BoolStr, skipReplicationChecks utils.BoolStr, skipNodeHealthChecks utils.BoolStr, displayAlertFunc func(alertMsg string)) error {
	nodeStatuses = make(map[string]bool)

	var err error
	loadBalancerEnabled, servers, err := yb.GetYBServers()
	if err != nil {
		return fmt.Errorf("error fetching servers informtion: %v", err)
	}
	for _, server := range servers {
		nodeStatuses[server.Host] = true
	}

	var nodeAlert string
	for {
		nodeAlert, err = monitorNodesStatusAndAdapt(yb, loadBalancerEnabled, skipNodeHealthChecks)
		if err != nil {
			log.Errorf("error monitoring the node status and adapt: %v", err)
		}

		//skipping the disk usage monitoring for now, util the metrics function changes are made
		err = monitorDiskUsageAndAbort(yb, true)
		if err != nil {
			log.Errorf("error monitoring the disk and memory: %v", err)
		}

		err = monitorReplicationOnTarget(yb, skipReplicationChecks)
		if err != nil {
			log.Errorf("error monitoring the replication: %v", err)
		}

		if nodeAlert != "" {
			alertMsg := color.RedString(fmt.Sprintf("Alert!\n%s\n", nodeAlert))
			displayAlertFunc(alertMsg)
		}
		time.Sleep(time.Duration(MONITOR_HEALTH_FREQUENCY_SECONDS) * time.Second)
	}
}

/*
1. if node goes down
remove its connections from pool and if any current importBatch is running it will fail and automatic retry will take care of using the rest conns

2. if node come back
the new connections will automatically go to that node -- handled via conn-pool

3. load balancer on the cluster
No node status checks will happen as we put the load on the load balancer IP and it will take care of this adaptation automatically

*/
func monitorNodesStatusAndAdapt(yb TargetDBForMonitorHealth, loadBalancerEnabled bool, skip utils.BoolStr) (string, error) {
	if bool(skip) || loadBalancerEnabled {
		return "", nil
	}
	_, currentNodes, err := yb.GetYBServers()
	if err != nil {
		return "", fmt.Errorf("error fetching servers informtion: %v", err)
	}
	downNodes := make([]string, 0)

	for node, up := range nodeStatuses {
		if up && !lo.ContainsBy(currentNodes, func(t *tgtdb.TargetConf) bool {
			return t.Host == node
		}) {
			downNodes = append(downNodes, node)
		}
	}
	for _, conf := range currentNodes {
		//Add the node in the main nodeStatuses initialised in the starting
		//which weren't present the starting and during the run it came back.
		if !slices.Contains(lo.Keys(nodeStatuses), conf.Host) {
			nodeStatuses[conf.Host] = true
		}
	}
	if len(downNodes) > 0 {
		downNodeMsg := fmt.Sprintf("Following nodes are not healthy, please check them - [%v]", strings.Join(downNodes, ", "))
		err := yb.RemoveConnectionsForHosts(downNodes)
		if err != nil {
			return downNodeMsg, fmt.Errorf("error while removing the connections for the down nodes[%v]: %v", downNodes, err)
		}
		return downNodeMsg, nil
	}
	return "", nil
}

/*
If disk space on any node is less than 10%, error out the import and let the user increase the disk

//TODO: enhance yb_server_metrics() to give these metrics.
*/
func monitorDiskUsageAndAbort(yb TargetDBForMonitorHealth, skip utils.BoolStr) error {
	if skip {
		return nil
	}
	nodeMetrics, err := yb.GetClusterMetrics()
	if err != nil {
		return fmt.Errorf("error fetching cluster metrics: %v", err)
	}
	var totalFreeDisk, totalDisk int64
	for _, metrics := range nodeMetrics {
		if metrics.Status != "OK" {
			continue
		}
		nodeFreeDisk, err := strconv.ParseInt(metrics.Metrics[METRIC_FREE_DISK_SPACE], 10, 64)
		if err != nil {
			return fmt.Errorf("parsing free disk space metric as int: %w", err)
		}
		nodeTotalDisk, err := strconv.ParseInt(metrics.Metrics[METRIC_TOTAL_DISK_SPACE], 10, 64)
		if err != nil {
			return fmt.Errorf("parsing total disk space metric as int: %w", err)
		}
		totalFreeDisk += nodeFreeDisk
		totalDisk += nodeTotalDisk
		freeDiskPct := (nodeFreeDisk * 100) / nodeTotalDisk
		if freeDiskPct < FREE_DISK_SPACE_THREASHOLD {
			utils.ErrExit("Free disk space available on the target cluster is low, increase the disk space on the target cluster and re-run the import.")
		}
	}
	return nil
}

/*
If any of the replication streams are enabled on the cluster during the import -
let the user know to enable it after import data is done to avoid wal or disk space issues because of replication

Caveats -
1. For all deployments - only logical replication info is easily detectable - using the pg_replciation_slots for getting num of logical replication slots
2. For YBA/yugabyted - with above also trying to figure the xcluster replication streams/CDC gRPC streams using the yb-client wrapper jar to fetch num of all cdc streams on the cluster
*/
func monitorReplicationOnTarget(yb TargetDBForMonitorHealth, skip utils.BoolStr) error {
	if skip {
		return nil
	}
	numOfSlots, err := yb.NumOfLogicalReplicationSlots()
	if err != nil {
		return fmt.Errorf("error fetching logical replication slots for checking if replication enabled - %s", err)
	}
	ybServers := lo.Keys(nodeStatuses)
	addresses := strings.Join(ybServers, ":7100,")
	// no need to specify the empty params as we need to only get num of cdc streams
	ybClient := dbzm.NewYugabyteDBCDCClient("", addresses, "", "", "", nil)
	err = ybClient.Init()
	if err != nil {
		return fmt.Errorf("failed to initialize YugabyteDB CDC client: %w", err)
	}
	numOfStreams, err := ybClient.GetNumOfReplicationStreams()
	if err != nil {
		return fmt.Errorf("error fetching num of replication streams: %v", err)
	}
	if numOfSlots > 0 || numOfStreams > 0 {
		utils.ErrExit(color.RedString(REPLICATION_GUARDRAIL_ALERT_MSG))
	}
	return nil
}
