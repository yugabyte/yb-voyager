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
package sqlname

import (
	"fmt"
	"path/filepath"
	"reflect"
	"strings"

	goerrors "github.com/go-errors/errors"
	"github.com/samber/lo"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
)

//================================================

// Can be a name of a table, sequence, materialised view, etc.
type ObjectName struct {
	SchemaName        Identifier
	FromDefaultSchema bool

	Qualified    Identifier
	Unqualified  Identifier
	MinQualified Identifier
}

// Assumption is to pass unquoted name for schema and table name with case sensitivity preserved
func NewObjectName(dbType, defaultSchemaName, schemaName, tableName string) *ObjectName {
	if IsQuoted(schemaName) || IsQuoted(tableName) {
		panic(fmt.Sprintf("schema or table name should be unquoted: %s or %s", schemaName, tableName))
	}
	schemaNameIdentifier := Identifier{
		Quoted:    quote2(dbType, schemaName),
		Unquoted:  schemaName,
		MinQuoted: minQuote2(schemaName, dbType),
	}
	result := &ObjectName{
		SchemaName:        schemaNameIdentifier,
		FromDefaultSchema: schemaName == defaultSchemaName,
		Qualified: Identifier{
			Quoted:    schemaNameIdentifier.Quoted + "." + quote2(dbType, tableName),
			Unquoted:  schemaNameIdentifier.Unquoted + "." + tableName,
			MinQuoted: schemaNameIdentifier.MinQuoted + "." + minQuote2(tableName, dbType),
		},
		Unqualified: Identifier{
			Quoted:    quote2(dbType, tableName),
			Unquoted:  tableName,
			MinQuoted: minQuote2(tableName, dbType),
		},
	}
	result.MinQualified = lo.Ternary(result.FromDefaultSchema, result.Unqualified, result.Qualified)
	return result
}

// Assumption - always quoted qualified name with case sensitivity preserved
func NewObjectNameWithQualifiedName(dbType, defaultSchemaName, objName string) *ObjectName {
	parts := strings.Split(objName, ".")
	if len(parts) != 2 {
		panic(fmt.Sprintf("invalid qualified name: %s", objName))
	}
	if !IsQuoted(parts[0]) || !IsQuoted(parts[1]) {
		panic(fmt.Sprintf("schema or table name should be quoted: %s or %s", parts[0], parts[1]))
	}
	return NewObjectName(dbType, defaultSchemaName, unquote(parts[0], dbType), unquote(parts[1], dbType))
}

func (nv *ObjectName) String() string {
	return nv.MinQualified.MinQuoted
}

func (o *ObjectName) Key() string {
	return o.Qualified.Unquoted
}

/*
Assumptions for both schema and table name:
if the pattern is quoted then complete case sensitivity is checked to match the pattern with  table
but if the pattern is not quoted then matching is done without case sensitivity to find the match.
*/
func (nv *ObjectName) MatchesPattern(pattern string) (bool, error) {
	parts := strings.Split(pattern, ".")
	switch true {
	case len(parts) == 2:
		//if the schema name matches completely with the quoted schema name of the object name then no problem
		//if the pattern "Abc" and objectname has Quoted-"Abc" and unquoted-Abc, then this quoted matches with the pattern
		if parts[0] != nv.SchemaName.Quoted {
			//if its not a complete match with quoted
			//then check if the equalfold without case sensitivity matches with the unquoted schema name of the object name
			//if the pattern has  Abc and objectname has Quoted-"abc" and unquoted-abc, so with equalfold it will match
			//if the pattern has "Abc" and objectname has Quoted-"abc" and unquoted-abc, so even with equalfold it will not match as the quotes are present in the pattern
			if !strings.EqualFold(parts[0], nv.SchemaName.Unquoted) {
				return false, nil
			}
		}
		pattern = parts[1]
	case len(parts) == 1:
		if !nv.FromDefaultSchema {
			return false, nil
		}
		pattern = parts[0]
	default:
		return false, goerrors.Errorf("invalid pattern: %s", pattern)
	}
	match1, err := filepath.Match(strings.ToLower(pattern), strings.ToLower(nv.Unqualified.Unquoted))
	if err != nil {
		return false, goerrors.Errorf("invalid pattern: %s", pattern)
	}
	if match1 {
		return true, nil
	}
	match2, err := filepath.Match(pattern, nv.Unqualified.Quoted)
	if err != nil {
		return false, goerrors.Errorf("invalid pattern: %s", pattern)
	}
	return match2, nil
}

// <SourceTableName, TargetTableName>
type NameTuple struct {
	// Mode        string
	CurrentName *ObjectName
	SourceName  *ObjectName
	TargetName  *ObjectName
}

func (t1 NameTuple) Equals(t2 NameTuple) bool {
	return reflect.DeepEqual(t1, t2)
}

func (t NameTuple) String() string {
	var curname, tname, sname string
	if t.CurrentName != nil {
		curname = t.CurrentName.String()
	}
	if t.SourceName != nil {
		sname = t.SourceName.String()
	}
	if t.TargetName != nil {
		tname = t.TargetName.String()
	}
	return fmt.Sprintf("[CurrentName=(%s) SourceName=(%s) TargetName=(%s)]", curname, sname, tname)
}

func (t NameTuple) MatchesPattern(pattern string) (bool, error) {
	for _, tableName := range []*ObjectName{t.SourceName, t.TargetName} {
		if tableName == nil {
			continue
		}
		match, err := tableName.MatchesPattern(pattern)
		if err != nil {
			return false, err
		}
		if match {
			return true, nil
		}
	}
	return false, nil
}

func (t NameTuple) TargetTableAvailable() bool {
	return t.TargetName != nil
}

func (t NameTuple) ForUserQuery() string {
	return t.CurrentName.Qualified.Quoted
}

func (t NameTuple) ForOutput() string {
	return t.CurrentName.Qualified.MinQuoted
}

func (t NameTuple) ForCatalogQuery() (string, string) {
	return t.CurrentName.SchemaName.Unquoted, t.CurrentName.Unqualified.Unquoted
}

func (t NameTuple) AsQualifiedCatalogName() string {
	return t.CurrentName.Qualified.Unquoted
}

func (t NameTuple) ForMinOutput() string {
	return t.CurrentName.MinQualified.MinQuoted
}

func (t NameTuple) ForKey() string {
	// sourcename will be nil only in the case of import-data-file
	if t.SourceName != nil {
		return t.SourceName.Qualified.Quoted
	}
	return t.TargetName.Qualified.Quoted
}

func (t NameTuple) ForKeyTableSchema() (string, string) {
	objName := t.SourceName
	if objName == nil {
		objName = t.TargetName
	}
	return objName.SchemaName.Unquoted, objName.Unqualified.Unquoted
}

func SetDifferenceNameTuples(a, b []NameTuple) []NameTuple {
	m := make(map[string]bool)
	for _, x := range b {
		m[x.String()] = true
	}
	var res []NameTuple
	for _, x := range a {
		if !m[x.String()] {
			res = append(res, x)
		}
	}
	return res
}

func SetDifferenceNameTuplesWithKey(a, b []NameTuple) []NameTuple {
	m := make(map[string]bool)
	for _, x := range b {
		m[x.ForKey()] = true
	}
	var res []NameTuple
	for _, x := range a {
		if !m[x.ForKey()] {
			res = append(res, x)
		}
	}
	return res
}

// Implements: utils.Keyer.Key()
func (t NameTuple) Key() string {
	return t.ForKey()
}

// ================================================
func quote2(dbType, name string) string {
	switch dbType {
	case constants.POSTGRESQL, constants.YUGABYTEDB,
		constants.ORACLE, constants.MYSQL:
		return `"` + name + `"`
	default:
		panic("unknown source db type " + dbType)
	}
}

func minQuote2(objectName, sourceDBType string) string {
	switch sourceDBType {
	case constants.YUGABYTEDB, constants.POSTGRESQL:
		if IsAllLowercase(objectName) && !IsReservedKeywordPG(objectName) {
			return objectName
		} else {
			return `"` + objectName + `"`
		}
	case constants.MYSQL:
		return `"` + objectName + `"`
	case constants.ORACLE:
		if IsAllUppercase(objectName) && !IsReservedKeywordOracle(objectName) {
			return objectName
		} else {
			return `"` + objectName + `"`
		}
	default:
		panic("invalid source db type")
	}
}

func NameTupleListToStrings(nameTuples []NameTuple) []string {
	result := make([]string, len(nameTuples))
	for i, nt := range nameTuples {
		result[i] = nt.ForOutput()
	}
	return result
}
