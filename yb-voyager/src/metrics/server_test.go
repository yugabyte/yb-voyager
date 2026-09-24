//go:build unit

package metrics

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServerServesMetrics(t *testing.T) {
	r := NewPrometheusRecorder("uuid-1", "sess-1")
	r.RecordImportSnapshotBatchIngested("target_db_importer", newTupleForTest("public", "orders"), 5, 50)

	srv := NewServer(0, r.Registry()) // port 0 -> OS assigns a free port
	if err := srv.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = srv.Shutdown(ctx)
	}()

	// Give the listener a moment to bind.
	waitForAddr(t, srv)

	resp, err := http.Get("http://" + srv.Addr() + "/metrics")
	if err != nil {
		t.Fatalf("GET /metrics: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if !strings.Contains(string(body), "yb_voyager_import_data_snapshot_rows_total") {
		t.Fatalf("metrics body missing expected metric:\n%s", body)
	}
}

func waitForAddr(t *testing.T, srv *Server) {
	t.Helper()
	for i := 0; i < 50; i++ {
		if srv.Addr() != "" {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("server did not bind an address in time")
}
