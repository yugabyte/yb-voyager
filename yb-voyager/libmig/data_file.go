package libmig

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type DataFile interface {
	Open() error
	Offset() int64
	Size() int64
	SkipRecords(n int) (int, bool, error)
	GetCopyCommand(tableID *TableID) (string, error)
}

func NewDataFile(fileName string, offset int64, desc *DataFileDescriptor) DataFile {
	switch desc.FileType {
	case FILE_TYPE_CSV, FILE_TYPE_TEXT:
		return NewCSVDataFile(fileName, offset, desc)
	case FILE_TYPE_ORA2PG:
		return NewOra2pgDataFile(fileName, offset, desc)
	default:
		panic(fmt.Sprintf("unknown file-type: %q", desc.FileType))
	}
}

//============================================================================

const (
	FILE_TYPE_CSV    = "CSV"
	FILE_TYPE_TEXT   = "TEXT"
	FILE_TYPE_ORA2PG = "ORA2PG"
)

type DataFileDescriptor struct {
	FileType string
}

//============================================================================

type CSVDataFile struct {
	*baseDataFile
}

func NewCSVDataFile(fileName string, offset int64, desc *DataFileDescriptor) *CSVDataFile {
	df := &CSVDataFile{}
	base := newBaseDataFile(fileName, offset, desc, df.isDataLine)
	df.baseDataFile = base
	return df
}

func (df *CSVDataFile) isDataLine(line string) bool {
	return !(line == "" || line == `\.`)
}

func (df *CSVDataFile) GetCopyCommand(tableID *TableID) (string, error) {
	// TODO Use Desc to correctly set FORMAT, DELIMITER, etc.
	cmd := fmt.Sprintf("COPY %s.%s FROM STDIN;", tableID.SchemaName, tableID.TableName)
	return cmd, nil
}

//============================================================================

type Ora2pgDataFile struct {
	*baseDataFile
	insideCopyStmt bool
}

func NewOra2pgDataFile(fileName string, offset int64, desc *DataFileDescriptor) *Ora2pgDataFile {
	df := &Ora2pgDataFile{}
	base := newBaseDataFile(fileName, offset, desc, df.isDataLine)
	df.baseDataFile = base
	return df
}

// Example: `COPY "Foo" ("v") FROM STDIN;`
var reCopy = regexp.MustCompile(`(?i)COPY .* FROM STDIN;`)

func (df *Ora2pgDataFile) isDataLine(line string) bool {
	emptyLine := (len(line) == 0)
	newLineChar := (line == "\n")
	endOfCopy := (line == "\\." || line == "\\.\n")

	if df.insideCopyStmt {
		if endOfCopy {
			df.insideCopyStmt = false
		}
		return !(emptyLine || newLineChar || endOfCopy)
	} else { // outside copy
		if reCopy.MatchString(line) {
			df.insideCopyStmt = true
		}
		return false
	}
}

func (df *Ora2pgDataFile) GetCopyCommand(tableID *TableID) (string, error) {
	fh, err := os.Open(df.FileName)
	if err != nil {
		return "", err
	}
	defer fh.Close()

	scanner := bufio.NewScanner(fh)
	for scanner.Scan() {
		line := scanner.Text()
		if reCopy.MatchString(line) {
			words := strings.Fields(line)
			words[1] = fmt.Sprintf("%s.%s", tableID.SchemaName, tableID.TableName)
			line = strings.Join(words, " ")
			return line, nil
		}
	}
	err = scanner.Err()
	if err == nil {
		err = fmt.Errorf("no COPY statement found in %q", df.FileName)
	}
	return "", err
}

//============================================================================

type baseDataFile struct {
	Desc       *DataFileDescriptor
	FileName   string
	offset     int64
	isDataLine func(string) bool

	fh      *os.File
	scanner *bufio.Scanner
	size    int64
}

func newBaseDataFile(fileName string, offset int64, desc *DataFileDescriptor, isDataLine func(string) bool) *baseDataFile {
	return &baseDataFile{FileName: fileName, offset: offset, Desc: desc, isDataLine: isDataLine}
}

func (df *baseDataFile) Open() error {
	fh, err := os.Open(df.FileName)
	if err != nil {
		return err
	}
	_, err = fh.Seek(df.offset, 0)
	if err != nil {
		return err
	}
	df.fh = fh
	df.scanner = bufio.NewScanner(fh)

	fi, err := fh.Stat()
	if err != nil {
		return err
	}
	df.size = fi.Size()

	return nil
}

func (df *baseDataFile) Offset() int64 {
	return df.offset
}

func (df *baseDataFile) Size() int64 {
	return df.size
}

func (df *baseDataFile) SkipRecords(n int) (int, bool, error) {
	count := 0
	for count < n && df.scanner.Scan() {
		line := df.scanner.Text()
		df.offset += int64(len(line)) + 1 // Add 1 to account for '\n'.
		if df.isDataLine(line) {
			count++
		}
	}
	eof := df.scanner.Err() == nil && count < n
	return count, eof, df.scanner.Err()
}

//============================================================================
