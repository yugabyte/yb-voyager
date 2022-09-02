package importdata

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
)

type DataFile interface {
	Open() error
	Close()
	Offset() int64
	Size() int64
	SkipRecords(n int) (int, bool, error)
	GetCopyCommand(tableID *TableID) (string, error)
	GetHeader() (string, error)
	SkipHeader() error
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
	FILE_TYPE_CSV    = "csv"
	FILE_TYPE_TEXT   = "text"
	FILE_TYPE_ORA2PG = "ora2pg"
)

type DataFileDescriptor struct {
	FileType   string
	Delimiter  string
	HasHeader  bool
	EscapeChar string
	QuoteChar  string
}

//============================================================================

type CSVDataFile struct {
	*baseDataFile
	header string
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
	var cmd string

	switch df.Desc.FileType {
	case FILE_TYPE_CSV:
		if df.Desc.HasHeader {
			header, err := df.GetHeader()
			if err != nil {
				return "", fmt.Errorf("get header: %w", err)
			}
			columnNames := strings.Join(strings.Split(header, df.Desc.Delimiter), ",")
			cmd = fmt.Sprintf(`COPY %s.%s(%s) FROM STDIN WITH (FORMAT CSV, DELIMITER '%s', ESCAPE '%s', QUOTE '%s', HEADER, ROWS_PER_TRANSACTION %%v)`,
				tableID.SchemaName, tableID.TableName, columnNames, df.Desc.Delimiter, df.Desc.EscapeChar, df.Desc.QuoteChar)
		} else {
			cmd = fmt.Sprintf(`COPY %s.%s FROM STDIN WITH (FORMAT CSV, DELIMITER '%s', ESCAPE '%s', QUOTE '%s', ROWS_PER_TRANSACTION %%v)`,
				tableID.SchemaName, tableID.TableName, df.Desc.Delimiter, df.Desc.EscapeChar, df.Desc.QuoteChar)
		}
	case FILE_TYPE_TEXT:
		cmd = fmt.Sprintf(`COPY %s.%s FROM STDIN WITH (FORMAT TEXT, DELIMITER '%s', ROWS_PER_TRANSACTION %%v)`,
			tableID.SchemaName, tableID.TableName, df.Desc.Delimiter)
	}
	return cmd, nil
}

func (df *CSVDataFile) GetHeader() (string, error) {
	if df.header != "" {
		return df.header, nil
	}
	if !df.Desc.HasHeader {
		return "", nil
	}

	fh, err := os.Open(df.FileName)
	if err != nil {
		return "", err
	}
	defer fh.Close()

	// Treat first non-empty line as a header.
	reader := bufio.NewReader(fh)
	line := ""
	for line == "" {
		line, err = reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		line = strings.TrimSpace(line)
	}

	df.header = line
	return df.header, nil
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
		return "", fmt.Errorf("open %s: %w", df.FileName, err)
	}
	defer fh.Close()

	scanner := newScanner(fh)
	for scanner.Scan() {
		line := scanner.Text()
		// Example line: `COPY "Foo" ("v") FROM STDIN;`
		if reCopy.MatchString(line) {
			words := strings.Fields(line)
			words[1] = fmt.Sprintf("%s.%s", tableID.SchemaName, tableID.TableName)
			words[len(words)-1] = "STDIN WITH (ROWS_PER_TRANSACTION %v);"
			line = strings.Join(words, " ")
			return line, nil
		}
	}
	err = scanner.Err()
	if err != nil {
		return "", fmt.Errorf("scan %s: %w", df.FileName, err)
	}
	return "", fmt.Errorf("no COPY statement found in %q", df.FileName)

}

func (df *Ora2pgDataFile) GetHeader() (string, error) {
	return "", nil
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
		return fmt.Errorf("open %s: %w", df.FileName, err)
	}
	_, err = fh.Seek(df.offset, 0)
	if err != nil {
		return fmt.Errorf("seek %s to %v: %w", df.FileName, df.offset, err)
	}
	df.fh = fh
	df.scanner = newScanner(fh)

	fi, err := fh.Stat()
	if err != nil {
		return fmt.Errorf("stat %s: %w", df.FileName, err)
	}
	df.size = fi.Size()

	return nil
}

func (df *baseDataFile) Close() {
	df.fh.Close()
	df.scanner = nil
}

func (df *baseDataFile) Offset() int64 {
	return df.offset
}

func (df *baseDataFile) Size() int64 {
	return df.size
}

func (df *baseDataFile) SkipHeader() error {
	if !df.Desc.HasHeader || df.offset != 0 {
		return nil
	}
	for df.scanner.Scan() {
		line := df.scanner.Text()
		df.offset += int64(len(line)) + 1 // Add 1 to account for '\n'.
		if strings.TrimSpace(line) == "" {
			continue
		}
		// Found the header.
		break
	}
	return df.scanner.Err()
}

const MAX_BATCH_SIZE_IN_BYTES = 200 * 1024 * 1024

func (df *baseDataFile) SkipRecords(n int) (int, bool, error) {
	byteCount := 0
	count := 0
	for count < n && byteCount < MAX_BATCH_SIZE_IN_BYTES && df.scanner.Scan() {
		line := df.scanner.Text()
		df.offset += int64(len(line)) + 1 // Add 1 to account for '\n'.
		byteCount += len(line) + 1
		if df.isDataLine(line) {
			count++
		}
	}
	eof := df.scanner.Err() == nil && count < n && byteCount < MAX_BATCH_SIZE_IN_BYTES
	return count, eof, df.scanner.Err()
}

//============================================================================
