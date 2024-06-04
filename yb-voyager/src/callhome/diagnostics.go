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
package callhome

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

// call-home json formats
var (
	SendDiagnostics utils.BoolStr
)

var (
	CALL_HOME_SERVICE_HOST = "34.105.39.253"
	CALL_HOME_SERVICE_PORT = 80
)

/*

Call-home diagnostics table structure - 
CREATE TABLE diagnostics (
    migration_uuid UUID,
    phase_start_time TIMESTAMP WITH TIME ZONE,
    source_db_details JSONB,
    target_db_details JSONB,
    yb_voyager_version TEXT,
    migration_phase TEXT,
    phase_payload JSONB,
    migration_type TEXT,
    source_db_size BIGINT,
    time_taken BIGINT,
    status TEXT,
    PRIMARY KEY (migration_uuid, phase_start_time, migration_phase)
);

*/
type Payload struct {
	MigrationUUID    uuid.UUID `json:"migration_uuid"`
	PhaseStartTime   string    `json:"phase_start_time"`
	SourceDBDetails  string    `json:"source_db_details"`
	TargetDBDetails  string    `json:"target_db_details"`
	YBVoyagerVersion string    `json:"yb_voyager_version"`
	MigrationPhase   string    `json:"migration_phase"`
	PhasePayload     string    `json:"phase_payload"`
	MigrationType    string    `json:"migration_type"`
	TimeTaken        int64     `json:"time_taken"`
	Status           string    `json:"status"`
}

type SourceDBDetails struct {
	Host      string `json:"host"`
	DBType    string `json:"db_type"`
	DBVersion string `json:"db_version"`
	DBSize    int    `json:"total_db_size"` //TODO add

	//...more info
}

type TargetDBDetails struct {
	Host string `json:"host"`
	DBVersion string `json:"db_version"`
	NodeCount int    `json:"node_count"`
	Cores     int    `json:"cores"`

	//...more info
}

type AssessMigrationPhasePayload struct {
	UnsupportedFeatures  string `json:"unsupported_features"`
	UnsupportedDataTypes string `json:"unsupported_datatypes"`
	Error                string `json:"error,omitempty"`

	//..more info
}

type ExportSchemaPhasePayload struct {
	StartClean bool `json:"start_clean"`
	SkipRecommendations bool `json:"skip_recommendations"`
}

type AnalyzePhasePayload struct {
	Issues          string `json:"issues"`
	DatabaseObjects string `json:"database_objects"`

	//..more info
}
type ExportDataPhasePayload struct {
	ParallelJobs     int64 `json:"parallel_jobs"`
	TotalRows        int64 `json:"total_rows"`
	LargestTableRows int64 `json:"largest_table_rows"`
	StartClean       bool  `json:"start_clean"`

	//..more info
}

type ImportSchemaPhasePayload struct {
	ContinueOnError    bool     `json:"continue_on_error"`
	Errors             []string `json:"errors"`
	PostSnapshotImport bool     `json:"post_snapshot_import"`
	StartClean         bool     `json:"start_clean"`

	//..more info
}

type ImportDataPhasePayload struct {
	ParallelJobs     int64 `json:"parallel_jobs"`
	TotalRows        int64 `json:"total_rows"`
	LargestTableRows int64 `json:"largest_table_rows"`
	StartClean       bool  `json:"start_clean"`

	//..more info
}

type ImportDataFilePhasePayload struct {
	ParallelJobs     int64  `json:"parallel_jobs"`
	TotalRows        int64  `json:"total_rows"`
	TotalSize        int64  `json:"total_size"`
	LargestTableRows int64  `json:"largest_table_rows"`
	LargestTableSize int64  `json:"largest_table_size"`
	FileStorageType  string `json:"file_storage_type"`
	StartClean       bool   `json:"start_clean"`

	//..more info
}

type EndMigrationPhasePayload struct {
	BackupLogFiles       bool `json:"backup_log_files"`
	BackupDataFiles      bool `json:"backup_data_files"`
	BackupSchemaFiles    bool `json:"backup_schema_files"`
	SaveMigrationReports bool `json:"save_migration_reports"`
}

// [For development] Read ENV VARS for value of SendDiagnostics
func ReadEnvSendDiagnostics() {
	rawSendDiag := os.Getenv("YB_VOYAGER_SEND_DIAGNOSTICS")
	for _, val := range []string{"0", "no", "false"} {
		if rawSendDiag == val {
			SendDiagnostics = false
		}
	}
}

func readCallHomeServiceEnv() {
	host := os.Getenv("LOCAL_CALL_HOME_SERVICE_HOST")
	port := os.Getenv("LOCAL_CALL_HOME_SERVICE_PORT")
	if host != "" {
		CALL_HOME_SERVICE_HOST = host
	}

	if port != "" {
		portNum, err := strconv.Atoi(port)
		if err != nil {
			utils.ErrExit("call-home port is not in valid format %s: %s", port, err)
		}
		CALL_HOME_SERVICE_PORT = portNum
	}
}

// Send http request to flask servers after saving locally
func PackAndSendPayload(payload *Payload) {
	if !SendDiagnostics {
		return
	}

	//for local call-home setup
	readCallHomeServiceEnv()

	postBody, err := json.Marshal(payload)
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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Errorf("Error while reading HTTP response from call-home server: %v", err)
		return
	}
	log.Infof("HTTP response after sending diagnostic.json: %s\n", string(body))

}
