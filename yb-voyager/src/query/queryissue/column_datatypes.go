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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type TypeKind int

const (
	// a type not defined in the exported schema: a built-in or an extension's type
	TypeKindBuiltIn TypeKind = iota
	TypeKindDomain
	TypeKindComposite
	TypeKindEnum
	// a user-defined range type, or the multirange type created with it
	TypeKindRange
)

// UnderlyingType is one of the types a column's type is built from.
type UnderlyingType struct {
	// qualified as written in the exported schema; a type not defined there keeps the name it
	// was looked up by
	Name string
	Kind TypeKind
}

// BaseName is the type's name without its schema, as the unsupported datatype lists spell it.
func (t UnderlyingType) BaseName() string {
	return utils.GetBaseTypeName(t.Name)
}

// GetUnderlyingTypes lists typeName and every type it is built from, typeName first: a domain
// is followed to its base type, an array to its element type, and a composite to the types of
// its fields, through the domains and types defined in the exported schema.
//
// typeName may carry a "[]" suffix. It is looked up exactly as written, never qualified with a
// schema: pg_dump qualifies every user type, and an unqualified name is a built-in, since PG
// searches pg_catalog first.
func (p *ParserIssueDetector) GetUnderlyingTypes(typeName string) []UnderlyingType {
	var types []UnderlyingType
	visited := make(map[string]bool)
	var walk func(name string)
	walk = func(name string) {
		name = strings.TrimSuffix(name, "[]")
		if domain, ok := p.domains[name]; ok {
			if visited[domain.GetObjectName()] {
				return
			}
			visited[domain.GetObjectName()] = true
			types = append(types, UnderlyingType{Name: domain.GetObjectName(), Kind: TypeKindDomain})
			walk(domain.GetFullBaseTypeName())
			return
		}
		if userDefinedType, ok := p.userDefinedTypes[name]; ok {
			if visited[userDefinedType.GetObjectName()] {
				return
			}
			visited[userDefinedType.GetObjectName()] = true
			if userDefinedType.IsEnum {
				types = append(types, UnderlyingType{Name: userDefinedType.GetObjectName(), Kind: TypeKindEnum})
				return
			}
			if userDefinedType.IsRange {
				types = append(types, UnderlyingType{Name: userDefinedType.GetObjectName(), Kind: TypeKindRange})
				return
			}
			types = append(types, UnderlyingType{Name: userDefinedType.GetObjectName(), Kind: TypeKindComposite})
			for _, field := range userDefinedType.CompositeFields {
				walk(field.GetFullTypeName())
			}
			return
		}
		if visited[name] {
			return
		}
		visited[name] = true
		types = append(types, UnderlyingType{Name: name, Kind: TypeKindBuiltIn})
	}
	walk(typeName)
	return types
}

// getColumnsWithTypes returns table's columns with their types. A typed table takes its columns
// from the fields of its composite type. If that type is not in the exported schema, only the
// column options from the DDL are returned.
func (p *ParserIssueDetector) getColumnsWithTypes(table *queryparser.Table) []queryparser.TableColumn {
	if table.OfTypeName == "" {
		return table.Columns
	}
	compositeType, ok := p.userDefinedTypes[table.OfTypeName]
	if !ok {
		return table.Columns
	}
	return compositeType.CompositeFields
}

// ColumnDatatypeSupport holds, for each migration flow, every distinct type that makes a column
// unsupported in that flow, or none when the flow supports every type the column is built from.
type ColumnDatatypeSupport struct {
	UnsupportedInOffline        []UnderlyingType
	UnsupportedInLive           []UnderlyingType
	UnsupportedInLiveWithFFOrFB []UnderlyingType
}

// GetColumnDatatypeSupport checks every type a PostgreSQL column of type typeName is built
// from against the unsupported datatypes of each migration flow.
func (p *ParserIssueDetector) GetColumnDatatypeSupport(typeName string) ColumnDatatypeSupport {
	types := p.GetUnderlyingTypes(typeName)
	liveWithFFOrFBUnsupportedDatatypes := srcdb.GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes()
	// a composite nested in another is part of the same user-defined value, so only the outermost is reported
	return ColumnDatatypeSupport{
		UnsupportedInOffline: filterUnderlyingTypes(types, func(t UnderlyingType) bool {
			return isListedDatatype(t, srcdb.PostgresUnsupportedDataTypes)
		}),
		UnsupportedInLive: filterUnderlyingTypes(types, func(t UnderlyingType) bool {
			return t.Kind == TypeKindRange || isListedDatatype(t, srcdb.GetPGLiveMigrationUnsupportedDatatypes())
		}),
		UnsupportedInLiveWithFFOrFB: filterUnderlyingTypes(types, func(t UnderlyingType) bool {
			return isListedDatatype(t, liveWithFFOrFBUnsupportedDatatypes)
		}),
	}
}

// isListedDatatype matches only types not defined in the exported schema, since the lists name
// built-in and extension types and a user-defined type such as public.point merely shares a name.
func isListedDatatype(t UnderlyingType, datatypes []string) bool {
	return t.Kind == TypeKindBuiltIn && utils.ContainsAnyStringFromSlice(datatypes, t.BaseName())
}

// filterUnderlyingTypes keeps one entry per base name for types not defined in the exported
// schema, since the walk keeps xml and pg_catalog.xml apart but both raise the same issue.
func filterUnderlyingTypes(types []UnderlyingType, matches func(UnderlyingType) bool) []UnderlyingType {
	var matched []UnderlyingType
	seen := make(map[string]bool)
	for _, t := range types {
		key := t.Name
		if t.Kind == TypeKindBuiltIn {
			key = t.BaseName()
		}
		if !matches(t) || seen[key] {
			continue
		}
		seen[key] = true
		matched = append(matched, t)
	}
	return matched
}
