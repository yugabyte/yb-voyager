package cmd

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

type MigInfo struct {
	SourceDBType    string
	SourceDBName    string
	SourceDBSchema  string
	SourceDBVersion string
	SourceDBSid     string
	SourceTNSAlias  string
	exportDir       string
}

func SaveMigInfo(miginfo *MigInfo) error {

	file, err := json.MarshalIndent(miginfo, "", " ")
	if err != nil {
		return fmt.Errorf("marshal miginfo: %w", err)
	}

	migInfoFilePath := filepath.Join(miginfo.exportDir, META_INFO_DIR_NAME, "miginfo.json")

	err = ioutil.WriteFile(migInfoFilePath, file, 0644)
	if err != nil {
		return fmt.Errorf("write to %q: %w", migInfoFilePath, err)
	}

	return nil
}

func LoadMigInfo(exportDir string) (*MigInfo, error) {

	migInfo := &MigInfo{}

	migInfoFilePath := filepath.Join(exportDir, META_INFO_DIR_NAME, "miginfo.json")

	log.Infof("loading db meta info from %q", migInfoFilePath)

	migInfoJson, err := ioutil.ReadFile(migInfoFilePath)
	if err != nil {
		return nil, fmt.Errorf("read %q: %w", migInfoFilePath, err)
	}

	err = json.Unmarshal(migInfoJson, &migInfo)
	if err != nil {
		return nil, fmt.Errorf("unmarshal miginfo: %w", err)
	}

	migInfo.exportDir = exportDir
	log.Infof("parsed source db meta info: %+v", migInfo)
	return migInfo, nil
}
