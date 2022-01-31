/*
Copyright © 2021 NAME HERE <EMAIL ADDRESS>

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
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"yb_migrate/src/fwk"
	"yb_migrate/src/utils"

	"github.com/spf13/cobra"
	"github.com/vbauerster/mpb/v7"

	"github.com/jackc/pgx/v4"
	"github.com/tevino/abool/v2"
)

var metaInfoDir = META_INFO_DIR_NAME
var importLockFile = fmt.Sprintf("%s/%s/data/.importLock", exportDir, metaInfoDir)
var numLinesInASplit = 1000
var parallelImportJobs = 0
var Done = abool.New()
var GenerateSplitsDone = abool.New()

var tablesProgressMetadata map[string]*utils.TableProgressMetadata

type ProgressContainer struct {
	mu        sync.Mutex
	container *mpb.Progress
}

var importProgressContainer ProgressContainer
var importTables []string
var allTables []string

type ExportTool int

const (
	Ora2Pg = iota
	YsqlDump
	PgDump
)

// importDataCmd represents the importData command
var importDataCmd = &cobra.Command{
	Use:   "data",
	Short: "This command imports data into YugabyteDB database",
	Long:  `This command will import the data exported from the source database into YugabyteDB database.`,

	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		cmd.Parent().PersistentPreRun(cmd.Parent(), args)
		// fmt.Println("Import Data PersistentPreRun")
	},

	Run: func(cmd *cobra.Command, args []string) {
		// exportTool := getExportTool()
		// fmt.Printf("tool = %d\n", exportTool)
		importData()
	},
}

func getExportTool() ExportTool {
	return Ora2Pg
}

func getYBServers() []*utils.Target {
	url := getTargetConnectionUri(&target)
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	rows, err := conn.Query(context.Background(), GET_SERVERS_QUERY)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var targets []*utils.Target
	for rows.Next() {
		clone := cloneTarget(&target, false)
		var host, nodeType, cloud, region, zone, public_ip string
		var port, num_conns int
		if err := rows.Scan(&host, &port, &num_conns,
			&nodeType, &cloud, &region, &zone, &public_ip); err != nil {
			log.Fatal(err)
		}
		clone.Host = host
		clone.Port = fmt.Sprintf("%d", port)
		clone.Uri = getTargetConnectionUri(clone)
		targets = append(targets, clone)
	}
	return targets
}

func cloneTarget(t *utils.Target, includeUri bool) *utils.Target {
	var clone utils.Target
	clone.User = t.User
	clone.DBName = t.DBName
	clone.Password = t.Password
	clone.Host = t.Host
	clone.Port = t.Port
	if includeUri {
		clone.Uri = t.Uri
	}
	return &clone
}

func getTargetFromYBUri(uri string) *utils.Target {
	uriParts := strings.Split(uri, ":")
	if len(uriParts) < 4 {
		panic("Bad uri: " + uri)
	}
	// pgdatatbase := uriParts[0]
	userPart := uriParts[1]
	user := strings.TrimPrefix(userPart, "//")
	passwdHostPart := uriParts[2]
	passwdHost := strings.Split(passwdHostPart, "@")
	password := passwdHost[0]
	host := passwdHost[1]
	portDBPart := uriParts[3]
	portDB := strings.Split(portDBPart, "/")
	port := portDB[0]
	dbName := portDB[1]
	// fmt.Printf("pgdb = %s\nuser = %s\npasswd = %s\nhost = %s\nport = %s\ndatabase = %s\n",
	// pgdatatbase, user, password, host, port, database)
	targetStruct := utils.Target{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		DBName:   dbName,
		Uri:      uri,
	}
	return &targetStruct
}

func getTargetConnectionUri(targetStruct *utils.Target) string {
	if len(targetStruct.Uri) != 0 {
		targetFromURi := getTargetFromYBUri(target.Uri)
		targetStruct.User = targetFromURi.User
		targetStruct.DBName = targetFromURi.DBName
		targetStruct.Password = targetFromURi.Password
		targetStruct.Host = targetFromURi.Host
		targetStruct.Port = targetFromURi.Port
		return targetStruct.Uri
	}
	uri := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		targetStruct.User, targetStruct.Password, targetStruct.Host, targetStruct.Port, targetStruct.DBName)
	targetStruct.Uri = uri
	return uri
}

func importData() {
	// TODO: Add later
	// acquireImportLock()
	// defer os.Remove(importLockFile)

	fmt.Printf("\nimport of data in '%s' database started\n", target.DBName)

	targets := getYBServers()
	var parallelism = parallelImportJobs
	if parallelism == -1 {
		parallelism = len(targets)
	}

	if source.VerboseMode {
		fmt.Printf("Number of parallel imports jobs at a time: %d\n", parallelism)
	}

	splitFilesChannel := make(chan *fwk.SplitFileImportTask, SPLIT_FILE_CHANNEL_SIZE)
	targetServerChannel := make(chan *utils.Target, 1)

	go roundRobinTargets(targets, targetServerChannel)
	generateSmallerSplits(splitFilesChannel)
	go doImport(splitFilesChannel, parallelism, targetServerChannel)
	checkForDone()

	time.Sleep(time.Second * 2)
	fmt.Printf("\nexiting...\n")
}

func checkForDone() {
	// doLoop := true
	for Done.IsNotSet() {
		if GenerateSplitsDone.IsSet() {
			// InProgress Pattern
			inProgressPattern := fmt.Sprintf("%s/%s/data/*.P", exportDir, metaInfoDir)
			m1, _ := filepath.Glob(inProgressPattern)
			inCreatedPattern := fmt.Sprintf("%s/%s/data/*.C", exportDir, metaInfoDir)
			m2, _ := filepath.Glob(inCreatedPattern)
			// in progress are interrupted ones
			if len(m1) > 0 || len(m2) > 0 {
				time.Sleep(2 * time.Second)
			} else {
				// doLoop = false
				Done.Set()
			}
		} else {
			time.Sleep(5 * time.Second)
		}
	}

}

func roundRobinTargets(targets []*utils.Target, channel chan *utils.Target) {
	index := 0
	for Done.IsNotSet() {
		channel <- targets[index%len(targets)]
		index++
	}
}

func acquireImportLock() {
}

func generateSmallerSplits(taskQueue chan *fwk.SplitFileImportTask) {
	doneTables, interruptedTables, remainingTables, _ := getTablesToImport()

	//TODO: print in verbose mode
	// fmt.Printf("doneTables: %s\n", doneTables)
	// fmt.Printf("interruptedTables: %s\n", interruptedTables)
	// fmt.Printf("remainingTables: %s\n", remainingTables)

	importTables = append(interruptedTables, remainingTables...)
	allTables = append(importTables, doneTables...)

	if startClean { //start data migraiton from beginning
		fmt.Println("cleaning the database and project directory")
		truncateTables(allTables)

		utils.CleanDir(exportDir + "/metainfo/data")

		importTables = allTables //since all tables needs to imported now
	}

	if source.VerboseMode {
		fmt.Printf("all the tables to be imported: %v\n", allTables)
		fmt.Printf("tables left to import: %v\n", importTables)
	}

	if len(importTables) == 0 {
		fmt.Printf("All the tables are already imported\n")
		Done.Set()
		return
	} else {
		fmt.Printf("Preparing the tables for import...\n")
	}

	//Preparing the tablesProgressMetadata array
	initializeImportDataStatus(exportDir, allTables)
	// fmt.Printf("TablesProgresMetadata after initializing: %v\n", tablesProgressMetadata)

	go splitDataFiles(importTables, taskQueue)
}

func truncateTables(tables []string) {
	fmt.Println("Truncating all the tables in the schema")

	url := getTargetConnectionUri(&target)
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	for _, tab := range tables {
		// log.Infof("Truncating table %s\n", tab)
		truncateStmnt := fmt.Sprintf("truncate table %s", tab)
		rows, err := conn.Query(context.Background(), truncateStmnt)
		if err != nil {
			log.Fatal(err)
		}
		rows.Close()
	}
}

func splitDataFiles(importTables []string, taskQueue chan *fwk.SplitFileImportTask) {

	for _, t := range importTables {
		origDataFile := exportDir + "/data/" + t + "_data.sql"
		// collect interrupted splits
		// make an import task and schedule them

		largestCreatedSplitSoFar := 0
		largestOffsetSoFar := 0
		fileFullySplit := false
		pattern := fmt.Sprintf("%s/%s/data/%s.[0-9]*.[0-9]*.[0-9]*.[CPD]", exportDir, metaInfoDir, t)
		matches, _ := filepath.Glob(pattern)
		// in progress are interrupted ones
		interruptedRegexStr := fmt.Sprintf(".+/%s\\.(\\d+)\\.(\\d+)\\.\\d+\\.[P]$", t)
		interruptedRegexp := regexp.MustCompile(interruptedRegexStr)
		for _, filename := range matches {
			// fmt.Printf("Matched file name = %s\n", filename)
			submatches := interruptedRegexp.FindAllStringSubmatch(filename, -1)
			for _, match := range submatches {
				// This means a match. Submit the task with interrupted = true
				splitNum, _ := strconv.Atoi(match[1])
				offset, _ := strconv.Atoi(match[2])
				if splitNum == LAST_SPLIT_NUM {
					fileFullySplit = true
				}
				if splitNum > largestCreatedSplitSoFar {
					largestCreatedSplitSoFar = splitNum
				}
				if offset > largestOffsetSoFar {
					largestOffsetSoFar = offset
				}
				addASplitTask("", t, filename, splitNum, true, taskQueue)
			}
		}
		// collect files which were generated but processing did not start
		// schedule import task for them
		createdButNotStartedRegexStr := fmt.Sprintf(".+/%s\\.(\\d+)\\.(\\d+)\\.\\d+\\.[C]$", t)
		createdButNotStartedRegex := regexp.MustCompile(createdButNotStartedRegexStr)
		// fmt.Printf("created but not started regex = %s\n", createdButNotStartedRegex.String())
		for _, filename := range matches {
			submatches := createdButNotStartedRegex.FindAllStringSubmatch(filename, -1)
			for _, match := range submatches {
				// This means a match. Submit the task with interrupted = true
				splitNum, _ := strconv.Atoi(match[1])
				offset, _ := strconv.Atoi(match[2])
				if splitNum > largestCreatedSplitSoFar {
					largestCreatedSplitSoFar = splitNum
				}
				if offset > largestOffsetSoFar {
					largestOffsetSoFar = offset
				}
				if splitNum == LAST_SPLIT_NUM {
					fileFullySplit = true
				}
				// for interrupted files offsets are already determined
				addASplitTask("", t, filename, splitNum, false, taskQueue)
			}
		}
		if !fileFullySplit {
			splitFilesForTable(origDataFile, t, taskQueue, largestCreatedSplitSoFar, largestOffsetSoFar)
		}
	}
	GenerateSplitsDone.Set()
}

func splitFilesForTable(dataFile string, t string, taskQueue chan *fwk.SplitFileImportTask, largestSplit int, largestOffset int) {
	splitNum := largestSplit + 1
	currTmpFileName := fmt.Sprintf("%s/%s/data/%s.%d.tmp", exportDir, metaInfoDir, t, splitNum)
	numLinesTaken := largestOffset
	numLinesInThisSplit := 0
	forig, err := os.Open(dataFile)
	if err != nil {
		log.Fatal(err)
	}
	defer forig.Close()

	r := bufio.NewReader(forig)
	sz := 0
	// fmt.Printf("curr temp file created = %s\n", currTmpFileName)
	outfile, err := os.Create(currTmpFileName)
	if err != nil {
		log.Fatal(err)
	}

	// skip till largest offset
	// fmt.Printf("Skipping %d lines from %s\n", largestOffset, dataFile)
	for i := 0; i < largestOffset; i++ {
		utils.Readline(r)
	}
	// Create a buffered writer from the file
	bufferedWriter := bufio.NewWriter(outfile)
	var readLineErr error = nil
	var line string
	linesWrittenToBuffer := false
	for readLineErr == nil {
		line, readLineErr = utils.Readline(r)

		if readLineErr == nil && !isDataLine(line) {
			continue
		} else if readLineErr == nil { //increment the count only if line is valid
			numLinesTaken += 1
			numLinesInThisSplit += 1
		}

		if linesWrittenToBuffer {
			line = fmt.Sprintf("\n%s", line)
		}
		length, err := bufferedWriter.WriteString(line)
		linesWrittenToBuffer = true
		if err != nil {
			log.Printf("Cannot write the read line into %s\n", outfile)
			return
		}
		sz += length
		if sz >= FOUR_MB {
			err = bufferedWriter.Flush()
			if err != nil {
				log.Printf("Cannot flush data in file = %s\n", outfile)
				return
			}
			bufferedWriter.Reset(outfile)
			sz = 0
		}

		if numLinesInThisSplit == numLinesInASplit || readLineErr != nil {
			err = bufferedWriter.Flush()
			if err != nil {
				log.Printf("Cannot flush data in file = %s\n", outfile)
				return
			}
			outfile.Close()
			fileSplitNumber := splitNum
			if readLineErr != nil {
				fileSplitNumber = 0 // TODO Constant
			}
			splitFile := fmt.Sprintf("%s/%s/data/%s.%d.%d.%d.C",
				exportDir, metaInfoDir, t, fileSplitNumber, numLinesTaken, numLinesInThisSplit)
			err = os.Rename(currTmpFileName, splitFile)
			if err != nil {
				log.Printf("Cannot rename %s to %s\n", currTmpFileName, splitFile)
				return
			}
			addASplitTask("", t, splitFile, splitNum, false, taskQueue)
			if fileSplitNumber != 0 {
				splitNum += 1
				numLinesInThisSplit = 0
				linesWrittenToBuffer = false
				currTmpFileName = fmt.Sprintf("%s/%s/data/%s.%d.tmp", exportDir, metaInfoDir, t, splitNum)
				outfile, err = os.Create(currTmpFileName)
				if err != nil {
					log.Printf("Cannot create %s\n", currTmpFileName)
					return
				}
				bufferedWriter = bufio.NewWriter(outfile)
			}
		}
	}
}

func isDataLine(line string) bool {
	if len(line) == 0 || strings.HasPrefix(
		line, "SET ") || strings.HasPrefix(
		line, "TRUNCATE ") || strings.HasPrefix(
		line, "COPY ") || strings.HasPrefix(line, "\\.") {
		// fmt.Printf("Returning non data line for %s\n", line)
		return false
	}
	return true
}

func addASplitTask(schemaName string, tableName string, filepath string, splitNumber int, interrupted bool,
	taskQueue chan *fwk.SplitFileImportTask) {
	var t fwk.SplitFileImportTask
	t.SchemaName = schemaName
	t.TableName = tableName
	t.SplitFilePath = filepath
	t.SplitNumber = splitNumber
	t.Interrupted = interrupted
	taskQueue <- &t
}

//TODO
func getPostSQLs(file string) []string {
	/*
		SQLs for
			Enabling Sequences
	*/
	return nil
}

func getTablesToImport() ([]string, []string, []string, error) {
	metaInfoDir := fmt.Sprintf("%s/%s", exportDir, metaInfoDir)

	_, err := os.Stat(metaInfoDir)
	if err != nil {
		fmt.Println("metainfo dir is missing. Exiting.")
		os.Exit(1)
	}
	metaInfoDataDir := fmt.Sprintf("%s/data", metaInfoDir)
	_, err = os.Stat(metaInfoDataDir)
	if err != nil {
		fmt.Println("metainfo data dir is missing. Exiting.")
		os.Exit(1)
	}

	exportDataDonePath := metaInfoDir + "/flags/exportDataDone"
	_, err = os.Stat(exportDataDonePath)
	if err != nil {
		fmt.Println("Export is not done yet. Exiting.")
		os.Exit(1)
	}

	exportDataDir := fmt.Sprintf("%s/data", exportDir)
	_, err = os.Stat(exportDataDir)
	if err != nil {
		fmt.Printf("Export data dir %s is missing. Exiting.\n", exportDataDir)
		os.Exit(1)
	}
	// Collect all the data files
	dataFilePatern := fmt.Sprintf("%s/*_data.sql", exportDataDir)
	datafiles, err := filepath.Glob(dataFilePatern)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}

	pat := regexp.MustCompile(`.+/(\S+)_data.sql`)
	var tables []string
	for _, v := range datafiles {
		tablenameMatches := pat.FindAllStringSubmatch(v, -1)
		for _, match := range tablenameMatches {
			tables = append(tables, match[1])
		}
	}

	var doneTables []string
	var interruptedTables []string
	var remainingTables []string
	for _, t := range tables {

		donePattern := fmt.Sprintf("%s/%s.[0-9]*.[0-9]*.[0-9]*.D", metaInfoDataDir, t)
		interruptedPattern := fmt.Sprintf("%s/%s.[0-9]*.[0-9]*.[0-9]*.P", metaInfoDataDir, t)
		createdPattern := fmt.Sprintf("%s/%s.[0-9]*.[0-9]*.[0-9]*.C", metaInfoDataDir, t)

		doneMatches, _ := filepath.Glob(donePattern)
		interruptedMatches, _ := filepath.Glob(interruptedPattern)
		createdMatches, _ := filepath.Glob(createdPattern)

		//Issue/TestCase: If rate of ingestion is faster than splitting
		if len(createdMatches) == 0 && len(interruptedMatches) == 0 && len(doneMatches) > 0 {
			doneTables = append(doneTables, t)
		} else if (len(createdMatches) > 0 && len(interruptedMatches)+len(doneMatches) == 0) ||
			(len(createdMatches)+len(interruptedMatches)+len(doneMatches) == 0) {
			remainingTables = append(remainingTables, t)
		} else {
			interruptedTables = append(interruptedTables, t)
		}
	}

	return doneTables, interruptedTables, remainingTables, nil
}

func doImport(taskQueue chan *fwk.SplitFileImportTask, parallelism int, targetChan chan *utils.Target) {
	if Done.IsSet() { //if import is already done, return
		return
	}

	parallelImportCount := int64(0)

	importProgressContainer = ProgressContainer{
		container: mpb.New(),
	}
	go importDataStatus()

	for Done.IsNotSet() {
		select {
		case t := <-taskQueue:
			// fmt.Printf("Got taskfile = %s putting on parallel channel\n", t.SplitFilePath)
			// parallelImportChannel <- t
			for parallelImportCount >= int64(parallelism) {
				time.Sleep(time.Second * 2)
			}
			atomic.AddInt64(&parallelImportCount, 1)
			go doImportInParallel(t, targetChan, &parallelImportCount)
		default:
			// fmt.Printf("No file sleeping for 2 seconds\n")
			time.Sleep(2 * time.Second)
		}
	}

	importProgressContainer.container.Wait()
}

func doImportInParallel(t *fwk.SplitFileImportTask, targetChan chan *utils.Target, parallelImportCount *int64) {
	doOneImport(t, targetChan)
	atomic.AddInt64(parallelImportCount, -1)
}

func doOneImport(t *fwk.SplitFileImportTask, targetChan chan *utils.Target) {
	splitImportDone := false
	for !splitImportDone {
		select {
		case targetServer := <-targetChan:
			// fmt.Printf("Got taskfile %s and target for that = %s\n", t.SplitFilePath, targetServer.Host)

			//this is done to signal start progress bar for this table
			if tablesProgressMetadata[t.TableName].CountLiveRows == -1 {
				tablesProgressMetadata[t.TableName].CountLiveRows = 0
			}

			// Rename the file to .P
			err := os.Rename(t.SplitFilePath, getInProgressFilePath(t))
			if err != nil {
				panic(err)
			}
			// Prepare a file with session commands followed by the COPY command
			sqlFile := getSqlFilePath(t)
			fp, err := os.Create(sqlFile)
			if err != nil {
				panic(fp)
			}
			defer os.Remove(sqlFile)
			bufferedWriter := bufio.NewWriter(fp)
			for _, statement := range IMPORT_SESSION_SETTERS {
				_, err = bufferedWriter.WriteString(statement)
				if err != nil {
					panic(err)
				}
				err = bufferedWriter.WriteByte(NEWLINE)
				if err != nil {
					panic(err)
				}
			}

			copyCommand := fmt.Sprintf("\\copy %s from '%s';", t.TableName, getInProgressFilePath(t))
			_, err = bufferedWriter.WriteString(copyCommand)
			if err != nil {
				panic(err)
			}
			err = bufferedWriter.Flush()
			if err != nil {
				panic(err)
			}
			// fmt.Printf("Processing sqlfile: %s\n", filepath.Base(sqlFile))
			errStr := executeSqlFile(sqlFile, targetServer)
			if len(errStr) > 0 {
				panic(errStr)
			}

			// fmt.Printf("Renaming sqlfile: %s to done\n", sqlFile)
			err = os.Rename(getInProgressFilePath(t), getDoneFilePath(t))
			if err != nil {
				panic(err)
			}
			// fmt.Printf("Renamed sqlfile: %s done\n", sqlFile)

			// update the import data status
			incrementImportedRowCount(t.TableName)
			// fmt.Printf("Inserted a batch of %d or less in table %s\n", numLinesInASplit, t.TableName)
			splitImportDone = true
		default:
			// fmt.Printf("No server sleeping for 2 seconds\n")
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func executeSqlFile(file string, server *utils.Target) string {
	// fmt.Printf("Executing Sql file %s on server: %s\n", filepath.Base(file), server.Host)
	connectionURI := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s?sslmode=disable",
		server.User, server.Password, server.Host, server.Port, server.DBName)
	// cmd := exec.Command(YSQL, connectionURI, "-b", "-f", file)
	cmd := exec.Command(YSQL, connectionURI, "-f", file)

	var errorBuffer bytes.Buffer

	cmd.Stderr = &errorBuffer

	//TODO: move to constants.go
	retryOnErrors := []string{"pgsql error 40001"}
	ignoreOnErrors := []string{"duplicate key value violates unique constraint"}

	//TODO: Error handling for cases when we need to abort the command

	retryFlag, totalRetryCount := true, 5
	for retryFlag && totalRetryCount > 0 {
		cmd.Run()

		errorString := errorBuffer.String()

		for _, str := range ignoreOnErrors {
			if strings.Contains(errorString, str) {
				retryFlag = false
				return ""
			}
		}

		for _, str := range retryOnErrors {
			if strings.Contains(errorString, str) {
				break
			}
		}

		totalRetryCount--
	}

	return errorBuffer.String()
}

func getInProgressFilePath(task *fwk.SplitFileImportTask) string {
	path := task.SplitFilePath
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	parts := strings.Split(base, ".")
	return fmt.Sprintf("%s/%s.%s.%s.%s.P", dir, parts[0], parts[1], parts[2], parts[3])
}

func getDoneFilePath(task *fwk.SplitFileImportTask) string {
	path := task.SplitFilePath
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	parts := strings.Split(base, ".")
	return fmt.Sprintf("%s/%s.%s.%s.%s.D", dir, parts[0], parts[1], parts[2], parts[3])
}

func getSqlFilePath(task *fwk.SplitFileImportTask) string {
	path := task.SplitFilePath
	base := filepath.Base(path)
	dir := filepath.Dir(path)
	parts := strings.Split(base, ".")
	return fmt.Sprintf("%s/%s.%s.%s.%s.sql", dir, parts[0], parts[1], parts[2], parts[3])
}

func incrementImportedRowCount(tableName string) {
	//TODO: some optimization can be done here
	tablesProgressMetadata[tableName].CountLiveRows += int64(numLinesInASplit)
}

func init() {
	importCmd.AddCommand(importDataCmd)

	importDataCmd.PersistentFlags().StringVar(&importMode, "offline", "",
		"By default the data migration mode is offline. Use '--mode online' to change the mode to online migration")

}
