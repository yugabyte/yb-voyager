/*
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
  "path/filepath"
  // "fmt"
  "log"
  "os"
  "regexp"
  "strconv"
  "strings"

  _ "github.com/lib/pq"
	"github.com/spf13/cobra"
)

var (
  canMigrate = true
  report     = "{"
  tblParts   = make(map[string]string)
  // key is partitioned table, value is filename where the ADD PRIMARY KEY statement resides
  primaryCons= make(map[string]string)
  casenum    = 0
  tablecnt   = 0
  modtablecnt= 0
  viewcnt    = 0
  multiRegex = regexp.MustCompile(`([a-zA-Z0-9_\.]+[,|;])`)

  gistRegex  = regexp.MustCompile("(?i)CREATE INDEX (IF NOT EXISTS )?([a-zA-Z0-9_]+).*USING GIST")
  brinRegex  = regexp.MustCompile("(?i)CREATE INDEX on ([a-zA-Z0-9_]+).*USING brin")
  spgistRegex= regexp.MustCompile("(?i)CREATE INDEX on ([a-zA-Z0-9_]+).*USING spgist")
  rtreeRegex = regexp.MustCompile("(?i)CREATE INDEX on ([a-zA-Z0-9_]+).*USING rtree")
  matViewRegex = regexp.MustCompile("(?i)MATERIALIZED[ \t\n]+VIEW ([a-zA-Z0-9_]+)")
  viewWithCheckRegex = regexp.MustCompile("(?i)VIEW[ \t\n]+([a-zA-Z0-9_]+).*WITH CHECK OPTION")
  rangeRegex = regexp.MustCompile("(?i)PRECEDING[ \t\n]+and[ \t\n]+.*:float")
  alterAggRegex = regexp.MustCompile("(?i)ALTER AGGREGATE")
  dropCollRegex = regexp.MustCompile("(?i)DROP COLLATION ([a-zA-Z0-9_]+), ([a-zA-Z0-9_]+)")
  dropIdxRegex = regexp.MustCompile("(?i)DROP INDEX ([a-zA-Z0-9_]+), ([a-zA-Z0-9_]+)")
  dropViewRegex = regexp.MustCompile("(?i)DROP VIEW ([a-zA-Z0-9_]+), ([a-zA-Z0-9_]+)")
  dropSeqRegex = regexp.MustCompile("(?i)DROP SEQUENCE ([a-zA-Z0-9_]+), ([a-zA-Z0-9_]+)")
  dropForeignRegex = regexp.MustCompile("(?i)DROP FOREIGN TABLE ([a-zA-Z0-9_]+), ([a-zA-Z0-9_]+)")
  dropMatViewRegex = regexp.MustCompile("(?i)DROP MATERIALIZED VIEW")
  concurIdxRegex = regexp.MustCompile("(?i)CREATE INDEX CONCURRENTLY")
  trigRefRegex = regexp.MustCompile("(?i)CREATE TRIGGER ([a-zA-Z0-9_]+).*REFERENCING")
  constrTrgRegex = regexp.MustCompile("(?i)CREATE CONSTRAINT TRIGGER ([a-zA-Z0-9_]+)")
  currentOfRegex = regexp.MustCompile("(?i)WHERE CURRENT OF")
  amRegex = regexp.MustCompile("(?i)CREATE ACCESS METHOD ([a-zA-Z0-9_]+)")
  idxConcRegex = regexp.MustCompile("(?i)REINDEX .*CONCURRENTLY ([a-zA-Z0-9_]+)")
  storedRegex = regexp.MustCompile("(?i)([a-zA-Z0-9_]+) [a-zA-Z0-9_]+ GENERATED ALWAYS .* STORED")
  createTblRegex = regexp.MustCompile("(?i)CREATE ([a-zA-Z_]+ )?TABLE ")
  createViewRegex = regexp.MustCompile("(?i)CREATE VIEW ")
  likeAllRegex = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_]+) .*LIKE .*INCLUDING ALL")
  likeRegex = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_]+) .*\\(like")
  inheritRegex = regexp.MustCompile("(?i)CREATE ([a-zA-Z_]+ )?TABLE (IF NOT EXISTS )?([a-zA-Z0-9_]+).*INHERITS[ |\\(]")
  withOidsRegex = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_]+) .*WITH OIDS")
  intvlRegex = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_]+) .*interval PRIMARY")

  alterOfRegex = regexp.MustCompile("(?i)ALTER TABLE (IF EXISTS )?([a-zA-Z0-9_]+).* OF ")
  alterNotOfRegex = regexp.MustCompile("(?i)ALTER TABLE (IF EXISTS )?([a-zA-Z0-9_]+).* NOT OF")
  alterColumnRegex = regexp.MustCompile("(?i)ALTER TABLE (IF EXISTS )?([a-zA-Z0-9_]+).* ALTER [column|COLUMN]")
  alterConstrRegex = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?(IF EXISTS )?TABLE ([a-zA-Z0-9_]+).* ALTER CONSTRAINT")
  setOidsRegex = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_]+).* SET WITH OIDS")
  clusterRegex = regexp.MustCompile("(?i)ALTER TABLE (IF EXISTS )?([a-zA-Z0-9_]+).* CLUSTER")
  withoutClusterRegex = regexp.MustCompile("(?i)ALTER TABLE (IF EXISTS )?([a-zA-Z0-9_]+).* SET WITHOUT CLUSTER")
  alterSetRegex = regexp.MustCompile("(?i)ALTER TABLE (IF EXISTS )?([a-zA-Z0-9_]+) SET ")
  alterIdxRegex = regexp.MustCompile("(?i)ALTER INDEX ([a-zA-Z0-9_]+) SET ")
  alterResetRegex = regexp.MustCompile("(?i)ALTER TABLE (IF EXISTS )?([a-zA-Z0-9_]+) RESET ")
  alterOptionsRegex = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_]+) OPTIONS")
  alterInhRegex = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_]+) INHERIT")
  valConstrRegex = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_]+) VALIDATE CONSTRAINT")
  deferRegex = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_]+).* unique .*deferrable")

  dropAttrRegex = regexp.MustCompile("(?i)ALTER TYPE ([a-zA-Z0-9_]+) DROP ATTRIBUTE")
  alterTypeRegex = regexp.MustCompile("(?i)ALTER TYPE ([a-zA-Z0-9_]+)")
  alterTblSpcRegex = regexp.MustCompile("(?i)ALTER TABLESPACE ([a-zA-Z0-9_]+) SET")

  // table partition. partitioned table is the key in tblParts map
  tblPartitionRegex = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_]+) .*PARTITION OF ([a-zA-Z0-9_]+)")
  addPrimaryRegex = regexp.MustCompile("(?i)ALTER TABLE (IF EXISTS )?([a-zA-Z0-9_]+) .*ADD PRIMARY KEY")
  primRegex = regexp.MustCompile("(?i)CREATE FOREIGN TABLE ([a-zA-Z0-9_]+).*PRIMARY KEY")
  foreignKeyRegex = regexp.MustCompile("(?i)CREATE FOREIGN TABLE ([a-zA-Z0-9_]+).*REFERENCES")
)

// Reports one case in JSON
func reportCase(fn string, reason string, issue string, suggestion string) {
    canMigrate = false
    casenum++
    if casenum > 1 {
        report += ","
    }
    report += "\n\"case " + strconv.Itoa(casenum) + "\": {"
    report += "\n\"reason\":\"" + reason + "\","
    report += "\n \"file\": \"" + fn + "\","
    if suggestion != "" {
        report += "\n \"suggestion\": \"" + suggestion + "\","
    }
    report += "\n \"GH\": \"" + issue + "\"}"
}

// Checks whether there is gist index
func checkGist(strArray []string, fn string) {
    for _, line := range strArray {
        if index := gistRegex.FindStringSubmatch(line); index != nil {
            reportCase(fn, "Schema contains gist index which is not supported. The index is: " + index[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1337", "")
        } else if idx := brinRegex.FindStringSubmatch(line); idx != nil {
            reportCase(fn, "index method 'brin' not supported yet. Table is: " + idx[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1337", "")
        } else if idx := spgistRegex.FindStringSubmatch(line); idx != nil {
            reportCase(fn, "index method 'spgist' not supported yet. Table is: " + idx[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1337", "")
        } else if idx := rtreeRegex.FindStringSubmatch(line); idx != nil {
            reportCase(fn, "index method 'rtree' is superceded by 'gist' which is not supported yet. Table is: " + idx[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1337", "")
        }
    }
}

// Checks compatibility of views
func checkViews(strArray []string, fn string) {
    for _, line := range strArray {
        if dropMatViewRegex.MatchString(line) {
            reportCase(fn, "DROP MATERIALIZED VIEW not supported yet.",
                "https://github.com/YugaByte/yugabyte-db/issues/10102", "")
        } else if view := matViewRegex.FindStringSubmatch(line); view != nil {
            reportCase(fn, "Schema contains materialized view which is not supported. The view is: " + view[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10102", "")
        } else if view := viewWithCheckRegex.FindStringSubmatch(line); view != nil {
            reportCase(fn, "Schema containing VIEW WITH CHECK OPTION is not supported yet. The view is: " + view[1], "", "")
        }
    }
}

// Separates the input line into multiple statements which are accepted by YB.
func separateMultiObj(objType string, line string) string {
    indexes := multiRegex.FindAllStringSubmatchIndex(line, -1)
    suggestion := ""
    for _, match := range indexes {
        start := match[2]
        end := match[3]
        obj := strings.Replace(line[start:end], ",", ";", -1)
        suggestion += objType + " " + obj
    }
    return suggestion
}

// Checks compatibility of SQL statements
func checkSql(strArray []string, fn string) {
    for _, line := range strArray {
        if rangeRegex.MatchString(line) {
            reportCase(fn,
                "RANGE with offset PRECEDING/FOLLOWING is not supported for column type numeric and offset type double precision",
                "https://github.com/yugabyte/yugabyte-db/issues/10692", "")
        } else if alterAggRegex.MatchString(line) {
            reportCase(fn, "ALTER AGGREGATE not supported yet.",
                "https://github.com/YugaByte/yugabyte-db/issues/2717", "")
        } else if dropCollRegex.MatchString(line) {
            reportCase(fn, "DROP multiple objects not supported yet.",
                "https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP COLLATION", line))
        } else if dropIdxRegex.MatchString(line) {
            reportCase(fn, "DROP multiple objects not supported yet.",
                "https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP INDEX", line))
        } else if dropViewRegex.MatchString(line) {
            reportCase(fn, "DROP multiple objects not supported yet.",
                "https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP VIEW", line))
        } else if dropSeqRegex.MatchString(line) {
            reportCase(fn, "DROP multiple objects not supported yet.",
                "https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP SEQUENCE", line))
        } else if dropForeignRegex.MatchString(line) {
            reportCase(fn, "DROP multiple objects not supported yet.",
                "https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP FOREIGN TABLE", line))
        } else if concurIdxRegex.MatchString(line) {
            reportCase(fn, "CREATE INDEX CONCURRENTLY not supported yet",
                "https://github.com/yugabyte/yugabyte-db/issues/10799", "")
        } else if trig := trigRefRegex.FindStringSubmatch(line); trig != nil {
            reportCase(fn, "REFERENCING clause (transition tables) not supported yet. Trigger is: " + trig[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1668", "")
        } else if trig := constrTrgRegex.FindStringSubmatch(line); trig != nil {
            reportCase(fn, "CREATE CONSTRAINT TRIGGER not supported yet. Trigger is: " + trig[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1709", "")
        } else if currentOfRegex.MatchString(line) {
            reportCase(fn, "WHERE CURRENT OF not supported yet", "https://github.com/YugaByte/yugabyte-db/issues/737", "")
        }
    }
}

func reportAddingPrimaryKey(fn string, tbl string) {
    reportCase(fn, "Adding primary key to a partitioned table is not yet implemented. Table is: " + tbl,
        "https://github.com/yugabyte/yugabyte-db/issues/10074", "")
}

// Checks unsupported DDL statements
func checkDDL(strArray []string, fn string) {

    for _, line := range strArray {
        if createTblRegex.FindStringSubmatch(line) != nil {
            tablecnt++
        } else if createViewRegex.FindStringSubmatch(line) != nil {
            viewcnt++
        }
        if am := amRegex.FindStringSubmatch(line); am != nil {
            reportCase(fn, "CREATE ACCESS METHOD is not supported. ACCESS METHOD: " + am[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10693", "")
        } else if tbl := idxConcRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "REINDEX CONCURRENTLY is not supported. Table to be indexed: " + tbl[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10694", "")
        } else if col := storedRegex.FindStringSubmatch(line); col != nil {
            reportCase(fn, "Stored generated column is not supported. Column is: " + col[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10695", "")
        } else if tbl := likeAllRegex.FindStringSubmatch(line); tbl != nil {
            modtablecnt++;
            reportCase(fn, "LIKE ALL is not supported yet. Table is: " + tbl[2],
                "https://github.com/yugabyte/yugabyte-db/issues/10697", "")
        } else if tbl := likeRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "LIKE clause not supported yet. Table is: " + tbl[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1129", "")
        } else if tbl := tblPartitionRegex.FindStringSubmatch(line); tbl != nil {
            tblParts[tbl[2]] = tbl[3]
            if filename, ok := primaryCons[tbl[2]]; ok {
                reportAddingPrimaryKey(filename, tbl[2])
            }
        } else if tbl := addPrimaryRegex.FindStringSubmatch(line); tbl != nil {
            if _, ok := tblParts[tbl[2]]; ok {
                reportAddingPrimaryKey(fn, tbl[2])
            }
            primaryCons[tbl[2]] = fn
        } else if tbl := inheritRegex.FindStringSubmatch(line); tbl != nil {
            modtablecnt++;
            reportCase(fn, "INHERITS not supported yet. Table is: " + tbl[3],
                "https://github.com/YugaByte/yugabyte-db/issues/1129", "")
        } else if tbl := withOidsRegex.FindStringSubmatch(line); tbl != nil {
            modtablecnt++;
            reportCase(fn, "OIDs are not supported for user tables. Table is: " + tbl[2],
                "https://github.com/yugabyte/yugabyte-db/issues/10273", "")
        } else if tbl := intvlRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "PRIMARY KEY containing column of type 'INTERVAL' not yet supported. Table is: " + tbl[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1397", "")
        } else if tbl := alterOfRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE OF not supported yet. Table is: " + tbl[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := alterNotOfRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE NOT OF not supported yet. Table is: " + tbl[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := alterColumnRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE ALTER column not supported yet. Table is: " + tbl[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := alterConstrRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE ALTER CONSTRAINT not supported yet. Table is: " + tbl[3],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := setOidsRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE SET WITH OIDS not supported yet. Table is: " + tbl[3],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := withoutClusterRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE SET WITHOUT CLUSTER not supported yet. Table is: " + tbl[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := clusterRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE CLUSTER not supported yet. Table is: " + tbl[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := alterSetRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE SET not supported yet. Table is: " + tbl[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := alterIdxRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE SET not supported yet. Table is: " + tbl[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := alterResetRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE RESET not supported yet. Table is: " + tbl[2],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := alterOptionsRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE not supported yet. Table is: " + tbl[3],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if typ := dropAttrRegex.FindStringSubmatch(line); typ != nil {
            reportCase(fn, "ALTER TYPE DROP ATTRIBUTE not supported yet. Type is: " + typ[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1893", "")
        } else if typ := alterTypeRegex.FindStringSubmatch(line); typ != nil {
            reportCase(fn, "ALTER TYPE not supported yet. Type is: " + typ[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1893", "")
        } else if tbl := alterInhRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE INHERIT not supported yet. Table is: " + tbl[3],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "")
        } else if tbl := valConstrRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE VALIDATE CONSTRAINT not supported yet. Table is: " + tbl[3],
                "https://github.com/YugaByte/yugabyte-db/issues/1124", "");
        } else if tbl := deferRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "DEFERRABLE unique constraints are not supported yet. Table is: " + tbl[3],
                "https://github.com/YugaByte/yugabyte-db/issues/1129", "")
        } else if spc := alterTblSpcRegex.FindStringSubmatch(line); spc != nil {
            reportCase(fn, "ALTER TABLESPACE not supported yet. Tablespace is: " + spc[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1153", "")
        }
    }
}

// check foreign table
func checkForeign(strArray []string, fn string) {
    for _, line := range strArray {
        if tbl := primRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "Primary key constraints are not supported on foreign tables. Table is: " + tbl[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10698", "")
        } else if tbl := foreignKeyRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "Foreign key constraints are not supported on foreign tables. Table is: " + tbl[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10699", "")
        }
    }
}

// Checks whether the script, fn, can be migrated to YB
func checkScript(fn string) {
    file, err := os.Open(fn)
    if err != nil {
        panic(err)
    }
    defer file.Close()

    scanner := bufio.NewScanner(file)
    var strArray []string
    line := ""
    // assemble array of lines, each line ends with semicolon
    for scanner.Scan() {
        curr := scanner.Text()
        if strings.HasPrefix(curr, "--") {
            continue
        }
        // ignore insertions
        if strings.HasPrefix(strings.ToLower(curr), "insert") {
            continue
        }
        if strings.HasPrefix(curr, " ") {
            line += curr
        } else {
            line += " " + curr
        }
        if strings.Contains(curr, ";") {
            strArray = append(strArray, line)
            line = ""
        }
    }
    // check whether there was error reading the script
    if scanner.Err() != nil {
        panic(scanner.Err())
    }
    checkViews(strArray, fn)
    checkSql(strArray, fn)
    checkGist(strArray, fn)
    checkDDL(strArray, fn)
    checkForeign(strArray, fn)

    if canMigrate {
        log.Println("Schema in " + fn + " can be migrated to Yugabyte DB")
    } else {
        // log.Println("\n\n" + fn + " has items to be modified before migration\n")
    }
}

// The command expects path to the directory containing .sql scripts followed by
// the filename to the summary report
func checker(args []string) {
    if len(args) < 1 {
        log.Fatal("Please specify path to the directory containing .sql scripts," +
        " followed by the filename to the summary report")
    }
    if len(args) < 2 {
        log.Fatal("Please specify path/filename to the summary report")
    }
    err := filepath.Walk(args[0],
      func(path string, info os.FileInfo, err error) error {
        if err != nil {
          return err
        }
        if strings.HasSuffix(info.Name(), ".sql") {
            checkScript(path)
        }
        // fmt.Println(path, info.Size())
        return nil
    })
    if err != nil {
        log.Println(err)
    }
    f, err := os.OpenFile(args[1], os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0644)
    if err != nil {
        panic(err)
    }
    defer f.Close()
    viewstr := ",\n\"view\":" + strconv.Itoa(viewcnt) + ","
    tablestr := "\n\"table\":{" + "\"count\":" + strconv.Itoa(tablecnt) + ", \"modification needed\":" + strconv.Itoa(modtablecnt) + "}"
    f.WriteString(report + viewstr + tablestr +"\n}")
}


// checkerCmd represents the checker command
var checkerCmd = &cobra.Command{
	Use:   "checker",
	Short: "command for checking .sql files under given directory for YB incompatible constructs",
	Long: `Sample command line:
  yb_migrate checker <dir> <path-to-json-output>
where <dir> can have subdirectories containing .sql files

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	Run: func(cmd *cobra.Command, args []string) {
        checker(args)
	},
}

func init() {
	rootCmd.AddCommand(checkerCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// checkerCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// checkerCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
