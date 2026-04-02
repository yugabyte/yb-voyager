//go:build unit

/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless assertd by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package queryparser

import (
	"slices"
	"testing"

	"github.com/google/go-cmp/cmp"
	pg_query "github.com/pganalyze/pg_query_go/v6"
	"github.com/stretchr/testify/assert"
)

func TestParsingIndexWhereClausePredicates(t *testing.T) {
	sqls := []string{
		`CREATE INDEX idx_simple ON public.test (status) WHERE status <> 'active';`,
		`CREATE INDEX idx_null ON public.test (status) WHERE status IS NOT NULL;`,
		`CREATE INDEX idx_combined ON public.test (status)
		WHERE status <> 'active'::text OR (status = 'inactive' AND status IS NULL);`,
		`CREATE INDEX idx_simple1 ON public.test (id) WHERE id != 11 AND employed <> 't'`,
		`CREATE INDEX idx_simple1 ON public.test (id) WHERE employed`,
		`CREATE INDEX access_log_client_ip_ix ON access_log (client_ip)
			WHERE NOT (client_ip > inet '192.168.100.0' AND
           client_ip < inet '192.168.100.255');`,
		`CREATE INDEX orders_unbilled_index ON orders (order_nr)
    WHERE billed is not true;`,
		`CREATE INDEX order_status on orders (status) WHERE status IN ('PROGRESS', 'DONE');`,
		`CREATE INDEX order_status on orders (status) WHERE status NOT IN ('PROGRESS');`,
		`CREATE INDEX idx_name1 ON public.users USING btree (name) WHERE ((name)::text <> ''::text);`,
		`CREATE INDEX idx_name1 ON public.users USING btree (name) WHERE name = '{afsd}'::test_vale;`,
		`CREATE INDEX idx_name_exclude ON users(name) WHERE name NOT LIKE 'Test%';`,
		`CREATE INDEX indx341 ON public.test_most_freq USING btree (id) WHERE (status = ANY (ARRAY['PROAO'::text, 'dfsad'::text]));`,
		`CREATE INDEX indx341 ON public.test_most_freq USING btree (id);`,
	}
	expectedWhereClausePredicates := map[string][]WhereClausePredicate{
		sqls[0]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "status",
				Value:    "active",
				Operator: "<>",
			},
		},
		sqls[1]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:      "status",
				ColIsNotNULL: true,
				ColIsNULL:    false,
			},
		},
		sqls[2]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "status",
				Value:    "active",
				Operator: "<>",
			},
			WhereClausePredicate{
				ColName:  "status",
				Value:    "inactive",
				Operator: "=",
			},
			WhereClausePredicate{
				ColName:      "status",
				ColIsNULL:    true,
				ColIsNotNULL: false,
			},
		},
		sqls[3]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "id",
				Value:    "11",
				Operator: "<>",
			},
			WhereClausePredicate{
				ColName:  "employed",
				Value:    "t",
				Operator: "<>",
			},
		},
		sqls[4]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName: "employed",
			},
		},
		sqls[5]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "client_ip",
				Value:    "192.168.100.0",
				Operator: ">",
			},
			WhereClausePredicate{
				ColName:  "client_ip",
				Value:    "192.168.100.255",
				Operator: "<",
			},
		},
		sqls[6]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName: "billed",
				Value:   "f",
			},
		},
		sqls[7]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "status",
				Value:    "PROGRESS",
				Operator: "=",
			},
			WhereClausePredicate{
				ColName:  "status",
				Value:    "DONE",
				Operator: "=",
			},
		},
		sqls[8]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "status",
				Value:    "PROGRESS",
				Operator: "<>",
			},
		},
		sqls[9]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "name",
				Value:    "",
				Operator: "<>",
			},
		},
		sqls[10]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "name",
				Value:    "{afsd}",
				Operator: "=",
			},
		},
		sqls[11]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "name",
				Value:    "Test%",
				Operator: "!~~",
			},
		},
		sqls[12]: []WhereClausePredicate{
			WhereClausePredicate{
				ColName:  "status",
				Value:    "PROAO",
				Operator: "=",
			},
			WhereClausePredicate{
				ColName:  "status",
				Value:    "dfsad",
				Operator: "=",
			},
		},
		sqls[13]: []WhereClausePredicate{},
	}
	for sql, expClauses := range expectedWhereClausePredicates {
		tree, err := pg_query.Parse(sql)
		assert.NoError(t, err)

		ip := NewIndexProcessor()
		indexNode, ok := GetCreateIndexStmtNode(tree)
		assert.True(t, ok)

		clauses := ip.parseWhereClause(indexNode.IndexStmt.WhereClause)

		assert.Equal(t, len(expClauses), len(clauses))
		for _, expClause := range expClauses {
			found := slices.ContainsFunc(clauses, func(clause WhereClausePredicate) bool {
				return cmp.Equal(expClause, clause)
			})
			assert.True(t, found, "Expected clause not found: %v in statement: %s and actual clauses: %s", expClause, sql, clauses)
		}
	}
}
