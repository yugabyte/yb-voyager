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

package cdcbench

import (
	"embed"
	"fmt"
)

//go:embed testdata
var testdataFS embed.FS

func mustRead(path string) string {
	raw, err := testdataFS.ReadFile("testdata/" + path)
	if err != nil {
		panic(fmt.Sprintf("cdcbench: reading embedded workload sql: %v", err))
	}
	return string(raw)
}

// The founding workload set. Together they span the conflict-detection
// behavior space:
//   - inserts-no-uk:             control; conflict machinery never engages
//   - updates-uk-no-conflict:    every event scans the cache, none conflict
//   - mixed-uk-no-conflict:      realistic op mix; only u/d events are cached
//   - updates-uk-conflict-pairs: real unique-key conflicts with waits/flushes
func init() {
	Register(Workload{
		Name:            "inserts-no-uk",
		SchemaSQL:       mustRead("inserts_no_uk/schema.sql"),
		SeedSQL:         mustRead("inserts_no_uk/seed.sql"),
		DMLSQL:          mustRead("inserts_no_uk/dml.sql"),
		TableList:       []string{"no_uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})
	Register(Workload{
		Name:            "updates-uk-no-conflict",
		SchemaSQL:       mustRead("updates_uk/schema.sql"),
		SeedSQL:         mustRead("updates_uk/seed.sql"),
		DMLSQL:          mustRead("updates_uk/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})
	Register(Workload{
		Name:            "mixed-uk-no-conflict",
		SchemaSQL:       mustRead("mixed_uk/schema.sql"),
		SeedSQL:         mustRead("mixed_uk/seed.sql"),
		DMLSQL:          mustRead("mixed_uk/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: false,
	})
	Register(Workload{
		Name:            "updates-uk-conflict-pairs",
		SchemaSQL:       mustRead("conflict_pairs_uk/schema.sql"),
		SeedSQL:         mustRead("conflict_pairs_uk/seed.sql"),
		DMLSQL:          mustRead("conflict_pairs_uk/dml.sql"),
		TableList:       []string{"uk_table"},
		ExpectedEvents:  20_000,
		ExpectConflicts: true,
	})
}
