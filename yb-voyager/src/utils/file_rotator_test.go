package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFileRotator_Rotation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	maxSize := int64(1024) // 1KB
	rotator, err := NewFileRotator(logFile, maxSize)
	if err != nil {
		t.Fatalf("failed to create FileRotator: %v", err)
	}

	// Write just under the limit
	data := strings.Repeat("a", int(maxSize-10))
	_, err = rotator.Write([]byte(data))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	info, err := os.Stat(logFile)
	if err != nil {
		t.Fatalf("stat failed: %v", err)
	}
	if info.Size() < maxSize-10 {
		t.Errorf("file size too small, got %d, want at least %d", info.Size(), maxSize-10)
	}

	// Write more to trigger rotation
	_, err = rotator.Write([]byte(strings.Repeat("b", 20)))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// After rotation, the log file should exist and be small (rotated file is kept by lumberjack)
	info, err = os.Stat(logFile)
	if err != nil {
		t.Fatalf("stat after rotation failed: %v", err)
	}
	if info.Size() > maxSize {
		t.Errorf("file size after rotation too large: %d > %d", info.Size(), maxSize)
	}

	// Assert presence of the rotated file (with numeric suffix)
	logFileBase := strings.TrimSuffix(logFile, filepath.Ext(logFile))
	rotatedFiles, err := filepath.Glob(logFileBase + "-*")
	if err != nil {
		t.Fatalf("glob for rotated files failed: %v", err)
	}

	if len(rotatedFiles) == 0 {
		t.Errorf("expected rotated file after rotation, found none")
	}
}

func TestFileRotator_WriteLargerThanMaxSize(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test_large.log")
	maxSize := int64(1024) // 1KB
	rotator, err := NewFileRotator(logFile, maxSize)
	if err != nil {
		t.Fatalf("failed to create FileRotator: %v", err)
	}

	// Write a chunk larger than maxSize
	bigData := strings.Repeat("x", int(maxSize+500))
	n, err := rotator.Write([]byte(bigData))
	if err != nil {
		t.Fatalf("write of large chunk failed: %v", err)
	}
	if n != len(bigData) {
		t.Errorf("write returned %d, want %d", n, len(bigData))
	}

	// After the write, rotation should have occurred, so a new log file should exist
	// and the rotated file should also exist (with a numeric suffix)
	logFileBase := strings.TrimSuffix(logFile, filepath.Ext(logFile))
	rotatedFiles, err := filepath.Glob(logFileBase + "-*")
	if err != nil {
		t.Fatalf("glob failed: %v", err)
	}
	if len(rotatedFiles) != 1 {
		t.Errorf("expected 1 rotated file after large write, found %d", len(rotatedFiles))
	} else {
		rotatedInfo, err := os.Stat(rotatedFiles[0])
		if err != nil {
			t.Fatalf("stat on rotated file failed: %v", err)
		}
		expectedSize := maxSize + 500
		if rotatedInfo.Size() != expectedSize {
			t.Errorf("rotated file size mismatch: got %d, want %d", rotatedInfo.Size(), expectedSize)
		}
	}
}
