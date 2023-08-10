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
	"strconv"
	"strings"

	"sync"

	"github.com/google/uuid"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Event struct {
	Vsn        int64              `json:"vsn"` // Voyager Sequence Number
	Op         string             `json:"op"`
	SchemaName string             `json:"schema_name"`
	TableName  string             `json:"table_name"`
	Key        map[string]*string `json:"key"`
	Fields     map[string]*string `json:"fields"`
}

var cachePreparedStmt = sync.Map{}

func (e *Event) String() string {
	return fmt.Sprintf("Event{vsn=%v, op=%v, schema=%v, table=%v, key=%v, fields=%v}",
		e.Vsn, e.Op, e.SchemaName, e.TableName, e.Key, e.Fields)
}

func (e *Event) GetSQLStmt(targetSchema string) string {
	switch e.Op {
	case "c":
		return e.getInsertStmt(targetSchema)
	case "u":
		return e.getUpdateStmt(targetSchema)
	case "d":
		return e.getDeleteStmt(targetSchema)
	default:
		panic("unknown op: " + e.Op)
	}
}

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

const insertTemplate = "INSERT INTO %s (%s) VALUES (%s)"
const updateTemplate = "UPDATE %s SET %s WHERE %s"
const deleteTemplate = "DELETE FROM %s WHERE %s"

func (event *Event) getInsertStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}
	columnList := make([]string, 0, len(event.Fields))
	valueList := make([]string, 0, len(event.Fields))
	for column, value := range event.Fields {
		columnList = append(columnList, column)
		if value == nil {
			valueList = append(valueList, "NULL")
		} else {
			valueList = append(valueList, *value)
		}
	}
	columns := strings.Join(columnList, ", ")
	values := strings.Join(valueList, ", ")
	stmt := fmt.Sprintf(insertTemplate, tableName, columns, values)
	return stmt
}

func (event *Event) getUpdateStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}
	var setClauses []string
	for column, value := range event.Fields {
		if value == nil {
			setClauses = append(setClauses, fmt.Sprintf("%s = NULL", column))
		} else {
			setClauses = append(setClauses, fmt.Sprintf("%s = %s", column, *value))
		}
	}
	setClause := strings.Join(setClauses, ", ")
	var whereClauses []string
	for column, value := range event.Key {
		if value == nil { // value can't be nil for keys
			panic("key value is nil")
		}
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", column, *value))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(updateTemplate, tableName, setClause, whereClause)
}

func (event *Event) getDeleteStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}
	var whereClauses []string
	for column, value := range event.Key {
		if value == nil { // value can't be nil for keys
			panic("key value is nil")
		}
		whereClauses = append(whereClauses, fmt.Sprintf("%s = %s", column, *value))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(deleteTemplate, tableName, whereClause)
}

func (event *Event) getPreparedInsertStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}

	columnList := make([]string, 0, len(event.Fields))
	valueList := make([]string, 0, len(event.Fields))
	keys := utils.GetMapKeysSorted(event.Fields)
	for pos, key := range keys {
		columnList = append(columnList, key)
		valueList = append(valueList, fmt.Sprintf("$%d", pos+1))
	}
	columns := strings.Join(columnList, ", ")
	values := strings.Join(valueList, ", ")
	stmt := fmt.Sprintf(insertTemplate, tableName, columns, values)
	return stmt
}

// NOTE: PS for each event of same table can be diffrent as it depends on columns being updated
func (event *Event) getPreparedUpdateStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}

	var setClauses []string
	keys := utils.GetMapKeysSorted(event.Fields)
	for pos, key := range keys {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, pos+1))
	}
	setClause := strings.Join(setClauses, ", ")

	var whereClauses []string
	keys = utils.GetMapKeysSorted(event.Key)
	for i, key := range keys {
		pos := i + 1 + len(event.Fields)
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", key, pos))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(updateTemplate, tableName, setClause, whereClause)
}

func (event *Event) getPreparedDeleteStmt(targetSchema string) string {
	tableName := event.SchemaName + "." + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "." + event.TableName
	}
	var whereClauses []string
	keys := utils.GetMapKeysSorted(event.Key)
	for pos, key := range keys {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", key, pos+1))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(deleteTemplate, tableName, whereClause)
}

func (event *Event) getInsertParams() []interface{} {
	var params []interface{}
	keys := utils.GetMapKeysSorted(event.Fields)
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
	keys := utils.GetMapKeysSorted(event.Fields)
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

	keys = utils.GetMapKeysSorted(event.Key)
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
	keys := utils.GetMapKeysSorted(event.Key)
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

func (event *Event) GetPreparedStmtName(targetSchema string) string {
	keys := strings.Join(utils.GetMapKeysSorted(event.Key), ",")
	tableName := event.SchemaName + "_" + event.TableName
	if targetSchema != "" {
		tableName = targetSchema + "_" + event.TableName
	}
	return tableName + "_" + event.Op + "on:keys=" + keys
}

// ==============================================================================================================================
type EventBatch struct {
	Events []*Event
	ChanNo int
}

func (eb EventBatch) GetLastVsn() int64 {
	return eb.Events[len(eb.Events)-1].Vsn
}

func (eb EventBatch) GetQueryToUpdateLastAppliedVSN(migrationUUID uuid.UUID) string {
	return fmt.Sprintf(`UPDATE %s SET last_applied_vsn=%d where migration_uuid='%s' AND channel_no=%d`,
		EVENT_CHANNELS_METADATA_TABLE_NAME, eb.GetLastVsn(), migrationUUID, eb.ChanNo)
}

type EventChannelMetaInfo struct {
	ChanNo         int
	LastAppliedVsn int64
}
