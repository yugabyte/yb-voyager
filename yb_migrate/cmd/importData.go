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
	"context"
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"yb_migrate/src/fwk"
	"yb_migrate/src/utils"

	"github.com/jackc/pgx/v4"
    "github.com/tevino/abool/v2"
)

var importMode string
var metaInfoDir = META_INFO_DIR_NAME
var importLockFile = fmt.Sprintf("%s/%s/data/.importLock", exportDir, metaInfoDir)
var numLinesInASplit = 1000
var parallelImportJobs = 0
var Done = abool.New()
var GenerateSplitsDone = abool.New()
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

	Run: func(cmd *cobra.Command, args []string) {
		// exportTool := getExportTool()
		// fmt.Printf("tool = %d\n", exportTool)
		importData()
	},
}

func getExportTool() ExportTool {
	return Ora2Pg
}

func getYBServers() []*Target {
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

	var targets []*Target
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

func cloneTarget(t *Target, includeUri bool) *Target {
	var clone Target
	clone.User = t.User
	clone.Database = t.Database
	clone.Password = t.Password
	clone.Host = t.Host
	clone.Port = t.Port
	if includeUri {
		clone.Uri = t.Uri
	}
	return &clone
}

func getTargetFromYBUri(uri string) *Target {
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
	database := portDB[1]
	// fmt.Printf("pgdb = %s\nuser = %s\npasswd = %s\nhost = %s\nport = %s\ndatabase = %s\n",
		// pgdatatbase, user, password, host, port, database)
	targetStruct := Target{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: database,
		Uri:      uri,
	}
	return &targetStruct
}

func getTargetConnectionUri(targetStruct *Target) string {
	if len(targetStruct.Uri) != 0 {
		targetFromURi := getTargetFromYBUri(target.Uri)
		targetStruct.User = targetFromURi.User
		targetStruct.Database = targetFromURi.Database
		targetStruct.Password = targetFromURi.Password
		targetStruct.Host = targetFromURi.Host
		targetStruct.Port = targetFromURi.Port
		return targetStruct.Uri
	}
	uri := fmt.Sprintf("postgres://%s:%s@%s:%s/%s",
		targetStruct.User, targetStruct.Password, targetStruct.Host, targetStruct.Port, targetStruct.Database)
	targetStruct.Uri = uri
	return uri
}

func importData() {
	// TODO: Add later
	// acquireImportLock()
	// defer os.Remove(importLockFile)
	targets := getYBServers()
	var parallelism = parallelImportJobs
	if parallelism == -1 {
		parallelism = len(targets)
	}
	fmt.Printf("Number of parallel imports at a time %d\n", parallelism)
	splitFilesChannel := make(chan *fwk.SplitFileImportTask, SPLIT_FILE_CHANNEL_SIZE)
	targetServerChannel := make(chan *Target, 1)
	go roundRobinTargets(targets, targetServerChannel)
	go generateSmallerSplits(splitFilesChannel)
	go doImport(splitFilesChannel, parallelism, targetServerChannel)
	checkForDone()
	fmt.Printf("\nDone. Exiting\n")
	fmt.Print("\033[?25h") // Hide the cursor
}

func checkForDone() {
	doLoop := true
	for doLoop {
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
				doLoop = false
				fmt.Printf("All imports done\n")
				Done.Set()
			}
		} else {
			time.Sleep(5 * time.Second)
		}
	}

}

func roundRobinTargets(targets []*Target, channel chan *Target) {
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
	truncateRemainingTables(remainingTables)
	// fmt.Printf("%v, %v, %v, %s", done, interrupted, remaining, err)
	splitDataFiles(doneTables, interruptedTables, remainingTables, taskQueue)
}

func truncateRemainingTables(tables []string) {
	url := getTargetConnectionUri(&target)
	conn, err := pgx.Connect(context.Background(), url)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}
	defer conn.Close(context.Background())

	for _, tab := range tables {
		fmt.Printf("Truncating table %s\n", tab)
		truncateStmnt := fmt.Sprintf("truncate table %s", tab)
		rows, err := conn.Query(context.Background(), truncateStmnt)
		if err != nil {
			log.Fatal(err)
		}
		rows.Close()
	}
}

func splitDataFiles(doneTables []string, interruptedTables []string, remainingTables []string,
	taskQueue chan *fwk.SplitFileImportTask) {
	allTables := append(interruptedTables, remainingTables...)
	// fmt.Printf("all tables = %v\n", allTables)
	// fmt.Printf("done tables = %v\n", doneTables)

	for _, t := range allTables {
		origDataFile := exportDir + "/data/" + t + "_data.sql"
		// collect interrupted splits
		// make an import task and schedule them
		alreadyImported := false
		for _, dt := range doneTables {
			// fmt.Printf("Comparing %s and %s and string compare = %d\n", dt, t, strings.Compare(dt, t))
			if strings.Compare(dt, t) == 0 {
				alreadyImported = true
				break
			}
		}
		if alreadyImported {
			continue
		}
		largestCreatedSplitSoFar := 0
		largestOffsetSoFar := 0
		fileFullySplit := false
		pattern := fmt.Sprintf("%s/%s/data/%s.*", exportDir, metaInfoDir, t)
		matches, _ := filepath.Glob(pattern)
		// in progress are interrupted ones
		interruptedRegexStr := fmt.Sprintf(".+/%s\\.(\\d+)\\.(\\d+)\\.[P]$", t)
		interruptedRegexp := regexp.MustCompile(interruptedRegexStr)
		for _, filename := range matches {
			// fmt.Printf("Matched file name = %s\n", filename)
			submatches := interruptedRegexp.FindAllStringSubmatch(filename, -1)
			for _, match := range submatches {
				// This means a match. Submit the task with interrupted = true
				splitNum, _ := strconv.Atoi(match[1])
				offset, _ := strconv.Atoi(match[2])
				if offset == -1 {
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
		createdButNotStartedRegexStr := fmt.Sprintf(".+/%s\\.(\\d+)\\.(\\d+)\\.[C]$", t)
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
				if splitNum == 0 { // TODO: Constant needed
					fileFullySplit = true
				}
				// for interrupted files offsets are already determined
				addASplitTask("", t, filename, splitNum, true, taskQueue)
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
		numLinesTaken += 1
		if readLineErr == nil && !isDataLine(line) {
			continue
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
		numLinesInThisSplit += 1
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
			splitFile := fmt.Sprintf("%s/%s/data/%s.%d.%d.C",
				exportDir, metaInfoDir, t, fileSplitNumber, numLinesTaken)
			err = os.Rename(currTmpFileName, splitFile)
			if err != nil {
				log.Printf("Cannot rename %s to %s\n", currTmpFileName, splitFile)
				return
			}
			addASplitTask("", t, splitFile, splitNum, true, taskQueue)
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

func getPreSQLs(file string) []string {
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

	exportDonePath := metaInfoDataDir + "/exportDone"
	_, err = os.Stat(exportDonePath)
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
	pat := regexp.MustCompile(`.+/(\S+)_data.sql`)
	var tables []string
	for _, v := range datafiles {
		tablenameMatches := pat.FindAllStringSubmatch(v, -1)
		for _, match := range tablenameMatches {
			tables = append(tables, match[1])
		}
	}

	// let's find out done first
	var doneTables []string
	for _, t := range tables {
		// split 0 means it is done
		donePattern := fmt.Sprintf("%s/%s/data/%s\\.0\\.\\d+\\.*D", exportDir, metaInfoDir, t)
		matches, _ := filepath.Glob(donePattern)
		// in progress are interrupted ones
		for _, filename := range matches {
			_, err = os.Stat(filename)
			if err == nil {
				fmt.Printf("err %s\n", err.Error())
				doneTables = append(doneTables, t)
			}
		}
	}

	// let's find out interrupted now
	// TODO Obsolete. Change accordingly
	var interruptedTables []string
	var remainingTables []string
	for _, t := range tables {
		fpstarted := fmt.Sprintf("%s/%s.splitstarted", metaInfoDataDir, t)
		fpdone := fmt.Sprintf("%s/%s.done", metaInfoDataDir, t)
		_, err1 := os.Stat(fpstarted)
		_, err2 := os.Stat(fpdone)
		if err1 == nil && err2 != nil {
			interruptedTables = append(interruptedTables, t)
		} else if err1 != nil && err2 != nil {
			remainingTables = append(remainingTables, t)
		}
	}

	return doneTables, interruptedTables, remainingTables, nil
}

func doImport(taskQueue chan *fwk.SplitFileImportTask, parallelism int, targetChan chan *Target) {
	parallelImportChannel := make(chan *fwk.SplitFileImportTask, parallelism)
	go doImportInParallel(parallelImportChannel, targetChan)
	for Done.IsNotSet() {
		select {
		case t := <-taskQueue:
			// fmt.Printf("Got taskfile = %s putting on parallel channel\n", t.SplitFilePath)
			parallelImportChannel <- t
		default:
			// fmt.Printf("No file sleeping for 2 seconds\n")
			time.Sleep(2 * time.Second)
		}
	}
}

func doImportInParallel(channel chan *fwk.SplitFileImportTask, targetChan chan *Target) {
	for Done.IsNotSet() {
		select {
		case t := <-channel:
			// fmt.Printf("Got taskfile from parallel = %s\n", t.SplitFilePath)
			go doOneImport(t, targetChan)
		default:
			// fmt.Printf("No file sleeping for 2 seconds\n")
			time.Sleep(100 * time.Millisecond)
		}
	}
}

func doOneImport(t *fwk.SplitFileImportTask, targetChan chan *Target) {
	splitImportDone := false
	for !splitImportDone {
		select {
		case targetServer := <-targetChan:
			// fmt.Printf("Got taskfile %s and target for that = %s\n", t.SplitFilePath, targetServer.Host)
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
			copyCommand := fmt.Sprintf("COPY %s from '%s';", t.TableName, getInProgressFilePath(t))
			_, err = bufferedWriter.WriteString(copyCommand)
			if err != nil {
				panic(err)
			}
			err = bufferedWriter.Flush()
			if err != nil {
				panic(err)
			}
			err = executeSqlFile(sqlFile, targetServer)
			if err != nil {
				panic(err)
			}
			err = os.Rename(getInProgressFilePath(t), getDoneFilePath(t))
			if err != nil {
				panic(err)
			}
			fmt.Printf("Inserted a batch of %d or less in table %s\n", numLinesInASplit, t.TableName)
			splitImportDone = true
		default:
			// fmt.Printf("No server sleeping for 2 seconds\n")
			time.Sleep(200 * time.Millisecond)
		}
	}
}

func executeSqlFile(file string, server *Target) error {
	cmd := exec.Command(YSQL, "-f", file)
	return cmd.Run()
}

func getInProgressFilePath(task *fwk.SplitFileImportTask) string {
	path := task.SplitFilePath
	base := filepath.Base(path)
	dir  := filepath.Dir(path)
	parts := strings.Split(base, ".")
	return fmt.Sprintf("%s/%s.%s.%s.P", dir, parts[0], parts[1], parts[2])
}

func getDoneFilePath(task *fwk.SplitFileImportTask) string {
	path := task.SplitFilePath
	base := filepath.Base(path)
	dir  := filepath.Dir(path)
	parts := strings.Split(base, ".")
	return fmt.Sprintf("%s/%s.%s.%s.D", dir, parts[0], parts[1], parts[2])
}

func getSqlFilePath(task *fwk.SplitFileImportTask) string {
	path := task.SplitFilePath
	base := filepath.Base(path)
	dir  := filepath.Dir(path)
	parts := strings.Split(base, ".")
	return fmt.Sprintf("%s/%s.%s.%s.sql", dir, parts[0], parts[1], parts[2])
}

func init() {
	importCmd.AddCommand(importDataCmd)

	importDataCmd.PersistentFlags().StringVar(&importMode, "offline", "",
		"By default the data migration mode is offline. Use '--mode online' to change the mode to online migration")
	importDataCmd.PersistentFlags().IntVar(&numLinesInASplit, "batch-size", 1000,
		"Maximum size of each batch import ")
	importDataCmd.PersistentFlags().IntVar(&parallelImportJobs, "parallel-jobs", -1,
		"-1 means number of servers in the Yugabyte cluster")
}
