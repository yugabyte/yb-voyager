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

type identifier struct {
	Quoted, Unquoted, MinQuoted string
}

func NewIdentifier(dbType, name string) identifier {
	if IsQuoted(name) {
		name = unquote(name, dbType)
	}
	return identifier{
		Quoted:    quote2(dbType, name),
		Unquoted:  name,
		MinQuoted: minQuote2(name, dbType),
	}
}

func (i identifier) Equals(other identifier) bool {
	return i.Quoted == other.Quoted && i.Unquoted == other.Unquoted && i.MinQuoted == other.MinQuoted
}

// Can be a name of a table, sequence, materialised view, etc.
type ObjectName struct {
	SchemaName        identifier
	FromDefaultSchema bool

	Qualified    identifier
	Unqualified  identifier
	MinQualified identifier
}

/*
understand command table usages and handling , assumptions  for both schema and table name:
inputs for schema should have same behaviour with  table
schema commands - namereg
*/
func NewObjectName(dbType, defaultSchemaName, schemaName, tableName string) *ObjectName {
	schemaNameIdentifier := identifier{
		Quoted:    quote2(dbType, schemaName),
		Unquoted:  schemaName,
		MinQuoted: minQuote2(schemaName, dbType),
	}
	result := &ObjectName{
		SchemaName:        schemaNameIdentifier,
		FromDefaultSchema: schemaName == defaultSchemaName,
		Qualified: identifier{
			Quoted:    schemaNameIdentifier.Quoted + "." + quote2(dbType, tableName),
			Unquoted:  schemaNameIdentifier.Unquoted + "." + tableName,
			MinQuoted: schemaNameIdentifier.MinQuoted + "." + minQuote2(tableName, dbType),
		},
		Unqualified: identifier{
			Quoted:    quote2(dbType, tableName),
			Unquoted:  tableName,
			MinQuoted: minQuote2(tableName, dbType),
		},
	}
	result.MinQualified = lo.Ternary(result.FromDefaultSchema, result.Unqualified, result.Qualified)
	return result
}

func NewObjectNameWithQualifiedName(dbType, defaultSchemaName, objName string) *ObjectName {
	parts := strings.Split(objName, ".")
	if len(parts) != 2 {
		panic(fmt.Sprintf("invalid qualified name: %s", objName))
	}
	return NewObjectName(dbType, defaultSchemaName, unquote(parts[0], dbType), unquote(parts[1], dbType))
}

func (nv *ObjectName) String() string {
	return nv.MinQualified.MinQuoted
}

func (o *ObjectName) Key() string {
	return o.Qualified.Unquoted
}

func (o *ObjectName) MinQuotedSchemaAndQuotedTableName() string {
	return o.Qualified.Quoted
	// return fmt.Sprintf("%s.%s", o.SchemaName.MinQuoted, o.Unqualified.Quoted)
}

/*

 */

/*
Assumptions for both schema and table name:
if the pattern is quoted then complete case sensitivity is checked to match the pattern with  table
but if the pattern is not quoted then matching is done without case sensitivity to find the match.
*/
func (nv *ObjectName) MatchesPattern(pattern string) (bool, error) {
	parts := strings.Split(pattern, ".")
	switch true {
	case len(parts) == 2:
		if parts[0] != nv.SchemaName.Quoted {
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
	return t.CurrentName.MinQuotedSchemaAndQuotedTableName()
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
		return t.SourceName.MinQuotedSchemaAndQuotedTableName()
	}
	return t.TargetName.MinQuotedSchemaAndQuotedTableName()
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
