package testcontainers

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
)

// containerRegistry to ensure one container per database(dbtype+version) [Singleton Pattern]
// Limitation - go test spawns different process for running tests of each package, hence the containers won't be shared across packages.
var (
	containerRegistry = make(map[string]TestContainer)
	registryMutex     sync.Mutex
)

type TestContainer interface {
	// lifecycle
	Start(ctx context.Context) error
	// Stop works for pausing the container, so that it can be restarted later
	Stop(ctx context.Context) error
	Terminate(ctx context.Context)

	// connectivity and config
	GetHostPort() (string, int, error)
	GetConfig() ContainerConfig
	GetConnectionString() string
	GetConnection() (*sql.DB, error)
	GetVersion() (string, error)

	// SQL helpers
	ExecuteSqls(sqls ...string)

	/*
		TODOs
			1. // Function to run sql script for a specific test case
			   SetupSqlScript(scriptName string, dbName string) error

			2. // Add Capability to run multiple versions of a dbtype parallely
	*/
}

type ContainerConfig struct {
	DBType    string
	DBVersion string
	User      string
	Password  string
	DBName    string
	Schema    string
}

func NewTestContainer(dbType string, containerConfig *ContainerConfig) TestContainer {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	// initialise containerConfig struct if nothing is provided
	if containerConfig == nil {
		containerConfig = &ContainerConfig{}
	}
	setContainerConfigDefaultsIfNotProvided(dbType, containerConfig)

	// check if container is already created after fetching default configs
	containerName := fmt.Sprintf("%s-%s", dbType, containerConfig.DBVersion)
	if container, exists := containerRegistry[containerName]; exists {
		log.Infof("container '%s' already exists in the registry", containerName)
		return container
	}

	var testContainer TestContainer
	switch dbType {
	case POSTGRESQL:
		testContainer = &PostgresContainer{
			ContainerConfig: *containerConfig,
		}
	case YUGABYTEDB:
		testContainer = &YugabyteDBContainer{
			ContainerConfig: *containerConfig,
		}
	case ORACLE:
		testContainer = &OracleContainer{
			ContainerConfig: *containerConfig,
		}
	case MYSQL:
		testContainer = &MysqlContainer{
			ContainerConfig: *containerConfig,
		}
	default:
		panic(fmt.Sprintf("unsupported db type '%q' for creating test container\n", dbType))
	}

	containerRegistry[containerName] = testContainer
	return testContainer
}

/*
Challenges in golang for running this a teardown step
1. In golang when you execute go test in the top level folder it executes all the tests one by one.
2. Where each defined package, can have its TestMain() which can control the setup and teardown steps for that package
3. There is no way to run these before/after the tests of first/last package in codebase

Potential solution: Implement a counter(total=number_of_package) based logic to execute teardown(i.e. TerminateAllContainers() in our case)
Figure out the best solution.

For now we can rely on TestContainer ryuk(the container repear), which terminates all the containers after the process exits.
But the test framework should have capability of terminating all containers at the end.
*/
func TerminateAllContainers() {
	registryMutex.Lock()
	defer registryMutex.Unlock()

	ctx := context.Background()
	for name, container := range containerRegistry {
		log.Infof("terminating the container '%s'", name)
		container.Terminate(ctx)
	}
}

func setContainerConfigDefaultsIfNotProvided(dbType string, config *ContainerConfig) {
	// TODO: discuss and decide the default DBVersion values for each dbtype

	config.DBType = dbType
	switch dbType {
	case POSTGRESQL:
		config.User = lo.Ternary(config.User == "", "ybvoyager", config.User)
		config.Password = lo.Ternary(config.Password == "", "passsword", config.Password)
		config.DBVersion = lo.Ternary(config.DBVersion == "", "11", config.DBVersion)
		config.Schema = lo.Ternary(config.Schema == "", "public", config.Schema)
		config.DBName = lo.Ternary(config.DBName == "", "postgres", config.DBName)

	case YUGABYTEDB:
		config.User = lo.Ternary(config.User == "", "yugabyte", config.User) // ybdb docker doesn't create specified user
		config.Password = lo.Ternary(config.Password == "", "passsword", config.Password)
		config.DBVersion = lo.Ternary(config.DBVersion == "", "2.20.7.1-b10", config.DBVersion)
		config.Schema = lo.Ternary(config.Schema == "", "public", config.Schema)
		config.DBName = lo.Ternary(config.DBName == "", "yugabyte", config.DBName)

	case ORACLE:
		config.User = lo.Ternary(config.User == "", "ybvoyager", config.User)
		config.Password = lo.Ternary(config.Password == "", "passsword", config.Password)
		config.DBVersion = lo.Ternary(config.DBVersion == "", "21", config.DBVersion)
		config.Schema = lo.Ternary(config.Schema == "", "YBVOYAGER", config.Schema)
		config.DBName = lo.Ternary(config.DBName == "", "DMS", config.DBName)

	case MYSQL:
		config.User = lo.Ternary(config.User == "", "ybvoyager", config.User)
		config.Password = lo.Ternary(config.Password == "", "passsword", config.Password)
		config.DBVersion = lo.Ternary(config.DBVersion == "", "8.4", config.DBVersion)
		config.DBName = lo.Ternary(config.DBName == "", "dms", config.DBName)

	default:
		panic(fmt.Sprintf("unsupported db type '%q' for creating test container\n", dbType))
	}
}
