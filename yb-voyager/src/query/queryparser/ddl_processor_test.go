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

func TestIndexParsingWhereClauses(t *testing.T) {
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
	}
	expectedWhereClauses := map[string][]WhereClause{
		sqls[0]: []WhereClause{
			WhereClause{
				ColName:  "status",
				Value:    "active",
				Operator: "<>",
			},
		},
		sqls[1]: []WhereClause{
			WhereClause{
				ColName:      "status",
				ColIsNotNULL: true,
				ColIsNULL:    false,
			},
		},
		sqls[2]: []WhereClause{
			WhereClause{
				ColName:  "status",
				Value:    "active",
				Operator: "<>",
			},
			WhereClause{
				ColName:  "status",
				Value:    "inactive",
				Operator: "=",
			},
			WhereClause{
				ColName:      "status",
				ColIsNULL:    true,
				ColIsNotNULL: false,
			},
		},
		sqls[3]: []WhereClause{
			WhereClause{
				ColName:  "id",
				Value:    "11",
				Operator: "<>",
			},
			WhereClause{
				ColName:  "employed",
				Value:    "t",
				Operator: "<>",
			},
		},
		sqls[4]: []WhereClause{
			WhereClause{
				ColName: "employed",
			},
		},
		sqls[5]: []WhereClause{
			WhereClause{
				ColName:  "client_ip",
				Value:    "192.168.100.0",
				Operator: ">",
			},
			WhereClause{
				ColName:  "client_ip",
				Value:    "192.168.100.255",
				Operator: "<",
			},
		},
		sqls[6]: []WhereClause{
			WhereClause{
				ColName: "billed",
				Value:   "f",
			},
		},
		sqls[7]: []WhereClause{
			WhereClause{
				ColName:  "status",
				Value:    "PROGRESS",
				Operator: "=",
			},
			WhereClause{
				ColName:  "status",
				Value:    "DONE",
				Operator: "=",
			},
		},
		sqls[8]: []WhereClause{
			WhereClause{
				ColName:  "status",
				Value:    "PROGRESS",
				Operator: "<>",
			},
		},
		sqls[9]: []WhereClause{
			WhereClause{
				ColName:  "name",
				Value:    "",
				Operator: "<>",
			},
		},
		sqls[10]: []WhereClause{
			WhereClause{
				ColName: "name",
				Value: "{afsd}",
				Operator: "=",
			},
		},
		sqls[11]: []WhereClause{
			WhereClause{
				ColName: "name",
				Value: "Test%",
				Operator: "!~~",
			},
		},
		sqls[12]: []WhereClause{
			WhereClause{
				ColName: "status",
				Value: "PROAO",
				Operator: "=",
			},
			WhereClause{
				ColName: "status",
				Value: "dfsad",
				Operator: "=",
			},
		},
	}
	for sql, expClauses := range expectedWhereClauses {
		tree, err := pg_query.Parse(sql)
		assert.NoError(t, err)

		ip := NewIndexProcessor()
		indexNode, ok := getCreateIndexStmtNode(tree)
		assert.True(t, ok)

		clauses := make([]WhereClause, 0)
		ip.parseWhereClause(indexNode.IndexStmt.WhereClause, &clauses)

		assert.Equal(t, len(expClauses), len(clauses))
		for _, expClause := range expClauses {
			found := slices.ContainsFunc(clauses, func(clause WhereClause) bool {
				return cmp.Equal(expClause, clause)
			})
			assert.True(t, found, "Expected clause not found: %v in statement: %s and actual clauses: %s", expClause, sql, clauses)
		}
	}
}
