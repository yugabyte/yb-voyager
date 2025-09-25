//go:build integration

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
package tgtdb

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"testing"

	_ "github.com/godror/godror"
	_ "github.com/jackc/pgx/v5/stdlib"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

type TestDB struct {
	testcontainers.TestContainer
	TargetDB
}

// TestTargetYugabyteDB embeds TargetYugabyteDB and overrides GetYBServers for testing
type TestTargetYugabyteDBCluster struct {
	*TargetYugabyteDB
	*testcontainers.YugabyteDBClusterContainer
}

var (
	testPostgresTarget          *TestDB
	testOracleTarget            *TestDB
	testYugabyteDBTarget        *TestDB
	testYugabyteDBTargetCluster *TestTargetYugabyteDBCluster
)

func TestMain(m *testing.M) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// 1. Create a postgres container
	postgresContainer := testcontainers.NewTestContainer("postgresql", nil)
	err := postgresContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start postgres container: %v", err)
	}
	host, port, err := postgresContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testPostgresTarget = &TestDB{
		TestContainer: postgresContainer,
		TargetDB: NewTargetDB(&TargetConf{
			TargetDBType: "postgresql",
			DBVersion:    postgresContainer.GetConfig().DBVersion,
			User:         postgresContainer.GetConfig().User,
			Password:     postgresContainer.GetConfig().Password,
			Schema:       postgresContainer.GetConfig().Schema,
			DBName:       postgresContainer.GetConfig().DBName,
			Host:         host,
			Port:         port,
			SSLMode:      "disable",
		}),
	}

	err = testPostgresTarget.Init()
	if err != nil {
		utils.ErrExit("Failed to connect to postgres database: %w", err)
	}
	defer testPostgresTarget.Finalize()

	// 2. Create a oracle container
	oracleContainer := testcontainers.NewTestContainer("oracle", nil)
	_ = oracleContainer.Start(ctx)
	host, port, err = oracleContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testOracleTarget = &TestDB{
		TestContainer: oracleContainer,
		TargetDB: NewTargetDB(&TargetConf{
			TargetDBType: "oracle",
			DBVersion:    oracleContainer.GetConfig().DBVersion,
			User:         oracleContainer.GetConfig().User,
			Password:     oracleContainer.GetConfig().Password,
			Schema:       oracleContainer.GetConfig().Schema,
			DBName:       oracleContainer.GetConfig().DBName,
			Host:         host,
			Port:         port,
		}),
	}

	err = testOracleTarget.Init()
	if err != nil {
		utils.ErrExit("Failed to connect to oracle database: %w", err)
	}
	defer testOracleTarget.Finalize()

	yugabytedbContainer := testcontainers.NewTestContainer("yugabytedb", nil)
	err = yugabytedbContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start yugabytedb container: %v", err)
	}
	host, port, err = yugabytedbContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testYugabyteDBTarget = &TestDB{
		TestContainer: yugabytedbContainer,
		TargetDB: NewTargetDB(&TargetConf{
			TargetDBType: "yugabytedb",
			DBVersion:    yugabytedbContainer.GetConfig().DBVersion,
			User:         yugabytedbContainer.GetConfig().User,
			Password:     yugabytedbContainer.GetConfig().Password,
			Schema:       yugabytedbContainer.GetConfig().Schema,
			DBName:       yugabytedbContainer.GetConfig().DBName,
			Host:         host,
			Port:         port,
		}),
	}

	// 3. Create a yugabytedb single node container
	err = testYugabyteDBTarget.Init()
	if err != nil {
		utils.ErrExit("Failed to connect to yugabytedb database: %w", err)
	}
	defer testYugabyteDBTarget.Finalize()

	// 4. Create a yugabytedb 3 node cluster container
	yugabytedbClusterContainer := testcontainers.NewYugabyteDBCluster(&testcontainers.ContainerConfig{
		DBType:            testcontainers.YUGABYTEDB,
		NodeCount:         3,
		ReplicationFactor: 3,
	})
	err = yugabytedbClusterContainer.Start(ctx)
	if err != nil {
		utils.ErrExit("Failed to start yugabytedb cluster: %v", err)
	}
	host, port, err = yugabytedbClusterContainer.GetHostPort()
	if err != nil {
		utils.ErrExit("%v", err)
	}
	testYugabyteDBTargetCluster = &TestTargetYugabyteDBCluster{
		YugabyteDBClusterContainer: yugabytedbClusterContainer,
		TargetYugabyteDB: newTargetYugabyteDB(&TargetConf{
			TargetDBType: "yugabytedb",
			DBVersion:    yugabytedbClusterContainer.GetConfig().DBVersion,
			User:         yugabytedbClusterContainer.GetConfig().User,
			Password:     yugabytedbClusterContainer.GetConfig().Password,
			Schema:       yugabytedbClusterContainer.GetConfig().Schema,
			DBName:       yugabytedbClusterContainer.GetConfig().DBName,
			Host:         host,
			Port:         port,
			SSLMode:      "disable",
		}),
	}
	err = testYugabyteDBTargetCluster.Init()
	if err != nil {
		utils.ErrExit("Failed to connect to yugabytedb cluster: %w", err)
	}
	defer testYugabyteDBTargetCluster.Finalize()

	// to avoid info level logs flooding the test output
	log.SetLevel(log.WarnLevel)

	exitCode := m.Run()

	// cleaning up all the running containers
	testcontainers.TerminateAllContainers()

	os.Exit(exitCode)
}

// Override GetYBServers to return actual cluster node configurations for testing
// because docker yugabyte cluster setup doesn't expose external reachable IPs in yb_servers()
func (tyb *TestTargetYugabyteDBCluster) GetYBServers() (bool, []*TargetConf, error) {
	hostPorts, err := tyb.GetHostPorts()
	if err != nil {
		return false, nil, fmt.Errorf("failed to get cluster host ports: %w", err)
	}

	var tconfs []*TargetConf
	for _, hostPort := range hostPorts {
		parts := strings.Split(hostPort, ":")
		if len(parts) != 2 {
			return false, nil, fmt.Errorf("invalid host:port format: %s", hostPort)
		}

		host := parts[0]
		port, err := strconv.Atoi(parts[1])
		if err != nil {
			return false, nil, fmt.Errorf("invalid port in %s: %w", hostPort, err)
		}

		// Create TargetConf for this node
		clone := tyb.Tconf.Clone()
		clone.Host = host
		clone.Port = port
		clone.Uri = getCloneConnectionUri(clone) // rebuild this other same connection will be made
		tconfs = append(tconfs, clone)
	}

	// Return false for loadBalancerUsed and the actual cluster node configs
	return false, tconfs, nil
}
