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
package ybm

import (
	"github.com/google/uuid"
	cp "github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
)

// MigrationEvent represents the payload for voyager-metadata API
type MigrationEvent struct {
	MigrationUUID       uuid.UUID          `json:"migration_uuid"`
	MigrationPhase      int                `json:"migration_phase"`
	InvocationSequence  int                `json:"invocation_sequence"`
	MigrationDirectory  string             `json:"migration_dir"`
	DatabaseName        string             `json:"database_name"`
	SchemaName          string             `json:"schema_name"`
	HostIP              string             `json:"host_ip"` // Plain IP address string, e.g., "10.9.72.172"
	Port                int                `json:"port"`
	DBVersion           string             `json:"db_version"`
	Payload             interface{}        `json:"payload"`             // Can accept any type - structs, maps, etc. - json.Marshal will handle it
	PayloadVersion      string             `json:"payload_version"`     // Version string, e.g., "1.0"
	VoyagerClientInfo   cp.VoyagerInstance `json:"voyager_client_info"` // API uses "voyager_client_info"
	DBType              string             `json:"db_type"`
	Status              string             `json:"status"`
	InvocationTimestamp string             `json:"invocation_timestamp"`
}

// YBMConfig holds the YBM control plane configuration
type YBMConfig struct {
	Domain    string
	AccountID string
	ProjectID string
	ClusterID string
	APIKey    string
}
