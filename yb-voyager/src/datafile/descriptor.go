package datafile

import (
	"encoding/json"
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

const (
	DESCRIPTOR_PATH = "/metainfo/dataFileDescriptor.json"
)

type Descriptor struct {
	FileFormat    string           `json:"FileFormat"`
	TableRowCount map[string]int64 `json:"TableRowCount"`
	TableFileSize map[string]int64 `json:"TableFileSize"`
	Delimiter     string           `json:"Delimiter"`
	HasHeader     bool             `json:"HasHeader"`
	ExportDir     string           `json:"-"`
	QuoteChar     byte             `json:"QuoteChar,omitempty"`
	EscapeChar    byte             `json:"EscapeChar,omitempty"`
}

func OpenDescriptor(exportDir string) *Descriptor {
	dfd := &Descriptor{
		ExportDir: exportDir,
	}

	filePath := exportDir + DESCRIPTOR_PATH
	log.Infof("loading DataFileDescriptor from %q", filePath)
	dfdJson, err := os.ReadFile(filePath)
	if err != nil {
		panic(fmt.Sprintf("Could not read file %q: %v", filePath, err))
		utils.ErrExit("load data descriptor file: %v", err)
	}

	err = json.Unmarshal(dfdJson, &dfd)
	if err != nil {
		utils.ErrExit("unmarshal dfd: %v", err)
	}
	log.Infof("Parsed DataFileDescriptor: %+v", dfd)
	return dfd
}

func (dfd *Descriptor) Save() {
	filePath := dfd.ExportDir + DESCRIPTOR_PATH
	log.Infof("storing DataFileDescriptor at %q", filePath)

	bytes, err := json.MarshalIndent(dfd, "", "\t")
	if err != nil {
		utils.ErrExit("marshalling the dfd struct: %v", err)
	}

	err = os.WriteFile(filePath, bytes, 0644)
	if err != nil {
		fmt.Printf("%+v\n", dfd)
		utils.ErrExit("writing DataFileDescriptor: %v", err)
	}
}
