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
	log "github.com/sirupsen/logrus"
	"google.golang.org/protobuf/reflect/protoreflect"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/queryparser"
)

const (
	ADVISORY_LOCKS = "Advisory Locks"
	SYSTEM_COLUMNS = "System Columns"
	XML_FUNCTIONS  = "XML Functions"
)

// To Add a new unsupported query construct implement this interface
type UnsupportedConstructDetector interface {
	Detect(msg protoreflect.Message) ([]string, error)
}

type FuncCallDetector struct {
	// right now it covers Advisory Locks and XML functions
	unsupportedFuncs map[string]string
}

func NewFuncCallDetector() *FuncCallDetector {
	unsupportedFuncs := make(map[string]string)
	for _, fname := range unsupportedAdvLockFuncs {
		unsupportedFuncs[fname] = ADVISORY_LOCKS
	}
	for _, fname := range unsupportedXmlFunctions {
		unsupportedFuncs[fname] = XML_FUNCTIONS
	}

	return &FuncCallDetector{
		unsupportedFuncs: unsupportedFuncs,
	}
}

// Detect checks if a FuncCall node uses an unsupported function.
func (d *FuncCallDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_FUNCCALL_NODE {
		return nil, nil
	}

	funcName := queryparser.GetFuncNameFromFuncCall(msg)
	log.Debugf("fetched function name from %s node: %q", queryparser.PG_QUERY_FUNCCALL_NODE, funcName)
	if constructType, isUnsupported := d.unsupportedFuncs[funcName]; isUnsupported {
		log.Debugf("detected unsupported function %q in msg - %+v", funcName, msg)
		return []string{constructType}, nil
	}
	return nil, nil
}

type ColumnRefDetector struct {
	unsupportedColumns map[string]string
}

func NewColumnRefDetector() *ColumnRefDetector {
	unsupportedColumns := make(map[string]string)
	for _, colName := range unsupportedSysCols {
		unsupportedColumns[colName] = SYSTEM_COLUMNS
	}

	return &ColumnRefDetector{
		unsupportedColumns: unsupportedColumns,
	}
}

// Detect checks if a ColumnRef node uses an unsupported system column
func (d *ColumnRefDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if queryparser.GetMsgFullName(msg) != queryparser.PG_QUERY_COLUMNREF_NODE {
		return nil, nil
	}

	colName := queryparser.GetColNameFromColumnRef(msg)
	log.Debugf("fetched column name from %s node: %q", queryparser.PG_QUERY_COLUMNREF_NODE, colName)
	if constructType, isUnsupported := d.unsupportedColumns[colName]; isUnsupported {
		log.Debugf("detected unsupported system column %q in msg - %+v", colName, msg)
		return []string{constructType}, nil
	}
	return nil, nil
}

type XmlExprDetector struct{}

func NewXmlExprDetector() *XmlExprDetector {
	return &XmlExprDetector{}
}

// Detect checks if a XmlExpr node is present, means Xml type/functions are used
func (d *XmlExprDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_XMLEXPR_NODE {
		log.Debug("detected xml expression")
		return []string{XML_FUNCTIONS}, nil
	}
	return nil, nil
}

/*
RangeTableFunc node manages functions that produce tables, structuring output into rows and columns
for SQL queries. Example: XMLTABLE()
*/
type RangeTableFuncDetector struct{}

func NewRangeTableFuncDetector() *RangeTableFuncDetector {
	return &RangeTableFuncDetector{}
}

// Detect checks if a RangeTableFunc node is present for a XMLTABLE() function
func (d *RangeTableFuncDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_RANGETABLEFUNC_NODE {
		if isXMLTable(msg) {
			return []string{XML_FUNCTIONS}, nil
		}
	}
	return nil, nil
}

/*
Note: XMLTABLE() is not a simple function(stored in FuncCall node), its a table function
which generates set of rows using the info(about rows, columns, content) provided to it
Hence its requires a more complex node structure(RangeTableFunc node) to represent.

XMLTABLE transforms XML data into relational table format, making it easier to query XML structures.
Detection in RangeTableFunc Node:
- docexpr: Refers to the XML data source, usually a column storing XML.
- rowexpr: XPath expression (starting with '/' or '//') defining the rows in the XML.
- columns: Specifies the data extraction from XML into relational columns.

Example: Converting XML data about books into a table:
SQL Query:

	SELECT x.*
	FROM XMLTABLE(
		'/bookstore/book'
		PASSING xml_column
		COLUMNS
			title TEXT PATH 'title',
			author TEXT PATH 'author'
	) AS x;

Parsetree: stmt:{select_stmt:{target_list:{res_target:{val:{column_ref:{fields:{string:{sval:"x"}}  fields:{a_star:{}}  location:7}}  location:7}}
from_clause:{range_table_func:
	{docexpr:{column_ref:{fields:{string:{sval:"xml_column"}}  location:57}}
	rowexpr:{a_const:{sval:{sval:"/bookstore/book"}  location:29}}
	columns:{range_table_func_col:{colname:"title"  type_name:{names:{string:{sval:"text"}}  typemod:-1  location:87}  colexpr:{a_const:{sval:{sval:"title"}  location:97}}  location:81}}
	columns:{range_table_func_col:{colname:"author"  type_name:{names:{string:{sval:"text"}}  typemod:-1  location:116}  colexpr:{a_const:{sval:{sval:"author"}  location:126}}  location:109}}
alias:{aliasname:"x"}  location:17}}  limit_option:LIMIT_OPTION_DEFAULT  op:SETOP_NONE}}  stmt_len:142

Here, 'docexpr' points to 'xml_column' containing XML data, 'rowexpr' selects each 'book' node, and 'columns' extract 'title' and 'author' from each book.
Hence Presence of XPath in 'rowexpr' and structured 'columns' typically indicates XMLTABLE usage.
*/
// Function to detect if a RangeTableFunc node represents XMLTABLE()
func isXMLTable(rangeTableFunc protoreflect.Message) bool {
	log.Infof("checking if range table func node is for XMLTABLE()")
	// Check for 'docexpr' field
	docexprField := rangeTableFunc.Descriptor().Fields().ByName("docexpr")
	if docexprField == nil {
		return false
	}
	docexprNode := rangeTableFunc.Get(docexprField).Message()
	if docexprNode == nil {
		return false
	}

	// Check for 'rowexpr' field
	rowexprField := rangeTableFunc.Descriptor().Fields().ByName("rowexpr")
	if rowexprField == nil {
		return false
	}
	rowexprNode := rangeTableFunc.Get(rowexprField).Message()
	if rowexprNode == nil {
		return false
	}

	xpath := queryparser.GetStringValueFromNode(rowexprNode)
	log.Debugf("xpath expression in the node: %s\n", xpath)
	// Keep both cases check(param placeholder and absolute check)
	if xpath == "" {
		// Attempt to check if 'rowexpr' is a parameter placeholder like '$1'
		isPlaceholder := queryparser.IsParameterPlaceholder(rowexprNode)
		if !isPlaceholder {
			return false
		}
	} else if !queryparser.IsXPathExprForXmlTable(xpath) {
		return false
	}

	// Check for 'columns' field
	columnsField := rangeTableFunc.Descriptor().Fields().ByName("columns")
	if columnsField == nil {
		return false
	}

	columnsList := rangeTableFunc.Get(columnsField).List()
	if columnsList.Len() == 0 {
		return false
	}

	// this means all the required fields of RangeTableFunc node for being a XMLTABLE() are present
	return true
}
