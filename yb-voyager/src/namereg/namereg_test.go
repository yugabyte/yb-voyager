package namereg

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestNameRegistry(t *testing.T) {
	assert := assert.New(t)

	reg := &NameRegistry{
		SourceDBType:              POSTGRESQL,
		SourceDBSchemaNames:       []string{"public", "schema1", "schema2"},
		TargetDBSchemaNames:       []string{"public", "schema1", "schema2"},
		DefaultSourceDBSchemaName: "public",
		DefaultTargetDBSchemaName: "public",
		SourceDBTableNames: map[string][]string{
			"public":  {"table1", "table2"},
			"schema1": {"table1", "table2"},
			"schema2": {"table1", "table2"},
		},
		TargetDBTableNames: map[string][]string{
			"public":  {"table1", "table2"},
			"schema1": {"table1", "table2"},
			"schema2": {"table1", "table2"},
		},
	}

	tableName, err := reg.LookupTableName("table1")
	assert.Nil(err)
	assert.NotNil(tableName)
	expectedTableName := &NameTuple{
		SourceTableName: &TableName{
			SchemaName:        "public",
			FromDefaultSchema: true,
			Qualified: identifier{
				Quoted:    `public."table1"`,
				Unquoted:  "public.table1",
				MinQuoted: "public.table1",
			},
			Unqualified: identifier{
				Quoted:    `"table1"`,
				Unquoted:  "table1",
				MinQuoted: "table1",
			},
			MinQualified: identifier{
				Quoted:    `"table1"`,
				Unquoted:  "table1",
				MinQuoted: "table1",
			},
		},
	}
	expectedTableName.TargetTableName = expectedTableName.SourceTableName // Source and Target are the same for PG source.
	assert.Equal(expectedTableName, tableName)
}
