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
	"bytes"
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"text/template"

	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

//go:embed data/sample-ora2pg.conf
var Ora2pgConfigFile string

type Ora2pgConfig struct {
	OracleDSN        string
	OracleUser       string
	OracleHome       string
	OraclePWD        string
	Schema           string
	ParallelTables   string
	UseOrafce        string
	DisablePartition string
	DisableComment   string
	Allow            string
	ModifyStruct     string
}

func getDefaultOra2pgConfig(source *Source) *Ora2pgConfig {
	conf := &Ora2pgConfig{}
	conf.OracleDSN = getSourceDSN(source)
	conf.OracleUser = source.User
	conf.ParallelTables = strconv.Itoa(source.NumConnections)
	conf.OraclePWD = source.Password
	conf.DisablePartition = "0"

	conf.OracleHome = source.GetOracleHome()
	if source.Schema != "" {
		conf.Schema = source.Schema
	} else {
		conf.Schema = source.User
	}
	if source.UseOrafce {
		conf.UseOrafce = "1"
	} else {
		conf.UseOrafce = "0"
	}
	if source.CommentsOnObjects {
		conf.DisableComment = "0"
	} else {
		conf.DisableComment = "1"
	}
	return conf
}

func populateOra2pgConfigFile(configFilePath string, conf *Ora2pgConfig) {
	baseConfigFilePath := filepath.Join("/", "etc", "yb-voyager", "base-ora2pg.conf")
	if utils.FileOrFolderExists(baseConfigFilePath) {
		BaseOra2pgConfigFile, err := os.ReadFile(baseConfigFilePath)
		if err != nil {
			utils.ErrExit("Error while reading base ora2pg configuration file: %v", err)
		}
		Ora2pgConfigFile = string(BaseOra2pgConfigFile)
	}

	tmpl, err := template.New("ora2pg").Parse(Ora2pgConfigFile)
	if err != nil {
		utils.ErrExit("Error while parsing ora2pg configuration file: %v", err)
	}

	var output bytes.Buffer
	err = tmpl.Execute(&output, conf)
	if err != nil {
		utils.ErrExit("Error while preparing ora2pg configuration file: %v", err)
	}

	err = os.WriteFile(configFilePath, output.Bytes(), 0644)
	if err != nil {
		utils.ErrExit("unable to update config file %q: %v\n", configFilePath, err)
	}
}

func getSourceDSN(source *Source) string {
	var sourceDSN string

	if source.DBType == "oracle" {
		if source.DBName != "" {
			sourceDSN = fmt.Sprintf("dbi:Oracle:host=%s;service_name=%s;port=%d", source.Host, source.DBName, source.Port)
		} else if source.DBSid != "" {
			sourceDSN = fmt.Sprintf("dbi:Oracle:host=%s;sid=%s;port=%d", source.Host, source.DBSid, source.Port)
		} else {
			sourceDSN = fmt.Sprintf("dbi:Oracle:%s", source.TNSAlias) //this option is ideal for ssl connectivity, provide in documentation if needed
		}
	} else if source.DBType == "mysql" {
		parseSSLString(source)
		sourceDSN = fmt.Sprintf("dbi:mysql:host=%s;database=%s;port=%d", source.Host, source.DBName, source.Port)
		sourceDSN = extrapolateDSNfromSSLParams(source, sourceDSN)
	} else {
		utils.ErrExit("Invalid Source DB Type.")
	}

	log.Infof("Source DSN used for export: %s", sourceDSN)
	return sourceDSN
}

func extrapolateDSNfromSSLParams(source *Source, DSN string) string {
	switch source.SSLMode {
	case "disable":
		DSN += ";mysql_ssl=0;mysql_ssl_optional=0"
	case "prefer":
		DSN += ";mysql_ssl_optional=1"
	case "require":
		DSN += ";mysql_ssl=1"
	case "verify-ca":
		DSN += ";mysql_ssl=1"
		if source.SSLRootCert != "" {
			DSN += fmt.Sprintf(";mysql_ssl_ca_file=%s", source.SSLRootCert)
		} else {
			utils.ErrExit("Root authority certificate needed for verify-ca mode.")
		}
	case "verify-full":
		DSN += ";mysql_ssl=1"
		if source.SSLRootCert != "" {
			DSN += fmt.Sprintf(";mysql_ssl_ca_file=%s;mysql_ssl_verify_server_cert=1", source.SSLRootCert)
		} else {
			utils.ErrExit("Root authority certificate needed for verify-full mode.")
		}
	default:
		utils.ErrExit("Incorrect sslmode provided. Please provide a correct value for sslmode and try again.")
	}

	if source.SSLCertPath != "" {
		DSN += fmt.Sprintf(";mysql_ssl_client_cert=%s", source.SSLCertPath)
	}
	if source.SSLKey != "" {
		DSN += fmt.Sprintf(";mysql_ssl_client_key=%s", source.SSLKey)
	}

	return DSN
}
