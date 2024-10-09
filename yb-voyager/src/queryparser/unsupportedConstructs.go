package queryparser

import (
	"slices"

	pg_query "github.com/pganalyze/pg_query_go/v5"
	log "github.com/sirupsen/logrus"
)

const (
	ADVISORY_LOCKS = "Advisory Locks"
	SYSTEM_COLUMNS = "System Columns"
	XML_FUNCTIONS  = "XML Functions"
)

// NOTE: pg parser converts the func names in parse tree to lower case by default
var unsupportedAdvLockFuncs = []string{
	"pg_advisory_lock", "pg_try_advisory_lock", "pg_advisory_xact_lock",
	"pg_advisory_unlock", "pg_advisory_unlock_all", "pg_try_advisory_xact_lock",
}

var unsupportedSysCols = []string{
	"xmin", "xmax", "cmin", "cmax", "ctid",
}

var xmlFunctions = []string{
	"cursor_to_xml", "cursor_to_xmlschema", // Cursor to XML
	"database_to_xml", "database_to_xml_and_xmlschema", "database_to_xmlschema", // Database to XML
	"query_to_xml", "query_to_xml_and_xmlschema", "query_to_xmlschema", // Query to XML
	"schema_to_xml", "schema_to_xml_and_xmlschema", "schema_to_xmlschema", // Schema to XML
	"table_to_xml", "table_to_xml_and_xmlschema", "table_to_xmlschema", // Table to XML
	"xmlagg", "xmlcomment", "xmlconcat2", // XML Aggregation and Construction
	"xmlexists", "xmlvalidate", // XML Existence and Validation
	"xpath", "xpath_exists", // XPath Functions
	"xml_in", "xml_out", "xml_recv", "xml_send", // System XML I/O
	"xml", // Data Type Conversion
}

func (qp *QueryParser) containsAdvisoryLocks() bool {
	if qp.ParseTree == nil {
		log.Infof("parse tree not available for query-%s", qp.QueryString)
		return false
	}

	selectStmtNode, isSelectStmt := qp.ParseTree.Stmts[0].Stmt.Node.(*pg_query.Node_SelectStmt)
	if !isSelectStmt {
		return false
	}

	// Check advisory locks in the target list
	if containsAdvisoryLocksInTargetList(selectStmtNode.SelectStmt.TargetList) {
		return true
	}

	// Check advisory locks in FROM clause
	if containsAdvisoryLocksInFromClause(selectStmtNode.SelectStmt.FromClause) {
		return true
	}

	// Check advisory locks in WHERE clause
	if containsAdvisoryLocksInWhereClause(selectStmtNode.SelectStmt.WhereClause) {
		return true
	}
	return false
}

/*
Checks for advisory lock functions in the main query's target list.

Example: SELECT pg_advisory_lock($1), COUNT(*) FROM cars
stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{func_call:{funcname:{string:{sval:"pg_advisory_lock"}}
args:{param_ref:{number:1 location:24}} funcformat:COERCE_EXPLICIT_CALL location:7}} location:7}}
target_list:{res_target:{val:{func_call:{funcname:{string:{sval:"count"}} agg_star:true funcformat:COERCE_EXPLICIT_CALL location:29}} location:29}}
from_clause:{range_var:{relname:"cars" inh:true relpersistence:"p" location:43}} limit_option:LIMIT_OPTION_DEFAULT op:SETOP_NONE}}}
*/

func containsAdvisoryLocksInTargetList(targetList []*pg_query.Node) bool {
	for _, target := range targetList {
		if resTarget := target.GetResTarget(); resTarget != nil {
			if funcCallNode, isFuncCall := resTarget.Val.Node.(*pg_query.Node_FuncCall); isFuncCall {
				funcList := funcCallNode.FuncCall.Funcname
				functionName := funcList[len(funcList)-1].GetString_().Sval
				if slices.Contains(unsupportedAdvLockFuncs, functionName) {
					return true
				}
			}
		}
	}
	return false
}

/*
Recursively checks the FROM clause for subqueries containing advisory locks.

Example: SELECT * FROM (SELECT pg_advisory_lock($1)) AS lock_acquired;
stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:7}}  location:7}}
from_clause:{range_subselect:{subquery:{select_stmt:{target_list:{res_target:{val:{func_call:{funcname:{string:{sval:"pg_advisory_lock"}}
args:{param_ref:{number:1  location:39}}  funcformat:COERCE_EXPLICIT_CALL  location:22}}  location:22}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}
alias:{aliasname:"lock_acquired"}}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}
*/
func containsAdvisoryLocksInFromClause(fromClause []*pg_query.Node) bool {
	for _, fromItem := range fromClause {
		if subselectNode, isSubselect := fromItem.Node.(*pg_query.Node_RangeSubselect); isSubselect {
			subSelectStmt := subselectNode.RangeSubselect.Subquery.GetSelectStmt()
			if subSelectStmt != nil {
				// Recursively check for advisory locks in the subquery's target list and FROM clause
				if containsAdvisoryLocksInTargetList(subSelectStmt.TargetList) {
					return true
				}
				if containsAdvisoryLocksInFromClause(subSelectStmt.FromClause) {
					return true
				}
			}
		}
	}
	return false
}

/*
Recursively checks the WHERE clause for advisory lock functions.

Example: SELECT id, first_name FROM employees WHERE pg_try_advisory_lock($1) IS TRUE;
stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"id"}}  location:7}}  location:7}}
target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"first_name"}}  location:11}}  location:11}}
from_clause:{range_var:{relname:"employees"  inh:true  relpersistence:"p"  location:27}}
where_clause:{boolean_test:{arg:{func_call:{funcname:{string:{sval:"pg_try_advisory_lock"}}  args:{param_ref:{number:1  location:64}}
funcformat:COERCE_EXPLICIT_CALL  location:43}}  booltesttype:IS_TRUE  location:68}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}
*/
func containsAdvisoryLocksInWhereClause(whereClause *pg_query.Node) bool {
	if whereClause == nil {
		return false
	}

	if funcCallNode := whereClause.GetFuncCall(); funcCallNode != nil {
		funcList := funcCallNode.Funcname
		functionName := funcList[len(funcList)-1].GetString_().Sval
		if slices.Contains(unsupportedAdvLockFuncs, functionName) {
			return true
		}
	}

	// Recursively check for advisory locks in nested expressions
	switch n := whereClause.Node.(type) {
	case *pg_query.Node_SubLink:
		/*
			SELECT id, first_name FROM employees WHERE salary > $1 AND EXISTS (SELECT $2 FROM pg_advisory_lock($3))
			subSelectStmt: target_list:{res_target:{val:{param_ref:{number:2 location:77}} location:77}}
			from_clause:{range_function:{functions:{list:{items:{func_call:{funcname:{string:{sval:"pg_advisory_lock"}}
			args:{param_ref:{number:3 location:102}} funcformat:COERCE_EXPLICIT_CAL
		*/
		if subSelectStmt := n.SubLink.Subselect.GetSelectStmt(); subSelectStmt != nil {
			return containsAdvisoryLocksInTargetList(subSelectStmt.TargetList) ||
				containsAdvisoryLocksInFromClause(subSelectStmt.FromClause)
		}
	case *pg_query.Node_BoolExpr:
		/*
			SELECT id, first_name FROM employees WHERE pg_try_advisory_lock($1) IS TRUE AND salary > $2
			:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"id"}}  location:7}}  location:7}}
			target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"first_name"}}  location:11}}  location:11}}
			from_clause:{range_var:{relname:"employees"  inh:true  relpersistence:"p"  location:28}}
			where_clause:{bool_expr:{boolop:AND_EXPR  args:{boolean_test:{arg:{func_call:{funcname:{string:{sval:"pg_try_advisory_lock"}}
			args:{param_ref:{number:1  location:66}}  funcformat:COERCE_EXPLICIT_CALL  location:45}}  booltesttype:IS_TRUE  location:70}}  args:{a_expr:{kind:AEXPR_OP  name:{string:{sval:">"}}  lexpr:{column_ref:{fields:{string:{sval:"salary"}}  location:82}}  rexpr:{param_ref:{number:2  location:91}}  location:89}}  location:78}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}
		*/
		return containsAdvisoryLocksInNodeList(n.BoolExpr.Args)
	case *pg_query.Node_BooleanTest:
		// The function call is in the 'arg' field of the boolean test
		return containsAdvisoryLocksInWhereClause(n.BooleanTest.Arg)
	}

	return false
}

// Helper function to check advisory locks within a node list (for WHERE clause and nested conditions).
func containsAdvisoryLocksInNodeList(nodes []*pg_query.Node) bool {
	for _, node := range nodes {
		if containsAdvisoryLocksInWhereClause(node) {
			return true
		}
	}
	return false
}

func (qp *QueryParser) containsSystemColumns() bool {
	if qp.ParseTree == nil {
		log.Infof("parse tree not available for query-%s", qp.QueryString)
		return false
	}

	selectStmtNode, isSelectStmt := qp.ParseTree.Stmts[0].Stmt.Node.(*pg_query.Node_SelectStmt)
	// Note: currently considering only SELECTs but I/U/D dmls are also possible
	if !isSelectStmt {
		return false
	}

	if containsSystemColumnsInTargetList(selectStmtNode.SelectStmt.TargetList) {
		return true
	}

	if containsSystemColumnsInFromClause(selectStmtNode.SelectStmt.FromClause) {
		return true
	}

	if containsSystemColumnsInWhereClause(selectStmtNode.SelectStmt.WhereClause) {
		return true
	}

	return false
}

/*
Query: SELECT xmin, xmax FROM employees
ParseTree:	version:160001  stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"xmin"}}  location:7}}  location:7}}
target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"xmax"}}  location:13}}  location:13}}
from_clause:{range_var:{relname:"employees"  inh:true  relpersistence:"p"  location:23}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}
*/
func containsSystemColumnsInTargetList(targetList []*pg_query.Node) bool {
	for _, target := range targetList {
		targetRes := target.GetResTarget()
		if targetRes == nil {
			continue
		}

		val := targetRes.GetVal()
		if val == nil {
			continue
		}

		columnRef := val.GetColumnRef()
		colName := getColumnName(columnRef)
		if slices.Contains(unsupportedSysCols, colName) {
			return true
		}
	}

	return false
}

/*
Detects subqueries within FROM clause

Query1: SELECT * FROM (SELECT * FROM employees WHERE xmin = $1) AS version_info
ParseTree: version:160001  stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:7}}  location:7}}
from_clause:{range_subselect:{subquery:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:22}}  location:22}}
from_clause:{range_var:{relname:"employees"  inh:true  relpersistence:"p"  location:29}}  where_clause:{a_expr:{kind:AEXPR_OP  name:{string:{sval:"="}}
lexpr:{column_ref:{fields:{string:{sval:"xmin"}}  location:45}}  rexpr:{param_ref:{number:1  location:52}}  location:50}}
limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}  alias:{aliasname:"version_info"}}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}

Query2: SELECT * FROM (SELECT xmin, xmax FROM employees) AS version_info
ParseTree: version:160001  stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:7}}  location:7}}
from_clause:{range_subselect:{subquery:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"xmin"}}  location:22}}  location:22}}
target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"xmax"}}  location:28}}  location:28}}
from_clause:{range_var:{relname:"employees"  inh:true  relpersistence:"p"  location:38}}
limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}  alias:{aliasname:"version_info"}}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}
*/
func containsSystemColumnsInFromClause(fromClause []*pg_query.Node) bool {
	for _, fromItem := range fromClause {
		if subselectNode, isSubselect := fromItem.Node.(*pg_query.Node_RangeSubselect); isSubselect {
			subSelectStmt := subselectNode.RangeSubselect.Subquery.GetSelectStmt()
			if subSelectStmt != nil {
				// Recursively check for advisory locks in the subquery's target list and FROM clause
				if containsSystemColumnsInTargetList(subSelectStmt.TargetList) {
					return true
				}
				if containsSystemColumnsInFromClause(subSelectStmt.FromClause) {
					return true
				}
				if containsSystemColumnsInWhereClause(subSelectStmt.WhereClause) {
					return true
				}
			}
		}
	}
	return false
}

/*
Query1: SELECT * FROM employees WHERE xmin = $1
ParseTree1: version:160001  stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:7}}  location:7}}
from_clause:{range_var:{relname:"employees"  inh:true  relpersistence:"p"  location:14}}
where_clause:{a_expr:{kind:AEXPR_OP  name:{string:{sval:"="}}
lexpr:{column_ref:{fields:{string:{sval:"xmin"}}  location:30}}  rexpr:{param_ref:{number:1  location:37}}  location:35}}
limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}

Query2: SELECT * FROM employees WHERE xmin = $1 AND xmax = $2
ParseTree2: version:160001 stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}} location:7}} location:7}}
from_clause:{range_var:{relname:"employees" inh:true relpersistence:"p" location:14}}
where_clause:{bool_expr:{boolop:AND_EXPR args:{a_expr:{kind:AEXPR_OP name:{string:{sval:"="}}
lexpr:{column_ref:{fields:{string:{sval:"xmin"}} location:30}}
rexpr:{param_ref:{number:1 location:37}} location:35}}
args:{a_expr:{kind:AEXPR_OP name:{string:{sval:"="}} lexpr:{column_ref:{fields:{string:{sval:"xmax"}} location:44}} rexpr:{param_ref:{number:2 location:51}} location:49}} location:40}} limit_option:LIMIT_OPTION_DEFAULT op:SETOP_NONE}}}
*/
func containsSystemColumnsInWhereClause(whereClause *pg_query.Node) bool {
	if whereClause == nil {
		return false
	}

	switch n := whereClause.Node.(type) {
	// Check for BoolExpr (AND/OR expressions)
	case *pg_query.Node_BoolExpr:
		// Recursively check all the arguments (conditions) in the BoolExpr
		for _, arg := range n.BoolExpr.Args {
			if containsSystemColumnsInWhereClause(arg) {
				return true
			}
		}
		return false

	// Handle the AExpr case (arithmetic/abstract expressions like comparisons)
	case *pg_query.Node_AExpr:
		arithExpr := whereClause.GetAExpr()
		if arithExpr == nil {
			return false
		}

		lexpr := arithExpr.GetLexpr()
		rexpr := arithExpr.GetRexpr()
		var leftColName, rightColName string
		if lexpr != nil {
			leftColName = getColumnName(lexpr.GetColumnRef())
		}
		if rexpr != nil {
			rightColName = getColumnName(rexpr.GetColumnRef())
		}

		return slices.Contains(unsupportedSysCols, leftColName) || slices.Contains(unsupportedSysCols, rightColName)
	}

	return false
}

func (qp *QueryParser) containsXmlFunctions() bool {
	selectStmtNode, isSelectStmt := qp.ParseTree.Stmts[0].Stmt.Node.(*pg_query.Node_SelectStmt)
	// Note: currently considering only SELECTs
	if !isSelectStmt {
		return false
	}

	if containsXmlFunctionsInTargetList(selectStmtNode.SelectStmt.TargetList) {
		return true
	}

	if containsXmlFunctionsInFromClause(selectStmtNode.SelectStmt.FromClause) {
		return true
	}

	if containsXmlFunctionsInWhereClause(selectStmtNode.SelectStmt.WhereClause) {
		return true
	}
	return false
}

/*
Query: SELECT id, xmlelement(name "employee", name) AS employee_data FROM employees
ParseTree: version:160001  stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"id"}}  location:7}}  location:7}}
target_list:{res_target:{name:"employee_data"  val:{xml_expr:{op:IS_XMLELEMENT  name:"employee"  args:{column_ref:{fields:{string:{sval:"name"}}  location:39}}  xmloption:XMLOPTION_DOCUMENT  location:11}}  location:11}}
from_clause:{range_var:{relname:"employees"  inh:true  relpersistence:"p"  location:67}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}
*/
func containsXmlFunctionsInTargetList(targetList []*pg_query.Node) bool {
	for _, target := range targetList {
		resTarget := target.GetResTarget()
		if resTarget == nil {
			return false
		}

		xmlExpr := resTarget.Val.GetXmlExpr()
		if xmlExpr != nil {
			return true
		}
	}
	return false
}

/*
Query: SELECT * FROM xmltable(

	$1
	PASSING xmlparse(document $2)
	COLUMNS name TEXT PATH $3

) AS emp_data
ParseTree: version:160001  stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{a_star:{}}  location:7}}  location:7}}
from_clause:{range_table_func:{docexpr:{xml_expr:{op:IS_XMLPARSE  args:{param_ref:{number:2  location:61}}
args:{a_const:{boolval:{}  location:-1}}  xmloption:XMLOPTION_DOCUMENT  location:43}}  rowexpr:{param_ref:{number:1  location:28}}
columns:{range_table_func_col:{colname:"name"  type_name:{names:{string:{sval:"text"}}  typemod:-1  location:82}  colexpr:{param_ref:{number:3  location:92}}  location:77}}  alias:{aliasname:"emp_data"}  location:14}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}
*/
func containsXmlFunctionsInFromClause(fromClause []*pg_query.Node) bool {
	for _, fromItem := range fromClause {
		// from clause is a subquery
		if subselectNode, isSubselect := fromItem.Node.(*pg_query.Node_RangeSubselect); isSubselect {
			subSelectStmt := subselectNode.RangeSubselect.Subquery.GetSelectStmt()
			if subSelectStmt != nil {
				// Recursively check for xml functions in the subquery's target list and FROM clause
				if containsXmlFunctionsInTargetList(subSelectStmt.TargetList) {
					return true
				}
				if containsXmlFunctionsInFromClause(subSelectStmt.FromClause) {
					return true
				}
				if containsXmlFunctionsInWhereClause(subSelectStmt.WhereClause) {
					return true
				}
			}
		} else { // normal case
			rangeTblFunc := fromItem.GetRangeTableFunc()
			if rangeTblFunc == nil {
				return false
			}

			xmlExpr := rangeTblFunc.Docexpr.GetXmlExpr()
			if xmlExpr != nil {
				return true
			}
		}
	}
	return false
}

/*
Query: SELECT id, name FROM employees WHERE xmlexists($1 PASSING BY VALUE data)
ParseTree: version:160001  stmts:{stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"id"}}  location:7}}  location:7}}
target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"name"}}  location:11}}  location:11}}
from_clause:{range_var:{relname:"employees"  inh:true  relpersistence:"p"  location:22}}
where_clause:{func_call:{funcname:{string:{sval:"pg_catalog"}}  funcname:{string:{sval:"xmlexists"}}
args:{param_ref:{number:1  location:49}}  args:{column_ref:{fields:{string:{sval:"data"}}  location:69}}
funcformat:COERCE_SQL_SYNTAX  location:39}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}}
*/
func containsXmlFunctionsInWhereClause(whereClause *pg_query.Node) bool {
	if whereClause == nil {
		return false
	}

	if funcCallNode := whereClause.GetFuncCall(); funcCallNode != nil {
		funcList := funcCallNode.Funcname
		functionName := funcList[len(funcList)-1].GetString_().Sval
		if slices.Contains(xmlFunctions, functionName) {
			return true
		}
	}

	return false
}

// ========= parse tree helper functions

// getColumnName extracts the column name from a ColumnRef node.
// It returns an empty string if no valid column name is found.
func getColumnName(columnRef *pg_query.ColumnRef) string {
	if columnRef == nil {
		return ""
	}

	fields := columnRef.GetFields()
	if len(fields) == 0 {
		return ""
	}

	// Extract the last field as the column name
	lastField := fields[len(fields)-1]
	if lastField == nil || lastField.GetString_() == nil {
		return ""
	}

	return lastField.GetString_().Sval
}
