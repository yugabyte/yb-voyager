package callhome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

//call-home json formats
var (
	jsonFilePath    string
	Payload         payload
	jsonBuf         []byte
	SendDiagnostics bool
)

const (
	CALL_HOME_SERVICE_HOST = "34.83.149.226"
	CALL_HOME_SERVICE_PORT = 80
)

type payload struct {
	MigrationUuid         uuid.UUID `json:"UUID"`
	StartTime             string    `json:"start_time"`
	LastUpdatedTime       string    `json:"last_updated_time"`
	SourceDBType          string    `json:"source_db_type"`
	SourceDBVersion       string    `json:"source_db_version"`
	Issues                string    `json:"issues"`
	DBObjects             string    `json:"database_objects"`
	TargetDBVersion       string    `json:"target_db_version"`
	NodeCount             int       `json:"node_count"`
	ParallelJobs          int       `json:"parallel_jobs"`
	TotalRows             int64     `json:"total_rows"`
	TotalSize             int64     `json:"total_size"`
	LargestTableRows      int64     `json:"largest_table_rows"`
	LargestTableSize      int64     `json:"largest_table_size"`
	TargetClusterLocation string    `json:"target_cluster_location"` //TODO
	TargetDBCores         int       `json:"target_db_cores"`         //TODO
	SourceCloudDBType     string    `json:"source_cloud_type"`       //TODO
}

// Fill in primary-key based fields, if needed
func initJSON(exportdir string) {

	jsonFilePath = filepath.Join(exportdir, "metainfo", "diagnostics.json")
	file, err := os.OpenFile(jsonFilePath, os.O_RDWR|os.O_CREATE, 0644)
	file.Close()
	if err != nil {
		log.Errorf("Error while creating/opening diagnostics.json file: %v", err)
		return
	}
	jsonBuf, err = os.ReadFile(jsonFilePath)
	if err != nil {
		log.Errorf("Error while reading diagnostics.json file: %v", err)
		return
	}

	if len(jsonBuf) != 0 {
		err = json.Unmarshal(jsonBuf, &Payload)
		if err != nil {
			log.Errorf("Invalid diagnostics.json file: %v", err)
			return
		}
	}

	if Payload.MigrationUuid == uuid.Nil {
		Payload.MigrationUuid, err = uuid.NewUUID()
		Payload.StartTime = time.Now().Format("2006-01-02 15:04:05")
		if err != nil {
			log.Errorf("Error while generating new UUID for diagnostics.json: %v", err)
			return
		}
	}

}

// Getter method for updating payload
func GetPayload(exportDir string) *payload {
	//if json isn't already initialized...
	if Payload.MigrationUuid == uuid.Nil {
		initJSON(exportDir)
	}
	return &Payload
}

// Send http request to flask servers after saving locally
func PackAndSendPayload(exportdir string) {
	if !SendDiagnostics {
		return
	}
	//Pack locally
	var err error
	jsonBuf, err = json.Marshal(Payload)
	if err != nil {
		log.Errorf("Error while packing diagnostics json: %v", err)
		return
	}
	os.WriteFile(jsonFilePath, jsonBuf, 0644)

	//Send request
	Payload.LastUpdatedTime = time.Now().Format("2006-01-02 15:04:05")
	postBody, err := json.Marshal(Payload)
	if err != nil {
		log.Errorf("Error while creating http request for diagnostics: %v", err)
		return
	}
	requestBody := bytes.NewBuffer(postBody)

	log.Infof("Payload being sent for diagnostic usage: %s\n", string(postBody))
	callhomeURL := fmt.Sprintf("http://%s:%d/", CALL_HOME_SERVICE_HOST, CALL_HOME_SERVICE_PORT)
	resp, err := http.Post(callhomeURL, "application/json", requestBody)

	if err != nil {
		log.Errorf("Error while sending diagnostic data: %v", err)
		return
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error while reading HTTP response from call-home server: %v", err)
		return
	}
	log.Infof("HTTP response after sending diagnostic.json: %s\n", string(body))

}

// Find the largest and total data sizes, and upload to diagnostics json
func UpdateDataSize(exportdir string) {
	datadirfiles := filepath.Join(exportdir, "data", "*_data*")

	files, err := filepath.Glob(datadirfiles)
	if err != nil {
		log.Errorf("Error while matching files in data dir for diagnostics: %v", err)
		return
	}
	var totalSize int64
	var maxFileSize int64
	for _, file := range files {
		fileInfo, err := os.Stat(file)
		if err != nil {
			log.Errorf("Error while querying files for size: %v", err)
			return
		}
		if maxFileSize < fileInfo.Size() {
			maxFileSize = fileInfo.Size()
		}
		totalSize += fileInfo.Size()
	}

	Payload.TotalSize = totalSize
	Payload.LargestTableSize = maxFileSize
}
