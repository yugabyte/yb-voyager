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

package migassessment

import (
	"database/sql"
	"fmt"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

type SizingReport struct {
	NumInstances      int
	VcpuPerNode       int
	MemGBPerNode      int
	InsertConnections int
	SelectConnections int
}

type SizingParams struct {
	// add any sizing specific parameters required from user here
	// dummy params for now
	UseNvme           bool `toml:"use_nvme"`
	MultiAzDeployment bool `toml:"multi_az_deployment"`
}

type SizingTableCountRecord struct {
	Count int `json:"count,string"`
}

var baseDownloadPath = "resources/remote/"
var DB *sql.DB

func SizingAssessment() error {
	// load the assessment data
	tableCountFile := filepath.Join(AssessmentDataDir, "sizing__num-tables-count.csv")

	log.Infof("loading metadata files for sharding assessment")
	tableCount, err := loadCSVDataFile[SizingTableCountRecord](tableCountFile)
	if err != nil {
		log.Errorf("failed to load table count file: %v", err)
		return fmt.Errorf("failed to load table count file: %w", err)
	}

	utils.PrintAndLog("performing sizing assessment...")
	inputs := make(map[string]int)
	for _, tableC := range tableCount {
		log.Infof("count: %+v", tableC)
		inputs["count"] = tableC.Count
	}
	// core assessment logic goes here - maybe call some external package APIs to perform the assessment
	Run("2.20", inputs)

	FinalReport.SizingReport = &SizingReport{
		// replace with actual assessment results
		NumInstances:      3,
		VcpuPerNode:       8,
		MemGBPerNode:      16,
		InsertConnections: 10,
		SelectConnections: 12,
	}
	return nil
}

func Run(targetYbVersion string, inputs map[string]int) {
	// read required inputs: may change from version to version
	tables := inputs["tables"]
	//requiredSelectThroughput := inputs["requiredSelectThroughput"]
	//requiredInsertThroughput := inputs["requiredInsertThroughput"]

	filePath := "src/migassessment/resources/yb_" + strings.ReplaceAll(targetYbVersion, ".", "_") + "_source.db"
	if checkInternetAccess() {
		remoteFileExists := checkFileExistsOnRemoteRepo(filePath)
		if remoteFileExists {
			// print the contents of the file
			//fmt.Println(contents)
			filePath = strings.ReplaceAll(filePath, "src/migassessment/resources/", baseDownloadPath)
			fmt.Println("connect to downloaded data")
		} else {
			// check if local file exists
			isFileExist := checkLocalFileExists(filePath)
			if isFileExist {
				fmt.Println("file exist locally")
			} else {
				fmt.Println("file doesn't exist locally")
			}
		}
	} else {
		// no network access
		fmt.Println("No network access. Checking file locally...")
		// check if local file exists
		isFileExist := checkLocalFileExists(filePath)
		if isFileExist {
			fmt.Println("file exist locally")
		} else {
			fmt.Println("file doesn't exist locally")
			panic("file doesn't exist locally")
		}
	}
	err := ConnectDatabase(filePath)
	checkErr(err)
	//printRows()
	checkTableLimits(tables)
	//getThroughputData(2, requiredInsertThroughput, requiredSelectThroughput)
}

func printRows() {
	rows, err := DB.Query("SELECT * from sizing limit 10")
	if err != nil {
		fmt.Println("no records found")
	}
	defer rows.Close()
	allMaps := convertToMap(rows)
	printMap(allMaps)

	err = rows.Err()
	if err != nil {
		fmt.Println("error occurred")
	}
}

func checkFileExistsOnRemoteRepo(fileName string) bool {
	remotePath := "https://raw.githubusercontent.com/yb-voyager/sshaikh/mat/" + fileName
	resp, _ := http.Get(remotePath)

	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			panic(err)
		}
	}(resp.Body)

	if resp.StatusCode != 200 {
		fmt.Println("File does not exist on remote location")
		return false
	} else {
		//body, err := io.ReadAll(resp.Body)
		downloadPath := strings.ReplaceAll(fileName, "resources/", baseDownloadPath)
		out, err := os.Create(downloadPath)
		defer out.Close()
		_, err = io.Copy(out, resp.Body)

		if err != nil {
			panic(err)
		}
		return true
	}
}

func ConnectDatabase(file string) error {
	db, err := sql.Open("sqlite3", file)
	if err != nil {
		return err
	}
	DB = db
	return nil
}

func checkTableLimits(req_tables int) {
	rows, err := DB.Query("select num_cores from sizing where num_tables > ? and dimension like '%TableLimits-3nodeRF=3%' order by num_cores", req_tables)
	if err != nil {
		fmt.Println("no records found")
	}
	defer rows.Close()
	allMaps := convertToMap(rows)
	printMap(allMaps)

	err = rows.Err()
	if err != nil {
		fmt.Println("error occurred")
	}
}

func getThroughputData(minCoresReq int, requiredInsertThroughput int, requiredSelectThroughput int) {
	//rows, err := DB.Query("select foo.* from (select id, (cast(?/inserts_per_core) as int + ((?/inserts_per_core) > cast(?/inserts_per_core) as int)) insert_total_cores, (cast(?/selects_per_core) as int + ((?/selects_per_core) > cast(?/selects_per_core) as int) select_total_cores, num_cores, num_nodes from sizing where dimension='MaxThroughput' and num_cores>=?) as foo order by select_total_cores + insert_total_cores, num_cores", requiredInsertThroughput, requiredInsertThroughput, requiredInsertThroughput, requiredSelectThroughput, requiredSelectThroughput, requiredSelectThroughput, minCoresReq)
	rows, err := DB.Query("select foo.* from (select id, ? , ?, num_cores, num_nodes from sizing where dimension='MaxThroughput' and num_cores>=?) as foo order by select_total_cores, insert_total_cores, num_cores", requiredInsertThroughput, requiredSelectThroughput, minCoresReq)
	if err != nil {
		fmt.Println("no records found")
	}
	defer rows.Close()
	err = rows.Err()

	allMaps := convertToMap(rows)
	printMap(allMaps)
	if err != nil {
		fmt.Println("error occurred")
	}
}
