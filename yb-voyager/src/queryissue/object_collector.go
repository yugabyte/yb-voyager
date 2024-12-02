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
package queryissue

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/queryparser"
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
Collect processes a node and extracts object names based on node type (ignore standard SQL functions like count(), sum() etc).
Cases covered:
1. SELECT queries - schema name can be with table/functions
2. Insert/Update/Delete queries
3. TODO: cover DDLs

Since Collect() will be called for each node so just the collection at concerned leaf node is enough
*/
func (c *ObjectCollector) Collect(msg protoreflect.Message) {
	if msg == nil || !msg.IsValid() {
		return
	}

	nodeType := queryparser.GetMsgFullName(msg)
	switch nodeType {
	// Extract table or view names in FROM clauses
	case queryparser.PG_QUERY_RANGEVAR_NODE:
		schemaName := queryparser.GetStringField(msg, "schemaname")
		relName := queryparser.GetStringField(msg, "relname")
		objectName := relName
		if schemaName != "" {
			objectName = fmt.Sprintf("%s.%s", schemaName, relName)
		}
		log.Debugf("[RangeVar] fetched schemaname=%s relname=%s objectname=%s field\n", schemaName, relName, objectName)
		c.addObject(objectName)

	// Extract target table names from DML statements
	case queryparser.PG_QUERY_INSERTSTMT_NODE, queryparser.PG_QUERY_UPDATESTMT_NODE, queryparser.PG_QUERY_DELETESTMT_NODE:
		relationMsg := queryparser.GetMessageField(msg, "relation")
		if relationMsg != nil {
			schemaName := queryparser.GetStringField(relationMsg, "schemaname")
			relName := queryparser.GetStringField(relationMsg, "relname")

			objectName := relName
			if schemaName != "" {
				objectName = fmt.Sprintf("%s.%s", schemaName, relName)
			}
			log.Debugf("[IUD] fetched schemaname=%s relname=%s objectname=%s field\n", schemaName, relName, objectName)
			c.addObject(objectName)
		}

	// Extract function names
	case queryparser.PG_QUERY_FUNCCALL_NODE:
		schemaName, functionName := queryparser.GetFuncNameFromFuncCall(msg)
		if functionName == "" {
			break
		}

		objectName := functionName
		if schemaName != "" {
			objectName = fmt.Sprintf("%s.%s", schemaName, functionName)
		}
		log.Debugf("[Funccall] fetched schemaname=%s objectname=%s field\n", schemaName, objectName)
		c.addObject(objectName)

		// Add more cases as needed for other message types
	}
}

// addObject adds an object name to the collector if it's not already present.
func (c *ObjectCollector) addObject(objectName string) {
	if _, exists := c.objectSet[objectName]; !exists {
		c.objectSet[objectName] = true
	}
}

// getObjects returns a slice of collected unique object names.
func (c *ObjectCollector) getObjects() []string {
	objects := make([]string, 0, len(c.objectSet))
	for obj, present := range c.objectSet {
		if present {
			objects = append(objects, obj)
		}
	}
	return objects
}

func (c *ObjectCollector) getSchemaList() []string {
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
