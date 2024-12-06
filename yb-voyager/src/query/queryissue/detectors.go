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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
)

const (
	ADVISORY_LOCKS_NAME = "Advisory Locks"
	SYSTEM_COLUMNS_NAME = "System Columns"
	XML_FUNCTIONS_NAME  = "XML Functions"
)

// To Add a new unsupported query construct implement this interface for all possible nodes for that construct
// each detector will work on specific type of node
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
		unsupportedFuncs[fname] = ADVISORY_LOCKS_NAME
	}
	for _, fname := range unsupportedXmlFunctions {
		unsupportedFuncs[fname] = XML_FUNCTIONS_NAME
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

	_, funcName := queryparser.GetFuncNameFromFuncCall(msg)
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
		unsupportedColumns[colName] = SYSTEM_COLUMNS_NAME
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

	_, colName := queryparser.GetColNameFromColumnRef(msg)
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
		return []string{XML_FUNCTIONS_NAME}, nil
	}
	return nil, nil
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
type RangeTableFuncDetector struct{}

func NewRangeTableFuncDetector() *RangeTableFuncDetector {
	return &RangeTableFuncDetector{}
}

// Detect checks if a RangeTableFunc node is present for a XMLTABLE() function
func (d *RangeTableFuncDetector) Detect(msg protoreflect.Message) ([]string, error) {
	if queryparser.GetMsgFullName(msg) == queryparser.PG_QUERY_RANGETABLEFUNC_NODE {
		if queryparser.IsXMLTable(msg) {
			return []string{XML_FUNCTIONS_NAME}, nil
		}
	}
	return nil, nil
}
