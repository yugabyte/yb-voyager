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
package yugabytedb

import (
	"github.com/google/uuid"
)

type MigrationEvent struct {
	MigrationUUID uuid.UUID `json:"migration_uuid"`
	EventType     string    `json:"event_type"`
	Payload       string    `json:"payload"`
	Status        string    `json:"status"`
}

type TableProgress struct {
	MigrationUUID     uuid.UUID
	EventType         string
	SchemaName        string
	TableName         string
	Status            string
	TotalRowCount     int64
	CompletedRowCount int64
}
