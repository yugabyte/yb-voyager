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
	"strings"

	"sync"

	"github.com/google/uuid"
	"github.com/samber/lo"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Event struct {
	Vsn          int64              `json:"vsn"` // Voyager Sequence Number
	Op           string             `json:"op"`
	SchemaName   string             `json:"schema_name"`
	TableName    string             `json:"table_name"`
	Key          map[string]*string `json:"key"`
	Fields       map[string]*string `json:"fields"`
	ExporterRole string             `json:"exporter_role"`
}

var cachePreparedStmt = sync.Map{}

func (e *Event) String() string {
	return fmt.Sprintf("Event{vsn=%v, op=%v, schema=%v, table=%v, key=%v, fields=%v}",
		e.Vsn, e.Op, e.SchemaName, e.TableName, e.Key, e.Fields)
}

func (e *Event) IsCutover() bool {
	return e.Op == "cutover"
}

func (e *Event) IsFallForward() bool {
	return e.Op == "fallforward"
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
	psName := e.GetPreparedStmtName(targetSchema)
	if stmt, ok := cachePreparedStmt.Load(psName); ok {
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

	cachePreparedStmt.Store(psName, ps)
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

func (event *Event) GetPreparedStmtName(targetSchema string) string {
	var ps strings.Builder
	ps.WriteString(event.getTableName(targetSchema))
	ps.WriteString("_")
	ps.WriteString(event.Op)
	if event.Op == "u" {
		keys := strings.Join(utils.GetMapKeysSorted(event.Fields), ",")
		ps.WriteString(":")
		ps.WriteString(keys)
	}
	return ps.String()
}

const insertTemplate = "INSERT INTO %s (%s) VALUES (%s)"
const updateTemplate = "UPDATE %s SET %s WHERE %s"
const deleteTemplate = "DELETE FROM %s WHERE %s"

func (event *Event) getInsertStmt(targetSchema string) string {
	tableName := event.getTableName(targetSchema)
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
	tableName := event.getTableName(targetSchema)
	setClauses := make([]string, 0, len(event.Fields))
	for column, value := range event.Fields {
		if value == nil {
			setClauses = append(setClauses, fmt.Sprintf("%s = NULL", column))
		} else {
			setClauses = append(setClauses, fmt.Sprintf("%s = %s", column, *value))
		}
	}
	setClause := strings.Join(setClauses, ", ")

	whereClauses := make([]string, 0, len(event.Key))
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
	tableName := event.getTableName(targetSchema)
	whereClauses := make([]string, 0, len(event.Key))
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
	tableName := event.getTableName(targetSchema)
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

// NOTE: PS for each event of same table can be different as it depends on columns being updated
func (event *Event) getPreparedUpdateStmt(targetSchema string) string {
	tableName := event.getTableName(targetSchema)
	setClauses := make([]string, 0, len(event.Fields))
	keys := utils.GetMapKeysSorted(event.Fields)
	for pos, key := range keys {
		setClauses = append(setClauses, fmt.Sprintf("%s = $%d", key, pos+1))
	}
	setClause := strings.Join(setClauses, ", ")

	whereClauses := make([]string, 0, len(event.Key))
	keys = utils.GetMapKeysSorted(event.Key)
	for i, key := range keys {
		pos := i + 1 + len(event.Fields)
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", key, pos))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(updateTemplate, tableName, setClause, whereClause)
}

func (event *Event) getPreparedDeleteStmt(targetSchema string) string {
	tableName := event.getTableName(targetSchema)
	whereClauses := make([]string, 0, len(event.Key))
	keys := utils.GetMapKeysSorted(event.Key)
	for pos, key := range keys {
		whereClauses = append(whereClauses, fmt.Sprintf("%s = $%d", key, pos+1))
	}
	whereClause := strings.Join(whereClauses, " AND ")
	return fmt.Sprintf(deleteTemplate, tableName, whereClause)
}

func (event *Event) getInsertParams() []interface{} {
	return getMapValuesForQuery(event.Fields)
}

func (event *Event) getUpdateParams() []interface{} {
	params := make([]interface{}, 0, len(event.Fields)+len(event.Key))
	params = append(params, getMapValuesForQuery(event.Fields)...)
	params = append(params, getMapValuesForQuery(event.Key)...)
	return params
}

func (event *Event) getDeleteParams() []interface{} {
	return getMapValuesForQuery(event.Key)
}

func getMapValuesForQuery(m map[string]*string) []interface{} {
	keys := utils.GetMapKeysSorted(m)
	values := make([]interface{}, 0, len(keys))
	for _, key := range keys {
		values = append(values, m[key])
	}
	return values
}

func (event *Event) getTableName(targetSchema string) string {
	tableName := strings.Join([]string{event.SchemaName, event.TableName}, ".")
	if targetSchema != "" {
		tableName = strings.Join([]string{targetSchema, event.TableName}, ".")
	}
	return tableName
}

// ==============================================================================================================================
type EventBatch struct {
	Events             []*Event
	ChanNo             int
	EventCounts        *EventCounter
	EventCountsByTable map[string]*EventCounter
}

func NewEventBatch(events []*Event, chanNo int, targetSchema string) *EventBatch {
	batch := &EventBatch{
		Events:             events,
		ChanNo:             chanNo,
		EventCounts:        &EventCounter{},
		EventCountsByTable: make(map[string]*EventCounter),
	}
	batch.updateCounts(targetSchema)
	return batch
}

func (eb *EventBatch) GetLastVsn() int64 {
	return eb.Events[len(eb.Events)-1].Vsn
}

func (eb *EventBatch) GetChannelMetadataUpdateQuery(migrationUUID uuid.UUID) string {
	queryTemplate := `UPDATE %s 
	SET 
		last_applied_vsn=%d, 
		num_inserts = num_inserts + %d, 
		num_updates = num_updates + %d, 
		num_deletes = num_deletes + %d  
	where 
		migration_uuid='%s' AND channel_no=%d
	`
	return fmt.Sprintf(queryTemplate,
		EVENT_CHANNELS_METADATA_TABLE_NAME,
		eb.GetLastVsn(),
		eb.EventCounts.NumInserts,
		eb.EventCounts.NumUpdates,
		eb.EventCounts.NumDeletes,
		migrationUUID, eb.ChanNo)
}

func (eb *EventBatch) GetQueriesToUpdateEventStatsByTable(migrationUUID uuid.UUID, tableName string) string {
	queryTemplate := `UPDATE %s 
	SET 
		total_events = total_events + %d, 
		num_inserts = num_inserts + %d, 
		num_updates = num_updates + %d, 
		num_deletes = num_deletes + %d  
	where 
		migration_uuid='%s' AND table_name='%s' AND channel_no=%d
	`
	return fmt.Sprintf(queryTemplate,
		EVENTS_PER_TABLE_METADATA_TABLE_NAME,
		eb.EventCountsByTable[tableName].TotalEvents,
		eb.EventCountsByTable[tableName].NumInserts,
		eb.EventCountsByTable[tableName].NumUpdates,
		eb.EventCountsByTable[tableName].NumDeletes,
		migrationUUID, tableName, eb.ChanNo)
}

func (eb *EventBatch) GetQueriesToInsertEventStatsByTable(migrationUUID uuid.UUID, tableName string) string {
	queryTemplate := `INSERT INTO %s 
	(migration_uuid, table_name, channel_no, total_events, num_inserts, num_updates, num_deletes) 
	VALUES ('%s', '%s', %d, %d, %d, %d, %d)
	`
	return fmt.Sprintf(queryTemplate,
		EVENTS_PER_TABLE_METADATA_TABLE_NAME,
		migrationUUID, tableName, eb.ChanNo,
		eb.EventCountsByTable[tableName].TotalEvents,
		eb.EventCountsByTable[tableName].NumInserts,
		eb.EventCountsByTable[tableName].NumUpdates,
		eb.EventCountsByTable[tableName].NumDeletes)
}

func (eb *EventBatch) GetTableNames() []string {
	return lo.Keys(eb.EventCountsByTable)
}

func (eb *EventBatch) updateCounts(targetSchema string) {
	for _, event := range eb.Events {
		tableName := event.getTableName(targetSchema)
		if _, ok := eb.EventCountsByTable[tableName]; !ok {
			eb.EventCountsByTable[tableName] = &EventCounter{}
		}
		eb.EventCountsByTable[tableName].CountEvent(event)
		eb.EventCounts.CountEvent(event)
	}
}

type EventCounter struct {
	TotalEvents int64
	NumInserts  int64
	NumUpdates  int64
	NumDeletes  int64
}

func (ec *EventCounter) CountEvent(ev *Event) {
	ec.TotalEvents++
	switch ev.Op {
	case "c":
		ec.NumInserts++
	case "u":
		ec.NumUpdates++
	case "d":
		ec.NumDeletes++
	}
}
