package queryparser

import (
	"slices"

	pg_query "github.com/pganalyze/pg_query_go/v5"
)

const (
	ADVISORY_LOCKS = "Advisory Locks"
	SYSTEM_COLUMNS = "System Columns"
	XML_FUNCTIONS  = "XML Functions"
)

// NOTE: pg parser converts the func names in parse tree to lower case by default
var advisoryLockFunctions = []string{
	"pg_advisory_lock", "pg_try_advisory_lock", "pg_advisory_xact_lock",
	"pg_advisory_unlock", "pg_advisory_unlock_all", "pg_try_advisory_xact_lock",
}

func (qp *QueryParser) containsAdvisoryLocks() bool {
	if qp.ParseTree == nil {
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
				if slices.Contains(advisoryLockFunctions, functionName) {
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
		if slices.Contains(advisoryLockFunctions, functionName) {
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
