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
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// ObjectPredicate defines a function signature that decides whether to include an object.
type ObjectPredicate func(schemaName string, objectName string, objectType string) bool

// Using a predicate makes it easy to adapt or swap filtering logic in the future without changing the collector.

// ObjectCollector collects unique schema-qualified object names based on the provided predicate.
type ObjectCollector struct {
	objectSet map[string]bool
	predicate ObjectPredicate
}

// AllObjectsPredicate always returns true, meaning it will collect all objects found.
func AllObjectsPredicate(schemaName, objectName, objectType string) bool {
	return true
}

// TablesOnlyPredicate returns true only if the object type is "table".
func TablesOnlyPredicate(schemaName, objectName, objectType string) bool {
	return objectType == constants.TABLE
}

func NewObjectCollector(predicate ObjectPredicate) *ObjectCollector {
	if predicate == nil {
		predicate = AllObjectsPredicate
	}

	return &ObjectCollector{
		objectSet: make(map[string]bool),
		predicate: predicate,
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
		// it will be either table or view, considering objectType=table for both
		if c.predicate(schemaName, relName, constants.TABLE) {
			c.addObject(objectName)
		}

	// Extract target table names from DML statements
	case PG_QUERY_INSERTSTMT_NODE, PG_QUERY_UPDATESTMT_NODE, PG_QUERY_DELETESTMT_NODE:
		relationMsg := GetMessageField(msg, "relation")
		if relationMsg != nil {
			schemaName := GetStringField(relationMsg, "schemaname")
			relName := GetStringField(relationMsg, "relname")
			objectName := lo.Ternary(schemaName != "", schemaName+"."+relName, relName)
			log.Debugf("[IUD] fetched schemaname=%s relname=%s objectname=%s field\n", schemaName, relName, objectName)
			if c.predicate(schemaName, relName, "table") {
				c.addObject(objectName)
			}
		}

	// Extract function names
	case PG_QUERY_FUNCCALL_NODE:
		schemaName, functionName := GetFuncNameFromFuncCall(msg)
		if functionName == "" {
			return
		}

		objectName := lo.Ternary(schemaName != "", schemaName+"."+functionName, functionName)
		log.Debugf("[Funccall] fetched schemaname=%s objectname=%s field\n", schemaName, objectName)
		if c.predicate(schemaName, functionName, constants.FUNCTION) {
			c.addObject(objectName)
		}

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
