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
	"gopkg.in/ini.v1"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type PgDumpArgs struct {
	Schema             string
	SchemaTempFilePath string
	ExtensionPattern   string
	TablesListPattern  string
	DataDirPath        string
	DataFormat         string
	ParallelJobs       string
	NoComments         string

	// default values from template file will be taken
	SchemaOnly           string
	NoOwner              string
	NoPrivileges         string
	NoTablespaces        string
	LoadViaPartitionRoot string
	DataOnly             string
	NoBlobs              string
}

var pgDumpArgs PgDumpArgs

//go:embed data/pg_dump-args.ini
var pgDumpArgsFile string

func getPgDumpArgsFromFile(sectionToRead string) string {
	var basePgDumpArgsFilePath string
	homebrewVoyagerDir := fmt.Sprintf("yb-voyager@%s", utils.YB_VOYAGER_VERSION)
	possiblePaths := []string{
		filepath.Join("/", "etc", "yb-voyager", "pg_dump-args.ini"),
		filepath.Join("/", "opt", "homebrew", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, "etc", "yb-voyager", "pg_dump-args.ini"),
		filepath.Join("/", "usr", "local", "Cellar", homebrewVoyagerDir, utils.YB_VOYAGER_VERSION, "etc", "yb-voyager", "pg_dump-args.ini"),
	}

	for _, path := range possiblePaths {
		if utils.FileOrFolderExists(path) {
			basePgDumpArgsFilePath = path
			break
		}
	}

	if basePgDumpArgsFilePath == "" {
		log.Infof("Using embedded pg_dump arguments file")
	} else {
		log.Infof("Using base pg_dump arguments file: %s", basePgDumpArgsFilePath)
		basePgDumpArgsFile, err := os.ReadFile(basePgDumpArgsFilePath)
		if err != nil {
			utils.ErrExit("Error while reading pg_dump arguments file: %w", err)
		}
		pgDumpArgsFile = string(basePgDumpArgsFile)
	}

	tmpl, err := template.New("pg_dump_args").Parse(string(pgDumpArgsFile))
	if err != nil {
		utils.ErrExit("Error while parsing pg_dump arguments: %w", err)
	}

	var output bytes.Buffer
	err = tmpl.Execute(&output, pgDumpArgs)
	if err != nil {
		utils.ErrExit("Error while preparing pg_dump arguments: %w", err)
	}

	iniData, err := ini.LoadSources(ini.LoadOptions{PreserveSurroundedQuote: true}, output.Bytes())
	if err != nil {
		utils.ErrExit("Error while ini loading pg_dump arguments file: %w", err)
	}
	section := iniData.Section(sectionToRead)
	var args strings.Builder
	for _, key := range section.Keys() {
		if key.Value() == "false" {
			continue
		}
		arg := fmt.Sprintf(` --%s`, key.Name()) // long option
		if len(key.Name()) == 1 {               // short option
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
