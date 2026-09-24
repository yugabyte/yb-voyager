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
package datastore

import (
	"io"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Byte-offset seek resumption is intentionally disabled for GCS and Azure for
// now (see gcsDatastore.go / azDatastore.go). Their OpenAt must return
// ErrOpenAtNotImplemented so the import caller falls back to the older
// SkipLines-based resumption path. These tests lock that contract in place so
// the disabled path is not silently re-enabled.

func TestGCSDataStoreOpenAtReturnsNotImplemented(t *testing.T) {
	ds := NewGCSDataStore("gs://some-bucket/data")

	reader, err := ds.OpenAt("gs://some-bucket/data/file.csv", 100)

	assert.Nil(t, reader, "GCS OpenAt should not return a reader while disabled")
	assert.ErrorIs(t, err, ErrOpenAtNotImplemented,
		"GCS must signal ErrOpenAtNotImplemented so the caller uses the SkipLines path")
}

func TestAzDataStoreOpenAtReturnsNotImplemented(t *testing.T) {
	ds := NewAzDataStore("https://account.blob.core.windows.net/container")

	reader, err := ds.OpenAt("https://account.blob.core.windows.net/container/file.csv", 100)

	assert.Nil(t, reader, "Azure OpenAt should not return a reader while disabled")
	assert.ErrorIs(t, err, ErrOpenAtNotImplemented,
		"Azure must signal ErrOpenAtNotImplemented so the caller uses the SkipLines path")
}

// Contrast case: Local (and, by the same token, S3) keep the new byte-offset
// seek path. Local can be exercised without any external dependency, so we
// assert here that it does NOT report ErrOpenAtNotImplemented and actually
// seeks to the requested offset.
func TestLocalDataStoreOpenAtSeeksToOffset(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "file.txt")
	contents := "0123456789ABCDEF"
	require.NoError(t, os.WriteFile(path, []byte(contents), 0644))

	ds := NewLocalDataStore(dir)

	reader, err := ds.OpenAt(path, 10)
	require.NoError(t, err)
	assert.NotErrorIs(t, err, ErrOpenAtNotImplemented,
		"Local keeps the byte-offset seek path and must not signal ErrOpenAtNotImplemented")
	require.NotNil(t, reader)
	defer reader.Close()

	got, err := io.ReadAll(reader)
	require.NoError(t, err)
	assert.Equal(t, "ABCDEF", string(got), "reader should start at the requested byte offset")
}
