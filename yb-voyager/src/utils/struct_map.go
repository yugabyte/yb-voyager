package utils

import "github.com/samber/lo"

// This file provides a generic map data structure that allows using custom structs as keys.
// In golang, by default, you can use structs as keys as long as the fields of the structs themselves are comparable.
// If the struct has pointer fields, this becomes problematic because when comparing for equality in the map,
// the pointer values(integers) of the struct are compared, and not the values pointed by the pointers.

// This implementation of StructMap overcomes this limitation. The custom objects are
// converted to strings using the Keyer interface. Internally golang maps are used to store the keys, and values.

// Keyer is an interface that should be implemented by custom objects that are used as keys
// in the map data structure.
type Keyer interface {
	// Key returns a string representation of the object that is used as a key in the map.
	// Note that every instance of the object should have a unique value for the Key()
	Key() string
}

// Map is a generic map data structure that allows using custom objects as keys.
// Note that every instance of the object should have a unique value for the Key()
type StructMap[K Keyer, V any] struct {
	kmap map[string]K
	vmap map[string]V
}

// NewMap creates a new Map data structure.
func NewStructMap[K Keyer, V any]() *StructMap[K, V] {
	return &StructMap[K, V]{
		kmap: make(map[string]K),
		vmap: make(map[string]V),
	}
}

// Put sets the value for the given key in the map.
func (m *StructMap[K, V]) Put(key K, value V) {
	m.kmap[key.Key()] = key
	m.vmap[key.Key()] = value
}

// Get returns the value for the given key in the map.
func (m *StructMap[K, V]) Get(key K) (V, bool) {
	value, ok := m.vmap[key.Key()]
	return value, ok
}

// Delete deletes the value for the given key in the map.
func (m *StructMap[K, V]) Delete(key K) {
	delete(m.kmap, key.Key())
	delete(m.vmap, key.Key())
}

// IterKV iterates over the key-value pairs of the map and calls the given function for each pair.
// The function should return false to stop the iteration.
func (m *StructMap[K, V]) IterKV(f func(key K, value V) (bool, error)) error {
	for k, key := range m.kmap {
		value := m.vmap[k]
		shouldContinue, err := f(key, value)
		if err != nil {
			return err
		}
		if !shouldContinue {
			break
		}
	}
	return nil
}

func (m *StructMap[K, V]) Clear() {
	m.kmap = make(map[string]K)
	m.vmap = make(map[string]V)
}

//Returns the internal keys used for the map
func (m *StructMap[K, V]) Keys() []string {
	return lo.Keys(m.kmap)
}

//Returns the actual keys of struct map
func (m *StructMap[K, V]) ActualKeys() []K {
	return lo.Values(m.kmap)
}