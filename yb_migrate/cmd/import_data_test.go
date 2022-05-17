package cmd

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsDataLine(t *testing.T) {
	assert := assert.New(t)
	testcases := []struct {
		line     string
		expected bool
	}{
		{`SET client_encoding TO 'UTF8';`, false},
		{`set client_encoding to 'UTF8';`, false},
		{`COPY "Foo" ("v") FROM STDIN;`, false},
		{"", false},
		{"\n", false},
		{"\\.\n", false},
		{"\\.", false},
	}
	insideCopyStmt := false
	for _, tc := range testcases {
		assert.Equal(tc.expected, isDataLine(tc.line, "oracle", &insideCopyStmt), "%q", tc.line)
	}
	insideCopyStmt = false
	for i := 3; i < len(testcases); i++ {
		assert.Equal(testcases[i].expected, isDataLine(testcases[i].line, "postgresql", &insideCopyStmt), "%q", testcases[i].line)
	}
}
