// Package utils provides utility functions and types for yb-voyager.
//
// file_rotator.go: Implements a fileRotator struct on top of lumberjack.Logger
// that does not error if a single write exceeds maxFileSize. Rotation is best-effort.

package utils

import (
	"errors"
	"os"

	"gopkg.in/natefinch/lumberjack.v2"
)

// FileRotator wraps lumberjack.Logger to provide best-effort file rotation based only on maxFileSize.
//
// We do not use lumberjack.Logger directly because it enforces a hard limit on MaxSize: if a single
// write exceeds MaxSize, it returns an error and does not write the data. FileRotator, instead,
// always writes the data and only rotates after the write if the file size exceeds maxFileSize.
// This ensures that large writes are never dropped or errored, and rotation is handled gracefully.
// Only maxFileSize is considered for rotation; all other lumberjack knobs are ignored or set to defaults.
//
// FileRotator implements io.Writer and wraps lumberjack.Logger to provide best-effort rotation.
type FileRotator struct {
	Logger      *lumberjack.Logger
	MaxFileSize int64 // in bytes
}

// NewFileRotator creates a new FileRotator with the given filename and maxFileSize (in bytes).
// If maxFileSize is 0, defaults to 5MB.
func NewFileRotator(filename string, maxFileSize int64) (*FileRotator, error) {
	const lumberjackMaxMB = 1000 // MB, set high so lumberjack doesn't auto-rotate
	const lumberjackMaxBytes = lumberjackMaxMB * 1024 * 1024

	if maxFileSize <= 0 {
		maxFileSize = 5 * 1024 * 1024 // 5MB default
	}
	if maxFileSize >= lumberjackMaxBytes {
		return nil, errors.New("maxFileSize must be less than 1000 MB (1048576000 bytes)")
	}
	return &FileRotator{
		Logger: &lumberjack.Logger{
			Filename: filename,
			MaxSize:  lumberjackMaxMB,
		},
		MaxFileSize: maxFileSize,
	}, nil
}

// Write implements io.Writer. It writes p to the file, rotating if needed.
// If a single write exceeds maxFileSize, it will still write the data and rotate after.
func (fr *FileRotator) Write(p []byte) (n int, err error) {
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
