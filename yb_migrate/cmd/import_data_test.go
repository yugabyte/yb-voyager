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
	checkCopyStmt := -1
	for _, tc := range testcases {
		assert.Equal(tc.expected, isDataLine(tc.line, 0, &checkCopyStmt, "oracle"), "%q", tc.line)
	}
}
