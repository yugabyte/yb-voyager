package utils

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

//call-home json formats
var (
	jsonFilePath string
	sendReq      sendRequest
	jsonBuf      []byte
)

type sendRequest struct {
	MigrationUuid         uuid.UUID  `json:"UUID"`                    //done
	StartTime             string     `json:"start_time"`              //done
	LastUpdatedTime       string     `json:"last_updated_time"`       //done
	SourceDBType          string     `json:"source_db_type"`          //done
	SourceDBVersion       string     `json:"source_db_version"`       //done
	Issues                []Issue    `json:"issues"`                  //test
	DBObjects             []DBObject `json:"database_objects"`        //test
	TargetDBVersion       string     `json:"target_db_version"`       //done
	NodeCount             int        `json:"node_count"`              //done
	ParallelJobs          int        `json:"parallel_jobs"`           //done
	TotalRows             int64      `json:"total_rows"`              //done
	TotalSize             int64      `json:"total_size"`              //test
	LargestTableRows      int64      `json:"largest_table_rows"`      //done
	LargestTableSize      int64      `json:"largest_table_size"`      //test
	TargetClusterLocation string     `json:"target_cluster_location"` //ask on slack channels for this (answer given is insufficient)
	TargetDBCores         int        `json:"target_db_cores"`         //unknown for now
	SourceCloudDBType     string     `json:"source_cloud_type"`       //unknown for now
}

//Fill in primary-key based fields, if needed
func InitJSON(exportdir string) {
	var err error
	jsonFilePath = filepath.Join(exportdir, "metainfo", "diagnostics.json")
	_, err = os.OpenFile(jsonFilePath, os.O_RDWR|os.O_CREATE, 0644)
	if err != nil {
		ErrExit("Error while creating/opening diagnostics.json file: ", err)
	}
	jsonBuf, err = os.ReadFile(jsonFilePath)
	if err != nil {
		ErrExit("Error while reading diagnostics.json file: ", err)
	}

	if len(jsonBuf) != 0 {
		err = json.Unmarshal(jsonBuf, &sendReq)
		if err != nil {
			ErrExit("Invalid diagnostics.json file: ", err)
		}
	}

	if sendReq.MigrationUuid == uuid.Nil {
		sendReq.MigrationUuid, err = uuid.NewUUID()
		sendReq.StartTime = time.Now().Format("2006-01-02 15:04:05")
		if err != nil {
			ErrExit("Error while generating new UUID for diagnostics.json")
		}
	}

}

//Getter method for updating payload
func GetPayload() *sendRequest {
	return &sendReq
}

//Save payload to diagnostics.json
func PackPayload(exportdir string) {
	jsonBuf, _ = json.Marshal(sendReq)
	os.WriteFile(jsonFilePath, jsonBuf, 0644)

}

//Send http request to flask servers
func SendPayload() {
	sendReq.LastUpdatedTime = time.Now().Format("2006-01-02 15:04:05")
	postBody, _ := json.Marshal(sendReq)
	requestBody := bytes.NewBuffer(postBody)

	resp, err := http.Post("http://127.0.0.1:5000/", "application/json", requestBody)

	if err != nil {
		ErrExit("Error while sending diagnostic data: ", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		ErrExit("Error while reading HTTP response from call-home server: ", err)
	}
	log.Infof("HTTP response after sending diagnostic.json: ", string(body))

}

func UpdateDataSize(exportdir string) {
	datadir := filepath.Join(exportdir, "data")
	totalSizeCmd := exec.Command("du", "-s", datadir)
	stdout, err := totalSizeCmd.CombinedOutput()
	if err != nil {
		ErrExit("Error while executing command: ", totalSizeCmd, err)
	}
	totalSize := strings.Split(string(stdout), "\t")[0]
	// largestSizeCmd := exec.Command("du", "-a", datadir, "|", "sort", "-n", "-r", "|", "head", "-n", "2", "|", "tail", "-n", "1")
	// stdout, err = largestSizeCmd.CombinedOutput()
	// if err != nil {
	// 	ErrExit("Error while executing command: ", largestSizeCmd, err)
	// }
	// largestSize := strings.Split(string(stdout), "\t")[0]

	sendReq.TotalSize, _ = strconv.ParseInt(totalSize, 10, 64)
	//sendReq.LargestTableSize, _ = strconv.ParseInt(largestSize, 10, 64)

	PackPayload(exportdir)
	SendPayload()

}
