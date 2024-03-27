package utils

import (
	"testing"
)

// Define a struct to use as keys in the map
type CustomKey struct {
	ID   int
	Name string
}

// Implement the Keyer interface for CustomKey
func (k CustomKey) Key() string {
	return k.Name
}

func TestStructMap(t *testing.T) {
	// Initialize a new StructMap
	m := NewStructMap[CustomKey, string]()

	// Test Put and Get methods
	m.Put(CustomKey{ID: 1, Name: "key1"}, "value1")
	m.Put(CustomKey{ID: 2, Name: "key2"}, "value2")

	val, ok := m.Get(CustomKey{ID: 1, Name: "key1"})
	if !ok || val != "value1" {
		t.Errorf("Expected 'value1', got %v", val)
	}

	// Test Put with duplicate key
	m.Put(CustomKey{ID: 1, Name: "key1"}, "value3")
	val, _ = m.Get(CustomKey{ID: 1, Name: "key1"})
	if val != "value3" {
		t.Errorf("Expected 'value3' for duplicate key, got %v", val)
	}

	// Test Delete method
	m.Delete(CustomKey{ID: 1, Name: "key1"})
	_, ok = m.Get(CustomKey{ID: 1, Name: "key1"})
	if ok {
		t.Error("Expected key 'key1' to be deleted")
	}

	// Test IterKV method
	m.Put(CustomKey{ID: 3, Name: "key3"}, "value3")
	m.Put(CustomKey{ID: 4, Name: "key4"}, "value4")

	keys := make(map[string]bool)
	m.IterKV(func(key CustomKey, value string) (bool, error) {
		keys[key.Key()] = true
		return true, nil
	})

	expectedKeys := map[string]bool{"key2": true, "key3": true, "key4": true}
	for key := range expectedKeys {
		if !keys[key] {
			t.Errorf("Expected key %s not found in IterKV result", key)
		}
	}

	// Test Clear method
	m.Clear()
	_, ok = m.Get(CustomKey{ID: 2, Name: "key2"})

	if ok {
		t.Error("Expected map to be cleared")
	}
}
