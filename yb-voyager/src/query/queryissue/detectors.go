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
	regexFuncsDetected         mapset.Set[string]
	loFuncsDetected            mapset.Set[string]
}

func NewFuncCallDetector(query string) *FuncCallDetector {
	return &FuncCallDetector{
		query:                      query,
		advisoryLocksFuncsDetected: mapset.NewThreadUnsafeSet[string](),
		xmlFuncsDetected:           mapset.NewThreadUnsafeSet[string](),
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
	if d.regexFuncsDetected.Cardinality() > 0 {
		issues = append(issues, NewRegexFunctionsIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
	}
	if d.loFuncsDetected.Cardinality() > 0 {
		issues = append(issues, NewLOFuntionsIssue(DML_QUERY_OBJECT_TYPE, "", d.query))
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
