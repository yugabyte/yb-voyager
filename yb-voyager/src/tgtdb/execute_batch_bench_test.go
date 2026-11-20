//go:build cdc_benchmark

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
package tgtdb

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/stretchr/testify/require"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	testcontainers "github.com/yugabyte/yb-voyager/yb-voyager/test/containers"
)

const execBatchBenchRows = 2000

// BenchmarkExecuteBatch drives the real CDC apply path against a YugabyteDB container.
// UPDATE events are queued as literal SQL, so every one is a distinct statement for pgx's
// statement cache (capacity 512); the 2000-event case exceeds it. INSERTs use voyager's own
// named prepared statement and serve as a control.
//
//	go test -tags cdc_benchmark -run '^$' -bench ExecuteBatch -benchtime 20x -count 5 ./src/tgtdb/
func BenchmarkExecuteBatch(b *testing.B) {
	log.SetLevel(log.WarnLevel)
	ctx := context.Background()
	container := testcontainers.NewTestContainer(testcontainers.YUGABYTEDB, nil)
	require.NoError(b, container.Start(ctx))
	b.Cleanup(func() { container.Terminate(ctx) })

	host, port, err := container.GetHostPort()
	require.NoError(b, err)
	cfg := container.GetConfig()
	yb := newTargetYugabyteDB(&TargetConf{
		TargetDBType: YUGABYTEDB,
		DBVersion:    cfg.DBVersion,
		User:         cfg.User,
		Password:     cfg.Password,
		Schemas:      []sqlname.Identifier{sqlname.NewIdentifier(YUGABYTEDB, cfg.Schema)},
		DBName:       cfg.DBName,
		Host:         host,
		Port:         port,
		SSLMode:      "disable",
	})
	require.NoError(b, yb.Init())
	b.Cleanup(yb.Finalize)
	require.NoError(b, yb.CreateVoyagerSchema())
	// One connection, matching a single import channel; avoids yb_servers() returning container-internal IPs.
	yb.connPool, err = NewConnectionPool(&ConnectionParams{
		NumConnections:    1,
		NumMaxConnections: 1,
		ConnUriList:       []string{yb.Tconf.GetConnectionUri()},
		SessionInitScript: yb.Tconf.SessionVars,
	})
	require.NoError(b, err)

	_, err = yb.Exec(`CREATE TABLE IF NOT EXISTS public.exec_batch_bench (id INT PRIMARY KEY, v TEXT, n INT)`)
	require.NoError(b, err)
	_, err = yb.Exec(fmt.Sprintf(`INSERT INTO public.exec_batch_bench SELECT g, 'init', 0 FROM generate_series(1, %d) g ON CONFLICT DO NOTHING`, execBatchBenchRows))
	require.NoError(b, err)

	tableObj := sqlname.NewObjectName(constants.YUGABYTEDB, "public", "public", "exec_batch_bench")
	tableTup := sqlname.NameTuple{SourceName: tableObj, TargetName: tableObj, CurrentName: tableObj}

	cases := []struct {
		name      string
		op        string
		batchSize int
	}{
		{name: "updates_500", op: "u", batchSize: 500},
		{name: "updates_2000", op: "u", batchSize: 2000},
		{name: "inserts_500", op: "c", batchSize: 500},
	}
	for chanNo, tc := range cases {
		b.Run(tc.name, func(b *testing.B) {
			migrationUUID := uuid.New()
			_, err := yb.Exec(fmt.Sprintf("INSERT INTO %s VALUES ('%s', %d, -1, 0, 0, 0)", EVENT_CHANNELS_METADATA_TABLE_NAME, migrationUUID, chanNo))
			require.NoError(b, err)

			// The testing package invokes this closure several times (probe run, -count) against the same
			// container, so insert ids and update literals must not repeat across invocations.
			var maxID int
			require.NoError(b, yb.QueryRow(`SELECT MAX(id) FROM public.exec_batch_bench`).Scan(&maxID))
			runTag := migrationUUID.String()[:8]

			vsn := int64(0)
			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				batch := buildExecBatchBenchBatch(tableTup, tc.op, tc.batchSize, chanNo, i, maxID, runTag, &vsn)
				b.StartTimer()
				require.NoError(b, yb.ExecuteBatch(migrationUUID, batch))
			}
		})
	}
}

func buildExecBatchBenchBatch(tableTup sqlname.NameTuple, op string, batchSize, chanNo, iteration, maxID int, runTag string, vsn *int64) *EventBatch {
	events := make([]*Event, 0, batchSize)
	for j := 0; j < batchSize; j++ {
		*vsn++
		id := strconv.Itoa(j%execBatchBenchRows + 1)
		event := &Event{Vsn: *vsn, Op: op, TableNameTup: tableTup}
		switch op {
		case "u":
			// A distinct literal per event, as with real UPDATE events, so no two statements share SQL text.
			v := fmt.Sprintf("'%s_it%d_ev%d'", runTag, iteration, j)
			event.Key = map[string]*string{"id": &id}
			event.Fields = map[string]*string{"v": &v}
		case "c":
			insertID := strconv.Itoa(maxID + iteration*batchSize + j + 1)
			v := "ins"
			n := "1"
			event.Key = map[string]*string{"id": &insertID}
			event.Fields = map[string]*string{"id": &insertID, "v": &v, "n": &n}
		}
		events = append(events, event)
	}
	return NewEventBatch(events, chanNo)
}
