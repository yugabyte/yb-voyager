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
	"strings"

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
	rangeAggFuncsDetected      mapset.Set[string]
	regexFuncsDetected         mapset.Set[string]
	loFuncsDetected            mapset.Set[string]
	anyValueFuncDetected       bool
}

func NewFuncCallDetector(query string) *FuncCallDetector {
	return &FuncCallDetector{
		query:                      query,
		advisoryLocksFuncsDetected: mapset.NewThreadUnsafeSet[string](),
		xmlFuncsDetected:           mapset.NewThreadUnsafeSet[string](),
		rangeAggFuncsDetected:      mapset.NewThreadUnsafeSet[string](),
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

	if unsupportedRangeAggFunctions.ContainsOne(funcName) {
		d.rangeAggFuncsDetected.Add(funcName)
	}

	if funcName == ANY_VALUE {
		d.anyValueFuncDetected = true
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
	if d.rangeAggFuncsDetected.Cardinality() > 0 {
		issues = append(issues, NewRangeAggregateFunctionIssue(DML_QUERY_OBJECT_TYPE, "", d.query, d.rangeAggFuncsDetected.ToSlice()))
	}
	if d.anyValueFuncDetected {
		issues = append(issues, NewAnyValueAggregateFunctionIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
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

type JsonbSubscriptingDetector struct {
	query          string
	jsonbColumns   []string
	detected       bool
	jsonbFunctions []string
}

func NewJsonbSubscriptingDetector(query string, jsonbColumns []string, jsonbFunctions []string) *JsonbSubscriptingDetector {
	return &JsonbSubscriptingDetector{
		query:          query,
		jsonbColumns:   jsonbColumns,
		jsonbFunctions: jsonbFunctions,
	}
}

func (j *JsonbSubscriptingDetector) Detect(msg protoreflect.Message) error {

	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_A_INDIRECTION_NODE {
		return nil
	}
	aIndirectionNode, ok := queryparser.GetAIndirectionNode(msg)
	if !ok {
		return nil
	}

	/*
		Indirection node is to determine if subscripting is happening in the query e.g. data['name'] - jsonb, numbers[1] - array type, and ('{"a": {"b": {"c": 1}}}'::jsonb)['a']['b']['c'];
		Arg is the data on which subscripting is happening e.g data, numbers (columns) and constant data type casted to jsonb ('{"a": {"b": {"c": 1}}}'::jsonb)
		Indices are the actual fields that are being accessed while subscripting or the index in case of array type e.g. name, 1, a, b etc.
		So we are checking the arg is of jsonb type here
	*/
	arg := aIndirectionNode.GetArg()
	if arg == nil {
		return nil
	}
	/*
		Caveats -

		Still with this approach we won't be able to cover all cases e.g.

		select ab_data['name'] from (select Data as ab_data from test_jsonb);`,

		parseTree - stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{a_indirection:{arg:{column_ref:{fields:{string:{sval:"ab_data"}}  location:9}}
		indirection:{a_indices:{uidx:{a_const:{sval:{sval:"name"}  location:17}}}}}}  location:9}}  from_clause:{range_subselect:{subquery:{select_stmt:{
		target_list:{res_target:{name:"ab_data"  val:{column_ref:{fields:{string:{sval:"data"}}  location:38}}  location:38}}
		from_clause:{range_var:{relname:"test_jsonb"  inh:true  relpersistence:"p"  location:59}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}}
		limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}
	*/
	if queryparser.DoesNodeHandleJsonbData(arg, j.jsonbColumns, j.jsonbFunctions) {
		j.detected = true
	}
	return nil
}

func (j *JsonbSubscriptingDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if j.detected {
		issues = append(issues, NewJsonbSubscriptingIssue(DML_QUERY_OBJECT_TYPE, "", j.query))
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
	if msg.Descriptor().FullName() != queryparser.PG_QUERY_COPY_STMT_NODE {
		return nil // Not a COPY statement, nothing to detect
	}

	// Check for COPY FROM ... WHERE clause
	fromField := queryparser.GetBoolField(msg, "is_from")
	whereField := queryparser.GetMessageField(msg, "where_clause")
	if fromField && whereField != nil {
		d.copyFromWhereConstructDetected = true
	}

	// Check for COPY ... ON_ERROR clause
	defNamesWithValues, err := queryparser.TraverseAndExtractDefNamesFromDefElem(msg)
	if err != nil {
		log.Errorf("error extracting defnames from COPY statement: %v", err)
	}
	if _, ok := defNamesWithValues["on_error"]; ok {
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

type MergeStatementDetector struct {
	query                    string
	isMergeStatementDetected bool
}

func NewMergeStatementDetector(query string) *MergeStatementDetector {
	return &MergeStatementDetector{
		query: query,
	}
}

func (m *MergeStatementDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_MERGE_STMT_NODE {
		m.isMergeStatementDetected = true
	}
	return nil

}

func (m *MergeStatementDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if m.isMergeStatementDetected {
		issues = append(issues, NewMergeStatementIssue(DML_QUERY_OBJECT_TYPE, "", m.query))
	}
	return issues
}

type UniqueNullsNotDistinctDetector struct {
	query    string
	detected bool
}

func NewUniqueNullsNotDistinctDetector(query string) *UniqueNullsNotDistinctDetector {
	return &UniqueNullsNotDistinctDetector{
		query: query,
	}
}

// Detect checks if a unique constraint is defined which has nulls not distinct
func (d *UniqueNullsNotDistinctDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_INDEX_STMT_NODE {
		indexStmt, err := queryparser.ProtoAsIndexStmt(msg)
		if err != nil {
			return err
		}

		if indexStmt.Unique && indexStmt.NullsNotDistinct {
			d.detected = true
		}
	} else if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_CONSTRAINT_NODE {
		constraintNode, err := queryparser.ProtoAsTableConstraint(msg)
		if err != nil {
			return err
		}

		if constraintNode.Contype == queryparser.UNIQUE_CONSTR_TYPE && constraintNode.NullsNotDistinct {
			d.detected = true
		}
	}

	return nil
}

func (d *UniqueNullsNotDistinctDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.detected {
		issues = append(issues, NewUniqueNullsNotDistinctIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	return issues
}

type JsonPredicateExprDetector struct {
	query    string
	detected bool
}

func NewJsonPredicateExprDetector(query string) *JsonPredicateExprDetector {
	return &JsonPredicateExprDetector{
		query: query,
	}
}
func (j *JsonPredicateExprDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_JSON_IS_PREDICATE_NODE {
		/*
			SELECT  js IS JSON "json?" FROM (VALUES ('123')) foo(js);
			stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"js"}}  location:337}}  location:337}}
			target_list:{res_target:{name:"json?"  val:{json_is_predicate:{expr:{column_ref:{fields:{string:{sval:"js"}}  location:341}}
			format:{format_type:JS_FORMAT_DEFAULT  encoding:JS_ENC_DEFAULT  location:-1}  item_type:JS_TYPE_ANY  location:341}}  location:341}} ...
		*/
		j.detected = true
	}
	return nil
}

func (j *JsonPredicateExprDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if j.detected {
		issues = append(issues, NewJsonPredicateIssue(DML_QUERY_OBJECT_TYPE, "", j.query))
	}
	return issues
}

type NonDecimalIntegerLiteralDetector struct {
	query    string
	detected bool
}

func NewNonDecimalIntegerLiteralDetector(query string) *NonDecimalIntegerLiteralDetector {
	return &NonDecimalIntegerLiteralDetector{
		query: query,
	}
}
func (n *NonDecimalIntegerLiteralDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_ACONST_NODE {
		return nil
	}
	aConstNode, err := queryparser.ProtoAsAConstNode(msg)
	if err != nil {
		return err
	}
	/*
		Caveats can't report this issue for cases like -
		1. DML having the constant change to parameters in PGSS - SELECT $1, $2 as binary;
		2. DDL having the CHECK or DEFAULT -
			- pg_dump will not dump non-decimal literal, it will give out the decimal constant only
			- even if in some user added DDL in schema file during analyze-schema it can't be detected as parse tree doesn't info
			e.g. -
			CREATE TABLE bitwise_example (
				id SERIAL PRIMARY KEY,
				flags INT DEFAULT 0x0F CHECK (flags & 0x01 = 0x01) -- Hexadecimal bitwise check
			);
			parseTree - create_stmt:{relation:{relname:"bitwise_example" inh:true relpersistence:"p" location:15} ...
			table_elts:{column_def:{colname:"flags" type_name:{names:{string:{sval:"pg_catalog"}} names:{string:{sval:"int4"}} typemod:-1
			location:70} is_local:true constraints:{constraint:{contype:CONSTR_DEFAULT raw_expr:{a_const:{ival:{ival:15} location:82}}
			location:74}} constraints:{constraint:{contype:CONSTR_CHECK initially_valid:true raw_expr:{a_expr:{kind:AEXPR_OP name:{string:{sval:"="}}
			lexpr:{a_expr:{kind:AEXPR_OP name:{string:{sval:"&"}} lexpr:{column_ref:{fields:{string:{sval:"flags"}} location:94}} rexpr:{a_const:{ival:{ival:1}
			location:102}} location:100}} rexpr:{a_const:{ival:{ival:1} location:109}} location:107}} ..

		So mostly be detecting this in PLPGSQL cases
	*/
	switch {
	case aConstNode.GetFval() != nil:
		/*
			fval - float val representation in postgres
			ival - integer val
			Fval is only one which stores the non-decimal integers information if used in queries, e.g. SELECT 5678901234, 0o52237223762 as octal;
			select_stmt:{target_list:{res_target:{val:{a_const:{fval:{fval:"5678901234"} location:9}} location:9}}
			target_list:{res_target:{name:"octal" val:{a_const:{fval:{fval:"0o52237223762"} location:21}} location:21}}

			ival stores the decimal integers if non-decimal is not used in the query, e.g. SELECT 1, 2;
			select_stmt:{target_list:{res_target:{val:{a_const:{ival:{ival:1} location:8}} location:8}}
			target_list:{res_target:{val:{a_const:{ival:{ival:2} location:10}} location:10}}
		*/
		fval := aConstNode.GetFval().Fval
		for _, literal := range nonDecimalIntegerLiterals {
			if strings.HasPrefix(fval, literal) {
				n.detected = true
			}
		}
	}
	return nil
}

func (n *NonDecimalIntegerLiteralDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if n.detected {
		issues = append(issues, NewNonDecimalIntegerLiteralIssue(DML_QUERY_OBJECT_TYPE, "", n.query))
	}
	return issues
}

type CommonTableExpressionDetector struct {
	query                      string
	materializedClauseDetected bool
}

func NewCommonTableExpressionDetector(query string) *CommonTableExpressionDetector {
	return &CommonTableExpressionDetector{
		query: query,
	}
}

func (c *CommonTableExpressionDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_CTE_NODE {
		return nil
	}
	/*
		with_clause:{ctes:{common_table_expr:{ctename:"cte" ctematerialized:CTEMaterializeNever
		ctequery:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}} location:939}} location:939}} from_clause:{range_var:{relname:"a" inh:true relpersistence:"p" location:946}} limit_option:LIMIT_OPTION_DEFAULT op:SETOP_NONE}} location:906}} location:901} op:SETOP_NONE}} stmt_location:898
	*/
	cteNode, err := queryparser.ProtoAsCTENode(msg)
	if err != nil {
		return err
	}
	if cteNode.Ctematerialized != queryparser.CTE_MATERIALIZED_DEFAULT {
		//MATERIALIZED / NOT MATERIALIZED clauses in CTE is not supported in YB
		c.materializedClauseDetected = true
	}
	return nil
}

func (c *CommonTableExpressionDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if c.materializedClauseDetected {
		issues = append(issues, NewCTEWithMaterializedIssue(DML_QUERY_OBJECT_TYPE, "", c.query))
	}
	return issues
}

type DatabaseOptionsDetector struct {
	query               string
	dbName              string
	pg15OptionsDetected mapset.Set[string]
	pg17OptionsDetected mapset.Set[string]
}

func NewDatabaseOptionsDetector(query string) *DatabaseOptionsDetector {
	return &DatabaseOptionsDetector{
		query:               query,
		pg15OptionsDetected: mapset.NewThreadUnsafeSet[string](),
		pg17OptionsDetected: mapset.NewThreadUnsafeSet[string](),
	}
}

func (d *DatabaseOptionsDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_CREATEDB_STMT_NODE {
		return nil
	}
	/*
		stmts:{stmt:{createdb_stmt:{dbname:"test" options:{def_elem:{defname:"oid" arg:{integer:{ival:121231}} defaction:DEFELEM_UNSPEC
		location:22}}}} stmt_len:32} stmts:{stmt:{listen_stmt:{conditionname:"my_table_changes"}} stmt_location:33 stmt_len:25}
	*/
	d.dbName = queryparser.GetStringField(msg, "dbname")
	defNames, err := queryparser.TraverseAndExtractDefNamesFromDefElem(msg)
	if err != nil {
		return err
	}
	for defName, _ := range defNames {
		if unsupportedDatabaseOptionsFromPG15.ContainsOne(defName) {
			d.pg15OptionsDetected.Add(defName)
		}
		if unsupportedDatabaseOptionsFromPG17.ContainsOne(defName) {
			d.pg17OptionsDetected.Add(defName)
		}
	}
	return nil
}

func (d *DatabaseOptionsDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if d.pg15OptionsDetected.Cardinality() > 0 {
		issues = append(issues, NewDatabaseOptionsPG15Issue("DATABASE", d.dbName, d.query, d.pg15OptionsDetected.ToSlice()))
	}
	if d.pg17OptionsDetected.Cardinality() > 0 {
		issues = append(issues, NewDatabaseOptionsPG17Issue("DATABASE", d.dbName, d.query, d.pg17OptionsDetected.ToSlice()))
	}
	return issues
}

type ListenNotifyIssueDetector struct {
	query    string
	detected bool
}

func NewListenNotifyIssueDetector(query string) *ListenNotifyIssueDetector {
	return &ListenNotifyIssueDetector{
		query: query,
	}
}

func (ln *ListenNotifyIssueDetector) Detect(msg protoreflect.Message) error {
	switch queryparser.GetMsgFullName(msg) {
	case queryparser.PG_QUERY_FUNCCALL_NODE:
		/*
			example-SELECT pg_notify('my_notification', 'Payload from pg_notify');
				parseTree - stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{func_call:{funcname:{string:{sval:"pg_notify"}}
				args:{a_const:{sval:{sval:"my_notification"}  location:129}}  args:{a_const:{sval:{sval:"Payload from pg_notify"}
		*/
		_, funcName := queryparser.GetFuncNameFromFuncCall(msg)
		if funcName == PG_NOTIFY_FUNC {
			ln.detected = true
		}
	case queryparser.PG_QUERY_LISTEN_STMT_NODE, queryparser.PG_QUERY_NOTIFY_STMT_NODE, queryparser.PG_QUERY_UNLISTEN_STMT_NODE:
		/*
			examples -
				LISTEN my_table_changes;
				NOTIFY my_notification, 'Payload from pg_notify';
				UNLISTEN my_notification;
			parseTrees-
				stmts:{stmt:{listen_stmt:{conditionname:"my_table_changes"}}  stmt_len:25}
				stmts:{stmt:{notify_stmt:{conditionname:"my_table_changes" payload:"Row inserted: id=1, name=Alice"}}  stmt_location:26  stmt_len:58}
				stmts:{stmt:{unlisten_stmt:{conditionname:"my_notification"}}  stmt_location:85  stmt_len:25}
		*/
		ln.detected = true
	}

	return nil
}

func (ln *ListenNotifyIssueDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if ln.detected {
		issues = append(issues, NewListenNotifyIssue(DML_QUERY_OBJECT_TYPE, "", ln.query))
	}
	return issues
}

type TwoPhaseCommitDetector struct {
	query    string
	detected bool
}

func NewTwoPhaseCommitDetector(query string) *TwoPhaseCommitDetector {
	return &TwoPhaseCommitDetector{
		query: query,
	}
}

func (t *TwoPhaseCommitDetector) Detect(msg protoreflect.Message) error {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_TRANSACTION_STMT_NODE {
		return nil
	}
	transactionStmtNode, err := queryparser.ProtoAsTransactionStmt(msg)
	if err != nil {
		return err
	}
	/*
		PREPARE TRANSACTION 'tx1';
		stmts:{stmt:{transaction_stmt:{kind:TRANS_STMT_PREPARE  gid:"txn1"  location:22}}  stmt_len:28}

		Caveats:
			Can't detect them from PGSS as the query is coming like this `PREPARE TRANSACTION $1` and parser is failing to parse it.
			Only detecting in some PLPGSQL cases.
	
	*/
	switch transactionStmtNode.Kind {
	case queryparser.PREPARED_TRANSACTION_KIND, queryparser.COMMIT_PREPARED_TRANSACTION_KIND,
		queryparser.ROLLBACK_PREPARED_TRANSACTION_KIND:
		t.detected = true
	}
	return nil
}

func (t *TwoPhaseCommitDetector) GetIssues() []QueryIssue {
	var issues []QueryIssue
	if t.detected {
		issues = append(issues, NewTwoPhaseCommitIssue(DML_QUERY_OBJECT_TYPE, "", t.query))
	}
	return issues
}
