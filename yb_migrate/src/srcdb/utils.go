package srcdb

import (
	"fmt"
	"log"
	"os"
)

func ErrExit(formatString string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, formatString+"\n", args...)
	log.Fatalf(formatString+"\n", args...)
}
