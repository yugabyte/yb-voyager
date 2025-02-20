package testcontainers

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/docker/go-connections/nat"
	log "github.com/sirupsen/logrus"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type OracleContainer struct {
	ContainerConfig
	container testcontainers.Container
}

func (ora *OracleContainer) Start(ctx context.Context) (err error) {
	if ora.container != nil && ora.container.IsRunning() {
		utils.PrintAndLog("Oracle-%s container already running", ora.DBVersion)
		return nil
	}

	// since these Start() can be called from anywhere so need a way to ensure that correct files(without needing abs path) are picked from project directories
	tmpFile, err := os.CreateTemp(os.TempDir(), "oracle_schema.sql")
	if err != nil {
		return fmt.Errorf("failed to create temp schema file: %w", err)
	}
	defer tmpFile.Close()

	if _, err := tmpFile.Write(oracleInitSchemaFile); err != nil {
		return fmt.Errorf("failed to write to temp schema file: %w", err)
	}

	// refer: https://hub.docker.com/r/gvenzl/oracle-xe
	req := testcontainers.ContainerRequest{
		// TODO: verify the docker images being used are the correct/certified ones (No license issue)
		Image:        fmt.Sprintf("gvenzl/oracle-xe:%s", ora.DBVersion),
		ExposedPorts: []string{"1521/tcp"},
		Env: map[string]string{
			"ORACLE_PASSWORD":   ora.Password, // for SYS user
			"ORACLE_DATABASE":   ora.DBName,
			"APP_USER":          ora.User,
			"APP_USER_PASSWORD": ora.Password,
		},
		WaitingFor: wait.ForLog("DATABASE IS READY TO USE").WithStartupTimeout(2 * time.Minute).WithPollInterval(5 * time.Second),
		Files: []testcontainers.ContainerFile{
			{
				HostFilePath:      tmpFile.Name(),
				ContainerFilePath: "docker-entrypoint-initdb.d/oracle_schema.sql",
				FileMode:          0755,
			},
		},
	}

	ora.container, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	printContainerLogs(ora.container)
	if err != nil {
		return fmt.Errorf("failed to start oracle container: %w", err)
	}

	err = pingDatabase("godror", ora.GetConnectionString())
	if err != nil {
		return fmt.Errorf("failed to ping oracle container: %w", err)
	}
	return nil
}

func (ora *OracleContainer) Terminate(ctx context.Context) {
	if ora == nil {
		return
	}

	err := ora.container.Terminate(ctx)
	if err != nil {
		log.Errorf("failed to terminate oracle container: %v", err)
	}
}

func (ora *OracleContainer) GetHostPort() (string, int, error) {
	if ora.container == nil {
		return "", -1, fmt.Errorf("oracle container is not started: nil")
	}

	ctx := context.Background()
	host, err := ora.container.Host(ctx)
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch host for oracle container: %w", err)
	}

	port, err := ora.container.MappedPort(ctx, nat.Port(DEFAULT_ORACLE_PORT))
	if err != nil {
		return "", -1, fmt.Errorf("failed to fetch mapped port for oracle container: %w", err)
	}

	return host, port.Int(), nil
}

func (ora *OracleContainer) GetConfig() ContainerConfig {
	return ora.ContainerConfig
}

func (ora *OracleContainer) GetConnectionString() string {
	config := ora.GetConfig()
	host, port, err := ora.GetHostPort()
	if err != nil {
		utils.ErrExit("failed to get host port for oracle connection string: %v", err)
	}

	connectString := fmt.Sprintf(`(DESCRIPTION = (ADDRESS = (PROTOCOL = TCP)(HOST = %s)(PORT = %d))(CONNECT_DATA = (SERVICE_NAME = %s)))`,
		host, port, config.DBName)
	return fmt.Sprintf(`user="%s" password="%s" connectString="%s"`, config.User, config.Password, connectString)
}

func (ora *OracleContainer) ExecuteSqls(sqls ...string) {

}
