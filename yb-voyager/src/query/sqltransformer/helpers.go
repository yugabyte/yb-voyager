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
package sqltransformer

import (
	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

// addColocatedOption adds "WITH (colocated = true)" to the "Options" array
// for either a CreateStmt or CreateMatViewStmt.
func addColocationOptionToCreateTable(createStmt *pg_query.CreateStmt) {
	if createStmt == nil {
		return
	}
	// If Options slice is nil, initialize it
	if createStmt.Options == nil {
		createStmt.Options = []*pg_query.Node{}
	}

	// Build DefElem: defname = "colocated", arg = "false"
	defElemNode := &pg_query.Node{
		Node: &pg_query.Node_DefElem{
			DefElem: &pg_query.DefElem{
				Defname: constants.COLOCATION_CLAUSE,
				Arg:     pg_query.MakeStrNode("false"),
			},
		},
	}

	createStmt.Options = append(createStmt.Options, defElemNode)
}

func addColocationOptionToCreateMaterializedView(createMatViewStmt *pg_query.CreateTableAsStmt) {
	if createMatViewStmt == nil {
		return
	}
	// If Options slice is nil, initialize it
	if createMatViewStmt.Into.Options == nil {
		createMatViewStmt.Into.Options = []*pg_query.Node{}
	}

	// Build DefElem: defname = "colocated", arg = "false"
	defElemNode := &pg_query.Node{
		Node: &pg_query.Node_DefElem{
			DefElem: &pg_query.DefElem{
				Defname: constants.COLOCATION_CLAUSE,
				Arg:     pg_query.MakeStrNode("false"),
			},
		},
	}

	createMatViewStmt.Into.Options = append(createMatViewStmt.Into.Options, defElemNode)
}
