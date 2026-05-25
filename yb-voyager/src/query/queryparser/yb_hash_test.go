//go:build unit

package queryparser

import (
	"testing"

	pg_query "github.com/pganalyze/pg_query_go/v6"
)

func TestYbSortByHashParses(t *testing.T) {
	cases := []struct {
		name string
		sql  string
	}{
		{"create index single col HASH", "CREATE INDEX i ON t (c HASH)"},
		{"create index mixed HASH and ASC", "CREATE INDEX i ON t (c1 HASH, c2 ASC)"},
		// PRIMARY KEY (col HASH) needs a separate columnList -> index_elem
		// rerouting in gram.y; tracked as follow-up.
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			tree, err := Parse(tc.sql)
			if err != nil {
				t.Fatalf("expected parse to succeed, got: %v", err)
			}
			if tree == nil || len(tree.Stmts) == 0 {
				t.Fatalf("expected non-empty parse tree")
			}
		})
	}
}

func TestYbSortByHashDeparses(t *testing.T) {
	sql := "CREATE INDEX i ON t (c1 HASH, c2 ASC)"
	tree, err := Parse(sql)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}

	out, err := pg_query.Deparse(tree)
	if err != nil {
		t.Fatalf("deparse: %v", err)
	}

	if !contains(out, "HASH") {
		t.Errorf("expected deparse output to contain HASH, got: %s", out)
	}
}

func TestYbSortByHashOrderingFieldSet(t *testing.T) {
	tree, err := Parse("CREATE INDEX i ON t (c HASH)")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	idx := tree.Stmts[0].Stmt.GetIndexStmt()
	if idx == nil {
		t.Fatalf("expected IndexStmt")
	}
	if len(idx.IndexParams) != 1 {
		t.Fatalf("expected 1 index param, got %d", len(idx.IndexParams))
	}
	elem := idx.IndexParams[0].GetIndexElem()
	if elem == nil {
		t.Fatalf("expected IndexElem")
	}
	if elem.Ordering != pg_query.SortByDir_SORTBY_HASH {
		t.Errorf("expected SORTBY_HASH ordering, got %v", elem.Ordering)
	}
}

func contains(haystack, needle string) bool {
	for i := 0; i+len(needle) <= len(haystack); i++ {
		if haystack[i:i+len(needle)] == needle {
			return true
		}
	}
	return false
}
