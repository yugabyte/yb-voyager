package utils

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

//call-home json formats
var (
	jsonFilePath string
	sendReq      sendRequest
	jsonBuf      []byte
)

type sendRequest struct {
	Migration_uuid uuid.UUID `json:"migration_uuid"`
	Payload        payload   `json:"payload"`
}

type payload struct {
	SourceDBType    string `json:"sourceDBType"`
	SourceDBVersion string `json:"sourceDBVersion"`
	Report          Report `json:"report"`
	TargetDBVersion string `json:"targetDBVersion"`
	NodeCount       int    `json:"nodeCount"`
}

func openOrCreateStoredJSON(exportdir string) *os.File {
	jsonFilePath = filepath.Join(exportdir, "metainfo", "diagnostics.json")

	jsonFile, err := os.OpenFile(exportdir, os.O_RDWR|os.O_APPEND|os.O_CREATE, 0644)
	if err != nil {
		ErrExit("Error while creating/opening diagnostics.json file: ", err)
	}
	return jsonFile

}

func initOrUpdateJSON(jsonFile *os.File) {
	var err error
	jsonBuf, err = os.ReadFile(jsonFilePath)

	if err != nil {
		ErrExit("Error while reading diagnostics.json file: ", err)
	}
	err = json.Unmarshal(jsonBuf, &sendReq)
	if err != nil {
		ErrExit("Invalid diagnostics.json file: ", err)
	}

	//create a new diagnostics.json if needed
	if sendReq.Migration_uuid == uuid.Nil {
		sendReq.Migration_uuid, err = uuid.NewUUID()
		if err != nil {
			ErrExit("Error while generating new UUID for diagnostics.json")
		}
	}

}

func packPayload() {

}

func sendPayload() {
	sendReq.Migration_uuid, _ = uuid.NewUUID()
	sendReq.Payload = payload{}
	postBody, _ := json.Marshal(sendReq)
	requestBody := bytes.NewBuffer(postBody)

	resp, err := http.Post("http://127.0.0.1:5000/", "application/json", requestBody)

	if err != nil {
		fmt.Printf("An Error Occured %v", err)
	}

	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err.Error())
	}
	sb := string(body)
	fmt.Println(sb)
}
