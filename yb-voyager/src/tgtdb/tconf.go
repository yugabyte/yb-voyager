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
package tgtdb

import (
	"fmt"
	"net/url"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

type TargetConf struct {
	Host                 string
	Port                 int
	User                 string
	Password             string
	DBName               string
	Schema               string
	SSLMode              string
	SSLCertPath          string
	SSLKey               string
	SSLRootCert          string
	SSLCRL               string
	SSLQueryString       string
	Uri                  string
	ContinueOnError      bool
	IgnoreIfExists       bool
	VerboseMode          bool
	TableList            string
	ExcludeTableList     string
	ImportMode           bool
	ImportObjects        string
	ExcludeImportObjects string
	dbVersion            string
}

func (t *TargetConf) Clone() *TargetConf {
	clone := *t
	return &clone
}

func (t *TargetConf) GetConnectionUri() string {
	if t.Uri == "" {
		hostAndPort := fmt.Sprintf("%s:%d", t.Host, t.Port)
		targetUrl := &url.URL{
			Scheme:   "postgresql",
			User:     url.UserPassword(t.User, t.Password),
			Host:     hostAndPort,
			Path:     t.DBName,
			RawQuery: generateSSLQueryStringIfNotExists(t),
		}

		t.Uri = targetUrl.String()
	}

	return t.Uri
}

// this function is only triggered when t.Uri==""
func generateSSLQueryStringIfNotExists(t *TargetConf) string {
	SSLQueryString := ""
	if t.SSLMode == "" {
		t.SSLMode = "prefer"
	}
	if t.SSLQueryString == "" {

		if t.SSLMode == "disable" || t.SSLMode == "allow" || t.SSLMode == "prefer" || t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {
			SSLQueryString = "sslmode=" + t.SSLMode
			if t.SSLMode == "require" || t.SSLMode == "verify-ca" || t.SSLMode == "verify-full" {

				if t.SSLCertPath != "" {
					SSLQueryString += "&sslcert=" + t.SSLCertPath
				}
				if t.SSLKey != "" {
					SSLQueryString += "&sslkey=" + t.SSLKey
				}
				if t.SSLRootCert != "" {
					SSLQueryString += "&sslrootcert=" + t.SSLRootCert
				}
				if t.SSLCRL != "" {
					SSLQueryString += "&sslcrl=" + t.SSLCRL
				}
			}
		} else {
			fmt.Println("Invalid sslmode entered")
		}
	} else {
		SSLQueryString = t.SSLQueryString
	}
	return SSLQueryString
}

func GetRedactedTargetConf(t *TargetConf) *TargetConf {
	redacted := *t
	redacted.Uri = utils.GetRedactedURLs([]string{t.Uri})[0]
	redacted.Password = "XXX"
	return &redacted
}
