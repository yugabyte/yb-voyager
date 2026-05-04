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

package migassessment

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
	goerrors "github.com/go-errors/errors"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/namereg"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ux"
)

type AskPasswordFunc func(destination string, user string, envVar string) (string, error)
type CheckDependenciesFunc func() ([]string, error)

type PreflightConfig struct {
	Source                      *srcdb.Source
	AssessmentMetadataDirFlag   string
	ExportType                  string
	SourceReadReplicaEndpoints  string
	PrimaryOnly                 bool
	AskPassword                 AskPasswordFunc
	CheckDependencies           CheckDependenciesFunc
	FetchSourceInfo             func()
	CheckSchemaUsagePermissions func() error
}

type PreflightResult struct {
	HasSourceConnectivity     bool
	ValidatedReplicaEndpoints []srcdb.ReplicaEndpoint
	ReplicaDiscoveryInfo      *ReplicaDiscoveryInfo
	PgssEnabledForAssessment  bool
}

func RunPreflightChecks(config PreflightConfig) (PreflightResult, error) {
	ux.PrintSectionHeader("Preflight Checks")

	result := PreflightResult{
		HasSourceConnectivity: config.AssessmentMetadataDirFlag == "",
	}
	if !result.HasSourceConnectivity {
		ux.PrintPreflightSkip("Source connectivity checks (using metadata dir)")
		return result, nil
	}

	if err := checkSourceDatabasePassword(config); err != nil {
		return result, err
	}
	if err := connectToSourceDatabase(config.Source); err != nil {
		return result, err
	}
	if err := runGuardrailChecks(config); err != nil {
		return result, err
	}
	if err := resolveSourceSchemas(config.Source); err != nil {
		return result, err
	}
	if config.FetchSourceInfo != nil {
		config.FetchSourceInfo()
	}

	replicaDiscoveryInfo, err := discoverAndValidateReplicas(config)
	if err != nil {
		return result, err
	}
	result.ValidatedReplicaEndpoints = replicaDiscoveryInfo.ValidatedReplicas
	result.ReplicaDiscoveryInfo = &replicaDiscoveryInfo

	pgssEnabled, err := checkAssessmentPermissions(config, result.ValidatedReplicaEndpoints)
	if err != nil {
		return result, err
	}
	result.PgssEnabledForAssessment = pgssEnabled

	return result, nil
}

func checkSourceDatabasePassword(config PreflightConfig) error {
	if config.Source.Password == "" {
		password, err := config.AskPassword("source DB", config.Source.User, "SOURCE_DB_PASSWORD")
		if err != nil {
			ux.PrintPreflightFail("Source database password")
			return fmt.Errorf("failed to get source DB password for assessing migration: %w", err)
		}
		config.Source.Password = password
	}
	ux.PrintPreflightCheck("Source database password")
	return nil
}

func connectToSourceDatabase(source *srcdb.Source) error {
	if err := source.DB().Connect(); err != nil {
		ux.PrintPreflightFail("Connect to source database")
		return fmt.Errorf("failed to connect source db for assessing migration: %w", err)
	}
	ux.PrintPreflightCheck("Connected to source database")
	return nil
}

func runGuardrailChecks(config PreflightConfig) error {
	if !bool(config.Source.RunGuardrailsChecks) {
		ux.PrintPreflightSkip("Source DB version check (guardrails disabled)")
		ux.PrintPreflightSkip("Dependency check (guardrails disabled)")
		return nil
	}

	log.Info("checking source DB version")
	if err := config.Source.DB().CheckSourceDBVersion(config.ExportType); err != nil {
		ux.PrintPreflightFail("Source DB version check")
		return fmt.Errorf("failed to check source DB version for assess migration: %w", err)
	}
	ux.PrintPreflightCheck(fmt.Sprintf("Source DB version compatible (%s)", config.Source.DB().GetVersion()))

	binaryCheckIssues, err := config.CheckDependencies()
	if err != nil {
		ux.PrintPreflightFail("Required dependencies present")
		return fmt.Errorf("failed to check dependencies for assess migration: %w", err)
	} else if len(binaryCheckIssues) > 0 {
		ux.PrintPreflightFail("Required dependencies present")
		return goerrors.Errorf("\n%s\n%s", color.RedString("Missing dependencies for assess migration:"), strings.Join(binaryCheckIssues, "\n"))
	}
	ux.PrintPreflightCheck("Required dependencies present")
	return nil
}

func resolveSourceSchemas(source *srcdb.Source) error {
	allSchemas, err := source.DB().GetAllSchemaNamesIdentifiers()
	if err != nil {
		ux.PrintPreflightFail("Schema resolution")
		return fmt.Errorf("failed to get all schema names identifiers: %w", err)
	}
	source.Schemas, err = namereg.SchemaNameMatcher(source.DBType, allSchemas, source.SchemaConfig)
	if err != nil {
		ux.PrintPreflightFail("Schema resolution")
		return fmt.Errorf("failed to match schema names: %w", err)
	}
	ux.PrintPreflightCheck(fmt.Sprintf("Schema resolution (%d schemas)", len(source.Schemas)))
	return nil
}

func discoverAndValidateReplicas(config PreflightConfig) (ReplicaDiscoveryInfo, error) {
	replicaDiscoveryInfo, err := HandleReplicaDiscoveryAndValidation(
		config.Source,
		config.SourceReadReplicaEndpoints,
		config.PrimaryOnly,
	)
	if err != nil {
		ux.PrintPreflightFail("Replica discovery and validation")
		return ReplicaDiscoveryInfo{}, fmt.Errorf("failed to handle replica discovery and validation: %w", err)
	}
	ux.PrintPreflightCheck("Replica discovery and validation")
	return replicaDiscoveryInfo, nil
}

func checkAssessmentPermissions(config PreflightConfig, validatedReplicaEndpoints []srcdb.ReplicaEndpoint) (bool, error) {
	if !bool(config.Source.RunGuardrailsChecks) {
		ux.PrintPreflightSkip("Permission checks (guardrails disabled)")
		return false, nil
	}

	if err := config.CheckSchemaUsagePermissions(); err != nil {
		ux.PrintPreflightFail("Schema USAGE permissions")
		return false, fmt.Errorf("schema usage permission check failed: %w", err)
	}
	pgssEnabled, err := CheckAssessmentPermissionsOnAllNodes(config.Source, validatedReplicaEndpoints)
	if err != nil {
		ux.PrintPreflightFail("Assessment permissions on all nodes")
		return false, fmt.Errorf("assessment permission check failed: %w", err)
	}
	ux.PrintPreflightCheck("Permissions verified on all nodes")
	return pgssEnabled, nil
}
