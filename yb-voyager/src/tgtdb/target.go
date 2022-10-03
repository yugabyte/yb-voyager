package tgtdb

import (
	"fmt"
	"net/url"
)

type Target struct {
	Host                   string
	Port                   int
	User                   string
	Password               string
	DBName                 string
	Schema                 string
	SSLMode                string
	SSLCertPath            string
	SSLKey                 string
	SSLRootCert            string
	SSLCRL                 string
	SSLQueryString         string
	Uri                    string
	ImportIndexesAfterData bool
	ContinueOnError        bool
	IgnoreIfExists         bool
	VerboseMode            bool
	TableList              string
	ExcludeTableList       string
	ImportMode             bool
	ImportObjects          string
	ExcludeImportObjects   string
	dbVersion              string

	db *TargetDB
}

func (t *Target) Clone() *Target {
	clone := *t
	clone.db = nil
	return &clone
}

func (t *Target) DB() *TargetDB {
	if t.db == nil {
		t.db = newTargetDB(t)
	}
	return t.db
}

func (t *Target) GetConnectionUri() string {
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

//this function is only triggered when t.Uri==""
func generateSSLQueryStringIfNotExists(t *Target) string {
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
