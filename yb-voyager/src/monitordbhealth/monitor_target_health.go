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
package monitordbhealth

import (
	"fmt"
	"strings"
	"time"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	MONITOR_HEALTH_FREQUENCY_SECONDS = 10
)

type TargetDBForMonitorHealth interface {
	GetYBServers() (bool, []*tgtdb.TargetConf, error)
	RemoveConnectionsForHosts([]string) error
	NumOfReplicationSlots() (int64, error)
	GetClusterMetrics() (map[string]tgtdb.NodeMetrics, error) // node_uuid:metric_name:metric_value
}

// bool variable is for indicating if a node is down or up
var nodeStatuses map[string]bool

func MonitorTargetHealth(yb TargetDBForMonitorHealth) error {
	nodeStatuses = make(map[string]bool)
	loadBalancerEnabled, servers, err := yb.GetYBServers()
	if err != nil {
		return fmt.Errorf("error fetching servers informtion: %v", err)
	}
	for _, server := range servers {
		nodeStatuses[server.Host] = true
	}
	for {
		time.Sleep(time.Duration(MONITOR_HEALTH_FREQUENCY_SECONDS) * time.Second)
		err := monitorNodesStatusAndAbort(yb, loadBalancerEnabled)
		if err != nil {
			utils.ErrExit("Following nodes are not healthy, please check and fix the issue and re-run the import\n %v", err)
		}
		err = monitorDiskAndMemoryStatusAndAbort(yb)
		if err != nil {
			log.Infof("error monitoring the disk and memory - %v", err)
		}
		numOfSlots, err := yb.NumOfReplicationSlots()
		if err != nil {
			log.Infof("error fetching logical replication slots for checking if replication enabled - %s", err)
		}
		if numOfSlots > 0 {
			utils.PrintAndLog("Logical Replciation is enabled on the database, enable it only once the voyager is done ingesting to keep the cluster stable")
		}
	}
}

/*
if node goes down
 1. remove its connections from pool and if any current importBAtch is running that it will fail and automatic retry will take of using the rest conns
 2. anything with adaptive num paraa?

if node come back

 1. add some connections for tha node in pool

    load balaner ??
*/
func monitorNodesStatusAndAbort(yb TargetDBForMonitorHealth, loadBalancerEnabled bool) error {
	if loadBalancerEnabled {
		return nil
	}
	_, currentNodes, err := yb.GetYBServers()
	if err != nil {
		return fmt.Errorf("error fetching servers informtion: %v", err)
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
		utils.PrintAndLog("Following nodes are not healthy, please check them - [%v]", strings.Join(downNodes, ", "))
		err := yb.RemoveConnectionsForHosts(downNodes)
		if err != nil {
			utils.PrintAndLog("error while re-initialising the conn pool: %v", err)
		}
	}
	return nil
}

func monitorDiskAndMemoryStatusAndAbort(yb TargetDBForMonitorHealth) error {
	//
	return nil
}