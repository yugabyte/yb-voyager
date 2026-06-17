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
	"fmt"
	"sync"
)

// Attr is a database-type-specific extension property attached to a per-object struct.
// The key is a stable dotted identifier (e.g. "postgres.replica_identity").
// The value is the comparable, renderable property value.
type Attr interface {
	AttrKey() string
	AttrValue() any
}

// attrDecoderRegistry maps an Attr key to a decoder that reconstructs the concrete
// Attr type from the JSON "value" field.
var (
	attrRegistryMu sync.RWMutex
	attrRegistry   = map[string]func(json.RawMessage) (Attr, error){}
)

// RegisterAttrDecoder registers a decoder for the given Attr key.
// Packages under databases/<dbtype>/ call this in their init() functions.
func RegisterAttrDecoder(key string, decoder func(json.RawMessage) (Attr, error)) {
	attrRegistryMu.Lock()
	defer attrRegistryMu.Unlock()
	attrRegistry[key] = decoder
}

// rawAttr holds an Attr whose key was absent from the registry at load time.
// It satisfies the Attr interface and re-serializes byte-for-byte on the next save.
type rawAttr struct {
	key   string          // the Attr key as read from disk (no registered decoder for it).
	value json.RawMessage // the raw JSON value, preserved verbatim for byte-for-byte re-serialization.
}

func (r rawAttr) AttrKey() string { return r.key }
func (r rawAttr) AttrValue() any  { return r.value }

// ─── Attrs named type with custom JSON (un)marshalling ───────────────────────

// Attrs is a slice of Attr values. It marshals each entry as
// {"key": "<AttrKey()>", "value": <AttrValue()>}
// and unmarshals using the registered decoders; unknown keys produce rawAttr.
type Attrs []Attr

// attrWire is the on-disk representation of a single Attr.
type attrWire struct {
	Key   string          `json:"key"`   // the Attr's stable dotted identity, e.g. "postgres.replica_identity".
	Value json.RawMessage `json:"value"` // the Attr's value, as produced by AttrValue() and decoded by the registered decoder.
}

// MarshalJSON serialises each Attr as {"key":"...", "value":<value>}.
func (a Attrs) MarshalJSON() ([]byte, error) {
	wires := make([]attrWire, 0, len(a))
	for _, attr := range a {
		v, err := json.Marshal(attr.AttrValue())
		if err != nil {
			return nil, fmt.Errorf("schemasnapshot: marshalling attr %q value: %w", attr.AttrKey(), err)
		}
		wires = append(wires, attrWire{Key: attr.AttrKey(), Value: json.RawMessage(v)})
	}
	return json.Marshal(wires)
}

// UnmarshalJSON deserialises a JSON array of {"key":…,"value":…} objects into Attrs.
// Keys present in the registry produce the concrete Attr; missing keys produce rawAttr.
func (a *Attrs) UnmarshalJSON(data []byte) error {
	var wires []attrWire
	if err := json.Unmarshal(data, &wires); err != nil {
		return fmt.Errorf("schemasnapshot: unmarshalling attrs array: %w", err)
	}

	result := make(Attrs, 0, len(wires))
	attrRegistryMu.RLock()
	defer attrRegistryMu.RUnlock()

	for _, w := range wires {
		if decode, ok := attrRegistry[w.Key]; ok {
			attr, err := decode(w.Value)
			if err != nil {
				return fmt.Errorf("schemasnapshot: decoding attr %q: %w", w.Key, err)
			}
			result = append(result, attr)
		} else {
			result = append(result, rawAttr{key: w.Key, value: w.Value})
		}
	}
	*a = result
	return nil
}
