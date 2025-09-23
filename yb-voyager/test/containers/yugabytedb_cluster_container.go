package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"sync"
	"time"

	"github.com/docker/go-connections/nat"
	_ "github.com/jackc/pgx/v5/stdlib"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/network"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type YugabyteDBClusterContainer struct {
	mutex sync.Mutex
	ContainerConfig
	nodes   []*YugabyteDBContainer
	network *testcontainers.DockerNetwork // docker network for nodes in cluster
}

func (cluster *YugabyteDBClusterContainer) Start(ctx context.Context) (err error) {
	cluster.mutex.Lock()
	defer cluster.mutex.Unlock()

	if len(cluster.nodes) > 0 {
		for _, node := range cluster.nodes {
			if node.container != nil && node.container.IsRunning() {
				utils.PrintAndLog("YugabyteDB cluster already running")
				return nil
			}
		}
	}

	cluster.network, err = network.New(ctx)
	if err != nil {
		return fmt.Errorf("failed to create network: %w", err)
	}

	masterNode, err := cluster.createNode(ctx, 1, "")
	if err != nil {
		return fmt.Errorf("failed to start master node: %w", err)
	}
	cluster.nodes = append(cluster.nodes, masterNode)

	// Get the master container's IP address for join operations
	masterJoinAddress, err := getContainerIPAddress(ctx, masterNode.container, cluster.network.Name)
	if err != nil {
		return fmt.Errorf("failed to get master container IP: %w", err)
	}

	for i := 2; i <= cluster.NodeCount; i++ {
		node, err := cluster.createNode(ctx, i, masterJoinAddress)
		if err != nil {
			return fmt.Errorf("failed to start node %d: %w", i, err)
		}
		cluster.nodes = append(cluster.nodes, node)
	}

	err = yugabyteClusterWait(cluster.NodeCount).WaitUntilReady(ctx, cluster.nodes[0].container)
	if err != nil {
		return fmt.Errorf("cluster not ready: %w", err)
	}

	utils.PrintAndLog("YugabyteDB cluster with %d nodes started successfully", cluster.NodeCount)
	return nil
}

func (cluster *YugabyteDBClusterContainer) createNode(ctx context.Context, nodeID int, masterJoinAddress string) (*YugabyteDBContainer, error) {
	nodeContainer := &YugabyteDBContainer{
		ContainerConfig: cluster.ContainerConfig,
	}

	cmd := []string{"bin/yugabyted", "start", "--daemon=false", "--ui=false"}
	if masterJoinAddress != "" {
		cmd = append(cmd, "--join", masterJoinAddress)
	}

	req := testcontainers.ContainerRequest{
		Image: fmt.Sprintf("yugabytedb/yugabyte:%s", cluster.DBVersion),
		ExposedPorts: []string{
			"5433/tcp",
		},
		Cmd:        cmd,
		Networks:   []string{cluster.network.Name},
		WaitingFor: yugabyteWait(),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	printContainerLogs(container)
	nodeContainer.container = container
	return nodeContainer, nil
}

func (cluster *YugabyteDBClusterContainer) Stop(ctx context.Context) error {
	cluster.mutex.Lock()
	defer cluster.mutex.Unlock()

	if len(cluster.nodes) == 0 {
		utils.PrintAndLog("YugabyteDB cluster already stopped")
		return nil
	}

	for i, node := range cluster.nodes {
		if node != nil {
			if err := node.Stop(ctx); err != nil {
				log.Errorf("failed to stop node %d: %v", i+1, err)
			}
		}
	}

	utils.PrintAndLog("YugabyteDB cluster stopped")
	return nil
}

func (cluster *YugabyteDBClusterContainer) Terminate(ctx context.Context) {
	cluster.mutex.Lock()
	defer cluster.mutex.Unlock()

	for _, node := range cluster.nodes {
		if node != nil {
			node.Terminate(ctx)
		}
	}
	cluster.nodes = nil

	if cluster.network != nil {
		if err := cluster.network.Remove(ctx); err != nil {
			log.Errorf("failed to remove network %s: %v", cluster.network.Name, err)
		}
		cluster.network = nil
	}
}

// GetHostPort returns the host and port of the master node
// for connecting to specific node, use the node specific GetHostPort()
func (cluster *YugabyteDBClusterContainer) GetHostPort() (string, int, error) {
	if len(cluster.nodes) == 0 {
		return "", -1, fmt.Errorf("cluster has no nodes")
	}
	return cluster.nodes[0].GetHostPort()
}

func (cluster *YugabyteDBClusterContainer) GetConfig() ContainerConfig {
	return cluster.ContainerConfig
}

func (cluster *YugabyteDBClusterContainer) GetConnectionString() string {
	if len(cluster.nodes) == 0 {
		utils.ErrExit("cluster has no nodes")
	}
	return cluster.nodes[0].GetConnectionString()
}

func (cluster *YugabyteDBClusterContainer) GetConnection() (*sql.DB, error) {
	if len(cluster.nodes) == 0 {
		return nil, fmt.Errorf("cluster has no nodes")
	}
	return cluster.nodes[0].GetConnection()
}

// GetNodeConnection returns a connection to a specific node in the cluster
// nodeIndex is 0-based: 0=first node (master), 1=second node, etc.
func (cluster *YugabyteDBClusterContainer) GetNodeConnection(nodeIndex int) (*sql.DB, error) {
	if len(cluster.nodes) == 0 {
		return nil, fmt.Errorf("cluster has no nodes")
	}
	if nodeIndex < 0 || nodeIndex >= len(cluster.nodes) {
		return nil, fmt.Errorf("invalid node index %d, cluster has %d nodes", nodeIndex, len(cluster.nodes))
	}
	return cluster.nodes[nodeIndex].GetConnection()
}

func (cluster *YugabyteDBClusterContainer) GetVersion() (string, error) {
	if len(cluster.nodes) == 0 {
		return "", fmt.Errorf("cluster has no nodes")
	}
	return cluster.nodes[0].GetVersion()
}

func (cluster *YugabyteDBClusterContainer) ExecuteSqls(sqls ...string) {
	if len(cluster.nodes) == 0 {
		utils.ErrExit("cluster has no nodes")
	}
	cluster.nodes[0].ExecuteSqls(sqls...)
}

func (cluster *YugabyteDBClusterContainer) Query(sql string, args ...interface{}) (*sql.Rows, error) {
	if len(cluster.nodes) == 0 {
		return nil, fmt.Errorf("cluster has no nodes")
	}
	return cluster.nodes[0].Query(sql, args...)
}

// yugabyteClusterWait returns a wait strategy that verifies cluster health
// by checking that all expected nodes are visible via yb_servers()
func yugabyteClusterWait(expectedNodes int) wait.Strategy {
	return wait.ForSQL(nat.Port("5433/tcp"), "pgx",
		func(host string, port nat.Port) string {
			return fmt.Sprintf(
				"postgres://yugabyte:password@%s:%s/yugabyte?sslmode=disable",
				host, port.Port())
		},
	).WithQuery(fmt.Sprintf("SELECT count(*) FROM yb_servers() HAVING count(*) = %d", expectedNodes)).
		WithStartupTimeout(3 * time.Minute)
}
