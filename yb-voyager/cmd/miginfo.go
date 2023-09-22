/*
Copyright (c) YugabyteDB, Inc.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

	http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/
package cmd

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/metadb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func SaveMigInfo(miginfo *metadb.MigInfo) error {
	log.Infof("saving miginfo(%+v) in metadb", miginfo)
	err := metaDB.UpdateMigrationStatusRecord(func(record *metadb.MigrationStatusRecord) {
		record.MigInfo = miginfo
	})
	if err != nil {
		return fmt.Errorf("update miginfo in migration status record: %w", err)
	}
	return nil
}

func LoadMigInfo() (*metadb.MigInfo, error) {
	msr, err := metaDB.GetMigrationStatusRecord()
	if err != nil {
		log.Errorf("loading miginfo: %v", err)
		return nil, fmt.Errorf("loading miginfo: %w", err)
	}

	if msr == nil {
		panic("migration status record's miginfo field is nil")
	}

	msr.MigInfo.ExportDir = exportDir
	log.Infof("loaded miginfo from metadb: %+v", msr.MigInfo)
	return msr.MigInfo, nil
}

// source db type
func GetSourceDBTypeFromMigInfo() string {
	miginfo, err := LoadMigInfo()
	if err != nil {
		utils.ErrExit("get source db type: loading miginfo: %v", err)
	}
	return miginfo.SourceDBType
}
