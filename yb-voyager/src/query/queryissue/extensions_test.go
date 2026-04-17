package queryissue

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

func TestExtensionsVersionSupport(t *testing.T) {
	parserIssueDetector := NewParserIssueDetector()

	testCases := []struct {
		name            string
		extension       string
		version         *ybversion.YBVersion
		shouldHaveIssue bool
	}{
		// Vector (should be supported everywhere now)
		{"vector_2024_1", "vector", ybversion.V2024_1_0_0, false},
		{"vector_2_25", "vector", ybversion.V2_25_0_0, false},

		// pg_partman (added in 2024.2 and 2.25)
		{"pg_partman_2024_1", "pg_partman", ybversion.V2024_1_0_0, true},
		{"pg_partman_2024_2", "pg_partman", ybversion.V2024_2_0_0, false},
		{"pg_partman_2_20", "pg_partman", ybversion.V2_23_0_0, true}, // Using V2_23 as proxy for < 2.25
		{"pg_partman_2_25", "pg_partman", ybversion.V2_25_0_0, false},

		// anon (added in 2024.2 and 2.25.1)
		{"anon_2024_2", "anon", ybversion.V2024_2_0_0, false},
		{"anon_2_25_0", "anon", ybversion.V2_25_0_0, true},
		{"anon_2_25_1", "anon", ybversion.V2_25_1_0, false},

		// timetravel (removed in 2.25+, 2025.1+)
		{"timetravel_2024_2", "timetravel", ybversion.V2024_2_0_0, false},
		{"timetravel_2_25", "timetravel", ybversion.V2_25_0_0, true},
		{"timetravel_2025_1", "timetravel", ybversion.V2025_1_0_0, true},

		// Unknown extension
		{"unknown_ext", "my_cool_ext", ybversion.V2025_1_0_0, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			stmt := "CREATE EXTENSION " + tc.extension + ";"
			issues, err := parserIssueDetector.GetDDLIssues(stmt, tc.version)
			assert.NoError(t, err)

			if tc.shouldHaveIssue {
				assert.NotEmpty(t, issues, "Expected issue for %s on %s", tc.extension, tc.version)
				assert.Equal(t, UNSUPPORTED_EXTENSION, issues[0].Issue.Type)
			} else {
				assert.Empty(t, issues, "Expected NO issue for %s on %s", tc.extension, tc.version)
			}
		})
	}
}
