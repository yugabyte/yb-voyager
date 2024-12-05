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
package queryparser

import (
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ObjectCollector collects unique schema-qualified object names.
type ObjectCollector struct {
	objectSet map[string]bool
}

func NewObjectCollector() *ObjectCollector {
	return &ObjectCollector{
		objectSet: make(map[string]bool),
	}
}

/*
Collect processes a given node and extracts object names based on node type.
Cases covered:
1. [DML] SELECT queries - collect table/function object in it
2. [DML]Insert/Update/Delete queries
3. TODO: Coverage for DDLs (right now it worked with some cases like CREATE VIEW and CREATE SEQUENCE, but need to ensure all cases are covered)

Collect() should be called from TraverseParseTree() to get all the objects in the parse tree of a stmt
*/
func (c *ObjectCollector) Collect(msg protoreflect.Message) {
	if msg == nil || !msg.IsValid() {
		return
	}

	nodeType := GetMsgFullName(msg)
	switch nodeType {
	// Extract table or view names in FROM clauses
	case PG_QUERY_RANGEVAR_NODE:
		schemaName := GetStringField(msg, "schemaname")
		relName := GetStringField(msg, "relname")
		objectName := lo.Ternary(schemaName != "", schemaName+"."+relName, relName)
		log.Debugf("[RangeVar] fetched schemaname=%s relname=%s objectname=%s field\n", schemaName, relName, objectName)
		c.addObject(objectName)

	// Extract target table names from DML statements
	case PG_QUERY_INSERTSTMT_NODE, PG_QUERY_UPDATESTMT_NODE, PG_QUERY_DELETESTMT_NODE:
		relationMsg := GetMessageField(msg, "relation")
		if relationMsg != nil {
			schemaName := GetStringField(relationMsg, "schemaname")
			relName := GetStringField(relationMsg, "relname")
			objectName := lo.Ternary(schemaName != "", schemaName+"."+relName, relName)
			log.Debugf("[IUD] fetched schemaname=%s relname=%s objectname=%s field\n", schemaName, relName, objectName)
			c.addObject(objectName)
		}

	// Extract function names
	case PG_QUERY_FUNCCALL_NODE:
		schemaName, functionName := GetFuncNameFromFuncCall(msg)
		if functionName == "" {
			return
		}

		objectName := lo.Ternary(schemaName != "", schemaName+"."+functionName, functionName)
		log.Debugf("[Funccall] fetched schemaname=%s objectname=%s field\n", schemaName, objectName)
		c.addObject(objectName)

		// Add more cases as needed for other message types
	}
}

// addObject adds an object name to the collector if it's not already present.
func (c *ObjectCollector) addObject(objectName string) {
	if _, exists := c.objectSet[objectName]; !exists {
		log.Debugf("adding object to object collector set: %s", objectName)
		c.objectSet[objectName] = true
	}
}

// GetObjects returns a slice of collected unique object names.
func (c *ObjectCollector) GetObjects() []string {
	return lo.Keys(c.objectSet)
}

func (c *ObjectCollector) GetSchemaList() []string {
	var schemaList []string
	for obj := range c.objectSet {
		splits := strings.Split(obj, ".")
		if len(splits) == 1 {
			schemaList = append(schemaList, "")
		} else if len(splits) == 2 {
			schemaList = append(schemaList, splits[0])
		}
	}
	return lo.Uniq(schemaList)
}
