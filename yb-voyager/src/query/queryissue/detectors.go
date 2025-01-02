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
	"slices"

	mapset "github.com/deckarep/golang-set/v2"
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
)

// To Add a new unsupported query construct implement this interface for all possible nodes for that construct
// each detector will work on specific type of node
type UnsupportedConstructDetector interface {
	Detect(msg protoreflect.Message) error
	GetIssues() []QueryIssue
}

type FuncCallDetector struct {
	query string

	advisoryLocksFuncsDetected mapset.Set[string]
	xmlFuncsDetected           mapset.Set[string]
	aggFuncsDetected           mapset.Set[string]
	regexFuncsDetected         mapset.Set[string]
	loFuncsDetected            mapset.Set[string]
}

func NewFuncCallDetector(query string) *FuncCallDetector {
	return &FuncCallDetector{
		query:                      query,
		advisoryLocksFuncsDetected: mapset.NewThreadUnsafeSet[string](),
		xmlFuncsDetected:           mapset.NewThreadUnsafeSet[string](),
		aggFuncsDetected:           mapset.NewThreadUnsafeSet[string](),
		regexFuncsDetected:         mapset.NewThreadUnsafeSet[string](),
		loFuncsDetected:            mapset.NewThreadUnsafeSet[string](),
	}
}

// Detect checks if a FuncCall node uses an unsupported function.
func (d *FuncCallDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_FUNCCALL_NODE {
		return nil
	}

	_, funcName := queryparser.GetFuncNameFromFuncCall(msg)
	log.Debugf("fetched function name from %s node: %q", queryparser.PG_QUERY_FUNCCALL_NODE, funcName)

	if unsupportedAdvLockFuncs.ContainsOne(funcName) {
		d.advisoryLocksFuncsDetected.Add(funcName)
	}
	if unsupportedXmlFunctions.ContainsOne(funcName) {
		d.xmlFuncsDetected.Add(funcName)
	}
	if unsupportedRegexFunctions.ContainsOne(funcName) {
		d.regexFuncsDetected.Add(funcName)
	}

	if unsupportedAggFunctions.ContainsOne(funcName) {
		d.aggFuncsDetected.Add(funcName)
	}
	if unsupportedLargeObjectFunctions.ContainsOne(funcName) {
		d.loFuncsDetected.Add(funcName)
	}

	return nil
}

func (d *FuncCallDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.advisoryLocksFuncsDetected.Cardinality() > 0 {
		issues = append(issues, NewAdvisoryLocksIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	if d.xmlFuncsDetected.Cardinality() > 0 {
		issues = append(issues, NewXmlFunctionsIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	if d.aggFuncsDetected.Cardinality() > 0 {
		issues = append(issues, NewAggregationFunctionIssue(DML_QUERY_OBJECT_TYPE, "", d.query, d.aggFuncsDetected.ToSlice()))
	}
	if d.regexFuncsDetected.Cardinality() > 0 {
		issues = append(issues, NewRegexFunctionsIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	if d.loFuncsDetected.Cardinality() > 0 {
		issues = append(issues, NewLOFuntionsIssue(DML_QUERY_OBJECT_TYPE, "", d.query, d.loFuncsDetected.ToSlice()))
	}
	return issues
}

type ColumnRefDetector struct {
	query                            string
	unsupportedSystemColumnsDetected mapset.Set[string]
}

func NewColumnRefDetector(query string) *ColumnRefDetector {
	return &ColumnRefDetector{
		query:                            query,
		unsupportedSystemColumnsDetected: mapset.NewThreadUnsafeSet[string](),
	}
}

// Detect checks if a ColumnRef node uses an unsupported system column
func (d *ColumnRefDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_COLUMNREF_NODE {
		return nil
	}

	_, colName := queryparser.GetColNameFromColumnRef(msg)
	log.Debugf("fetched column name from %s node: %q", queryparser.PG_QUERY_COLUMNREF_NODE, colName)

	if unsupportedSysCols.ContainsOne(colName) {
		d.unsupportedSystemColumnsDetected.Add(colName)
	}
	return nil
}

func (d *ColumnRefDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.unsupportedSystemColumnsDetected.Cardinality() > 0 {
		issues = append(issues, NewSystemColumnsIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	return issues
}

type XmlExprDetector struct {
	query    string
	detected bool
}

func NewXmlExprDetector(query string) *XmlExprDetector {
	return &XmlExprDetector{
		query: query,
	}
}

// Detect checks if a XmlExpr node is present, means Xml type/functions are used
func (d *XmlExprDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_XMLEXPR_NODE {
		d.detected = true
	}
	return nil
}

func (d *XmlExprDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.detected {
		issues = append(issues, NewXmlFunctionsIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	return issues
}

/*
RangeTableFunc node manages functions that produce tables, structuring output into rows and columns
for SQL queries. Example: XMLTABLE()

ASSUMPTION:
- RangeTableFunc is used for representing XMLTABLE() only as of now
- Comments from Postgres code:
  - RangeTableFunc - raw form of "table functions" such as XMLTABLE
  - Note: JSON_TABLE is also a "table function", but it uses JsonTable node,
  - not RangeTableFunc.

- link: https://github.com/postgres/postgres/blob/ea792bfd93ab8ad4ef4e3d1a741b8595db143677/src/include/nodes/parsenodes.h#L651
*/
type RangeTableFuncDetector struct {
	query    string
	detected bool
}

func NewRangeTableFuncDetector(query string) *RangeTableFuncDetector {
	return &RangeTableFuncDetector{
		query: query,
	}
}

// Detect checks if a RangeTableFunc node is present for a XMLTABLE() function
func (d *RangeTableFuncDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_RANGETABLEFUNC_NODE {
		if queryparser.IsXMLTable(msg) {
			d.detected = true
		}
	}
	return nil
}

func (d *RangeTableFuncDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.detected {
		issues = append(issues, NewXmlFunctionsIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	return issues
}

type SelectStmtDetector struct {
	query                       string
	limitOptionWithTiesDetected bool
}

func NewSelectStmtDetector(query string) *SelectStmtDetector {
	return &SelectStmtDetector{
		query: query,
	}
}

func (d *SelectStmtDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_SELECTSTMT_NODE {
		selectStmtNode, err := queryparser.ProtoAsSelectStmt(msg)
		if err != nil {
			return err
		}
		// checks if a SelectStmt node uses a FETCH clause with TIES
		// https://www.postgresql.org/docs/13/sql-select.html#SQL-LIMIT
		if selectStmtNode.LimitOption == queryparser.LIMIT_OPTION_WITH_TIES {
			d.limitOptionWithTiesDetected = true
		}
	}
	return nil
}

func (d *SelectStmtDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.limitOptionWithTiesDetected {
		issues = append(issues, NewFetchWithTiesIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	return issues
}

type CopyCommandUnsupportedConstructsDetector struct {
	query                          string
	copyFromWhereConstructDetected bool
	copyOnErrorConstructDetected   bool
}

func NewCopyCommandUnsupportedConstructsDetector(query string) *CopyCommandUnsupportedConstructsDetector {
	return &CopyCommandUnsupportedConstructsDetector{
		query: query,
	}
}

// Detect if COPY command uses unsupported syntax i.e. COPY FROM ... WHERE and COPY... ON_ERROR
func (d *CopyCommandUnsupportedConstructsDetector) Detect(msg protoreflect.Message) error {
	// Check if the message is a COPY statement
	if msg.Descriptor().FullName() != queryparser.PG_QUERY_COPYSTSMT_NODE {
		return nil // Not a COPY statement, nothing to detect
	}

	// Check for COPY FROM ... WHERE clause
	fromField := queryparser.GetBoolField(msg, "is_from")
	whereField := queryparser.GetMessageField(msg, "where_clause")
	if fromField && whereField != nil {
		d.copyFromWhereConstructDetected = true
	}

	// Check for COPY ... ON_ERROR clause
	defNames, err := queryparser.TraverseAndExtractDefNamesFromDefElem(msg)
	if err != nil {
		log.Errorf("error extracting defnames from COPY statement: %v", err)
	}
	if slices.Contains(defNames, "on_error") {
		d.copyOnErrorConstructDetected = true
	}

	return nil
}

func (d *CopyCommandUnsupportedConstructsDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.copyFromWhereConstructDetected {
		issues = append(issues, NewCopyFromWhereIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	if d.copyOnErrorConstructDetected {
		issues = append(issues, NewCopyOnErrorIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	return issues
}

type JsonConstructorFuncDetector struct {
	query                                       string
	unsupportedJsonConstructorFunctionsDetected mapset.Set[string]
}

func NewJsonConstructorFuncDetector(query string) *JsonConstructorFuncDetector {
	return &JsonConstructorFuncDetector{
		query: query,
		unsupportedJsonConstructorFunctionsDetected: mapset.NewThreadUnsafeSet[string](),
	}
}

func (j *JsonConstructorFuncDetector) Detect(msg protoreflect.Message) error {
	switch queryparser.GetMsgFullName(msg) {
	case queryparser.PG_QUERY_JSON_ARRAY_AGG_NODE:
		j.unsupportedJsonConstructorFunctionsDetected.Add(JSON_ARRAYAGG)
	case queryparser.PG_QUERY_JSON_ARRAY_CONSTRUCTOR_AGG_NODE:
		j.unsupportedJsonConstructorFunctionsDetected.Add(JSON_ARRAY)
	case queryparser.PG_QUERY_JSON_OBJECT_AGG_NODE:
		j.unsupportedJsonConstructorFunctionsDetected.Add(JSON_OBJECTAGG)
	case queryparser.PG_QUERY_JSON_OBJECT_CONSTRUCTOR_NODE:
		j.unsupportedJsonConstructorFunctionsDetected.Add(JSON_OBJECT)
	}
	return nil
}

func (d *JsonConstructorFuncDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.unsupportedJsonConstructorFunctionsDetected.Cardinality() > 0 {
		issues = append(issues, NewJsonConstructorFunctionIssue(DML_QUERY_OBJECT_TYPE, "", d.query, d.unsupportedJsonConstructorFunctionsDetected.ToSlice()))
	}
	return issues
}

type JsonQueryFunctionDetector struct {
	query                                 string
	unsupportedJsonQueryFunctionsDetected mapset.Set[string]
}

func NewJsonQueryFunctionDetector(query string) *JsonQueryFunctionDetector {
	return &JsonQueryFunctionDetector{
		query:                                 query,
		unsupportedJsonQueryFunctionsDetected: mapset.NewThreadUnsafeSet[string](),
	}
}

func (j *JsonQueryFunctionDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_JSON_TABLE_NODE {
		/*
			SELECT * FROM json_table(
				'[{"a":10,"b":20},{"a":30,"b":40}]'::jsonb,
				'$[*]'
				COLUMNS (
					column_a int4 path '$.a',
					column_b int4 path '$.b'
				)
			);
			stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:530}}  location:530}}
			from_clause:{json_table:{context_item:{raw_expr:{type_cast:{arg:{a_const:{sval:{sval:"[{\"a\":10,\"b\":20},{\"a\":30,\"b\":40}]"}
			location:553}}  type_name:{names:{string:{sval:"jsonb"}}  .....  name_location:-1  location:601}
			columns:{json_table_column:{coltype:JTC_REGULAR  name:"column_a"  type_name:{names:{string:{sval:"int4"}}  typemod:-1  location:639}
			pathspec:{string:{a_const:{sval:{sval:"$.a"}  location:649}}  name_location:-1  location:649} ...
		*/
		j.unsupportedJsonQueryFunctionsDetected.Add(JSON_TABLE)
		return nil
	}
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_JSON_FUNC_EXPR_NODE {
		return nil
	}
	/*
		JsonExprOp -
			enumeration of SQL/JSON query function types
		typedef enum JsonExprOp
		{
			1. JSON_EXISTS_OP,				 JSON_EXISTS()
			2. JSON_QUERY_OP,				 JSON_QUERY()
			3. JSON_VALUE_OP,				 JSON_VALUE()
			4. JSON_TABLE_OP,				JSON_TABLE()
		} JsonExprOp;
	*/
	jsonExprFuncOpNum := queryparser.GetEnumNumField(msg, "op")
	switch jsonExprFuncOpNum {
	case 1:
		j.unsupportedJsonQueryFunctionsDetected.Add(JSON_EXISTS)
	case 2:
		j.unsupportedJsonQueryFunctionsDetected.Add(JSON_QUERY)
	case 3:
		j.unsupportedJsonQueryFunctionsDetected.Add(JSON_VALUE)
	case 4:
		j.unsupportedJsonQueryFunctionsDetected.Add(JSON_TABLE)
	}
	return nil
}

func (d *JsonQueryFunctionDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.unsupportedJsonQueryFunctionsDetected.Cardinality() > 0 {
		issues = append(issues, NewJsonQueryFunctionIssue(DML_QUERY_OBJECT_TYPE, "", d.query, d.unsupportedJsonQueryFunctionsDetected.ToSlice()))
	}
	return issues
}
