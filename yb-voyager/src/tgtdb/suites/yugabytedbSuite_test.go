//go:build unit

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
// package for tgtdb value converter suite
package tgtdbsuite

import (
	"encoding/base64"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStringConversionWithFormattingWithDoubleQuotes(t *testing.T) {
	// Given
	value := "abc\"def"
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["STRING"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, "'abc\"def'", result)
}

func TestStringConversionWithFormattingWithSingleQuotesEscaped(t *testing.T) {
	// Given
	value := "abc'def"
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["STRING"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, "'abc''def'", result)
}

func TestJsonConversionWithFormattingWithDoubleQuotes(t *testing.T) {
	// Given
	value := `{"key":"value"}`
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Json"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'{"key":"value"}'`, result)
}

func TestJsonConversionWithFormattingWithSingleQuotesEscaped(t *testing.T) {
	// Given
	value := `{"key":"value's"}`
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Json"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'{"key":"value''s"}'`, result)
}

func TestEnumConversionWithFormattingWithDoubleQuotes(t *testing.T) {
	// Given
	value := `enum"Value`
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Enum"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'enum"Value'`, result)
}

func TestEnumConversionWithFormattingWithSingleQuotesEscaped(t *testing.T) {
	// Given
	value := "enum'Value"
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Enum"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, "'enum''Value'", result)
}

func TestUUIDConversionWithFormatting(t *testing.T) {
	// Given
	value := "123e4567-e89b-12d3-a456-426614174000"
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["io.debezium.data.Uuid"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'123e4567-e89b-12d3-a456-426614174000'`, result)
}

func TestBytesConversionWithFormatting(t *testing.T) {
	//small data example
	value := "////wv=="
	// When we convert with formatIfRequired is true
	result, err := YBValueConverterSuite["BYTES"](value, true, nil)
	assert.NoError(t, err)
	// Then
	assert.Equal(t, `'\xffffffc2'`, result)

	//large data example for bytea - create a large base64 encoded string
	// Generate 10KB of binary data (10240 bytes)
	largeData := make([]byte, 10240)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}
	// Encode to base64
	largeBase64Value := base64.StdEncoding.EncodeToString(largeData)

	// When we convert with formatIfRequired is true
	result, err = YBValueConverterSuite["BYTES"](largeBase64Value, true, nil)
	assert.NoError(t, err)
	//verify the result is same as expected
	assert.Equal(t, len(result), 20484) //20480 hex chars + 4 for '\x' + 2 for quotes

	largeData10MB := make([]byte, 10000000)
	for i := range largeData10MB {
		largeData10MB[i] = byte(i % 256)
	}
	// Encode to base64
	largeBase64Value10MB := base64.StdEncoding.EncodeToString(largeData10MB)

	// When we convert with formatIfRequired is true
	result, err = YBValueConverterSuite["BYTES"](largeBase64Value10MB, true, nil)
	assert.NoError(t, err)

	//verify the result is same as expected
	assert.Equal(t, len(result), 20000004) //20000000 hex chars + 4 for '\x' + 2 for quotes

	largeData200MB := make([]byte, 200000000)
	for i := range largeData200MB {
		largeData200MB[i] = byte(i % 256)
	}
	// Encode to base64
	largeBase64Value200MB := base64.StdEncoding.EncodeToString(largeData200MB)

	// When we convert with formatIfRequired is true
	result, err = YBValueConverterSuite["BYTES"](largeBase64Value200MB, true, nil)
	assert.NoError(t, err)

	//verify the result is same as expected
	assert.Equal(t, len(result), 400000004) //400000000 hex chars + 4 for '\x' + 2 for quotes
}

func TestZonedTimeConversionWithFormatting(t *testing.T) {
	value := "12:34:56.789+05:30"
	typeName := "io.debezium.time.ZonedTime"
	
	converterFn := YBValueConverterSuite[typeName]
	assert.NotNil(t, converterFn, "Converter function for %s should NOT be NIL after fix", typeName)

	result, err := converterFn(value, true, nil)
	assert.NoError(t, err)
	assert.Equal(t, "'"+value+"'", result, "Expected value to be quoted after fix")
}
