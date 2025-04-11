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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	MONITOR_HEALTH_FREQUENCY_SECONDS = 5 // 2 mins interval for monitoring the cluster checks
	METRIC_FREE_DISK_SPACE           = "free_disk_space"
	METRIC_TOTAL_DISK_SPACE          = "total_disk_space"
	FREE_DISK_SPACE_THREASHOLD       = 10
	REPLICATION_GUARDRAIL_ALERT_MSG  = "It is NOT recommended to have any form of replication (CDC/xCluster) running on the target YugabyteDB cluster during data import. If you still wish to proceed with replication, use the '--skip-replication-checks true' flag to disable the check."
	NODE_GOES_DOWN_MSG               = "Nodes in the target YugabyteDB cluster are unhealthy, continuing data import using the available healthy nodes. Please check them on the target cluster"
	NODE_COMES_BACK_UP_MSG           = "Previously unhealthy nodes in the target YugabyteDB cluster have recovered"
)

type TargetDBForMonitorHealth interface {
	GetYBServers() (bool, []*tgtdb.TargetConf, error)
	RemoveConnectionsForHosts([]string) error
	NumOfLogicalReplicationSlots() (int64, error)
	GetClusterMetrics() (map[string]tgtdb.NodeMetrics, error) // node_uuid:metric_name:metric_value
}

type YugabyteDBClient interface {
	Init() error
	SetYBServers(string)
	SetSSLRootCert(string)
	GetNumOfReplicationStreams() (int, error)
}

type MonitorTargetYBHealth struct {
	// key is the target cluster node and value is bool variable for indicating if a node is down or up
	nodesStatus map[string]bool
	//ybClient instance to fetch streams
	ybClient dbzm.YugabyteDBCDCClient
	//yb tdb instance to query target for monitoring
	yb TargetDBForMonitorHealth

	//Skip booleans as per flags
	skipNodeCheck        bool
	skipDiskCheck        bool
	skipReplicationCheck bool

	//Display function for the printing any msg on the console
	displayMsgFunc func(string)

	tconf tgtdb.TargetConf
}

func NewMonitorTargetYBHealth(yb TargetDBForMonitorHealth, skipDiskUsageHealthChecks utils.BoolStr, skipReplicationChecks utils.BoolStr, skipNodeHealthChecks utils.BoolStr, displayMsgFunc func(msg string), ybClient YugabyteDBClient, tconf tgtdb.TargetConf) MonitorTargetYBHealth {
	return MonitorTargetYBHealth{
		nodesStatus:          make(map[string]bool),
		yb:                   yb,
		skipNodeCheck:        bool(skipNodeHealthChecks),
		skipDiskCheck:        bool(skipDiskUsageHealthChecks),
		skipReplicationCheck: bool(skipReplicationChecks),
		displayMsgFunc:       displayMsgFunc,
		tconf:                tconf,
	}
}

func (m *MonitorTargetYBHealth) StartMonitoring() error {

	var err error
	loadBalancerEnabled, servers, err := m.yb.GetYBServers()
	if err != nil {
		return fmt.Errorf("error fetching servers informtion: %v", err)
	}
	for _, server := range servers {
		m.nodesStatus[server.Host] = true
	}

	if loadBalancerEnabled {
		m.skipNodeCheck = true
	}

	//skipping the disk usage monitoring for now, util the metrics function changes are made
	m.skipDiskCheck = true

	for {
		err = m.monitorNodesStatusAndAdapt()
		if err != nil {
			log.Errorf("error monitoring the node status and adapt: %v", err)
		}

		err = m.monitorDiskUsageAndAbort()
		if err != nil {
			log.Errorf("error monitoring the disk and memory: %v", err)
		}

		err = m.monitorReplicationOnTarget()
		if err != nil {
			log.Errorf("error monitoring the replication: %v", err)
		}
		time.Sleep(time.Duration(MONITOR_HEALTH_FREQUENCY_SECONDS) * time.Second)
	}
}

/*
1. if node goes down
remove its connections from pool and if any current importBatch is running it will fail and automatic retry will take care of using the rest conns
Put a msg on the console that node went down continuing import witht rest live nodes
and not monitoring that node until it comes back up so the msg on the console will be printed once only.

2. if node come back
the new connections will automatically go to that node -- handled via conn-pool
Put a msg on the console that node came back and now monitoring that node again to see if it goes down again

3. load balancer on the cluster
No node status checks will happen as we put the load on the load balancer IP and it will take care of this adaptation automatically
*/
func (m *MonitorTargetYBHealth) monitorNodesStatusAndAdapt() error {
	if m.skipNodeCheck {
		return nil
	}
	_, currentNodes, err := m.yb.GetYBServers()
	if err != nil {
		return fmt.Errorf("error fetching servers informtion: %v", err)
	}
	downNodes := make([]string, 0)
	upNodes := make([]string, 0)

	for node, up := range m.nodesStatus {
		isNodeLive := lo.ContainsBy(currentNodes, func(t *tgtdb.TargetConf) bool {
			return t.Host == node
		})
		if up && !isNodeLive {
			//Node goes down
			downNodes = append(downNodes, node)
			m.nodesStatus[node] = false
		}

		if !up && isNodeLive {
			//Node comes back up
			upNodes = append(upNodes, node)
			m.nodesStatus[node] = true
		}
	}
	for _, conf := range currentNodes {
		//Add the node in the main nodeStatuses initialised in the starting
		//which weren't present the starting and during the run it came back.
		if !slices.Contains(lo.Keys(m.nodesStatus), conf.Host) {
			m.nodesStatus[conf.Host] = true
		}
	}

	if len(downNodes) > 0 {
		downNodeMsg := color.RedString(fmt.Sprintf("ALERT: %s. Unhealthy nodes: %v\n", NODE_GOES_DOWN_MSG, strings.Join(downNodes, ", ")))
		m.displayMsgFunc(downNodeMsg)
		err := m.yb.RemoveConnectionsForHosts(downNodes)
		if err != nil {
			return fmt.Errorf("error while removing the connections for the down nodes[%v]: %v", downNodes, err)
		}
		return nil
	}
	if len(upNodes) > 0 {
		upNodeMsg := color.GreenString(fmt.Sprintf("OK: %s. Healthy nodes: %v\n", NODE_COMES_BACK_UP_MSG, strings.Join(upNodes, ", ")))
		m.displayMsgFunc(upNodeMsg)
	}
	return nil
}

/*
If disk space on any node is less than 10%, error out the import and let the user increase the disk

//TODO: enhance yb_server_metrics() to give these metrics.
*/
func (m *MonitorTargetYBHealth) monitorDiskUsageAndAbort() error {
	if m.skipDiskCheck {
		return nil
	}
	nodeMetrics, err := m.yb.GetClusterMetrics()
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
func (m *MonitorTargetYBHealth) monitorReplicationOnTarget() error {
	if m.skipReplicationCheck {
		return nil
	}
	numOfSlots, err := m.yb.NumOfLogicalReplicationSlots()
	if err != nil {
		return fmt.Errorf("error fetching logical replication slots for checking if replication enabled - %s", err)
	}
	if numOfSlots > 0 {
		utils.ErrExit(color.RedString(REPLICATION_GUARDRAIL_ALERT_MSG))
	}

	err = m.ybClient.Init()
	if err != nil {
		return fmt.Errorf("error intialising the yb client : %v", err)
	}
	ybServers := lo.Keys(m.nodesStatus)
	addresses := strings.Join(ybServers, ":7100,")

	m.ybClient.SetYBServers(addresses)
	m.ybClient.SetSSLRootCert(m.tconf.SSLRootCert)

	numOfStreams, err := m.ybClient.GetNumOfReplicationStreams()
	if err != nil {
		return fmt.Errorf("error fetching num of replication streams: %v", err)
	}
	if numOfStreams > 0 {
		utils.ErrExit(color.RedString(REPLICATION_GUARDRAIL_ALERT_MSG))
	}
	return nil
}
