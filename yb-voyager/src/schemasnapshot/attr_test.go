//go:build unit

// Copyright (c) YugabyteDB, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package schemasnapshot

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Test Attr implementations ────────────────────────────────────────────────

// stringAttr is a test-only Attr implementation that holds a string value.
type stringAttr struct {
	key string
	val string
}

func (s stringAttr) AttrKey() string { return s.key }
func (s stringAttr) AttrValue() any  { return s.val }

// TestAttrsMarshalUnmarshalRoundTrip checks that a registered Attr round-trips
// through Attrs JSON encoding/decoding.
func TestAttrsMarshalUnmarshalRoundTrip(t *testing.T) {
	// Register a test decoder for "test.string_attr" key.
	RegisterAttrDecoder("test.string_attr", func(raw json.RawMessage) (Attr, error) {
		var v string
		if err := json.Unmarshal(raw, &v); err != nil {
			return nil, err
		}
		return stringAttr{key: "test.string_attr", val: v}, nil
	})

	original := Attrs{stringAttr{key: "test.string_attr", val: "hello"}}

	data, err := json.Marshal(original)
	require.NoError(t, err)

	var decoded Attrs
	require.NoError(t, json.Unmarshal(data, &decoded))

	require.Len(t, decoded, 1)
	assert.Equal(t, "test.string_attr", decoded[0].AttrKey())
	assert.Equal(t, "hello", decoded[0].AttrValue())
	// Should decode to our concrete type.
	sa, ok := decoded[0].(stringAttr)
	require.True(t, ok, "decoded Attr must be a stringAttr, got %T", decoded[0])
	assert.Equal(t, "hello", sa.val)
}

// TestAttrsUnknownKeyBecomesRawAttr checks that an unregistered key produces a rawAttr
// that preserves the original value bytes.
func TestAttrsUnknownKeyBecomesRawAttr(t *testing.T) {
	// Write a JSON array with a key we never registered.
	raw := `[{"key":"unknown.future_attr","value":"some_value"}]`
	var decoded Attrs
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))

	require.Len(t, decoded, 1)
	assert.Equal(t, "unknown.future_attr", decoded[0].AttrKey())
	// rawAttr.AttrValue() returns json.RawMessage — check it round-trips.
	v, ok := decoded[0].AttrValue().(json.RawMessage)
	require.True(t, ok, "unknown attr's AttrValue() must be json.RawMessage, got %T", decoded[0].AttrValue())
	assert.JSONEq(t, `"some_value"`, string(v))
}

// TestAttrsEmptySliceMarshal checks that an empty Attrs marshals to "[]" (not null),
// because the MarshalJSON always produces an array.
func TestAttrsEmptySliceMarshal(t *testing.T) {
	var a Attrs
	data, err := json.Marshal(a)
	require.NoError(t, err)
	assert.Equal(t, "[]", string(data))
}

// TestRegisterAttrDecoderOverwrite checks that registering a new decoder for the
// same key overwrites the old one (last-writer-wins semantics).
func TestRegisterAttrDecoderOverwrite(t *testing.T) {
	RegisterAttrDecoder("test.overwrite_key", func(raw json.RawMessage) (Attr, error) {
		return stringAttr{key: "test.overwrite_key", val: "first"}, nil
	})
	RegisterAttrDecoder("test.overwrite_key", func(raw json.RawMessage) (Attr, error) {
		return stringAttr{key: "test.overwrite_key", val: "second"}, nil
	})

	raw := `[{"key":"test.overwrite_key","value":null}]`
	var decoded Attrs
	require.NoError(t, json.Unmarshal([]byte(raw), &decoded))

	require.Len(t, decoded, 1)
	sa, ok := decoded[0].(stringAttr)
	require.True(t, ok)
	assert.Equal(t, "second", sa.val)
}
