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

// Checks for advisory lock functions in the main query's target list.
// This includes direct function calls like:
// Example: SELECT pg_advisory_lock(1);
// Example: SELECT col1, pg_try_advisory_lock(2) FROM my_table;
func containsAdvisoryLocksInTargetList(targetList []*pg_query.Node) bool {
	for _, target := range targetList {
		if resTarget := target.GetResTarget(); resTarget != nil {
			if funcCallNode, isFuncCall := resTarget.Val.Node.(*pg_query.Node_FuncCall); isFuncCall {
				funcList := funcCallNode.FuncCall.Funcname
				functionName := funcList[0].GetString_().Sval
				if slices.Contains(advisoryLockFunctions, functionName) {
					return true
				}
			}
		}
	}
	return false
}

// Recursively checks the FROM clause for subqueries containing advisory locks.
// This covers advisory locks embedded in subqueries such as:
// Example: SELECT * FROM (SELECT pg_advisory_lock(1)) AS lock_query;
// Example: SELECT * FROM my_table JOIN LATERAL (SELECT pg_try_advisory_xact_lock(3)) AS lock_check ON true;
func containsAdvisoryLocksInFromClause(fromClause []*pg_query.Node) bool {
	for _, fromItem := range fromClause {
		if subselectNode, isSubselect := fromItem.Node.(*pg_query.Node_RangeSubselect); isSubselect {
			subSelectStmt := subselectNode.RangeSubselect.Subquery.GetSelectStmt()
			if subSelectStmt != nil {
				// Recursively check for advisory locks in the subquery's target list
				if containsAdvisoryLocksInTargetList(subSelectStmt.TargetList) {
					return true
				}
				// Recursively check for advisory locks in the subquery's FROM clause
				if containsAdvisoryLocksInFromClause(subSelectStmt.FromClause) {
					return true
				}
			}
		}
	}
	return false
}

// Recursively checks the WHERE clause for advisory lock functions.
// This allows for advisory locks embedded within conditions like:
// Example: SELECT * FROM my_table WHERE pg_advisory_lock(1) = true;
func containsAdvisoryLocksInWhereClause(whereClause *pg_query.Node) bool {
	if whereClause == nil {
		return false
	}

	if funcCallNode := whereClause.GetFuncCall(); funcCallNode != nil {
		funcList := funcCallNode.Funcname
		functionName := funcList[0].GetString_().Sval
		if slices.Contains(advisoryLockFunctions, functionName) {
			return true
		}
	}

	// Recursively check for advisory locks in nested expressions
	switch n := whereClause.Node.(type) {
	case *pg_query.Node_BoolExpr:
		return containsAdvisoryLocksInNodeList(n.BoolExpr.Args)
	case *pg_query.Node_SubLink:
		return containsAdvisoryLocksInWhereClause(n.SubLink.Subselect)
		// Add more cases for other types of expressions as needed
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
