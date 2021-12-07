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
  "fmt"
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
)

func reportCase(fn string, reason string, issue string) {
    canMigrate = false
    casenum++
    if casenum > 1 {
        report += ","
    }
    report += "\n\"case " + strconv.Itoa(casenum) + "\": {"
    report += "\n\"reason\":\"" + reason + "\","
    report += "\n \"file\": \"" + fn + "\","
    report += "\n \"GH\": \"" + issue + "\"}"
}

// Checks whether there is gist index
func checkGist(strArray []string, fn string) {
    gistRegex := regexp.MustCompile("[create|CREATE] INDEX ([a-zA-Z0-9_]+).*USING GIST")
    for _, line := range strArray {
        if index := gistRegex.FindStringSubmatch(line); index != nil {
            reportCase(fn, "Schema contains gist index which is not supported. The index is: " + index[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1337")
        }
    }
}

// Checks compatibility of views
func checkViews(strArray []string, fn string) {
    matViewRegex := regexp.MustCompile("MATERIALIZED[ \t\n]+VIEW ([a-zA-Z0-9_]+)")
    viewWithCheckRegex := regexp.MustCompile("VIEW[ \t\n]+([a-zA-Z0-9_]+).*WITH CHECK OPTION")
    for _, line := range strArray {
        if view := matViewRegex.FindStringSubmatch(line); view != nil {
            reportCase(fn, "Schema contains materialized view which is not supported. The view is: " + view[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10102")
        } else if view := viewWithCheckRegex.FindStringSubmatch(line); view != nil {
            reportCase(fn, "Schema containing VIEW WITH CHECK OPTION is not supported yet. The view is: " + view[1], "")
        }
    }
}

// Checks compatibility of SQL statements
func checkSql(strArray []string, fn string) {
    rangeRegex := regexp.MustCompile("PRECEDING[ \t\n]+and[ \t\n]+.*:float")
    for _, line := range strArray {
        if rangeRegex.MatchString(line) {
            reportCase(fn,
                "RANGE with offset PRECEDING/FOLLOWING is not supported for column type numeric and offset type double precision",
                "https://github.com/yugabyte/yugabyte-db/issues/10692")
        }
    }
}

func reportAddingPrimaryKey(fn string, tbl string) {
    reportCase(fn, "Adding primary key to a partitioned table is not yet implemented. Table is: " + tbl,
        "https://github.com/yugabyte/yugabyte-db/issues/10074")
}

// Checks unsupported DDL statements
func checkDDL(strArray []string, fn string) {
    amRegex := regexp.MustCompile("CREATE ACCESS METHOD ([a-zA-Z0-9_]+)")
    idxConcRegex := regexp.MustCompile("REINDEX .*CONCURRENTLY ([a-zA-Z0-9_]+)")
    storedRegex := regexp.MustCompile("([a-zA-Z0-9_]+) [a-zA-Z0-9_]+ GENERATED ALWAYS .* STORED")
    createTblRegex := regexp.MustCompile("CREATE TABLE ")
    createViewRegex := regexp.MustCompile("CREATE VIEW ")
    likeAllRegex := regexp.MustCompile("CREATE TABLE ([a-zA-Z0-9_]+) .*LIKE .*INCLUDING ALL")
    inheritRegex := regexp.MustCompile("CREATE TABLE ([a-zA-Z0-9_]+) .*INHERITS")
    withOidsRegex := regexp.MustCompile("CREATE TABLE ([a-zA-Z0-9_]+) .*WITH OIDS")

    alterOfRegex := regexp.MustCompile("ALTER TABLE ([a-zA-Z0-9_]+).* OF ")
    alterNotOfRegex := regexp.MustCompile("ALTER TABLE ([a-zA-Z0-9_]+).* NOT OF")
    alterColumnRegex := regexp.MustCompile("ALTER TABLE ([a-zA-Z0-9_]+).* ALTER [column|COLUMN]")
    clusterRegex := regexp.MustCompile("ALTER TABLE ([a-zA-Z0-9_]+).* CLUSTER")
    // the following regex is not used since the partitioned table creation statement implies this information
    // partTblRegex := regexp.MustCompile("CREATE TABLE ([a-zA-Z0-9_]+) .*PARTITION BY")

    // table partition. partitioned table is the key in tblParts map
    tblPartitionRegex := regexp.MustCompile("CREATE TABLE ([a-zA-Z0-9_]+) .*PARTITION OF ([a-zA-Z0-9_]+)")
    addPrimaryRegex := regexp.MustCompile("ALTER TABLE ([a-zA-Z0-9_]+) .*ADD PRIMARY KEY")

    for _, line := range strArray {
        if createTblRegex.FindStringSubmatch(line) != nil {
            tablecnt++
        } else if createViewRegex.FindStringSubmatch(line) != nil {
            viewcnt++
        }
        if am := amRegex.FindStringSubmatch(line); am != nil {
            reportCase(fn, "CREATE ACCESS METHOD is not supported. ACCESS METHOD: " + am[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10693")
        } else if tbl := idxConcRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "REINDEX CONCURRENTLY is not supported. Table to be indexed: " + tbl[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10694")
        } else if col := storedRegex.FindStringSubmatch(line); col != nil {
            reportCase(fn, "Stored generated column is not supported. Column is: " + col[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10695")
        } else if tbl := likeAllRegex.FindStringSubmatch(line); tbl != nil {
            modtablecnt++;
            reportCase(fn, "LIKE ALL is not supported yet. Table is: " + tbl[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10697")
        } else if tbl := tblPartitionRegex.FindStringSubmatch(line); tbl != nil {
            tblParts[tbl[1]] = tbl[2]
            if filename, ok := primaryCons[tbl[1]]; ok {
                reportAddingPrimaryKey(filename, tbl[1])
            }
        } else if tbl := addPrimaryRegex.FindStringSubmatch(line); tbl != nil {
            if _, ok := tblParts[tbl[1]]; ok {
                reportAddingPrimaryKey(fn, tbl[1])
            }
            primaryCons[tbl[1]] = fn
        } else if tbl := inheritRegex.FindStringSubmatch(line); tbl != nil {
            modtablecnt++;
            reportCase(fn, "INHERITS not supported yet. Table is: " + tbl[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1129")
        } else if tbl := withOidsRegex.FindStringSubmatch(line); tbl != nil {
            modtablecnt++;
            reportCase(fn, "OIDs are not supported for user tables. Table is: " + tbl[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10273")
        } else if tbl := alterOfRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE OF not supported yet. Table is: " + tbl[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1124")
        } else if tbl := alterNotOfRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE NOT OF not supported yet. Table is: " + tbl[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1124")
        } else if tbl := alterColumnRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE ALTER column not supported yet. Table is: " + tbl[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1124")
        } else if tbl := clusterRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "ALTER TABLE CLUSTER not supported yet. Table is: " + tbl[1],
                "https://github.com/YugaByte/yugabyte-db/issues/1124")
        }
    }
}

// check foreign table
func checkForeign(strArray []string, fn string) {
    primRegex := regexp.MustCompile("CREATE FOREIGN TABLE ([a-zA-Z0-9_]+).*PRIMARY KEY")
    foreignKeyRegex := regexp.MustCompile("CREATE FOREIGN TABLE ([a-zA-Z0-9_]+).*REFERENCES")
    for _, line := range strArray {
        if tbl := primRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "Primary key constraints are not supported on foreign tables. Table is: " + tbl[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10698")
        } else if tbl := foreignKeyRegex.FindStringSubmatch(line); tbl != nil {
            reportCase(fn, "Foreign key constraints are not supported on foreign tables. Table is: " + tbl[1],
                "https://github.com/yugabyte/yugabyte-db/issues/10699")
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
        summary := "\n\n" + fn + " has items to be modified before migration\n"
        log.Println(summary)
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
        fmt.Println(path, info.Size())
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
