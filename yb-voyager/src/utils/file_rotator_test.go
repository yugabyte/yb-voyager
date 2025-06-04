package utils

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// getRotatedFiles returns the list of rotated log files for a given log file path.
func getRotatedFiles(logFile string) ([]string, error) {
	logFileBase := strings.TrimSuffix(logFile, filepath.Ext(logFile))
	return filepath.Glob(logFileBase + "-*")
}

// assertRotatedFilesCount checks the number of rotated log files and fails the test if the count does not match expectedCount.
func assertRotatedFilesCount(t *testing.T, logFile string, expectedCount int, context string) {
	rotatedFiles, err := getRotatedFiles(logFile)
	if err != nil {
		t.Fatalf("glob for rotated files failed (%s): %v", context, err)
	}
	if len(rotatedFiles) != expectedCount {
		t.Errorf("expected %d rotated files %s, found %d", expectedCount, context, len(rotatedFiles))
	}
}

// assertFileSizeLessThan checks that the file size is less than or equal to maxSize and fails the test if not.
func assertFileSizeLessThan(t *testing.T, filePath string, maxSize int64, context string) {
	info, err := os.Stat(filePath)
	if err != nil {
		t.Fatalf("stat failed (%s): %v", context, err)
	}
	if info.Size() > maxSize {
		t.Errorf("file size after %s too large: %d > %d", context, info.Size(), maxSize)
	}
}

func TestFileRotator_Rotation(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test.log")
	maxSize := int64(1024) // 1KB
	rotator, err := NewRotatableFile(logFile, maxSize)
	if err != nil {
		t.Fatalf("failed to create FileRotator: %v", err)
	}

	// Write just under the limit
	data := strings.Repeat("a", int(maxSize-10))
	_, err = rotator.Write([]byte(data))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	assertFileSizeLessThan(t, logFile, maxSize, "before rotation")
	assertRotatedFilesCount(t, logFile, 0, "before rotation")

	// Write more to trigger rotation
	_, err = rotator.Write([]byte(strings.Repeat("b", 20)))
	if err != nil {
		t.Fatalf("write failed: %v", err)
	}

	// After rotation, the log file should exist and be small (rotated file is kept by lumberjack)
	assertFileSizeLessThan(t, logFile, maxSize, "rotation")

	// Assert presence of the rotated file (with time suffix)
	assertRotatedFilesCount(t, logFile, 1, "after rotation")
}

func TestFileRotator_WriteLargerThanMaxSize(t *testing.T) {
	dir := t.TempDir()
	logFile := filepath.Join(dir, "test_large.log")
	maxSize := int64(1024) // 1KB
	rotator, err := NewRotatableFile(logFile, maxSize)
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
	assertRotatedFilesCount(t, logFile, 1, "after large write")
	rotatedFiles, _ := getRotatedFiles(logFile)

	rotatedInfo, err := os.Stat(rotatedFiles[0])
	if err != nil {
		t.Fatalf("stat on rotated file failed: %v", err)
	}
	expectedSize := maxSize + 500
	if rotatedInfo.Size() != expectedSize {
		t.Errorf("rotated file size mismatch: got %d, want %d", rotatedInfo.Size(), expectedSize)
	}

}
