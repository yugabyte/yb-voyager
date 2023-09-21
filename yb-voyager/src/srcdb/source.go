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
package srcdb

import (
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type Source struct {
	DBType                string
	Host                  string
	Port                  int
	User                  string
	Password              string
	DBName                string
	CDBName               string
	DBSid                 string
	CDBSid                string
	OracleHome            string
	TNSAlias              string
	CDBTNSAlias           string
	Schema                string
	SSLMode               string
	SSLCertPath           string
	SSLKey                string
	SSLRootCert           string
	SSLCRL                string
	SSLQueryString        string
	SSLKeyStore           string
	SSLKeyStorePassword   string
	SSLTrustStore         string
	SSLTrustStorePassword string
	Uri                   string
	NumConnections        int
	VerboseMode           utils.BoolStr
	TableList             string
	ExcludeTableList      string
	UseOrafce             utils.BoolStr
	CommentsOnObjects     utils.BoolStr

	sourceDB SourceDB
}

func (s *Source) DB() SourceDB {
	if s.sourceDB == nil {
		s.sourceDB = newSourceDB(s)
	}
	return s.sourceDB
}

func (s *Source) GetOracleHome() string {
	if s.OracleHome != "" {
		return s.OracleHome
	} else {
		return "/usr/lib/oracle/21/client64"
	}
}

func (s *Source) IsOracleCDBSetup() bool {
	return (s.CDBName != "" || s.CDBTNSAlias != "" || s.CDBSid != "")
}

func parseSSLString(source *Source) {
	if source.SSLQueryString == "" {
		return
	}

	validParams := []string{"sslmode", "sslcert", "sslrootcert", "sslkey"}

	sslParams := strings.Split(source.SSLQueryString, "&")
	for _, param := range sslParams {
		slicedparam := strings.Split(param, "=")
		for i, checkParam := range validParams {
			if checkParam == slicedparam[0] {
				switch i {
				case 0:
					source.SSLMode = slicedparam[1]
				case 1:
					source.SSLCertPath = slicedparam[1]
				case 2:
					source.SSLRootCert = slicedparam[1]
				case 3:
					source.SSLKey = slicedparam[1]
				}
				break
			}
		}
	}
}
