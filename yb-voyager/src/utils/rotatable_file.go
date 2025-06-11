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
package utils

import (
	"errors"
	"fmt"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	lumberjackMaxMB        = 1000            // MB, set high so lumberjack doesn't auto-rotate
	defaultRotatorMaxBytes = 5 * 1024 * 1024 // 5MB default
)

// RotatableFile wraps lumberjack.Logger to provide best-effort file rotation based only on maxFileSize.
//
// We do not use lumberjack.Logger directly because it enforces a hard limit on MaxSize: if a single
// write exceeds MaxSize, it returns an error and does not write the data. RotatableFile, instead,
// always writes the data and only rotates after the write if the file size exceeds maxFileSize.
// This ensures that large writes are never dropped or errored, and rotation is handled gracefully.
// Only maxFileSize is considered for rotation; all other lumberjack knobs are ignored or set to defaults.
//
// RotatableFile implements io.Writer and wraps lumberjack.Logger to provide best-effort rotation.
type RotatableFile struct {
	Logger      *lumberjack.Logger
	MaxFileSize int64 // in bytes
}

// NewRotatableFile creates a new RotatableFile with the given filename and maxFileSize (in bytes).
// If maxFileSize is 0, defaults to 5MB.
func NewRotatableFile(filename string, maxFileSize int64) (*RotatableFile, error) {
	if maxFileSize <= 0 {
		maxFileSize = defaultRotatorMaxBytes
	}
	if maxFileSize >= lumberjackMaxMB*1024*1024 {
		return nil, errors.New(fmt.Sprintf("maxFileSize must be less than %d MB", lumberjackMaxMB))
	}
	return &RotatableFile{
		Logger: &lumberjack.Logger{
			Filename: filename,
			MaxSize:  lumberjackMaxMB,
		},
		MaxFileSize: maxFileSize,
	}, nil
}

// Write implements io.Writer. It writes p to the file, rotating if needed.
// If a single write exceeds maxFileSize, it will still write the data and rotate after.
func (fr *RotatableFile) Write(p []byte) (n int, err error) {
	// Check file size before writing (not strictly needed, but can be used for future logic)
	// Write the data
	n, err = fr.Logger.Write(p)
	if err != nil {
		return n, err
	}

	// Check file size after writing
	fileInfo, statErr := os.Stat(fr.Logger.Filename)
	if statErr != nil {
		return n, statErr
	}
	if fileInfo.Size() > fr.MaxFileSize {
		if rotateErr := fr.Logger.Rotate(); rotateErr != nil {
			return n, rotateErr
		}
	}
	return n, nil
}
