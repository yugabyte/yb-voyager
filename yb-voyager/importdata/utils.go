package importdata

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strconv"
)

var MAX_LINE_BUF_SIZE_MB = 64

func init() {
	var err error
	s := os.Getenv("MAX_LINE_BUF_SIZE_MB")
	if s != "" {
		MAX_LINE_BUF_SIZE_MB, err = strconv.Atoi(s)
		if err != nil {
			panic(fmt.Sprintf("could not convert value of MAX_LINE_BUF_SIZE_MB (%q) to int: %s", MAX_LINE_BUF_SIZE_MB, err))
		}
		if MAX_LINE_BUF_SIZE_MB < 1 {
			panic(fmt.Sprintf("buf size too small: %v", MAX_LINE_BUF_SIZE_MB))
		}
	}
}

func newScanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	buf := make([]byte, 0, 1*1024*1024)
	scanner.Buffer(buf, MAX_LINE_BUF_SIZE_MB*1024*1024)
	return scanner
}
