/*
Copyright (c) YugaByte, Inc.

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
package cmd

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/davecgh/go-spew/spew"
	"github.com/fatih/color"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/slices"
	"golang.org/x/sync/semaphore"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/tevino/abool/v2"
)

var splitFileChannelSize = SPLIT_FILE_CHANNEL_SIZE
var metaInfoDirName = META_INFO_DIR_NAME
var numLinesInASplit = int64(0)
var parallelImportJobs = 0
var Done = abool.New()
var batchImportSemaphore *semaphore.Weighted

var tablesProgressMetadata map[string]*utils.TableProgressMetadata

// stores the data files description in a struct
var dataFileDescriptor *datafile.Descriptor

type ProgressContainer struct {
	mu        sync.Mutex
	container *mpb.Progress
}

var importProgressContainer ProgressContainer
var importTables []string
var allTables []string
var usePublicIp bool
var targetEndpoints string
var copyTableFromCommands = make(map[string]string)
var loadBalancerUsed bool           // specifies whether load balancer is used in front of yb servers
var enableUpsert bool               // upsert instead of insert for import data
var disableTransactionalWrites bool // to disable transactional writes for copy command
var truncateSplits bool             // to truncate *.D splits after import

const (
	LB_WARN_MSG = "Warning: Based on internal anaylsis, --target-db-host is identified as a load balancer IP which will be used to create connections for data import.\n" +
		"\t To control the parallelism and servers used, refer to help for --parallel-jobs and --target-endpoints flags.\n"
)

type SplitFileImportTask struct {
	TableName           string
	SchemaName          string
	SplitFilePath       string
	OffsetStart         int64
	OffsetEnd           int64
	TmpConnectionString string
	SplitNumber         int64
	Interrupted         bool
}

var importDataCmd = &cobra.Command{
	Use:   "data",
	Short: "This command imports data into YugabyteDB database",
	Long:  `This command will import the data exported from the source database into YugabyteDB database.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateImportFlags(cmd)
	},

	Run: func(cmd *cobra.Command, args []string) {
		target.ImportMode = true
		sourceDBType = ExtractMetaInfo(exportDir).SourceDBType
		sqlname.SourceDBType = sourceDBType
		dataStore = datastore.NewDataStore(filepath.Join(exportDir, "data"))
		checkExportDataDoneFlag()
		importData()
	},
}

func getYBServers() []*tgtdb.Target {
	var targets []*tgtdb.Target

	if targetEndpoints != "" {
		msg := fmt.Sprintf("given yb-servers for import data: %q\n", targetEndpoints)
		utils.PrintIfTrue(msg, target.VerboseMode)
		log.Infof(msg)

		ybServers := utils.CsvStringToSlice(targetEndpoints)
		for _, ybServer := range ybServers {
			clone := target.Clone()

			if strings.Contains(ybServer, ":") {
				clone.Host = strings.Split(ybServer, ":")[0]
				var err error
				clone.Port, err = strconv.Atoi(strings.Split(ybServer, ":")[1])

				if err != nil {
					utils.ErrExit("error in parsing useYbServers flag: %v", err)
				}
			} else {
				clone.Host = ybServer
			}

			clone.Uri = getCloneConnectionUri(clone)
			log.Infof("using yb server for import data: %+v", tgtdb.GetRedactedTarget(clone))
			targets = append(targets, clone)
		}
	} else {
		loadBalancerUsed = true
		url := target.GetConnectionUri()
		conn, err := pgx.Connect(context.Background(), url)
		if err != nil {
			utils.ErrExit("Unable to connect to database: %v", err)
		}
		defer conn.Close(context.Background())

		rows, err := conn.Query(context.Background(), GET_YB_SERVERS_QUERY)
		if err != nil {
			utils.ErrExit("error in query rows from yb_servers(): %v", err)
		}
		defer rows.Close()

		var hostPorts []string
		for rows.Next() {
			clone := target.Clone()
			var host, nodeType, cloud, region, zone, public_ip string
			var port, num_conns int
			if err := rows.Scan(&host, &port, &num_conns,
				&nodeType, &cloud, &region, &zone, &public_ip); err != nil {
				utils.ErrExit("error in scanning rows of yb_servers(): %v", err)
			}

			// check if given host is one of the server in cluster
			if loadBalancerUsed {
				if isSeedTargetHost(host, public_ip) {
					loadBalancerUsed = false
				}
			}

			if usePublicIp {
				if public_ip != "" {
					clone.Host = public_ip
				} else {
					var msg string
					if host == "" {
						msg = fmt.Sprintf("public ip is not available for host: %s."+
							"Refer to help for more details for how to enable public ip.", host)
					} else {
						msg = fmt.Sprintf("public ip is not available for host: %s but private ip are available. "+
							"Either refer to help for how to enable public ip or remove --use-public-up flag and restart the import", host)
					}
					utils.ErrExit(msg)
				}
			} else {
				clone.Host = host
			}

			clone.Port = port
			clone.Uri = getCloneConnectionUri(clone)
			targets = append(targets, clone)

			hostPorts = append(hostPorts, fmt.Sprintf("%s:%v", host, port))
		}
		log.Infof("Target DB nodes: %s", strings.Join(hostPorts, ","))
	}

	if loadBalancerUsed { // if load balancer is used no need to check direct connectivity
		utils.PrintAndLog(LB_WARN_MSG)
		if parallelImportJobs == -1 {
			parallelImportJobs = 2 * len(targets)
			utils.PrintAndLog("Using %d parallel jobs by default. Use --parallel-jobs to specify a custom value", parallelImportJobs)
		}
		targets = []*tgtdb.Target{&target}
	} else {
		targets = testAndFilterYbServers(targets)
	}
	return targets
}

func fetchDefaultParllelJobs(targets []*tgtdb.Target) int {
	totalCores := 0
	targetCores := 0
	for _, target := range targets {
		log.Infof("Determining CPU core count on: %s", utils.GetRedactedURLs([]string{target.Uri})[0])
		conn, err := pgx.Connect(context.Background(), target.Uri)
		if err != nil {
			log.Warnf("Unable to reach target while querying cores: %v", err)
			return len(targets) * 2
		}
		defer conn.Close(context.Background())

		cmd := "CREATE TEMP TABLE yb_voyager_cores(num_cores int);"
		_, err = conn.Exec(context.Background(), cmd)
		if err != nil {
			log.Warnf("Unable to create tables on target DB: %v", err)
			return len(targets) * 2
		}

		cmd = "COPY yb_voyager_cores(num_cores) FROM PROGRAM 'grep processor /proc/cpuinfo|wc -l';"
		_, err = conn.Exec(context.Background(), cmd)
		if err != nil {
			log.Warnf("Error while running query %s on host %s: %v", cmd, utils.GetRedactedURLs([]string{target.Uri}), err)
			return len(targets) * 2
		}

		cmd = "SELECT num_cores FROM yb_voyager_cores;"
		if err = conn.QueryRow(context.Background(), cmd).Scan(&targetCores); err != nil {
			log.Warnf("Error while running query %s: %v", cmd, err)
			return len(targets) * 2
		}
		totalCores += targetCores
	}
	if totalCores == 0 { //if target is running on MacOS, we are unable to determine totalCores
		return 3
	}
	return totalCores / 2
}

// this function will check the reachability to each of the nodes and returns list of ones which are reachable
func testAndFilterYbServers(targets []*tgtdb.Target) []*tgtdb.Target {
	var availableTargets []*tgtdb.Target

	for _, target := range targets {
		log.Infof("testing server: %s\n", spew.Sdump(tgtdb.GetRedactedTarget(target)))
		conn, err := pgx.Connect(context.Background(), target.GetConnectionUri())
		if err != nil {
			utils.PrintAndLog("unable to use yb-server %q: %v", target.Host, err)
		} else {
			availableTargets = append(availableTargets, target)
			conn.Close(context.Background())
		}
	}

	if len(availableTargets) == 0 {
		utils.ErrExit("no yb servers available for data import")
	}
	return availableTargets
}

func isSeedTargetHost(names ...string) bool {
	var allIPs []string
	for _, name := range names {
		if name != "" {
			allIPs = append(allIPs, utils.LookupIP(name)...)
		}
	}

	seedHostIPs := utils.LookupIP(target.Host)
	for _, seedHostIP := range seedHostIPs {
		if slices.Contains(allIPs, seedHostIP) {
			log.Infof("Target.Host=%s matched with one of ips in %v\n", seedHostIP, allIPs)
			return true
		}
	}
	return false
}

func getCloneConnectionUri(clone *tgtdb.Target) string {
	var cloneConnectionUri string
	if clone.Uri == "" {
		//fallback to constructing the URI from individual parameters. If URI was not set for target, then its other necessary parameters must be non-empty (or default values)
		cloneConnectionUri = clone.GetConnectionUri()
	} else {
		targetConnectionUri, err := url.Parse(clone.Uri)
		if err == nil {
			targetConnectionUri.Host = fmt.Sprintf("%s:%d", clone.Host, clone.Port)
			cloneConnectionUri = fmt.Sprint(targetConnectionUri)
		} else {
			panic(err)
		}
	}
	return cloneConnectionUri
}

func importData() {
	utils.PrintAndLog("import of data in %q database started", target.DBName)
	err := target.DB().Connect()
	if err != nil {
		utils.ErrExit("Failed to connect to the target DB: %s", err)
	}
	target.Schema = strings.ToLower(target.Schema)
	targetDBVersion := target.DB().GetVersion()
	fmt.Printf("Target YugabyteDB version: %s\n", targetDBVersion)

	payload := callhome.GetPayload(exportDir)
	payload.TargetDBVersion = targetDBVersion
	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	targets := getYBServers()
	payload.NodeCount = len(targets)

	var targetUriList []string
	for _, t := range targets {
		targetUriList = append(targetUriList, t.Uri)
	}
	log.Infof("targetUriList: %s", utils.GetRedactedURLs(targetUriList))

	if parallelImportJobs == -1 {
		parallelImportJobs = fetchDefaultParllelJobs(targets)
		utils.PrintAndLog("Using %d parallel jobs by default. Use --parallel-jobs to specify a custom value", parallelImportJobs)
	}

	params := &tgtdb.ConnectionParams{
		NumConnections:    parallelImportJobs + 1,
		ConnUriList:       targetUriList,
		SessionInitScript: getYBSessionInitScript(),
	}
	connPool := tgtdb.NewConnectionPool(params)
	err = createVoyagerSchemaOnTarget(connPool)
	if err != nil {
		utils.ErrExit("Failed to create voyager metadata schema on target DB: %s", err)
	}
	var parallelism = parallelImportJobs
	log.Infof("parallelism=%v", parallelism)
	payload.ParallelJobs = parallelism
	if target.VerboseMode {
		fmt.Printf("Number of parallel imports jobs at a time: %d\n", parallelism)
	}

	if parallelism > SPLIT_FILE_CHANNEL_SIZE {
		splitFileChannelSize = parallelism + 1
	}
	taskQueue := make(chan *SplitFileImportTask, splitFileChannelSize)

	determineTablesToImport()
	if startClean {
		cleanImportState()
	}
	if target.VerboseMode {
		fmt.Printf("all the tables to be imported: %v\n", allTables)
		fmt.Printf("tables left to import: %v\n", importTables)
	}

	if len(importTables) == 0 {
		fmt.Printf("All the tables are already imported, nothing left to import\n")
		Done.Set()
	} else {
		fmt.Printf("Importing tables: %v\n", importTables)
		initializeImportDataStatus(exportDir, importTables)
		if !disablePb {
			go importDataStatus()
		}
		batchImportSemaphore = semaphore.NewWeighted(int64(parallelism))
		go processTaskQueue(taskQueue, parallelism, connPool)
		for _, t := range importTables {
			importTable(t, taskQueue)
		}
		for !isImportDone() {
			time.Sleep(5 * time.Second)
		}
		Done.Set()
		time.Sleep(time.Second * 2)
	}
	executePostImportDataSqls()
	callhome.PackAndSendPayload(exportDir)

	if liveMigration {
		fmt.Println("streaming changes to target DB...")
		targetSchema := target.Schema
		if sourceDBType == POSTGRESQL {
			targetSchema = ""
		}
		err = streamChanges(connPool, targetSchema)
		if err != nil {
			utils.ErrExit("Failed to stream changes from source DB: %s", err)
		}
	}
	fmt.Printf("\nexiting...\n")
}

func createVoyagerSchemaOnTarget(connPool *tgtdb.ConnectionPool) error {
	cmds := []string{
		"CREATE SCHEMA IF NOT EXISTS ybvoyager_metadata",
		`CREATE TABLE IF NOT EXISTS ybvoyager_metadata.ybvoyager_import_data_batches_metainfo (
			schema_name VARCHAR(250),
			file_name VARCHAR(250),
			rows_imported BIGINT,
			PRIMARY KEY (schema_name, file_name)
		);`,
	}

	maxAttempts := 12
	var conn *pgx.Conn
	var err error
outer:
	for _, cmd := range cmds {
		for attempt := 1; attempt <= maxAttempts; attempt++ {
			log.Infof("Executing on target: [%s]", cmd)
			if conn == nil {
				conn = newTargetConn()
			}
			_, err = conn.Exec(context.Background(), cmd)
			if err == nil {
				continue outer
			}
			log.Warnf("Error while running [%s] attempt %d: %s", cmd, attempt, err)
			time.Sleep(5 * time.Second)
			conn.Close(context.Background())
			conn = nil
		}
		if err != nil {
			return fmt.Errorf("create ybvoyager schema on target: %w", err)
		}
	}
	if conn != nil {
		conn.Close(context.Background())
	}
	return nil
}

func isImportDone() bool {
	importDone := true
	for _, table := range importTables {
		inProgressPattern := fmt.Sprintf("%s/%s/data/%s.*.P", exportDir, metaInfoDirName, table)
		m1, err := filepath.Glob(inProgressPattern)
		if err != nil {
			utils.ErrExit("glob %q: %s", inProgressPattern, err)
		}
		inCreatedPattern := fmt.Sprintf("%s/%s/data/%s.*.C", exportDir, metaInfoDirName, table)
		m2, err := filepath.Glob(inCreatedPattern)
		if err != nil {
			utils.ErrExit("glob %q: %s", inCreatedPattern, err)
		}

		if len(m1) > 0 || len(m2) > 0 {
			importDone = false
			time.Sleep(2 * time.Second)
			break
		}
	}
	if importDone {
		log.Infof("No in-progress or newly-created splits. Import Done.")
	}
	return importDone
}

func determineTablesToImport() {
	doneTables, interruptedTables, remainingTables, _ := getTablesToImport()

	log.Infof("doneTables: %s", doneTables)
	log.Infof("interruptedTables: %s", interruptedTables)
	log.Infof("remainingTables: %s", remainingTables)

	if target.TableList == "" { //no table-list is given by user
		importTables = append(interruptedTables, remainingTables...)
		allTables = append(importTables, doneTables...)
	} else {
		allTables = utils.CsvStringToSlice(target.TableList)

		//filter allTables to remove tables in case not present in --table-list flag
		for _, table := range allTables {
			//TODO: 'table' can be schema_name.table_name, so split and proceed
			notDone := true
			for _, t := range doneTables {
				if t == table {
					notDone = false
					break
				}
			}

			if notDone {
				importTables = append(importTables, table)
			}
		}
		if target.VerboseMode {
			fmt.Printf("given table-list: %v\n", target.TableList)
		}
	}

	excludeTableList := utils.CsvStringToSlice(target.ExcludeTableList)
	importTables = utils.SetDifference(importTables, excludeTableList)
	allTables = utils.SetDifference(allTables, excludeTableList)
	sort.Strings(allTables)
	sort.Strings(importTables)

	log.Infof("allTables: %s", allTables)
	log.Infof("importTables: %s", importTables)
	if !startClean {
		fmt.Printf("skipping already imported tables: %s\n", doneTables)
	}
}

func cleanImportState() {
	conn := newTargetConn()
	defer conn.Close(context.Background())
	nonEmptyTableNames := getNonEmptyTables(conn, allTables)
	if len(nonEmptyTableNames) > 0 {
		utils.ErrExit("Following tables are not empty. "+
			"TRUNCATE them before importing data with --start-clean.\n%s",
			strings.Join(nonEmptyTableNames, ", "))
	}

	for _, table := range allTables {
		tableSplitsPatternStr := fmt.Sprintf("%s.%s", table, SPLIT_INFO_PATTERN)
		filePattern := filepath.Join(exportDir, "metainfo/data", tableSplitsPatternStr)
		log.Infof("clearing the generated splits for table %q matching %q pattern", table, filePattern)
		utils.ClearMatchingFiles(filePattern)

		cmd := fmt.Sprintf(`DELETE FROM ybvoyager_metadata.ybvoyager_import_data_batches_metainfo WHERE file_name LIKE '%s.%%'`, table)
		res, err := conn.Exec(context.Background(), cmd)
		if err != nil {
			utils.ErrExit("remove %q related entries from ybvoyager_metadata.ybvoyager_import_data_batches_metainfo: %s", table, err)
		}
		log.Infof("query: [%s] => rows affected %v", cmd, res.RowsAffected())
	}

	importTables = allTables //since all tables needs to imported now
}

func getNonEmptyTables(conn *pgx.Conn, tables []string) []string {
	result := []string{}

	for _, table := range tables {
		log.Infof("Checking if table %q is empty.", table)
		tmp := false
		stmt := fmt.Sprintf("SELECT TRUE FROM %s LIMIT 1;", table)
		err := conn.QueryRow(context.Background(), stmt).Scan(&tmp)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			utils.ErrExit("failed to check whether table %q empty: %s", table, err)
		}
		result = append(result, table)
	}
	log.Infof("non empty tables: %v", result)
	return result
}

func importTable(t string, taskQueue chan *SplitFileImportTask) {

	origDataFile := exportDir + "/data/" + t + "_data.sql"
	extractCopyStmtForTable(t, origDataFile)
	log.Infof("Start splitting table %q: data-file: %q", t, origDataFile)

	log.Infof("Collect all interrupted/remaining splits.")
	largestCreatedSplitSoFar := int64(0)
	largestOffsetSoFar := int64(0)
	fileFullySplit := false
	pattern := fmt.Sprintf("%s/%s/data/%s.[0-9]*.[0-9]*.[0-9]*.[CPD]", exportDir, metaInfoDirName, t)
	matches, _ := filepath.Glob(pattern)

	doneSplitRegexStr := fmt.Sprintf(".+/%s\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.[D]$", t)
	doneSplitRegexp := regexp.MustCompile(doneSplitRegexStr)
	splitFileNameRegexStr := fmt.Sprintf(".+/%s\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.(\\d+)\\.[CPD]$", t)
	splitFileNameRegex := regexp.MustCompile(splitFileNameRegexStr)

	for _, filepath := range matches {
		submatches := splitFileNameRegex.FindAllStringSubmatch(filepath, -1)
		for _, match := range submatches {
			// fmt.Printf("filepath: %s, %v\n", filepath, match)
			/*
				offsets are 0-based, while numLines are 1-based
				offsetStart is the line in original datafile from where current split starts
				offsetEnd   is the line in original datafile from where next split starts
			*/
			splitNum, _ := strconv.ParseInt(match[1], 10, 64)
			offsetEnd, _ := strconv.ParseInt(match[2], 10, 64)
			numLines, _ := strconv.ParseInt(match[3], 10, 64)
			offsetStart := offsetEnd - numLines
			if splitNum == LAST_SPLIT_NUM {
				fileFullySplit = true
			}
			if splitNum > largestCreatedSplitSoFar {
				largestCreatedSplitSoFar = splitNum
			}
			if offsetEnd > largestOffsetSoFar {
				largestOffsetSoFar = offsetEnd
			}
			if !doneSplitRegexp.MatchString(filepath) {
				addASplitTask("", t, filepath, splitNum, offsetStart, offsetEnd, true, taskQueue)
			}
		}
	}

	if !fileFullySplit {
		splitFilesForTable(origDataFile, t, taskQueue, largestCreatedSplitSoFar, largestOffsetSoFar)
	}
}

func splitFilesForTable(filePath string, t string, taskQueue chan *SplitFileImportTask, largestSplit int64, largestOffset int64) {
	log.Infof("Split data file %q: tableName=%q, largestSplit=%v, largestOffset=%v", filePath, t, largestSplit, largestOffset)
	splitNum := largestSplit + 1
	currTmpFileName := fmt.Sprintf("%s/%s/data/%s.%d.tmp", exportDir, metaInfoDirName, t, splitNum)
	numLinesTaken := largestOffset
	numLinesInThisSplit := int64(0)

	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	reader, err := dataStore.Open(filePath)
	if err != nil {
		utils.ErrExit("preparing reader for split generation on file %q: %v", filePath, err)
	}
	dataFile, err := datafile.NewDataFile(filePath, reader, dataFileDescriptor)
	if err != nil {
		utils.ErrExit("open datafile %q: %v", filePath, err)
	}
	defer dataFile.Close()

	sz := 0
	log.Infof("current temp file: %s", currTmpFileName)
	outfile, err := os.Create(currTmpFileName)
	if err != nil {
		utils.ErrExit("create file %q: %s", currTmpFileName, err)
	}

	log.Infof("Skipping %d lines from %q", largestOffset, filePath)
	err = dataFile.SkipLines(largestOffset)
	if err != nil {
		utils.ErrExit("skipping line for offset=%d: %v", largestOffset, err)
	}

	// Create a buffered writer from the file
	bufferedWriter := bufio.NewWriter(outfile)
	if dataFileDescriptor.HasHeader {
		bufferedWriter.WriteString(dataFile.GetHeader() + "\n")
	}
	var readLineErr error = nil
	var line string
	linesWrittenToBuffer := false
	for readLineErr == nil {
		line, readLineErr = dataFile.NextLine()
		if readLineErr == nil || (readLineErr == io.EOF && line != "") {
			// handling possible case: last dataline(i.e. EOF) but no newline char at the end
			numLinesTaken += 1
			numLinesInThisSplit += 1
		}

		var length int
		if linesWrittenToBuffer {
			bufferedWriter.WriteString("\n")
			length = len("\n")
		}
		bytesWritten, err := bufferedWriter.WriteString(line)
		length += bytesWritten
		linesWrittenToBuffer = true
		if err != nil {
			utils.ErrExit("Write line to %q: %s", outfile.Name(), err)
		}
		sz += length
		if sz >= FOUR_MB {
			err = bufferedWriter.Flush()
			if err != nil {
				utils.ErrExit("flush data in file %q: %s", outfile.Name(), err)
			}
			bufferedWriter.Reset(outfile)
			sz = 0
		}

		if numLinesInThisSplit == numLinesInASplit ||
			dataFile.GetBytesRead() >= MAX_SPLIT_SIZE_BYTES ||
			readLineErr != nil {

			err = bufferedWriter.Flush()
			if err != nil {
				utils.ErrExit("flush data in file %q: %s", outfile.Name(), err)
			}
			outfile.Close()
			fileSplitNumber := splitNum
			if readLineErr == io.EOF {
				fileSplitNumber = LAST_SPLIT_NUM
				log.Infof("Preparing last split of %q", filePath)
			} else if readLineErr != nil {
				utils.ErrExit("read line from data file %q: %s", filePath, readLineErr)
			}

			offsetStart := numLinesTaken - numLinesInThisSplit
			offsetEnd := numLinesTaken
			splitFile := fmt.Sprintf("%s/%s/data/%s.%d.%d.%d.%d.C",
				exportDir, metaInfoDirName, t, fileSplitNumber, offsetEnd, numLinesInThisSplit, dataFile.GetBytesRead())
			log.Infof("Renaming %q to %q", currTmpFileName, splitFile)
			err = os.Rename(currTmpFileName, splitFile)
			if err != nil {
				utils.ErrExit("rename %q to %q: %s", currTmpFileName, splitFile, err)
			}
			dataFile.ResetBytesRead()
			addASplitTask("", t, splitFile, splitNum, offsetStart, offsetEnd, false, taskQueue)

			if fileSplitNumber != LAST_SPLIT_NUM {
				splitNum += 1
				numLinesInThisSplit = 0
				linesWrittenToBuffer = false
				currTmpFileName = fmt.Sprintf("%s/%s/data/%s.%d.tmp", exportDir, metaInfoDirName, t, splitNum)
				log.Infof("create next temp file: %q", currTmpFileName)
				outfile, err = os.Create(currTmpFileName)
				if err != nil {
					utils.ErrExit("create %q: %s", currTmpFileName, err)
				}
				bufferedWriter = bufio.NewWriter(outfile)
				if dataFileDescriptor.HasHeader {
					bufferedWriter.WriteString(dataFile.GetHeader() + "\n")
				}
			}
		}
	}
	log.Infof("splitFilesForTable: done splitting data file %q for table %q", filePath, t)
}

func addASplitTask(schemaName string, tableName string, filepath string, splitNumber int64, offsetStart int64, offsetEnd int64, interrupted bool,
	taskQueue chan *SplitFileImportTask) {
	var t SplitFileImportTask
	t.SchemaName = schemaName
	t.TableName = tableName
	t.SplitFilePath = filepath
	t.SplitNumber = splitNumber
	t.OffsetStart = offsetStart
	t.OffsetEnd = offsetEnd
	t.Interrupted = interrupted
	taskQueue <- &t
	log.Infof("Queued an import task: %s", spew.Sdump(t))
}

func executePostImportDataSqls() {
	sequenceFilePath := filepath.Join(exportDir, "data", "postdata.sql")
	if utils.FileOrFolderExists(sequenceFilePath) {
		fmt.Printf("setting resume value for sequences %10s\n", "")
		executeSqlFile(sequenceFilePath, "SEQUENCE", func(_, _ string) bool { return false })
	}
}

func getTablesToImport() ([]string, []string, []string, error) {
	metaInfoDataDir := filepath.Join(exportDir, metaInfoDirName, "data")
	exportDataDir := filepath.Join(exportDir, "data")
	_, err := os.Stat(exportDataDir)
	if err != nil {
		utils.ErrExit("Export data dir %s is missing. Exiting.\n", exportDataDir)
	}
	// Collect all the data files
	dataFilePatern := fmt.Sprintf("%s/*_data.sql", exportDataDir)
	datafiles, err := filepath.Glob(dataFilePatern)
	if err != nil {
		utils.ErrExit("find data files in %q: %s", exportDataDir, err)
	}

	pat := regexp.MustCompile(`.+/(\S+)_data.sql`)
	var tables []string
	for _, v := range datafiles {
		tablenameMatches := pat.FindAllStringSubmatch(v, -1)
		for _, match := range tablenameMatches {
			tables = append(tables, match[1]) //ora2pg data files named like TABLE_data.sql
		}
	}

	var doneTables []string
	var interruptedTables []string
	var remainingTables []string
	for _, t := range tables {

		donePattern := fmt.Sprintf("%s/%s.%s.D", metaInfoDataDir, t, SPLIT_INFO_PATTERN)
		interruptedPattern := fmt.Sprintf("%s/%s.%s.P", metaInfoDataDir, t, SPLIT_INFO_PATTERN)
		createdPattern := fmt.Sprintf("%s/%s.%s.C", metaInfoDataDir, t, SPLIT_INFO_PATTERN)
		lastSplitPattern := fmt.Sprintf("%s/%s.%s.[CPD]", metaInfoDataDir, t, LAST_SPLIT_PATTERN)

		doneMatches, _ := filepath.Glob(donePattern)
		interruptedMatches, _ := filepath.Glob(interruptedPattern)
		createdMatches, _ := filepath.Glob(createdPattern)
		lastSplitMatches, _ := filepath.Glob(lastSplitPattern)

		if len(lastSplitMatches) > 1 {
			utils.ErrExit("More than one last split is present, which is not valid scenario. last split names are: %v", lastSplitMatches)
		}

		if len(lastSplitMatches) > 0 && len(createdMatches) == 0 && len(interruptedMatches) == 0 && len(doneMatches) > 0 {
			doneTables = append(doneTables, t)
		} else {
			if (len(createdMatches) > 0 && len(interruptedMatches)+len(doneMatches) == 0) ||
				(len(createdMatches)+len(interruptedMatches)+len(doneMatches) == 0) {
				remainingTables = append(remainingTables, t)
			} else {
				interruptedTables = append(interruptedTables, t)
			}
		}
	}

	return doneTables, interruptedTables, remainingTables, nil
}

func processTaskQueue(taskQueue chan *SplitFileImportTask, parallelism int, connPool *tgtdb.ConnectionPool) {
	for {
		t := <-taskQueue
		batchImportSemaphore.Acquire(context.Background(), 1)
		go func() {
			doOneImport(t, connPool)
			batchImportSemaphore.Release(1)
		}()
	}
}

func doOneImport(task *SplitFileImportTask, connPool *tgtdb.ConnectionPool) {
	log.Infof("Importing %q", task.SplitFilePath)
	//this is done to signal start progress bar for this table
	if tablesProgressMetadata[task.TableName].CountLiveRows == -1 {
		tablesProgressMetadata[task.TableName].CountLiveRows = 0
	}
	// Rename the file to .P
	inProgressFilePath := getInProgressFilePath(task)
	log.Infof("Renaming file from %q to %q", task.SplitFilePath, inProgressFilePath)
	err := os.Rename(task.SplitFilePath, inProgressFilePath)
	if err != nil {
		utils.ErrExit("rename %q to %q: %s", task.SplitFilePath, inProgressFilePath, err)
	}
	file, err := os.Open(inProgressFilePath)
	if err != nil {
		utils.ErrExit("open %q: %s", inProgressFilePath, err)
	}

	copyCommand := getCopyCommand(task.TableName)
	// copyCommand is empty when there are no rows for that table
	if copyCommand == "" {
		markTaskDone(task)
		return
	}
	copyCommand = fmt.Sprintf(copyCommand, (task.OffsetEnd - task.OffsetStart))
	log.Infof("COPY command: %s", copyCommand)
	var rowsAffected int64
	attempt := 0
	sleepIntervalSec := 0
	copyFn := func(conn *pgx.Conn) (bool, error) {
		var err error
		attempt++
		rowsAffected, err = importSplit(conn, task, file, copyCommand)
		if err == nil ||
			utils.InsensitiveSliceContains(NonRetryCopyErrors, err.Error()) ||
			attempt == COPY_MAX_RETRY_COUNT {
			return false, err
		}
		log.Warnf("COPY FROM file %q: %s", inProgressFilePath, err)
		sleepIntervalSec += 10
		if sleepIntervalSec > MAX_SLEEP_SECOND {
			sleepIntervalSec = MAX_SLEEP_SECOND
		}
		log.Infof("sleep for %d seconds before retrying the file %s (attempt %d)",
			sleepIntervalSec, inProgressFilePath, attempt)
		time.Sleep(time.Duration(sleepIntervalSec) * time.Second)
		return true, err
	}
	err = connPool.WithConn(copyFn)
	log.Infof("%q => %d rows affected", copyCommand, rowsAffected)
	if err != nil {
		utils.ErrExit("COPY %q FROM file %q: %s", task.TableName, inProgressFilePath, err)
	}
	incrementImportProgressBar(task.TableName, inProgressFilePath)
	markTaskDone(task)
}

func importSplit(conn *pgx.Conn, task *SplitFileImportTask, file *os.File, copyCmd string) (rowsAffected int64, err error) {
	// reset the reader to begin for every call
	file.Seek(0, io.SeekStart)
	//setting the schema so that COPY command can acesss the table
	setTargetSchema(conn)

	// NOTE: DO NOT DEFINE A NEW err VARIABLE IN THIS FUNCTION. ELSE, IT WILL MASK THE err FROM RETURN LIST.
	ctx := context.Background()
	var tx pgx.Tx
	tx, err = conn.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return 0, fmt.Errorf("begin transaction: %w", err)
	}
	defer func() {
		var err2 error
		if err != nil {
			err2 = tx.Rollback(ctx)
			if err2 != nil {
				rowsAffected = 0
				err = fmt.Errorf("rollback txn: %w (while processing %s)", err2, err)
			}
		} else {
			err2 = tx.Commit(ctx)
			if err2 != nil {
				rowsAffected = 0
				err = fmt.Errorf("commit txn: %w", err2)
			}
		}
	}()

	// Check if the split is already imported.
	var alreadyImported bool
	alreadyImported, rowsAffected, err = splitIsAlreadyImported(task, tx)
	if err != nil {
		return 0, err
	}
	if alreadyImported {
		return rowsAffected, nil
	}

	// Import the split using COPY command.
	var res pgconn.CommandTag
	res, err = tx.Conn().PgConn().CopyFrom(context.Background(), file, copyCmd)
	if err != nil {
		var pgerr *pgconn.PgError
		if errors.As(err, &pgerr) {
			err = fmt.Errorf("%s, %s in %s", err.Error(), pgerr.Where, task.SplitFilePath)
		}
		return res.RowsAffected(), err
	}

	// Record an entry in ybvoyager_metadata.ybvoyager_import_data_batches_metainfo, that the split is imported.
	rowsAffected = res.RowsAffected()
	fileName := filepath.Base(getInProgressFilePath(task))
	schemaName := getTargetSchemaName(task.TableName)
	cmd := fmt.Sprintf(
		`INSERT INTO ybvoyager_metadata.ybvoyager_import_data_batches_metainfo (schema_name, file_name, rows_imported)
		VALUES ('%s', '%s', %v);`, schemaName, fileName, rowsAffected)
	_, err = tx.Exec(ctx, cmd)
	if err != nil {
		return 0, fmt.Errorf("insert into ybvoyager_metadata.ybvoyager_import_data_batches_metainfo: %w", err)
	}
	log.Infof("Inserted (%q, %q, %v) in ybvoyager_metadata.ybvoyager_import_data_batches_metainfo", schemaName, fileName, rowsAffected)
	return rowsAffected, nil
}

func splitIsAlreadyImported(task *SplitFileImportTask, tx pgx.Tx) (bool, int64, error) {
	var rowsImported int64
	fileName := filepath.Base(getInProgressFilePath(task))
	schemaName := getTargetSchemaName(task.TableName)
	query := fmt.Sprintf(
		"SELECT rows_imported FROM ybvoyager_metadata.ybvoyager_import_data_batches_metainfo WHERE schema_name = '%s' AND file_name = '%s';",
		schemaName, fileName)
	err := tx.QueryRow(context.Background(), query).Scan(&rowsImported)
	if err == nil {
		log.Infof("%v rows from %q are already imported", rowsImported, fileName)
		return true, rowsImported, nil
	}
	if err == pgx.ErrNoRows {
		log.Infof("%q is not imported yet", fileName)
		return false, 0, nil
	}
	return false, 0, fmt.Errorf("check if %s is already imported: %w", fileName, err)
}

func markTaskDone(task *SplitFileImportTask) {
	inProgressFilePath := getInProgressFilePath(task)
	doneFilePath := getDoneFilePath(task)
	log.Infof("Renaming %q => %q", inProgressFilePath, doneFilePath)
	err := os.Rename(inProgressFilePath, doneFilePath)
	if err != nil {
		utils.ErrExit("rename %q => %q: %s", inProgressFilePath, doneFilePath, err)
	}

	if truncateSplits {
		err = os.Truncate(doneFilePath, 0)
		if err != nil {
			log.Warnf("truncate file %q: %s", doneFilePath, err)
		}
	}
}

func newTargetConn() *pgx.Conn {
	conn, err := pgx.Connect(context.Background(), target.GetConnectionUri())
	if err != nil {
		utils.WaitChannel <- 1
		<-utils.WaitChannel
		utils.ErrExit("connect to target db: %s", err)
	}

	setTargetSchema(conn)
	return conn
}

func setTargetSchema(conn *pgx.Conn) {
	if sourceDBType == POSTGRESQL || target.Schema == YUGABYTEDB_DEFAULT_SCHEMA {
		// For PG, schema name is already included in the object name.
		// No need to set schema if importing in the default schema.
		return
	}
	checkSchemaExistsQuery := fmt.Sprintf("SELECT count(schema_name) FROM information_schema.schemata WHERE schema_name = '%s'", target.Schema)
	var cntSchemaName int

	if err := conn.QueryRow(context.Background(), checkSchemaExistsQuery).Scan(&cntSchemaName); err != nil {
		utils.ErrExit("run query %q on target %q to check schema exists: %s", checkSchemaExistsQuery, target.Host, err)
	} else if cntSchemaName == 0 {
		utils.ErrExit("schema '%s' does not exist in target", target.Schema)
	}

	setSchemaQuery := fmt.Sprintf("SET SCHEMA '%s'", target.Schema)
	_, err := conn.Exec(context.Background(), setSchemaQuery)
	if err != nil {
		utils.ErrExit("run query %q on target %q: %s", setSchemaQuery, target.Host, err)
	}

	if sourceDBType == ORACLE && enableOrafce {
		// append oracle schema in the search_path for orafce
		updateSearchPath := `SELECT set_config('search_path', current_setting('search_path') || ', oracle', false)`
		_, err := conn.Exec(context.Background(), updateSearchPath)
		if err != nil {
			utils.ErrExit("unable to update search_path for orafce extension: %v", err)
		}
	}
}

func dropIdx(conn *pgx.Conn, idxName string) {
	dropIdxQuery := fmt.Sprintf("DROP INDEX IF EXISTS %s", idxName)
	log.Infof("Dropping index: %q", dropIdxQuery)
	_, err := conn.Exec(context.Background(), dropIdxQuery)
	if err != nil {
		utils.ErrExit("Failed in dropping index %q: %v", idxName, err)
	}
}

func executeSqlFile(file string, objType string, skipFn func(string, string) bool) {
	log.Infof("Execute SQL file %q on target %q", file, target.Host)
	conn := newTargetConn()
	defer func() {
		if conn != nil {
			conn.Close(context.Background())
		}
	}()

	sqlInfoArr := createSqlStrInfoArray(file, objType)
	for _, sqlInfo := range sqlInfoArr {
		if conn == nil {
			conn = newTargetConn()
		}

		setOrSelectStmt := strings.HasPrefix(strings.ToUpper(sqlInfo.stmt), "SET ") ||
			strings.HasPrefix(strings.ToUpper(sqlInfo.stmt), "SELECT ")
		if !setOrSelectStmt && skipFn != nil && skipFn(objType, sqlInfo.stmt) {
			continue
		}

		err := executeSqlStmtWithRetries(&conn, sqlInfo, objType)
		if err != nil {
			conn.Close(context.Background())
			conn = nil
		}
	}
}

func getIndexName(sqlQuery string, indexName string) (string, error) {
	// Return the index name itself if it is aleady qualified with schema name
	if len(strings.Split(indexName, ".")) == 2 {
		return indexName, nil
	}

	parts := strings.FieldsFunc(sqlQuery, func(c rune) bool { return unicode.IsSpace(c) || c == '(' || c == ')' })

	for index, part := range parts {
		if strings.EqualFold(part, "ON") {
			tableName := parts[index+1]
			schemaName := getTargetSchemaName(tableName)
			return fmt.Sprintf("%s.%s", schemaName, indexName), nil
		}
	}
	return "", fmt.Errorf("could not find `ON` keyword in the CREATE INDEX statement")
}

func executeSqlStmtWithRetries(conn **pgx.Conn, sqlInfo sqlInfo, objType string) error {
	var err error
	log.Infof("On %s run query:\n%s\n", target.Host, sqlInfo.formattedStmt)
	for retryCount := 0; retryCount <= DDL_MAX_RETRY_COUNT; retryCount++ {
		if retryCount > 0 { // Not the first iteration.
			log.Infof("Sleep for 5 seconds before retrying for %dth time", retryCount)
			time.Sleep(time.Second * 5)
			log.Infof("RETRYING DDL: %q", sqlInfo.stmt)
		}
		_, err = (*conn).Exec(context.Background(), sqlInfo.formattedStmt)
		if err == nil {
			utils.PrintSqlStmtIfDDL(sqlInfo.stmt, utils.GetObjectFileName(filepath.Join(exportDir, "schema"), objType))
			return nil
		}

		log.Errorf("DDL Execution Failed for %q: %s", sqlInfo.formattedStmt, err)
		if strings.Contains(strings.ToLower(err.Error()), "conflicts with higher priority transaction") {
			// creating fresh connection
			(*conn).Close(context.Background())
			*conn = newTargetConn()
			continue
		} else if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(SCHEMA_VERSION_MISMATCH_ERR)) &&
			objType == "INDEX" || objType == "PARTITION_INDEX" { // retriable error
			// creating fresh connection
			(*conn).Close(context.Background())
			*conn = newTargetConn()

			// Extract the schema name and add to the index name
			fullyQualifiedObjName, err := getIndexName(sqlInfo.stmt, sqlInfo.objName)
			if err != nil {
				utils.ErrExit("extract qualified index name from DDL [%v]: %v", sqlInfo.stmt, err)
			}

			// DROP INDEX in case INVALID index got created
			dropIdx(*conn, fullyQualifiedObjName)
			continue
		} else if missingRequiredSchemaObject(err) {
			log.Infof("deffering execution of SQL: %s", sqlInfo.formattedStmt)
			defferedSqlStmts = append(defferedSqlStmts, sqlInfo)
		} else if isAlreadyExists(err.Error()) {
			// pg_dump generates `CREATE SCHEMA public;` in the schemas.sql. Because the `public`
			// schema already exists on the target YB db, the create schema statement fails with
			// "already exists" error. Ignore the error.
			if target.IgnoreIfExists || strings.EqualFold(strings.Trim(sqlInfo.stmt, " \n"), "CREATE SCHEMA public;") {
				err = nil
			}
		}
		break // no more iteration in case of non retriable error
	}
	if err != nil {
		if missingRequiredSchemaObject(err) {
			// Do nothing
		} else {
			utils.PrintSqlStmtIfDDL(sqlInfo.stmt, utils.GetObjectFileName(filepath.Join(exportDir, "schema"), objType))
			color.Red(fmt.Sprintf("%s\n", err.Error()))
			if target.ContinueOnError {
				log.Infof("appending stmt to failedSqlStmts list: %s\n", utils.GetSqlStmtToPrint(sqlInfo.stmt))
				errString := "/*\n" + err.Error() + "\n*/\n"
				failedSqlStmts = append(failedSqlStmts, errString+sqlInfo.formattedStmt)
			} else {
				utils.ErrExit("error: %s\n", err)
			}
		}
	}
	return err
}

func getInProgressFilePath(task *SplitFileImportTask) string {
	path := task.SplitFilePath
	return path[0:len(path)-1] + "P" // *.C -> *.P
}

func getDoneFilePath(task *SplitFileImportTask) string {
	path := task.SplitFilePath
	return path[0:len(path)-1] + "D" // *.P -> *.D
}

func getTargetSchemaName(tableName string) string {
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		return parts[0]
	}
	return target.Schema // default set to "public"
}

func incrementImportProgressBar(tableName string, splitFilePath string) {
	tablesProgressMetadata[tableName].CountLiveRows += getProgressAmount(splitFilePath)
	log.Infof("Table %q, total rows-copied/progress-made until now %v", tableName, tablesProgressMetadata[tableName].CountLiveRows)
}

func findCopyCommandForDebeziumExportedFiles(tableName, dataFilePath string) (string, error) {
	exportStatusFilePath := filepath.Join(exportDir, "data", "export_status.json")
	status, err := dbzm.ReadExportStatus(exportStatusFilePath)
	if err != nil {
		return "", err
	}
	if status == nil {
		// export_status.json is not present. This is the case of Offline migration.
		// The data is exported using pg_dump or ora2pg.
		return "", nil
	}

	dfd := datafile.OpenDescriptor(exportDir)
	reader, err := dataStore.Open(dataFilePath)
	if err != nil {
		utils.ErrExit("preparing reader to find copy command %q: %v", dataFilePath, err)
	}

	df, err := datafile.NewDataFile(dataFilePath, reader, dfd)
	if err != nil {
		utils.ErrExit("opening datafile %q to prepare copy command: %v", err)
	}
	defer df.Close()
	columnNames := quoteColumnNamesIfRequired(df.GetHeader())
	stmt := fmt.Sprintf(
		`COPY %s(%s) FROM STDIN WITH (FORMAT CSV, DELIMITER ',', HEADER, ROWS_PER_TRANSACTION %%v);`,
		tableName, columnNames)
	return stmt, nil
}

/*
Valid cases requiring column name quoting:
1. ReservedKeyWords in case of any source database type
2. CaseSensitive column names in case of PostgreSQL(Oracle and MySQL columns are exported as case-insensitive by ora2pg)
*/
func quoteColumnNamesIfRequired(csvHeader string) string {
	columnNames := strings.Split(csvHeader, ",")
	for i := 0; i < len(columnNames); i++ {
		columnNames[i] = strings.TrimSpace(columnNames[i])
		if sqlname.IsReservedKeyword(columnNames[i]) || (sourceDBType == POSTGRESQL && sqlname.IsCaseSensitive(columnNames[i], sourceDBType)) {
			columnNames[i] = fmt.Sprintf(`"%s"`, columnNames[i])
		}
	}

	return strings.Join(columnNames, ",")
}

func extractCopyStmtForTable(table string, fileToSearchIn string) {
	if getCopyCommand(table) != "" {
		return
	}
	// When snapshot is exported by Debezium, the data files are in CSV format,
	// irrespective of the source database type.
	stmt, err := findCopyCommandForDebeziumExportedFiles(table, fileToSearchIn)
	// If the export_status.json file is not present, the err will be nil and stmt will be empty.
	if err != nil {
		utils.ErrExit("could not extract copy statement for table %q: %v", table, err)
	}
	if stmt != "" {
		copyTableFromCommands[table] = stmt
		log.Infof("copyTableFromCommand for table %q is %q", table, stmt)
		return
	}
	// pg_dump and ora2pg always have columns - "COPY table (col1, col2) FROM STDIN"
	var copyCommandRegex *regexp.Regexp
	if sourceDBType == "postgresql" {
		// find the line from toc.txt file
		fileToSearchIn = exportDir + "/data/toc.txt"

		conn := newTargetConn()
		defer conn.Close(context.Background())

		parentTable := getParentTable(table, conn)
		tableInCopyRegex := table
		if parentTable != "" { // needed to find COPY command for table partitions
			tableInCopyRegex = parentTable
		}
		if len(strings.Split(tableInCopyRegex, ".")) == 1 {
			// happens only when table is in public schema, use public schema with table name for regexp
			copyCommandRegex = regexp.MustCompile(fmt.Sprintf(`(?i)COPY public.%s[\s]+\(.*\) FROM STDIN`, tableInCopyRegex))
		} else {
			copyCommandRegex = regexp.MustCompile(fmt.Sprintf(`(?i)COPY %s[\s]+\(.*\) FROM STDIN`, tableInCopyRegex))
		}
		log.Infof("parentTable=%s for table=%s", parentTable, table)
	} else if sourceDBType == "oracle" || sourceDBType == "mysql" {
		// For oracle, there is only unique COPY per datafile
		// In case of table partitions, parent table name is used in COPY
		copyCommandRegex = regexp.MustCompile(`(?i)COPY .*[\s]+\(.*\) FROM STDIN`)
	}

	file, err := os.Open(fileToSearchIn)
	if err != nil {
		utils.ErrExit("could not open file during extraction of copy stmt from file %q: %v", fileToSearchIn, err)
	}
	defer file.Close()

	reader := bufio.NewReader(file)
	for {
		line, err := utils.Readline(reader)
		if err == io.EOF { // EOF will mean NO COPY command
			return
		} else if err != nil {
			utils.ErrExit("error while readline for extraction of copy stmt from file %q: %v", fileToSearchIn, err)
		}
		if copyCommandRegex.MatchString(line) {
			line = strings.Trim(line, ";") + ` WITH (ROWS_PER_TRANSACTION %v)`
			copyTableFromCommands[table] = line
			log.Infof("copyTableFromCommand for table %q is %q", table, line)
			return
		}
	}
}

// get the parent table if its a partitioned table in yugabytedb
// also covers the case when the partitions are further subpartitioned to multiple levels
func getParentTable(table string, conn *pgx.Conn) string {
	var parentTable string
	query := fmt.Sprintf(`SELECT inhparent::pg_catalog.regclass
	FROM pg_catalog.pg_class c JOIN pg_catalog.pg_inherits ON c.oid = inhrelid
	WHERE c.oid = '%s'::regclass::oid`, table)

	var currentParentTable string
	err := conn.QueryRow(context.Background(), query).Scan(&currentParentTable)
	if err != pgx.ErrNoRows && err != nil {
		utils.ErrExit("Error in querying parent tablename for table=%s: %v", table, err)
	}

	if len(currentParentTable) == 0 {
		return ""
	} else {
		parentTable = getParentTable(currentParentTable, conn)
		if len(parentTable) == 0 {
			parentTable = currentParentTable
		}
	}

	return parentTable
}

func getCopyCommand(table string) string {
	if copyCommand, ok := copyTableFromCommands[table]; ok {
		return copyCommand
	} else {
		log.Infof("No COPY command for table %q", table)
	}
	return "" // no-op
}

func getProgressAmount(filePath string) int64 {
	splitName := filepath.Base(filePath)
	parts := strings.Split(splitName, ".")

	var p int64
	var err error
	if dataFileDescriptor.TableRowCount != nil { // case of importData where row counts is available
		p, err = strconv.ParseInt(parts[len(parts)-3], 10, 64)
	} else { // case of importDataFileCommand where file size is available not row counts
		p, err = strconv.ParseInt(parts[len(parts)-2], 10, 64)
	}

	if err != nil {
		utils.ErrExit("parsing progress amount of file %q: %v", filePath, err)
	}

	log.Debugf("got progress amount=%d for file %q", p, filePath)
	return p
}

func getYBSessionInitScript() []string {
	var sessionVars []string
	if checkSessionVariableSupport(SET_CLIENT_ENCODING_TO_UTF8) {
		sessionVars = append(sessionVars, SET_CLIENT_ENCODING_TO_UTF8)
	}
	if checkSessionVariableSupport(SET_SESSION_REPLICATE_ROLE_TO_REPLICA) {
		sessionVars = append(sessionVars, SET_SESSION_REPLICATE_ROLE_TO_REPLICA)
	}

	if enableUpsert {
		// upsert_mode parameters was introduced later than yb_disable_transactional writes in yb releases
		// hence if upsert_mode is supported then its safe to assume yb_disable_transactional_writes is already there
		if checkSessionVariableSupport(SET_YB_ENABLE_UPSERT_MODE) {
			sessionVars = append(sessionVars, SET_YB_ENABLE_UPSERT_MODE)
			// 	SET_YB_DISABLE_TRANSACTIONAL_WRITES is used only with & if upsert_mode is supported
			if disableTransactionalWrites {
				if checkSessionVariableSupport(SET_YB_DISABLE_TRANSACTIONAL_WRITES) {
					sessionVars = append(sessionVars, SET_YB_DISABLE_TRANSACTIONAL_WRITES)
				} else {
					disableTransactionalWrites = false
				}
			}
		} else {
			log.Infof("Falling back to transactional inserts of batches during data import")
		}
	}

	sessionVarsPath := "/etc/yb-voyager/ybSessionVariables.sql"
	if !utils.FileOrFolderExists(sessionVarsPath) {
		log.Infof("YBSessionInitScript: %v\n", sessionVars)
		return sessionVars
	}

	varsFile, err := os.Open(sessionVarsPath)
	if err != nil {
		utils.PrintAndLog("Unable to open %s : %v. Using default values.", sessionVarsPath, err)
		log.Infof("YBSessionInitScript: %v\n", sessionVars)
		return sessionVars
	}
	defer varsFile.Close()
	fileScanner := bufio.NewScanner(varsFile)

	var curLine string
	for fileScanner.Scan() {
		curLine = strings.TrimSpace(fileScanner.Text())
		if curLine != "" && checkSessionVariableSupport(curLine) {
			sessionVars = append(sessionVars, curLine)
		}
	}
	log.Infof("YBSessionInitScript: %v\n", sessionVars)
	return sessionVars
}

func checkSessionVariableSupport(sqlStmt string) bool {
	conn, err := pgx.Connect(context.Background(), target.GetConnectionUri())
	if err != nil {
		utils.ErrExit("error while creating connection for checking session parameter(%q) support: %v", sqlStmt, err)
	}
	defer conn.Close(context.Background())

	_, err = conn.Exec(context.Background(), sqlStmt)
	if err != nil {
		if !strings.Contains(err.Error(), "unrecognized configuration parameter") {
			utils.ErrExit("error while executing sqlStatement=%q: %v", sqlStmt, err)
		} else {
			log.Warnf("Warning: %q is not supported: %v", sqlStmt, err)
		}
	}

	return err == nil
}

func checkExportDataDoneFlag() {
	metaInfoDir := fmt.Sprintf("%s/%s", exportDir, metaInfoDirName)
	_, err := os.Stat(metaInfoDir)
	if err != nil {
		utils.ErrExit("metainfo dir is missing. Exiting.")
	}
	metaInfoDataDir := fmt.Sprintf("%s/data", metaInfoDir)
	_, err = os.Stat(metaInfoDataDir)
	if err != nil {
		utils.ErrExit("metainfo data dir is missing. Exiting.")
	}

	exportDataDonePath := metaInfoDir + "/flags/exportDataDone"
	_, err = os.Stat(exportDataDonePath)
	if err != nil {
		utils.ErrExit("Export Data is not complete yet. Exiting.")
	}
}

func init() {
	importCmd.AddCommand(importDataCmd)
	registerCommonGlobalFlags(importDataCmd)
	registerCommonImportFlags(importDataCmd)
	registerImportDataFlags(importDataCmd)
}
