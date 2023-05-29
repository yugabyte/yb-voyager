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
	"github.com/sourcegraph/conc/pool"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/callhome"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datafile"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/datastore"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/dbzm"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/tgtdb"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
	"golang.org/x/exp/slices"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v8"

	"github.com/jackc/pgconn"
	"github.com/jackc/pgx/v4"
	"github.com/tevino/abool/v2"
)

var metaInfoDirName = META_INFO_DIR_NAME
var numLinesInASplit = int64(0)
var parallelImportJobs = 0
var Done = abool.New()
var batchImportPool *pool.Pool
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
var copyTableFromCommands = sync.Map{}
var loadBalancerUsed bool           // specifies whether load balancer is used in front of yb servers
var enableUpsert bool               // upsert instead of insert for import data
var disableTransactionalWrites bool // to disable transactional writes for copy command
var truncateSplits bool             // to truncate *.D splits after import

const (
	LB_WARN_MSG = "Warning: Based on internal anaylsis, --target-db-host is identified as a load balancer IP which will be used to create connections for data import.\n" +
		"\t To control the parallelism and servers used, refer to help for --parallel-jobs and --target-endpoints flags.\n"
)

var importDataCmd = &cobra.Command{
	Use:   "data",
	Short: "This command imports data into YugabyteDB database",
	Long:  `This command will import the data exported from the source database into YugabyteDB database.`,

	PreRun: func(cmd *cobra.Command, args []string) {
		validateImportFlags(cmd)
	},
	Run: importDataCommandFn,
}

func importDataCommandFn(cmd *cobra.Command, args []string) {
	target.ImportMode = true
	sourceDBType = ExtractMetaInfo(exportDir).SourceDBType
	sqlname.SourceDBType = sourceDBType
	dataStore = datastore.NewDataStore(filepath.Join(exportDir, "data"))
	checkExportDataDoneFlag()
	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	importFileToTableMap := discoverFilesToImport()
	importFileToTableMap = applyTableListFilter(importFileToTableMap)
	importData(importFileToTableMap)
}

func discoverFilesToImport() map[string]string {
	result := make(map[string]string)
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
	for _, v := range datafiles {
		tablenameMatches := pat.FindAllStringSubmatch(v, -1)
		for _, match := range tablenameMatches {
			tableName := match[1]
			result[v] = tableName
		}
	}
	return result
}

func applyTableListFilter(importFileToTableMap map[string]string) map[string]string {
	result := make(map[string]string)
	includeList := utils.CsvStringToSlice(target.TableList)
	excludeList := utils.CsvStringToSlice(target.ExcludeTableList)
	for fileName, tableName := range importFileToTableMap {
		if len(includeList) > 0 && !slices.Contains(includeList, tableName) {
			log.Infof("Skipping table %q (fileName: %s) as it is not in the include list", tableName, fileName)
			continue
		}
		if len(excludeList) > 0 && slices.Contains(excludeList, tableName) {
			log.Infof("Skipping table %q (fileName: %s) as it is in the exclude list", tableName, fileName)
			continue
		}
		result[fileName] = tableName
	}
	return result
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

func importData(importFileToTableMap map[string]string) {
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
		NumConnections:    parallelImportJobs,
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

	state := NewImportDataState(exportDir)
	determineTablesToImport(state)
	if startClean {
		cleanImportState(state)
	}
	if target.VerboseMode {
		fmt.Printf("all the tables to be imported: %v\n", allTables)
		fmt.Printf("tables left to import: %v\n", importTables)
	}

	if len(importTables) == 0 {
		fmt.Printf("All the tables are already imported, nothing left to import\n")
	} else {
		fmt.Printf("Importing tables: %v\n", importTables)
		initializeImportDataStatus(state, exportDir, importTables)
		if !disablePb {
			go importDataStatus()
		}
		poolSize := 20
		if parallelism > poolSize {
			poolSize = parallelism + parallelism/2
		}
		// The code can produce `poolSize` number of batches at a time. But, it can consume only
		// `parallelism` number of batches at a time.
		batchImportPool = pool.New().WithMaxGoroutines(poolSize)
		for _, t := range importTables {
			importTable(state, t, connPool)
		}
		// wait for all the import jobs to finish
		batchImportPool.Wait()
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

func determineTablesToImport(state *ImportDataState) {
	doneTables, interruptedTables, remainingTables, _ := getTablesToImport(state)

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

func cleanImportState(state *ImportDataState) {
	conn := newTargetConn()
	defer conn.Close(context.Background())

	nonEmptyTableNames := getNonEmptyTables(conn, allTables)
	if len(nonEmptyTableNames) > 0 {
		utils.ErrExit("Following tables are not empty. "+
			"TRUNCATE them before importing data with --start-clean.\n%s",
			strings.Join(nonEmptyTableNames, ", "))
	}

	for _, tableName := range allTables {
		err := state.Clean(tableName, conn)
		if err != nil {
			utils.ErrExit("failed to clean import data state for table %q: %s", tableName, err)
		}
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

func importTable(state *ImportDataState, t string, connPool *tgtdb.ConnectionPool) {

	origDataFile := exportDir + "/data/" + t + "_data.sql"
	extractCopyStmtForTable(t, origDataFile)
	log.Infof("Start splitting table %q: data-file: %q", t, origDataFile)

	log.Infof("Collect all interrupted/remaining splits.")
	pendingBatches, lastBatchNumber, lastOffset, fileFullySplit, err := state.Recover(t)
	if err != nil {
		utils.ErrExit("recovering state for table %q: %s", t, err)
	}
	for _, batch := range pendingBatches {
		submitBatch(batch, connPool)
	}
	if !fileFullySplit {
		splitFilesForTable(state, origDataFile, t, connPool, lastBatchNumber, lastOffset)
	}
}

func splitFilesForTable(state *ImportDataState, filePath string, t string, connPool *tgtdb.ConnectionPool, lastBatchNumber int64, lastOffset int64) {
	log.Infof("Split data file %q: tableName=%q, largestSplit=%v, largestOffset=%v", filePath, t, lastBatchNumber, lastOffset)
	batchNum := lastBatchNumber + 1
	numLinesTaken := lastOffset

	reader, err := dataStore.Open(filePath)
	if err != nil {
		utils.ErrExit("preparing reader for split generation on file %q: %v", filePath, err)
	}

	dataFileDescriptor = datafile.OpenDescriptor(exportDir)
	dataFile, err := datafile.NewDataFile(filePath, reader, dataFileDescriptor)
	if err != nil {
		utils.ErrExit("open datafile %q: %v", filePath, err)
	}
	defer dataFile.Close()

	log.Infof("Skipping %d lines from %q", lastOffset, filePath)
	err = dataFile.SkipLines(lastOffset)
	if err != nil {
		utils.ErrExit("skipping line for offset=%d: %v", lastOffset, err)
	}

	var readLineErr error = nil
	var line string
	var batchWriter *BatchWriter
	header := ""
	if dataFileDescriptor.HasHeader {
		header = dataFile.GetHeader()
	}
	for readLineErr == nil {

		if batchWriter == nil {
			batchWriter = state.NewBatchWriter(t, batchNum)
			err := batchWriter.Init()
			if err != nil {
				utils.ErrExit("initializing batch writer for table %q: %s", t, err)
			}
			if header != "" {
				err = batchWriter.WriteHeader(header)
				if err != nil {
					utils.ErrExit("writing header for table %q: %s", t, err)
				}
			}
		}

		line, readLineErr = dataFile.NextLine()
		if readLineErr == nil || (readLineErr == io.EOF && line != "") {
			// handling possible case: last dataline(i.e. EOF) but no newline char at the end
			numLinesTaken += 1
		}
		err = batchWriter.WriteRecord(line)
		if err != nil {
			utils.ErrExit("Write to batch %d: %s", batchNum, err)
		}
		if batchWriter.NumRecordsWritten == numLinesInASplit ||
			dataFile.GetBytesRead() >= MAX_SPLIT_SIZE_BYTES ||
			readLineErr != nil {

			isLastBatch := false
			if readLineErr == io.EOF {
				isLastBatch = true
			} else if readLineErr != nil {
				utils.ErrExit("read line from data file %q: %s", filePath, readLineErr)
			}

			offsetEnd := numLinesTaken
			batch, err := batchWriter.Done(isLastBatch, offsetEnd, dataFile.GetBytesRead())
			if err != nil {
				utils.ErrExit("finalizing batch %d: %s", batchNum, err)
			}
			dataFile.ResetBytesRead()
			submitBatch(batch, connPool)

			if !isLastBatch {
				batchNum += 1
				batchWriter = nil
			}
		}
	}
	log.Infof("splitFilesForTable: done splitting data file %q for table %q", filePath, t)
}

func submitBatch(batch *Batch, connPool *tgtdb.ConnectionPool) {
	batchImportPool.Go(func() {
		// There are `poolSize` number of competing go-routines trying to invoke COPY.
		// But the `connPool` will allow only `parallelism` number of connections to be
		// used at a time. Thus limiting the number of concurrent COPYs to `parallelism`.
		doOneImport(batch, connPool)
	})
	log.Infof("Queued batch: %s", spew.Sdump(batch))
}

func executePostImportDataSqls() {
	sequenceFilePath := filepath.Join(exportDir, "data", "postdata.sql")
	if utils.FileOrFolderExists(sequenceFilePath) {
		fmt.Printf("setting resume value for sequences %10s\n", "")
		executeSqlFile(sequenceFilePath, "SEQUENCE", func(_, _ string) bool { return false })
	}
}

func getTablesToImport(state *ImportDataState) ([]string, []string, []string, error) {
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
		tableImportState, err := state.GetTableImportState(t)
		if err != nil {
			utils.ErrExit("getting table import state for %q: %s", t, err)
		}
		log.Infof("Table %q import state: %s", t, tableImportState)
		switch tableImportState {
		case TABLE_IMPORT_COMPLETED:
			doneTables = append(doneTables, t)
		case TABLE_IMPORT_IN_PROGRESS:
			interruptedTables = append(interruptedTables, t)
		case TABLE_IMPORT_NOT_STARTED:
			remainingTables = append(remainingTables, t)
		default:
			utils.ErrExit("unknown table import state %q for table %q", tableImportState, t)
		}
	}

	return doneTables, interruptedTables, remainingTables, nil
}

func doOneImport(batch *Batch, connPool *tgtdb.ConnectionPool) {
	//this is done to signal start progress bar for this table
	if tablesProgressMetadata[batch.TableName].CountLiveRows == -1 {
		tablesProgressMetadata[batch.TableName].CountLiveRows = 0
	}
	err := batch.MarkPending()
	if err != nil {
		utils.ErrExit("marking batch %d as pending: %s", batch.Number, err)
	}
	log.Infof("Importing %q", batch.FilePath)

	copyCommand := getCopyCommand(batch.TableName)
	// copyCommand is empty when there are no rows for that table
	if copyCommand == "" {
		err = batch.MarkDone()
		if err != nil {
			utils.ErrExit("marking batch %q as done: %s", batch.FilePath, err)
		}
		return
	}
	copyCommand = fmt.Sprintf(copyCommand, (batch.OffsetEnd - batch.OffsetStart))
	log.Infof("COPY command: %s", copyCommand)
	var rowsAffected int64
	attempt := 0
	sleepIntervalSec := 0
	copyFn := func(conn *pgx.Conn) (bool, error) {
		var err error
		attempt++
		rowsAffected, err = importBatch(conn, batch, copyCommand)
		if err == nil ||
			utils.InsensitiveSliceContains(NonRetryCopyErrors, err.Error()) ||
			attempt == COPY_MAX_RETRY_COUNT {
			return false, err
		}
		log.Warnf("COPY FROM file %q: %s", batch.FilePath, err)
		sleepIntervalSec += 10
		if sleepIntervalSec > MAX_SLEEP_SECOND {
			sleepIntervalSec = MAX_SLEEP_SECOND
		}
		log.Infof("sleep for %d seconds before retrying the file %s (attempt %d)",
			sleepIntervalSec, batch.FilePath, attempt)
		time.Sleep(time.Duration(sleepIntervalSec) * time.Second)
		return true, err
	}
	err = connPool.WithConn(copyFn)
	log.Infof("%q => %d rows affected", copyCommand, rowsAffected)
	if err != nil {
		utils.ErrExit("COPY %q FROM file %q: %s", batch.TableName, batch.FilePath, err)
	}
	if dataFileDescriptor.TableRowCount != nil {
		incrementImportProgressBar(batch.TableName, batch.RecordCount)
	} else {
		incrementImportProgressBar(batch.TableName, batch.ByteCount)
	}
	err = batch.MarkDone()
	if err != nil {
		utils.ErrExit("marking batch %q as done: %s", batch.FilePath, err)
	}
}

func importBatch(conn *pgx.Conn, batch *Batch, copyCmd string) (rowsAffected int64, err error) {
	var file *os.File
	file, err = batch.Open()
	if err != nil {
		utils.ErrExit("opening batch %s: %s", batch.FilePath, err)
	}
	defer file.Close()

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
	alreadyImported, rowsAffected, err = batch.IsAlreadyImported(tx)
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
			err = fmt.Errorf("%s, %s in %s", err.Error(), pgerr.Where, batch.FilePath)
		}
		return res.RowsAffected(), err
	}

	err = batch.RecordEntryInDB(tx, res.RowsAffected())
	if err != nil {
		err = fmt.Errorf("record entry in DB for batch %q: %w", batch.FilePath, err)
	}
	return res.RowsAffected(), err
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

func getTargetSchemaName(tableName string) string {
	parts := strings.Split(tableName, ".")
	if len(parts) == 2 {
		return parts[0]
	}
	return target.Schema // default set to "public"
}

func incrementImportProgressBar(tableName string, progressAmount int64) {
	tablesProgressMetadata[tableName].CountLiveRows += progressAmount
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
		copyTableFromCommands.Store(table, stmt)
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
			copyTableFromCommands.Store(table, line)
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
	if copyCommand, ok := copyTableFromCommands.Load(table); ok {
		return copyCommand.(string)
	} else {
		log.Infof("No COPY command for table %q", table)
	}
	return "" // no-op
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
