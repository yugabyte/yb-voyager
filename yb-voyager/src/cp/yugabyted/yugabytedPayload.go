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
package yugabyted

import (
	"github.com/google/uuid"
)

type MigrationEvent struct {
	MigrationUUID       uuid.UUID `json:"migration_uuid"`
	MigrationPhase      int       `json:"migration_phase"`
	InvocationSequence  int       `json:"invocation_sequence"`
	MigrationDirectory  string    `json:"migration_dir"`
	DatabaseName        string    `json:"database_name"`
	SchemaName          string    `json:"schema_name"`
	DBIP                string    `json:"db_ip"`
	Port                int       `json:"port"`
	DBVersion           string    `json:"db_version"`
	Payload             string    `json:"payload"`
	VoyagerInfo         string    `json:"voyager_info"`
	DBType              string    `json:"db_type"`
	Status              string    `json:"status"`
	InvocationTimestamp string    `json:"invocation_timestamp"`
}
