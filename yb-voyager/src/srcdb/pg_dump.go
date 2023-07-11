package srcdb

import (
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"gopkg.in/ini.v1"
)

var pgDumpArgs PgDumpArgs

//go:embed data/pg_dump-args.ini
var pgDumpArgsFile string

func getPgDumpArgsFromFile(sectionToRead string) string {
	basePgDumpArgsFilePath := filepath.Join("/", "etc", "yb-voyager", "pg_dump-args.ini")
	if utils.FileOrFolderExists(basePgDumpArgsFilePath) {
		log.Infof("Using base pg_dump arguments file: %s", basePgDumpArgsFilePath)
		basePgDumpArgsFile, err := os.ReadFile(basePgDumpArgsFilePath)
		if err != nil {
			utils.ErrExit("Error while reading pg_dump arguments file: %v", err)
		}
		pgDumpArgsFile = string(basePgDumpArgsFile)
	}

	tmpl, err := template.New("pg_dump_args").Parse(string(pgDumpArgsFile))
	if err != nil {
		utils.ErrExit("Error while parsing pg_dump arguments: %v", err)
	}

	var output bytes.Buffer
	err = tmpl.Execute(&output, pgDumpArgs)
	if err != nil {
		utils.ErrExit("Error while preparing pg_dump arguments: %v", err)
	}

	iniData, err := ini.Load(output.Bytes())
	if err != nil {
		utils.ErrExit("Error while ini loading pg_dump arguments file: %v", err)
	}
	section := iniData.Section(sectionToRead)
	var args strings.Builder
	for _, key := range section.Keys() {
		if key.Value() == "false" {
			continue
		}
		arg := fmt.Sprintf(` --%s`, key.Name())
		if len(key.Name()) == 1 {
			arg = fmt.Sprintf(` -%s`, key.Name())
		}
		if key.Value() == "true" {
			// no value to specify
		} else if key.Name() == "schema" {
			// value is comma separated schema names which need to be quoted
			arg += fmt.Sprintf(`="%s"`, key.Value())
		} else {
			arg += fmt.Sprintf(`=%s`, key.Value())
		}

		args.WriteString(arg)
	}
	return args.String()
}
