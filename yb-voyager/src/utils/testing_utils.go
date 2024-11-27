package utils

import (
	"os"
	"reflect"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// CompareStructAndReport compares two struct types and reports any mismatches.
func CompareStructAndReport(t *testing.T, actual, expected reflect.Type, structName string) {
	if actual.Kind() != reflect.Struct || expected.Kind() != reflect.Struct {
		t.Fatalf("Both %s and expected type must be structs. There is some breaking change!", structName)
	}

	if actual.NumField() != expected.NumField() {
		t.Errorf("%s: Number of fields mismatch. Got %d, expected %d. There is some breaking change!", structName, actual.NumField(), expected.NumField())
	}

	for i := 0; i < max(actual.NumField(), expected.NumField()); i++ {
		var actualField, expectedField reflect.StructField
		var actualExists, expectedExists bool

		if i < actual.NumField() {
			actualField = actual.Field(i)
			actualExists = true
		}
		if i < expected.NumField() {
			expectedField = expected.Field(i)
			expectedExists = true
		}

		// Compare field names
		if actualExists && expectedExists && actualField.Name != expectedField.Name {
			t.Errorf("%s: Field name mismatch at position %d. Got %s, expected %s. There is some breaking change!", structName, i, actualField.Name, expectedField.Name)
		}

		// Compare field types
		if actualExists && expectedExists && actualField.Type != expectedField.Type {
			t.Errorf("%s: Field type mismatch for %s. Got %s, expected %s. There is some breaking change!", structName, actualField.Name, actualField.Type, expectedField.Type)
		}

		// Compare tags
		if actualExists && expectedExists && actualField.Tag != expectedField.Tag {
			t.Errorf("%s: Field tag mismatch for %s. Got %s, expected %s. There is some breaking change!", structName, actualField.Name, actualField.Tag, expectedField.Tag)
		}

		// Report missing fields
		if !actualExists && expectedExists {
			t.Errorf("%s: Missing field %s of type %s. There is some breaking change!", structName, expectedField.Name, expectedField.Type)
		}
		if actualExists && !expectedExists {
			t.Errorf("%s: Unexpected field %s of type %s. There is some breaking change!", structName, actualField.Name, actualField.Type)
		}
	}
}

// CompareJsonStructs compares two structs by marshalling them into JSON and reports any differences.
func CompareJson(t *testing.T, outputFilePath string, expectedJSON string, exportDir string) {
	// Read the output JSON file
	outputBytes, err := os.ReadFile(outputFilePath)
	if err != nil {
		t.Fatalf("Failed to read output JSON file: %v", err)
	}

	// Compare the output JSON with the expected JSON
	if diff := cmp.Diff(expectedJSON, string(outputBytes)); diff != "" {
		t.Errorf("JSON file mismatch (-expected +actual):\n%s", diff)
	}

	// Optionally, remove the test directory if empty
	if err := os.RemoveAll(exportDir); err != nil {
		t.Logf("Failed to remove test export directory: %v", err)
	}
}
