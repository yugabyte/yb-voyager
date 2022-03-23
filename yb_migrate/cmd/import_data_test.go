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
		{"SET search_path = sakila_test,public;", false},
		{`TRUNCATE TABLE "Foo";`, false},
		{`truncate table "Foo";`, false},
		{`COPY "Foo" ("v") FROM STDIN;`, false},
		{"", false},
		{"\n", false},
		{`\.`, false},
		{"SET MAX 530\n", true},
		{"TRUNCATE FOO BAR", true},
	}
	for _, tc := range testcases {
		assert.Equal(tc.expected, isDataLine(tc.line), "%q", tc.line)
	}
}
