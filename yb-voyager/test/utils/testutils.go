package testutils

import (
	"sort"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"gotest.tools/assert"
)

// === assertion helper functions
func AssertEqualStringSlices(t *testing.T, expected, actual []string) {
	t.Helper()
	if len(expected) != len(actual) {
		t.Errorf("Mismatch in slice length. Expected: %v, Actual: %v", expected, actual)
	}

	sort.Strings(expected)
	sort.Strings(actual)
	assert.DeepEqual(t, expected, actual)
}

func AssertEqualSourceNameSlices(t *testing.T, expected, actual []*sqlname.SourceName) {
	SortSourceNames(expected)
	SortSourceNames(actual)
	assert.DeepEqual(t, expected, actual)
}

func SortSourceNames(tables []*sqlname.SourceName) {
	sort.Slice(tables, func(i, j int) bool {
		return tables[i].Qualified.MinQuoted < tables[j].Qualified.MinQuoted
	})
}
