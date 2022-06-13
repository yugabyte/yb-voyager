package callhome

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

//call-home json formats
var (
	jsonFilePath    string
	Payload         payload
	jsonBuf         []byte
	SendDiagnostics bool
)

type payload struct {
	MigrationUuid         uuid.UUID `json:"UUID"`                    //done
	StartTime             string    `json:"start_time"`              //done
	LastUpdatedTime       string    `json:"last_updated_time"`       //done
	SourceDBType          string    `json:"source_db_type"`          //done
	SourceDBVersion       string    `json:"source_db_version"`       //done
	Issues                string    `json:"issues"`                  //done
	DBObjects             string    `json:"database_objects"`        //done
	TargetDBVersion       string    `json:"target_db_version"`       //done
	NodeCount             int       `json:"node_count"`              //done
	ParallelJobs          int       `json:"parallel_jobs"`           //done
	TotalRows             int64     `json:"total_rows"`              //done
	TotalSize             int64     `json:"total_size"`              //done
	LargestTableRows      int64     `json:"largest_table_rows"`      //done
	LargestTableSize      int64     `json:"largest_table_size"`      //done
	TargetClusterLocation string    `json:"target_cluster_location"` //ask on slack channels for this (answer given is insufficient)
	TargetDBCores         int       `json:"target_db_cores"`         //unknown for now
	SourceCloudDBType     string    `json:"source_cloud_type"`       //unknown for now
}

// Fill in primary-key based fields, if needed
func InitJSON(exportdir string) {

	jsonFilePath = filepath.Join(exportdir, "metainfo", "diagnostics.json")
	file, err := os.OpenFile(jsonFilePath, os.O_RDWR|os.O_CREATE, 0644)
	file.Close()
	if err != nil {
		utils.ErrExit("Error while creating/opening diagnostics.json file: %v", err)
	}
	jsonBuf, err = os.ReadFile(jsonFilePath)
	if err != nil {
		utils.ErrExit("Error while reading diagnostics.json file: %v", err)
	}

	if len(jsonBuf) != 0 {
		err = json.Unmarshal(jsonBuf, &Payload)
		if err != nil {
			utils.ErrExit("Invalid diagnostics.json file: %v", err)
		}
	}

	if Payload.MigrationUuid == uuid.Nil {
		Payload.MigrationUuid, err = uuid.NewUUID()
		Payload.StartTime = time.Now().Format("2006-01-02 15:04:05")
		if err != nil {
			utils.ErrExit("Error while generating new UUID for diagnostics.json: %v", err)
		}
	}

}

// Getter method for updating payload
func GetPayload() *payload {
	return &Payload
}

// Send http request to flask servers after saving locally
func PackAndSendPayload(exportdir string) {
	if !SendDiagnostics {
	} else {
		//Pack locally
		jsonBuf, _ = json.Marshal(Payload)
		os.WriteFile(jsonFilePath, jsonBuf, 0644)

		//Send request
		Payload.LastUpdatedTime = time.Now().Format("2006-01-02 15:04:05")
		postBody, _ := json.Marshal(Payload)
		requestBody := bytes.NewBuffer(postBody)

		log.Infof("Payload being sent for diagnostic usage: %s\n", string(postBody))
		resp, err := http.Post("http://10.150.5.149:5000/", "application/json", requestBody)

		if err != nil {
			utils.ErrExit("Error while sending diagnostic data: %v", err)
		}

		defer resp.Body.Close()

		body, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			utils.ErrExit("Error while reading HTTP response from call-home server: %v", err)
		}
		log.Infof("HTTP response after sending diagnostic.json: %s\n", string(body))
	}
}

// Find the largest and total data sizes, and upload to diagnostics json
func UpdateDataSize(exportdir string) {
	datadirfiles := filepath.Join(exportdir, "data", "*_data*")

	files, err := filepath.Glob(datadirfiles)
	if err != nil {
		utils.ErrExit("Error while matching files in data dir for diagnostics: %v", err)
	}
	var totalSize int64
	var maxFileSize int64
	for _, file := range files {
		fileInfo, err := os.Stat(file)
		if err != nil {
			utils.ErrExit("Error while querying files for size: %v", err)
		}
		if maxFileSize < fileInfo.Size() {
			maxFileSize = fileInfo.Size()
		}
		totalSize += fileInfo.Size()
	}

	Payload.TotalSize = totalSize
	Payload.LargestTableSize = maxFileSize

	PackAndSendPayload(exportdir)

}
