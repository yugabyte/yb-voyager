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
package tgtdb

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Event struct {
	Op         string             `json:"op"`
	SchemaName string             `json:"schema_name"`
	TableName  string             `json:"table_name"`
	Key        map[string]*string `json:"key"`
	Fields     map[string]*string `json:"fields"`
}

var cachePreparedStmt = sync.Map{}

func (e *Event) GetPreparedSQLStmt(targetSchema string) string {
	if stmt, ok := cachePreparedStmt.Load(e.GetPreparedStmtName()); ok {
		return stmt.(string)
	}
	var ps string
	switch e.Op {
	case "c":
		ps = e.getPreparedInsertStmt(targetSchema)
	case "u":
		ps = e.getPreparedUpdateStmt(targetSchema)
	case "d":
		ps = e.getPreparedDeleteStmt(targetSchema)
	default:
		panic("unknown op: " + e.Op)
	}

	cachePreparedStmt.Store(e.GetPreparedStmtName(), ps)
	return ps
}

func (e *Event) GetParams() []interface{} {
	switch e.Op {
	case "c":
		return e.getInsertParams()
	case "u":
		return e.getUpdateParams()
	case "d":
		return e.getDeleteParams()
	default:
		panic("unknown op: " + e.Op)
	}
}

const insertTemplate = "INSERT INTO %s (%s) VALUES (%s);"
const updateTemplate = "UPDATE %s SET %s WHERE %s;"
const deleteTemplate = "DELETE FROM %s WHERE %s;"

func (event *Event) getPreparedInsertStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}

	columnList := make([]string, 0, len(event.Fields))
	valueList := make([]string, 0, len(event.Fields))
	keys := utils.SortMapKeys(event.Fields)
	for pos, key := range keys {
		columnList = append(columnList, key)
		valueList = append(valueList, fmt.Sprintf("$%d", pos+1))
	}
	columns := strings.Join(columnList, ", ")
	values := strings.Join(valueList, ", ")
	stmt := fmt.Sprintf(insertTemplate, tableName, columns, values)
	return stmt
}

func (event *Event) getPreparedUpdateStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}

	var setClauses []string
	keys := utils.SortMapKeys(event.Fields)
	for pos, key := range keys {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, pos+1))
	}
	sort.Strings(setClauses)
	setClause := strings.Join(setClauses, ", ")

	var whereClauses []string
	keys = utils.SortMapKeys(event.Key)
	for i, key := range keys {
		pos := i + 1 + len(event.Fields)
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", key, pos))
	}
	sort.Strings(whereClauses)
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(updateTemplate, tableName, setClause, whereClause)
}

func (event *Event) getPreparedDeleteStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}
	var whereClauses []string
	keys := utils.SortMapKeys(event.Key)
	for pos, key := range keys {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", key, pos+1))
	}
	sort.Strings(whereClauses)
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(deleteTemplate, tableName, whereClause)
}

func (event *Event) getInsertParams() []interface{} {
	var params []interface{}
	keys := utils.SortMapKeys(event.Fields)
	for _, key := range keys {
		value := event.Fields[key]
		if value == nil {
			params = append(params, nil)
			continue
		}
		unquotedValue, err := strconv.Unquote(*value)
		if err != nil {
			unquotedValue = *value
		}
		params = append(params, unquotedValue)
	}
	return params
}

func (event *Event) getUpdateParams() []interface{} {
	var params []interface{}
	keys := utils.SortMapKeys(event.Fields)
	for _, key := range keys {
		value := event.Fields[key]
		if value == nil {
			params = append(params, nil)
			continue
		}
		unquotedValue, err := strconv.Unquote(*value)
		if err != nil {
			unquotedValue = *value
		}
		params = append(params, unquotedValue)
	}

	keys = utils.SortMapKeys(event.Key)
	for _, key := range keys {
		value := event.Key[key]
		if value == nil {
			params = append(params, nil)
			continue
		}
		unquotedValue, err := strconv.Unquote(*value)
		if err != nil {
			unquotedValue = *value
		}
		params = append(params, unquotedValue)
	}
	return params
}

func (event *Event) getDeleteParams() []interface{} {
	var params []interface{}
	keys := utils.SortMapKeys(event.Key)
	for _, key := range keys {
		value := event.Key[key]
		if value == nil {
			params = append(params, nil)
			continue
		}
		unquotedValue, err := strconv.Unquote(*value)
		if err != nil {
			unquotedValue = *value
		}
		params = append(params, unquotedValue)
	}
	return params
}

func (event *Event) GetPreparedStmtName() string {
	return event.SchemaName + "_" + event.TableName + "_" + event.Op
}
