package srcdb

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
)

func ErrExit(formatString string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, formatString+"\n", args...)
	log.Errorf(formatString+"\n", args...)
	os.Exit(1)
}
