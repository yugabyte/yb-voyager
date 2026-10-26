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

	"github.com/yugabyte/yb-voyager/yb-voyager/src/srcdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type TypeKind int

const (
	// a type not defined in the exported schema: a built-in or an extension's type
	TypeKindOther TypeKind = iota
	TypeKindDomain
	TypeKindComposite
	TypeKindEnum
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
// typeName may be qualified or not and may carry a "[]" suffix. An unqualified name is looked
// up as written, then in defaultSchema.
func (p *ParserIssueDetector) GetUnderlyingTypes(typeName string, defaultSchema string) []UnderlyingType {
	var types []UnderlyingType
	visited := make(map[string]bool)
	var walk func(name string, defaultSchema string)
	walk = func(name string, defaultSchema string) {
		name = strings.TrimSuffix(name, "[]")
		if domain, ok := lookupSchemaObject(p.domains, name, defaultSchema); ok {
			if visited[domain.GetObjectName()] {
				return
			}
			visited[domain.GetObjectName()] = true
			types = append(types, UnderlyingType{Name: domain.GetObjectName(), Kind: TypeKindDomain})
			walk(domain.GetFullBaseTypeName(), domain.SchemaName)
			return
		}
		if userDefinedType, ok := lookupSchemaObject(p.userDefinedTypes, name, defaultSchema); ok {
			if visited[userDefinedType.GetObjectName()] {
				return
			}
			visited[userDefinedType.GetObjectName()] = true
			if userDefinedType.IsEnum {
				types = append(types, UnderlyingType{Name: userDefinedType.GetObjectName(), Kind: TypeKindEnum})
				return
			}
			types = append(types, UnderlyingType{Name: userDefinedType.GetObjectName(), Kind: TypeKindComposite})
			for _, field := range userDefinedType.CompositeFields {
				walk(field.GetFullTypeName(), userDefinedType.SchemaName)
			}
			return
		}
		if visited[name] {
			return
		}
		visited[name] = true
		types = append(types, UnderlyingType{Name: name, Kind: TypeKindOther})
	}
	walk(typeName, defaultSchema)
	return types
}

func lookupSchemaObject[T any](objects map[string]T, name string, defaultSchema string) (T, bool) {
	if object, ok := objects[name]; ok {
		return object, true
	}
	var none T
	if strings.Contains(name, ".") {
		return none, false
	}
	if defaultSchema == "" {
		return none, false
	}
	object, ok := objects[utils.BuildObjectName(defaultSchema, name)]
	return object, ok
}

// ColumnDatatypeSupport holds, for each migration flow, the type that makes a column
// unsupported in that flow, or nil when the flow supports every type the column is built from.
type ColumnDatatypeSupport struct {
	UnsupportedInOffline        *UnderlyingType
	UnsupportedInLive           *UnderlyingType
	UnsupportedInLiveWithFFOrFB *UnderlyingType
}

// GetColumnDatatypeSupport checks every type a PostgreSQL column of type typeName is built
// from against the unsupported datatypes of each migration flow.
func (p *ParserIssueDetector) GetColumnDatatypeSupport(typeName string, defaultSchema string) ColumnDatatypeSupport {
	types := p.GetUnderlyingTypes(typeName, defaultSchema)
	liveWithFFOrFBUnsupportedDatatypes := srcdb.GetPGLiveMigrationWithFFOrFBUnsupportedDatatypes()
	return ColumnDatatypeSupport{
		UnsupportedInOffline: findUnderlyingType(types, func(t UnderlyingType) bool {
			return isListedDatatype(t, srcdb.PostgresUnsupportedDataTypes)
		}),
		UnsupportedInLive: findUnderlyingType(types, func(t UnderlyingType) bool {
			return isListedDatatype(t, srcdb.GetPGLiveMigrationUnsupportedDatatypes())
		}),
		UnsupportedInLiveWithFFOrFB: findUnderlyingType(types, func(t UnderlyingType) bool {
			return t.Kind == TypeKindComposite || isListedDatatype(t, liveWithFFOrFBUnsupportedDatatypes)
		}),
	}
}

// isListedDatatype matches only types not defined in the exported schema, since the lists name
// built-in and extension types and a user-defined type such as public.point merely shares a name.
func isListedDatatype(t UnderlyingType, datatypes []string) bool {
	return t.Kind == TypeKindOther && utils.ContainsAnyStringFromSlice(datatypes, t.BaseName())
}

func findUnderlyingType(types []UnderlyingType, matches func(UnderlyingType) bool) *UnderlyingType {
	for i := range types {
		if matches(types[i]) {
			return &types[i]
		}
	}
	return nil
}
