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
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/vbauerster/mpb/v8"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	MONITOR_HEALTH_FREQUENCY_SECONDS = 10
	METRIC_FREE_DISK_SPACE           = "free_disk_space"
	METRIC_TOTAL_DISK_SPACE          = "total_disk_space"
	FREE_DISK_SPACE_THREASHOLD       = 10
)

type TargetDBForMonitorHealth interface {
	GetYBServers() (bool, []*tgtdb.TargetConf, error)
	RemoveConnectionsForHosts([]string) error
	NumOfLogicalReplicationSlots() (int64, error)
	GetClusterMetrics() (map[string]tgtdb.NodeMetrics, error) // node_uuid:metric_name:metric_value
}

// bool variable is for indicating if a node is down or up
var nodeStatuses map[string]bool
var monitorProgress *mpb.Progress
var monitorBar *mpb.Bar

func MonitorTargetHealth(yb TargetDBForMonitorHealth, metaDB *metadb.MetaDB) error {
	nodeStatuses = make(map[string]bool)
	loadBalancerEnabled, servers, err := yb.GetYBServers()
	if err != nil {
		return fmt.Errorf("error fetching servers informtion: %v", err)
	}
	for _, server := range servers {
		nodeStatuses[server.Host] = true
	}
	for {
		nodeAlert, err := monitorNodesStatusAndAbort(yb, loadBalancerEnabled)
		if err != nil {
			utils.ErrExit("Following nodes are not healthy, please check and fix the issue and re-run the import\n %v", err)
		}
		err = monitorDiskAndMemoryStatusAndAbort(yb)
		if err != nil {
			log.Infof("error monitoring the disk and memory - %v", err)
		}
		replicationAlert, err := monitorReplicationOnTarget(yb)
		if err != nil {
			log.Infof("error monitoring the replication - %v", err)
		}
		if nodeAlert != "" || replicationAlert != "" {
			err = metadb.UpdateJsonObjectInMetaDB(metaDB, metadb.MONITOR_TARGET_HEALTH_KEY, func(s *string) {
				*s = fmt.Sprintf("Alert!\n %s", strings.Join([]string{nodeAlert, replicationAlert}, "\n"))
			})
			if err != nil {
				utils.ErrExit("error pfsdjkfhskhdfk")
			}
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
func monitorNodesStatusAndAbort(yb TargetDBForMonitorHealth, loadBalancerEnabled bool) (string, error) {
	if loadBalancerEnabled {
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
	if len(downNodes) > 0 {
		err := yb.RemoveConnectionsForHosts(downNodes)
		if err != nil {
			return "", fmt.Errorf("error while re-initialising the conn pool: %v", err)
		}
		return fmt.Sprintf("Following nodes are not healthy, please check them - [%v].", strings.Join(downNodes, ", ")), nil
	}
	return "", nil
}

/*
If disk space on any node is less than 10%, error out the import and let the user increase the disk

//TODO: enhance yb_server_metrics() to give these metrics.
*/
func monitorDiskAndMemoryStatusAndAbort(yb TargetDBForMonitorHealth) error {
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
			return fmt.Errorf("parsing tserver root memory soft limit as int: %w", err)
		}
		nodeTotalDisk, err := strconv.ParseInt(metrics.Metrics[METRIC_TOTAL_DISK_SPACE], 10, 64)
		if err != nil {
			return fmt.Errorf("parsing tserver root memory soft limit as int: %w", err)
		}
		totalFreeDisk += nodeFreeDisk
		totalDisk += nodeTotalDisk
		freeDiskPct := (nodeFreeDisk * 100) / nodeTotalDisk
		if freeDiskPct < FREE_DISK_SPACE_THREASHOLD {
			utils.ErrExit("Free Disk space available on the node is low, increase the disk space on the node and re-run the import.")
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
func monitorReplicationOnTarget(yb TargetDBForMonitorHealth) (string, error) {
	numOfSlots, err := yb.NumOfLogicalReplicationSlots()
	if err != nil {
		return "", fmt.Errorf("error fetching logical replication slots for checking if replication enabled - %s", err)
	}
	ybServers := lo.Keys(nodeStatuses)
	addresses := strings.Join(ybServers, ":7100,")
	// no need to specify the empty params as we need to only get num of cdc streams
	ybClient := dbzm.NewYugabyteDBCDCClient("", addresses, "", "", "", nil)
	err = ybClient.Init()
	if err != nil {
		return "", fmt.Errorf("failed to initialize YugabyteDB CDC client: %w", err)
	}
	numOfStreams, err := ybClient.GetNumOfReplicationStreams()
	if err != nil {
		return "", fmt.Errorf("error fetching num of replication streams: %v", err)
	}
	if numOfSlots > 0 || numOfStreams > 0 {
		return "Logical Replication is enabled on the cluster, enable it only once the import-data is done ingesting to keep the cluster stable.", nil
	}
	return "", nil
}
