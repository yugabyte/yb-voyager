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
	"fmt"
	"io"
	"math"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"golang.org/x/exp/slices"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"

	"github.com/jackc/pgx/v4"
	"github.com/tevino/abool/v2"
)

var splitFileChannelSize = SPLIT_FILE_CHANNEL_SIZE
var metaInfoDir = META_INFO_DIR_NAME
var numLinesInASplit = int64(0)
var parallelImportJobs = 0
var Done = abool.New()
var GenerateSplitsDone = abool.New()

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
		validateImportFlags()
	},

	Run: func(cmd *cobra.Command, args []string) {
		target.ImportMode = true
		sourceDBType = ExtractMetaInfo(exportDir).SourceDBType
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
			log.Infof("using yb server for import data: %+v", clone)
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

	if parallelImportJobs == -1 {
		parallelImportJobs = len(targets)
	}

	if loadBalancerUsed { // if load balancer is used no need to check direct connectivity
		utils.PrintAndLog(LB_WARN_MSG)
		targets = []*tgtdb.Target{&target}
	} else {
		testYbServers(targets)
	}
	return targets
}

func testYbServers(targets []*tgtdb.Target) {
	if len(targets) == 0 {
		utils.ErrExit("no yb servers available/given for data import")
	}
	for _, target := range targets {
		log.Infof("testing server: %s\n", spew.Sdump(target))
		conn, err := pgx.Connect(context.Background(), target.GetConnectionUri())
		if err != nil {
			utils.ErrExit("error while testing yb servers: %v", err)
		}
		conn.Close(context.Background())
	}
	log.Infof("all target servers are accessible")
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
	log.Infof("targetUriList: %s", targetUriList)
	params := &tgtdb.ConnectionParams{
		NumConnections:    parallelImportJobs + 1,
		ConnUriList:       targetUriList,
		SessionInitScript: getYBSessionInitScript(),
	}
	connPool := tgtdb.NewConnectionPool(params)

	var parallelism = parallelImportJobs
	log.Infof("parallelism=%v", parallelism)
	payload.ParallelJobs = parallelism
	if target.VerboseMode {
		fmt.Printf("Number of parallel imports jobs at a time: %d\n", parallelism)
	}

	if parallelism > SPLIT_FILE_CHANNEL_SIZE {
		splitFileChannelSize = parallelism + 1
	}
	splitFilesChannel := make(chan *SplitFileImportTask, splitFileChannelSize)

	generateSmallerSplits(splitFilesChannel)
	go doImport(splitFilesChannel, parallelism, connPool)
	checkForDone()
	time.Sleep(time.Second * 2)
	executePostImportDataSqls()
	callhome.PackAndSendPayload(exportDir)
	fmt.Printf("\nexiting...\n")
}

func checkForDone() {
	for Done.IsNotSet() {
		if GenerateSplitsDone.IsSet() {
			checkPassed := true
			for _, table := range importTables {
				inProgressPattern := fmt.Sprintf("%s/%s/data/%s.*.P", exportDir, metaInfoDir, table)
				m1, err := filepath.Glob(inProgressPattern)
				if err != nil {
					utils.ErrExit("glob %q: %s", inProgressPattern, err)
				}
				inCreatedPattern := fmt.Sprintf("%s/%s/data/%s.*.C", exportDir, metaInfoDir, table)
				m2, err := filepath.Glob(inCreatedPattern)
				if err != nil {
					utils.ErrExit("glob %q: %s", inCreatedPattern, err)
				}

				if len(m1) > 0 || len(m2) > 0 {
					checkPassed = false
					time.Sleep(2 * time.Second)
					break
				}
			}
			if checkPassed {
				// once above loop is executed for each table, import is done
				log.Infof("No in-progress or newly-created splits. Import Done.")
				Done.Set()
			}
		} else {
			time.Sleep(5 * time.Second)
		}
	}
}

func generateSmallerSplits(taskQueue chan *SplitFileImportTask) {
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

	if startClean {
		nonEmptyTableNames := getNonEmptyTables(allTables)
		if len(nonEmptyTableNames) > 0 {
			utils.ErrExit("Following tables still has rows. "+
				"TRUNCATE them before importing data with --start-clean.\n%s",
				nonEmptyTableNames)
		}

		for _, table := range allTables {
			tableSplitsPatternStr := fmt.Sprintf("%s.%s", table, SPLIT_INFO_PATTERN)
			filePattern := filepath.Join(exportDir, "metainfo/data", tableSplitsPatternStr)
			log.Infof("clearing the generated splits for table %q matching %q pattern", table, filePattern)
			utils.ClearMatchingFiles(filePattern)
		}
		importTables = allTables //since all tables needs to imported now
	} else {
		//truncate tables with no primary key
		utils.PrintIfTrue("looking for tables without a Primary Key...\n", target.VerboseMode)
		for _, tableName := range importTables {
			if !checkPrimaryKey(tableName) {
				fmt.Printf("truncate table '%s' with NO Primary Key for import of data to restart from beginning...\n", tableName)
				utils.ClearMatchingFiles(exportDir + "/metainfo/data/" + tableName + ".[0-9]*.[0-9]*.[0-9]*.*") //correct and complete pattern to avoid matching cases with other table names
				truncateTables([]string{tableName})
			}
		}
	}

	if target.VerboseMode {
		fmt.Printf("all the tables to be imported: %v\n", allTables)
	}

	if !startClean {
		fmt.Printf("skipping already imported tables: %s\n", doneTables)
	}

	if target.VerboseMode {
		fmt.Printf("tables left to import: %v\n", importTables)
	}

	if len(importTables) == 0 {
		fmt.Printf("All the tables are already imported, nothing left to import\n")
		Done.Set()
		return
	} else {
		fmt.Printf("Preparing to import the tables: %v\n", importTables)
	}

	//Preparing the tablesProgressMetadata array
	initializeImportDataStatus(exportDir, importTables)

	go splitDataFiles(importTables, taskQueue)
}

func checkPrimaryKey(tableName string) bool {
	url := target.GetConnectionUri()
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		utils.ErrExit("Unable to connect to database (uri=%s): %s", url, err)
	}
	defer conn.Close(context.Background())

	var table, schema, originalTableName string
	if len(strings.Split(tableName, ".")) == 2 {
		schema = strings.Split(tableName, ".")[0]
		table = strings.Split(tableName, ".")[1]
	} else {
		schema = target.Schema
		table = strings.Split(tableName, ".")[0]
	}
	originalTableName = table

	if utils.IsQuotedString(table) {
		table = strings.Trim(table, `"`)
	} else {
		table = strings.ToLower(table)
	}

	checkTableSql := fmt.Sprintf(`SELECT '%s.%s'::regclass;`, schema, originalTableName)
	log.Infof("Running query on target DB: %s", checkTableSql)

	rows, err := conn.Query(context.Background(), checkTableSql)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			utils.ErrExit("table %q doesn't exist in target DB", table)
		} else {
			utils.ErrExit("error in querying to check table %q is present: %v", table, err)
		}
	} else {
		log.Infof("table %s is present in DB", table)
	}

	rows.Close()

	/* currently object names for yugabytedb is implemented as case-sensitive i.e. lower-case
	but in case of oracle exported data files(which we use for to extract tablename)
	so eg. file EMPLOYEE_data.sql -> table EMPLOYEE which needs to converted for using further */

	checkPKSql := fmt.Sprintf(`SELECT * FROM information_schema.table_constraints
	WHERE constraint_type = 'PRIMARY KEY' AND table_name = '%s' AND table_schema = '%s';`, table, schema)

	log.Infof("Running query on target DB: %s", checkPKSql)
	rows, err = conn.Query(context.Background(), checkPKSql)
	if err != nil {
		utils.ErrExit("error in querying to check PK on table %q: %v", table, err)
	}
	defer rows.Close()

	if rows.Next() {
		log.Infof("table %q has a PK", table)
		return true
	} else {
		log.Infof("table %q does not have a PK", table)
		return false
	}
}

func truncateTables(tables []string) {
	log.Infof("Truncating tables: %v", tables)
	log.Infof("Source DB type: %q", sourceDBType)

	conn := newTargetConn()
	defer conn.Close(context.Background())
	for _, table := range tables {
		log.Infof("Truncating table: %q", table)
		if target.VerboseMode {
			fmt.Printf("Truncating table %s...\n", table)
		}
		truncateStmt := fmt.Sprintf("TRUNCATE TABLE %s", table)
		_, err := conn.Exec(context.Background(), truncateStmt)
		if err != nil {
			utils.ErrExit("error while truncating table %q: %s", table, err)
		}
	}
}

func getNonEmptyTables(tables []string) []string {
	result := []string{}
	conn := newTargetConn()
	defer conn.Close(context.Background())

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

func splitDataFiles(importTables []string, taskQueue chan *SplitFileImportTask) {
	log.Infof("Started goroutine: splitDataFiles")
	for _, t := range importTables {
		origDataFile := exportDir + "/data/" + t + "_data.sql"
		extractCopyStmtForTable(t, origDataFile)
		log.Infof("Start splitting table %q: data-file: %q", t, origDataFile)

		log.Infof("Collect all interrupted/remaining splits.")
		largestCreatedSplitSoFar := int64(0)
		largestOffsetSoFar := int64(0)
		fileFullySplit := false
		pattern := fmt.Sprintf("%s/%s/data/%s.[0-9]*.[0-9]*.[0-9]*.[CPD]", exportDir, metaInfoDir, t)
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
	log.Info("All table data files are split.")
	GenerateSplitsDone.Set()
}

func splitFilesForTable(filePath string, t string, taskQueue chan *SplitFileImportTask, largestSplit int64, largestOffset int64) {
	log.Infof("Split data file %q: tableName=%q, largestSplit=%v, largestOffset=%v", filePath, t, largestSplit, largestOffset)
	splitNum := largestSplit + 1
	currTmpFileName := fmt.Sprintf("%s/%s/data/%s.%d.tmp", exportDir, metaInfoDir, t, splitNum)
	numLinesTaken := largestOffset
	numLinesInThisSplit := int64(0)

	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	dataFile, err := datafile.OpenDataFile(filePath, dataFileDescriptor)
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

		if linesWrittenToBuffer {
			line = fmt.Sprintf("\n%s", line)
		}
		length, err := bufferedWriter.WriteString(line)
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
				exportDir, metaInfoDir, t, fileSplitNumber, offsetEnd, numLinesInThisSplit, dataFile.GetBytesRead())
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
				currTmpFileName = fmt.Sprintf("%s/%s/data/%s.%d.tmp", exportDir, metaInfoDir, t, splitNum)
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
	sequenceFilePath := filepath.Join(exportDir, "/data/postdata.sql")
	if utils.FileOrFolderExists(sequenceFilePath) {
		fmt.Printf("setting resume value for sequences %10s", "")
		executeSqlFile(sequenceFilePath, "SEQUENCE", func(_, _ string) bool { return false })
	}
}

func getTablesToImport() ([]string, []string, []string, error) {
	metaInfoDir := fmt.Sprintf("%s/%s", exportDir, metaInfoDir)

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
		utils.ErrExit("Export is not done yet. Exiting.")
	}

	exportDataDir := fmt.Sprintf("%s/data", exportDir)
	_, err = os.Stat(exportDataDir)
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

func doImport(taskQueue chan *SplitFileImportTask, parallelism int, connPool *tgtdb.ConnectionPool) {
	if Done.IsSet() { //if import is already done, return
		log.Infof("Done is already set.")
		return
	}
	parallelImportCount := int64(0)

	importProgressContainer = ProgressContainer{
		container: mpb.New(),
	}
	if !disablePb {
		go importDataStatus()
	}

	for Done.IsNotSet() {
		select {
		case t := <-taskQueue:
			// fmt.Printf("Got taskfile = %s putting on parallel channel\n", t.SplitFilePath)
			// parallelImportChannel <- t
			for parallelImportCount >= int64(parallelism) {
				time.Sleep(time.Second * 2)
			}
			atomic.AddInt64(&parallelImportCount, 1)
			go doImportInParallel(t, connPool, &parallelImportCount)
		default:
			// fmt.Printf("No file sleeping for 2 seconds\n")
			time.Sleep(2 * time.Second)
		}
	}

	importProgressContainer.container.Wait()
}

func doImportInParallel(t *SplitFileImportTask, connPool *tgtdb.ConnectionPool, parallelImportCount *int64) {
	doOneImport(t, connPool)
	atomic.AddInt64(parallelImportCount, -1)
}

func doOneImport(task *SplitFileImportTask, connPool *tgtdb.ConnectionPool) {
	splitImportDone := false
	for !splitImportDone {
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

		copyCommand := getCopyCommand(task.TableName)
		reader, err := os.Open(inProgressFilePath)
		if err != nil {
			utils.ErrExit("open %q: %s", inProgressFilePath, err)
		}

		// copyCommand is empty when there are no rows for that table
		if copyCommand != "" {
			copyCommand = fmt.Sprintf(copyCommand, (task.OffsetEnd - task.OffsetStart))
			log.Infof("COPY command: %s", copyCommand)
			var rowsAffected int64
			var copyErr error
			// if retry=n, total try call will be n+1
			remainingRetries := COPY_MAX_RETRY_COUNT + 1

			copyErr = connPool.WithConn(func(conn *pgx.Conn) (bool, error) {
				// reset the reader to begin for every call
				reader.Seek(0, io.SeekStart)
				//setting the schema so that COPY command can acesss the table
				setTargetSchema(conn)
				res, err := conn.PgConn().CopyFrom(context.Background(), reader, copyCommand)
				rowsAffected = res.RowsAffected()

				if err != nil && utils.InsensitiveSliceContains(NonRetryCopyErrors, err.Error()) {
					return false, err
				}

				/*
					Note: If a user retries after deleting some row(s) from a batch,
					yb-voyager will never be able to mark the batch as completed
					github issue: https://github.com/yugabyte/yb-voyager/issues/223
				*/
				if err != nil {
					log.Warnf("COPY FROM file %q: %s", inProgressFilePath, err)
					log.Errorf("RETRYING.. COPY %q FROM file %q due to encountered error: %v ", task.TableName, inProgressFilePath, err)

					remainingRetries--
					if remainingRetries > 0 {
						retryNum := COPY_MAX_RETRY_COUNT + 1 - remainingRetries
						duration := time.Duration(math.Min(MAX_SLEEP_SECOND, 10*float64(retryNum)))
						log.Infof("sleep for duration %d before retrying the file %s for %d time...",
							duration, inProgressFilePath, retryNum)
						time.Sleep(time.Second * duration)
					}
					return remainingRetries > 0, err
				}

				return false, err
			})

			log.Infof("%q => %d rows affected", copyCommand, rowsAffected)
			if copyErr != nil {
				if !disableTransactionalWrites && strings.Contains(copyErr.Error(), "violates unique constraint") {
					log.Infof("Ignoring encountered Error: %v, Assuming batch is already imported due to transactional mode", copyErr)
				} else {
					utils.ErrExit("COPY %q FROM file %q: %s", task.TableName, inProgressFilePath, copyErr)
				}
			}

			incrementImportProgressBar(task.TableName, inProgressFilePath)
		}
		doneFilePath := getDoneFilePath(task)
		log.Infof("Renaming %q => %q", inProgressFilePath, doneFilePath)
		err = os.Rename(inProgressFilePath, doneFilePath)
		if err != nil {
			utils.ErrExit("rename %q => %q: %s", inProgressFilePath, doneFilePath, err)
		}

		err = os.Truncate(doneFilePath, 0)
		if err != nil {
			log.Warnf("truncate file %q: %s", doneFilePath, err)
		}
		splitImportDone = true
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

	setSchemaQuery := fmt.Sprintf("SET SCHEMA '%s'", target.Schema)
	_, err := conn.Exec(context.Background(), setSchemaQuery)
	if err != nil {
		utils.ErrExit("run query %q on target %q: %s", setSchemaQuery, target.Host, err)
	}

	if sourceDBType == ORACLE {
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
		utils.ErrExit("Failed in dropping index %s: %v", idxName, err)
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
		if !setOrSelectStmt {
			if len(sqlInfo.stmt) < 80 {
				fmt.Printf("%s\n", sqlInfo.stmt)
			} else {
				fmt.Printf("%s ...\n", sqlInfo.stmt[:80])
			}
		}

		err := executeSqlStmtWithRetries(&conn, sqlInfo, objType)
		if err != nil {
			conn.Close(context.Background())
			conn = nil
		}
	}
}

func executeSqlStmtWithRetries(conn **pgx.Conn, sqlInfo sqlInfo, objType string) error {
	var err error
	log.Infof("On %s run query:\n%s\n", target.Host, sqlInfo.formattedStmtStr)
	for retryCount := 0; retryCount <= DDL_MAX_RETRY_COUNT; retryCount++ {
		if retryCount > 0 { // Not the first iteration.
			log.Infof("Sleep for 5 seconds before retrying for %dth time", retryCount)
			time.Sleep(time.Second * 5)
			log.Infof("RETRYING DDL: %q", sqlInfo.stmt)
		}
		_, err = (*conn).Exec(context.Background(), sqlInfo.stmt)
		if err == nil {
			return nil
		}

		log.Errorf("DDL Execution Failed: %s", err)
		if strings.Contains(strings.ToLower(err.Error()), "conflicts with higher priority transaction") {
			// creating fresh connection
			(*conn).Close(context.Background())
			*conn = newTargetConn()
			continue
		} else if strings.Contains(strings.ToLower(err.Error()), strings.ToLower(SCHEMA_VERSION_MISMATCH_ERR)) &&
			objType == "INDEX" { // retriable error
			// creating fresh connection
			(*conn).Close(context.Background())
			*conn = newTargetConn()
			// DROP INDEX in case INVALID index got created
			dropIdx(*conn, sqlInfo.objName)
			continue
		} else if strings.Contains(err.Error(), "already exists") ||
			strings.Contains(err.Error(), "multiple primary keys") {
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
		fmt.Printf("\b \n    %s\n", err.Error())
		fmt.Printf("    STATEMENT: %s\n", sqlInfo.formattedStmtStr)
		if !target.ContinueOnError { //default case
			os.Exit(1)
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

func incrementImportProgressBar(tableName string, splitFilePath string) {
	tablesProgressMetadata[tableName].CountLiveRows += getProgressAmount(splitFilePath)
	log.Infof("Table %q, total rows-copied/progress-made until now %v", tableName, tablesProgressMetadata[tableName].CountLiveRows)
}

func extractCopyStmtForTable(table string, fileToSearchIn string) {
	if getCopyCommand(table) != "" {
		return
	}
	// pg_dump and ora2pg always have columns - "COPY table (col1, col2) FROM STDIN"
	copyCommandRegex := regexp.MustCompile(fmt.Sprintf(`(?i)COPY %s[\s]+\(.*\) FROM STDIN`, table))
	if sourceDBType == "postgresql" {
		// find the line from toc.txt file
		fileToSearchIn = exportDir + "/data/toc.txt"

		// if no schema then add public in tableName as it is there in postgres' toc file
		if len(strings.Split(table, ".")) == 1 {
			copyCommandRegex = regexp.MustCompile(fmt.Sprintf(`(?i)COPY public.%s[\s]+\(.*\) FROM STDIN`, table))
		}
	} else if sourceDBType == "oracle" {
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

func init() {
	importCmd.AddCommand(importDataCmd)
	registerCommonImportFlags(importDataCmd)
	registerImportDataFlags(importDataCmd)
}
