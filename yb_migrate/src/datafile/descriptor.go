package datafile

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-db-migration/yb_migrate/src/utils"
)

const (
	DESCRIPTOR_PATH = "/metainfo/dataFileDescriptor.json"
)

type Descriptor struct {
	FileType      string           `json:"FileType"`
	TableRowCount map[string]int64 `json:"TableRowCount"`
	Delimiter     string           `json:"Delimiter"`
	HasHeader     bool             `json:"HasHeader"`
	ExportDir     string           `json:"-"`
}

func OpenDescriptor(exportDir string) *Descriptor {
	dfd := &Descriptor{
		ExportDir: exportDir,
	}

	filePath := exportDir + DESCRIPTOR_PATH
	log.Infof("loading DataFileDescriptor from %q", filePath)
	dfdJson, err := ioutil.ReadFile(filePath)
	if err != nil {
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

	err = ioutil.WriteFile(filePath, bytes, 0644)
	if err != nil {
		fmt.Printf("%+v\n", dfd)
		utils.ErrExit("writing DataFileDescriptor: %v", err)
	}
}
