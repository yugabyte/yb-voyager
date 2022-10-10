package cmd

import (
	"encoding/json"
	"io/ioutil"
	"path/filepath"

	log "github.com/sirupsen/logrus"
)

type MigInfo struct {
	SourceDBType    string
	SourceDBName    string
	SourceDBSchema  string
	SourceDBVersion string
	ExportDir       string
}

func SaveMigInfo(miginfo *MigInfo) error {

	file, err := json.MarshalIndent(miginfo, "", " ")
	if err != nil {
		// unable to parse meta info data into json
		return err
	}

	migInfoFilePath := filepath.Join(miginfo.ExportDir, META_INFO_DIR_NAME, "miginfo.json")

	err = ioutil.WriteFile(migInfoFilePath, file, 0644)
	if err != nil {
		// unable to write miginfo.json file
		return err
	}

	return nil
}

func LoadMigInfo(exportDir string) (*MigInfo, error) {

	metaInfo := &MigInfo{}

	migInfoFilePath := filepath.Join(exportDir, META_INFO_DIR_NAME, "miginfo.json")

	log.Infof("loading db meta info from %q", migInfoFilePath)

	metaInfoJson, err := ioutil.ReadFile(migInfoFilePath)
	if err != nil {
		// unable to load miginfo.json file
		return nil, err
	}

	err = json.Unmarshal(metaInfoJson, &metaInfo)
	if err != nil {
		// unable to unmarshal miginfo json
		return nil, err
	}

	log.Infof("parsed source db meta info: %+v", metaInfo)
	return metaInfo, nil
}
