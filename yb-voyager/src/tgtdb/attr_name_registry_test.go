package tgtdb

import (
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

func TestAttributeNameRegistry_QuoteAttributeName_POSTGRES(t *testing.T) {
	reg := NewAttributeNameRegistry(nil, &TargetConf{TargetDBType: POSTGRESQL})
	o := sqlname.NewObjectName(constants.POSTGRESQL, "public", "public", "test_table")
	tableNameTup := sqlname.NameTuple{SourceName: o, TargetName: o, CurrentName: o}

	// Mocking GetListOfTableAttributes to return a list of attributes
	reg.tdb = &MockTargetDB{
		listOfTableAttributes: []string{"id", "Name", "USER_NAME", "MultipleChoice", "multiplechoice", "multiplechoiceMIXED", "multipleCHOICEmixed"},
	}

	tests := []struct {
		name       string
		columnName string
		wantQuoted string
		wantErr    bool
	}{
		{
			name:       "Lower case",
			columnName: "id",
			wantQuoted: `"id"`,
			wantErr:    false,
		},
		{
			name:       "Upper case",
			columnName: "NAME",
			wantQuoted: `"Name"`,
			wantErr:    false,
		},
		{
			name:       "Lower case",
			columnName: "user_name",
			wantQuoted: `"USER_NAME"`,
			wantErr:    false,
		},
		{
			name:       "Quoted input",
			columnName: `"id"`,
			wantQuoted: `"id"`,
			wantErr:    false,
		},
		{
			name:       "multiple choice pass",
			columnName: "multiplechoice",
			wantQuoted: `"multiplechoice"`, // in postgresql, we would default to all lowercase
			wantErr:    false,
		},
		{
			name:       "multiple choice fail",
			columnName: "multiplechoicemixed",
			wantQuoted: ``, // in postgresql, we would default to lowercase
			wantErr:    true,
		},
		{
			name:       "Non-existent column",
			columnName: "non_existent",
			wantQuoted: ``,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quotedColName, err := reg.QuoteAttributeName(tableNameTup, tt.columnName)
			if (err != nil) != tt.wantErr {
				t.Errorf("QuoteAttributeName() for %v error = %v, wantErr %v, quotedColName = %v", tt, err, tt.wantErr, quotedColName)
				return
			}
			if quotedColName != tt.wantQuoted {
				t.Errorf("QuoteAttributeName() for %v = %v, want %v", tt, quotedColName, tt.wantQuoted)
			}
		})
	}
}
func TestAttributeNameRegistry_QuoteAttributeName_ORACLE(t *testing.T) {
	reg := NewAttributeNameRegistry(nil, &TargetConf{TargetDBType: ORACLE})
	o := sqlname.NewObjectName(constants.ORACLE, "TEST", "TEST", "test_table")
	tableNameTup := sqlname.NameTuple{SourceName: o, TargetName: o, CurrentName: o}

	// Mocking GetListOfTableAttributes to return a list of attributes
	reg.tdb = &MockTargetDB{
		listOfTableAttributes: []string{"id", "Name", "USER_NAME", "MultipleChoice", "MULTIPLECHOICE", "multiplechoiceMIXED", "multipleCHOICEmixed"},
	}
	tests := []struct {
		name       string
		columnName string
		wantQuoted string
		wantErr    bool
	}{
		{
			name:       "Upper case",
			columnName: "NAME",
			wantQuoted: `"Name"`,
			wantErr:    false,
		},
		{
			name:       "Lower case",
			columnName: "user_name",
			wantQuoted: `"USER_NAME"`,
			wantErr:    false,
		},
		{
			name:       "Quoted input",
			columnName: `"id"`,
			wantQuoted: `"id"`,
			wantErr:    false,
		},
		{
			name:       "multiple choice pass",
			columnName: "multiplechoice",
			wantQuoted: `"MULTIPLECHOICE"`, // Oracle defaults to uppercase
			wantErr:    false,
		},
		{
			name:       "multiple choice fail",
			columnName: "multiplechoicemixed",
			wantQuoted: ``, // Oracle defaults to uppercase, mixed case without quotes will fail
			wantErr:    true,
		},
		{
			name:       "Non-existent column",
			columnName: "non_existent",
			wantQuoted: ``,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			quotedColName, err := reg.QuoteAttributeName(tableNameTup, tt.columnName)
			if (err != nil) != tt.wantErr {
				t.Errorf("QuoteAttributeName() for %v error = %v, wantErr %v, quotedColName = %v", tt, err, tt.wantErr, quotedColName)
				return
			}
			if quotedColName != tt.wantQuoted {
				t.Errorf("QuoteAttributeName() for %v = %v, want %v", tt, quotedColName, tt.wantQuoted)
			}
		})
	}
}

type MockTargetDB struct {
	TargetDB
	listOfTableAttributes []string
}

func (m *MockTargetDB) GetListOfTableAttributes(tableNameTup sqlname.NameTuple) ([]string, error) {
	return m.listOfTableAttributes, nil
}
