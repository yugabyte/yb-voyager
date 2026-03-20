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
package exportdata

import (
	"github.com/google/uuid"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/cp"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

// ExportContext carries all the state that exporters need. It replaces the
// package-level globals that the cmd package currently relies on.
// Constructed once in cmd and passed into exportdata.Run().
type ExportContext struct {
	// Core configuration (immutable after construction).
	ExportDir     string
	ExportType    string
	ExporterRole  string
	MigrationUUID uuid.UUID
	StartClean    bool
	DisablePb     bool
	UseDebezium   bool

	// Source database.
	Source   *srcdb.Source
	SourceDB srcdb.SourceDB

	// Dependencies (injected by cmd).
	MetaDB       *metadb.MetaDB
	ControlPlane cp.ControlPlane

	// State populated by the cmd layer's setup before Run() is called.
	FinalTableList      []sqlname.NameTuple
	TablesColumnList    *utils.StructMap[sqlname.NameTuple, []string]
	PartitionsToRootMap map[string]string
	LeafPartitions      *utils.StructMap[sqlname.NameTuple, []sqlname.NameTuple]
}
