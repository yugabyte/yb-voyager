//go:build unit

package cmd

import "testing"

func TestResolveMetricsPort(t *testing.T) {
	if p, on := resolveMetricsPort("target_db_importer", 0, false); on || p != 0 {
		t.Fatalf("port 0 with no legacy fallback must disable metrics; got (%d,%t)", p, on)
	}
	if p, on := resolveMetricsPort("target_db_importer", 9200, false); !on || p != 9200 {
		t.Fatalf("explicit port must enable; got (%d,%t)", p, on)
	}
	if p, on := resolveMetricsPort("target_db_importer", 9200, true); !on || p != 9200 {
		t.Fatalf("explicit port must win over legacy fallback; got (%d,%t)", p, on)
	}
	if p, on := resolveMetricsPort("target_db_importer", 0, true); !on || p != 9101 {
		t.Fatalf("legacy --profile fallback must use role default port; got (%d,%t)", p, on)
	}
	if p, on := resolveMetricsPort("import_file", 0, true); !on || p != 9102 {
		t.Fatalf("legacy --profile fallback must use role default port; got (%d,%t)", p, on)
	}
	if p, on := resolveMetricsPort("source_db_exporter", 0, true); on || p != 0 {
		t.Fatalf("legacy --profile fallback has no default for export roles; got (%d,%t)", p, on)
	}
}
