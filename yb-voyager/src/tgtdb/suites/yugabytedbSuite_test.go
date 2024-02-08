package tgtdbsuite

import "testing"

func TestTimestampType(t *testing.T) {
	epochMilliseconds := "1707384224000"
	expectedOutput := "2024-02-08 09:23:44"
	fn := YBValueConverterSuite["io.debezium.time.Timestamp"]
	output, err := fn(epochMilliseconds, false)
	if err != nil {
		t.Error(err)
	}
	if output != expectedOutput {
		t.Errorf("Result was incorrect, got: %s, want: %s.", output, expectedOutput)
	}
}
