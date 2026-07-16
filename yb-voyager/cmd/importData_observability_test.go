//go:build unit

package cmd

import "testing"

func TestResolveMetricsPort(t *testing.T) {
	tests := []struct {
		name           string
		role           string
		override       int
		legacyProfile  bool
		wantPort       int
		wantEnabled    bool
		failureMessage string
	}{
		{
			name:           "port 0 with no legacy fallback disables metrics",
			role:           "target_db_importer",
			override:       0,
			legacyProfile:  false,
			wantPort:       0,
			wantEnabled:    false,
			failureMessage: "port 0 with no legacy fallback must disable metrics",
		},
		{
			name:           "explicit port enables metrics",
			role:           "target_db_importer",
			override:       9200,
			legacyProfile:  false,
			wantPort:       9200,
			wantEnabled:    true,
			failureMessage: "explicit port must enable",
		},
		{
			name:           "explicit port wins over legacy fallback",
			role:           "target_db_importer",
			override:       9200,
			legacyProfile:  true,
			wantPort:       9200,
			wantEnabled:    true,
			failureMessage: "explicit port must win over legacy fallback",
		},
		{
			name:           "legacy fallback uses target_db_importer default port",
			role:           "target_db_importer",
			override:       0,
			legacyProfile:  true,
			wantPort:       9101,
			wantEnabled:    true,
			failureMessage: "legacy --profile fallback must use role default port",
		},
		{
			name:           "legacy fallback uses import_file default port",
			role:           "import_file",
			override:       0,
			legacyProfile:  true,
			wantPort:       9102,
			wantEnabled:    true,
			failureMessage: "legacy --profile fallback must use role default port",
		},
		{
			name:           "legacy fallback has no default for export roles",
			role:           "source_db_exporter",
			override:       0,
			legacyProfile:  true,
			wantPort:       0,
			wantEnabled:    false,
			failureMessage: "legacy --profile fallback has no default for export roles",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			p, on := resolveMetricsPort(tc.role, tc.override, tc.legacyProfile)
			if on != tc.wantEnabled || p != tc.wantPort {
				t.Fatalf("%s; got (%d,%t), want (%d,%t)", tc.failureMessage, p, on, tc.wantPort, tc.wantEnabled)
			}
		})
	}
}
