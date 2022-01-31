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
	"encoding/xml"
	"fmt"
	"strconv"
	"yb_migrate/src/migration"
	"yb_migrate/src/utils"

	"os"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

type summaryInfo struct {
	totalCount   int
	invalidCount int
	objSet       map[string]bool
	details      map[string]bool //any details about the object type
}

var (
	outputFormat  string
	sourceObjList []string
	canMigrate    = true
	report        string
	tblParts      = make(map[string]string)
	// key is partitioned table, value is filename where the ADD PRIMARY KEY statement resides
	primaryCons      = make(map[string]string)
	summaryMap       = make(map[string]*summaryInfo)
	casenum          = 0
	multiRegex       = regexp.MustCompile(`([a-zA-Z0-9_\.]+[,|;])`)
	dollarQuoteRegex = regexp.MustCompile("\\$.*\\$")
	createConvRegex  = regexp.MustCompile("(?i)CREATE (DEFAULT )?CONVERSION ([a-zA-Z0-9_.]+)")
	alterConvRegex   = regexp.MustCompile("(?i)ALTER CONVERSION ([a-zA-Z0-9_.]+)")
	gistRegex        = regexp.MustCompile("(?i)CREATE INDEX (IF NOT EXISTS )?([a-zA-Z0-9_.]+).*USING GIST")
	brinRegex        = regexp.MustCompile("(?i)CREATE INDEX on ([a-zA-Z0-9_.]+).*USING brin")
	spgistRegex      = regexp.MustCompile("(?i)CREATE INDEX on ([a-zA-Z0-9_.]+).*USING spgist")
	rtreeRegex       = regexp.MustCompile("(?i)CREATE INDEX on ([a-zA-Z0-9_.]+).*USING rtree")
	// matViewRegex       = regexp.MustCompile("(?i)MATERIALIZED[ \t\n]+VIEW ([a-zA-Z0-9_.]+)")
	viewWithCheckRegex = regexp.MustCompile("(?i)VIEW[ \t\n]+([a-zA-Z0-9_.]+).*WITH CHECK OPTION")
	rangeRegex         = regexp.MustCompile("(?i)PRECEDING[ \t\n]+and[ \t\n]+.*:float")
	fetchRegex         = regexp.MustCompile("(?i)FETCH .*FROM")
	fetchRelativeRegex = regexp.MustCompile("(?i)FETCH RELATIVE")
	backwardRegex      = regexp.MustCompile("(?i)MOVE BACKWARD")
	fetchAbsRegex      = regexp.MustCompile("(?i)FETCH ABSOLUTE")
	alterAggRegex      = regexp.MustCompile("(?i)ALTER AGGREGATE ([a-zA-Z0-9_.]+)")
	dropCollRegex      = regexp.MustCompile("(?i)DROP COLLATION (IF EXISTS )?[a-zA-Z0-9_.]+[ ]*(,)([ ]*(,)?[ ]*[a-zA-Z0-9_.]+)+")
	dropIdxRegex       = regexp.MustCompile("(?i)DROP INDEX (IF EXISTS )?[a-zA-Z0-9_.]+[ ]*(,)([ ]*(,)?[ ]*[a-zA-Z0-9_.]+)+")
	dropViewRegex      = regexp.MustCompile("(?i)DROP VIEW (IF EXISTS )?[a-zA-Z0-9_.]+[ ]*(,)([ ]*(,)?[ ]*[a-zA-Z0-9_.]+)+")
	dropSeqRegex       = regexp.MustCompile("(?i)DROP SEQUENCE (IF EXISTS )?[a-zA-Z0-9_.]+[ ]*(,)([ ]*(,)?[ ]*[a-zA-Z0-9_.]+)+")
	dropForeignRegex   = regexp.MustCompile("(?i)DROP FOREIGN TABLE (IF EXISTS )?[a-zA-Z0-9_.]+[ ]*(,)([ ]*(,)?[ ]*[a-zA-Z0-9_.]+)+")
	// dropMatViewRegex   = regexp.MustCompile("(?i)DROP MATERIALIZED VIEW")
	createIdxConcurRegex = regexp.MustCompile("(?i)CREATE (UNIQUE )?INDEX CONCURRENTLY (IF NOT EXISTS )?([a-zA-Z0-9_.]+)")
	dropIdxConcurRegex   = regexp.MustCompile("(?i)DROP INDEX CONCURRENTLY (IF EXISTS )?([a-zA-Z0-9_.]+)")
	trigRefRegex         = regexp.MustCompile("(?i)CREATE TRIGGER ([a-zA-Z0-9_.]+).*REFERENCING")
	constrTrgRegex       = regexp.MustCompile("(?i)CREATE CONSTRAINT TRIGGER ([a-zA-Z0-9_.]+)")
	currentOfRegex       = regexp.MustCompile("(?i)WHERE CURRENT OF")
	amRegex              = regexp.MustCompile("(?i)CREATE ACCESS METHOD ([a-zA-Z0-9_.]+)")
	idxConcRegex         = regexp.MustCompile("(?i)REINDEX .*CONCURRENTLY ([a-zA-Z0-9_.]+)")
	storedRegex          = regexp.MustCompile("(?i)([a-zA-Z0-9_]+) [a-zA-Z0-9_]+ GENERATED ALWAYS .* STORED")
	createTblRegex       = regexp.MustCompile("(?i)CREATE ([a-zA-Z_]+ )?TABLE ")
	createViewRegex      = regexp.MustCompile("(?i)CREATE VIEW ")
	likeAllRegex         = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_.]+) .*LIKE .*INCLUDING ALL")
	likeRegex            = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_.]+) .*\\(like")
	inheritRegex         = regexp.MustCompile("(?i)CREATE ([a-zA-Z_]+ )?TABLE (IF NOT EXISTS )?([a-zA-Z0-9_.]+).*INHERITS[ |\\(]")
	withOidsRegex        = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_.]+) .*WITH OIDS")
	intvlRegex           = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_.]+) .*interval PRIMARY")

	alterOfRegex        = regexp.MustCompile("(?i)ALTER TABLE (ONLY )?(IF EXISTS )?([a-zA-Z0-9_.]+).* OF ")
	alterSchemaRegex    = regexp.MustCompile("(?i)ALTER TABLE (ONLY )?(IF EXISTS )?([a-zA-Z0-9_.]+).* SET SCHEMA ")
	createSchemaRegex   = regexp.MustCompile("(?i)CREATE SCHEMA .* CREATE TABLE")
	alterNotOfRegex     = regexp.MustCompile("(?i)ALTER TABLE (ONLY )?(IF EXISTS )?([a-zA-Z0-9_.]+).* NOT OF")
	alterColumnRegex    = regexp.MustCompile("(?i)ALTER TABLE (ONLY )?(IF EXISTS )?([a-zA-Z0-9_.]+).* ALTER [column|COLUMN]")
	alterConstrRegex    = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?(IF EXISTS )?TABLE ([a-zA-Z0-9_.]+).* ALTER CONSTRAINT")
	setOidsRegex        = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_.]+).* SET WITH OIDS")
	clusterRegex        = regexp.MustCompile("(?i)ALTER TABLE (ONLY )?(IF EXISTS )?([a-zA-Z0-9_.]+).* CLUSTER")
	withoutClusterRegex = regexp.MustCompile("(?i)ALTER TABLE (ONLY )?(IF EXISTS )?([a-zA-Z0-9_.]+).* SET WITHOUT CLUSTER")
	alterSetRegex       = regexp.MustCompile("(?i)ALTER TABLE (ONLY )?(IF EXISTS )?([a-zA-Z0-9_.]+) SET ")
	alterIdxRegex       = regexp.MustCompile("(?i)ALTER INDEX ([a-zA-Z0-9_.]+) SET ")
	alterResetRegex     = regexp.MustCompile("(?i)ALTER TABLE (ONLY )?(IF EXISTS )?([a-zA-Z0-9_.]+) RESET ")
	alterOptionsRegex   = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_.]+) OPTIONS")
	alterInhRegex       = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_.]+) INHERIT")
	valConstrRegex      = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_.]+) VALIDATE CONSTRAINT")
	deferRegex          = regexp.MustCompile("(?i)ALTER ([a-zA-Z_]+ )?TABLE (IF EXISTS )?([a-zA-Z0-9_.]+).* unique .*deferrable")

	dropAttrRegex    = regexp.MustCompile("(?i)ALTER TYPE ([a-zA-Z0-9_.]+) DROP ATTRIBUTE")
	alterTypeRegex   = regexp.MustCompile("(?i)ALTER TYPE ([a-zA-Z0-9_.]+)")
	alterTblSpcRegex = regexp.MustCompile("(?i)ALTER TABLESPACE ([a-zA-Z0-9_.]+) SET")

	// table partition. partitioned table is the key in tblParts map
	tblPartitionRegex = regexp.MustCompile("(?i)CREATE TABLE (IF NOT EXISTS )?([a-zA-Z0-9_.]+) .*PARTITION OF ([a-zA-Z0-9_.]+)")
	addPrimaryRegex   = regexp.MustCompile("(?i)ALTER TABLE (ONLY )?(IF EXISTS )?([a-zA-Z0-9_.]+) .*ADD PRIMARY KEY")
	primRegex         = regexp.MustCompile("(?i)CREATE FOREIGN TABLE ([a-zA-Z0-9_.]+).*PRIMARY KEY")
	foreignKeyRegex   = regexp.MustCompile("(?i)CREATE FOREIGN TABLE ([a-zA-Z0-9_.]+).*REFERENCES")

	// unsupported SQLs exported by ora2pg
	compoundTrigRegex          = regexp.MustCompile("(?i)CREATE (OR REPLACE )?TRIGGER ([a-zA-Z0-9_.]+).*COMPOUND.*")
	unsupportedCommentRegex1   = regexp.MustCompile("(?i)--.*(unsupported)")
	packageSupportCommentRegex = regexp.MustCompile("(?i)--.*Oracle package '([a-zA-Z0-9_.]+)'.*please edit to match PostgreSQL syntax")
	unsupportedCommentRegex2   = regexp.MustCompile("(?i)--.*please edit to match PostgreSQL syntax")
	typeUnsupportedRegex       = regexp.MustCompile("(?i)Inherited types are not supported.*replacing with inherited table")
	bulkCollectRegex           = regexp.MustCompile("BULK COLLECT") // ora2pg unable to convert this oracle feature into a PostgreSQL compatible syntax
)

// Reports one case in JSON
func reportCase(fpath string, reason string, issue string, suggestion string, objType string, objName string, sqlStmt string) {
	canMigrate = false
	casenum++
	if casenum > 1 {
		report += ","
	}

	report += "{" //for each case's object in issues arrays
	// report += `"case ` + strconv.Itoa(casenum) + `": {`
	//TODO: add objectname, objecttype, Parent, action
	report += `"objectType": "` + objType + `",`
	report += `"objectName": "` + objName + `",`
	report += `"reason": "` + reason + `",`
	report += `"sqlStatement": "` + sqlStmt + `",`
	report += `"filePath": "` + fpath + `",`
	if suggestion != "" {
		report += `"suggestion": "` + suggestion + `",`
	}
	report += `"GH": "` + issue + `"}`
	// report += "}"
}

func reportAddingPrimaryKey(fpath string, tbl string, line string) {
	reportCase(fpath, "Adding primary key to a partitioned table is not yet implemented.",
		"https://github.com/yugabyte/yugabyte-db/issues/10074", "", "", tbl, line)
}

func reportBasedOnComment(comment int, fpath string, issue string, suggestion string, objName string, objType string, line string) {
	if comment == 1 {
		reportCase(fpath, "Unsupported, please edit to match PostgreSQL syntax", issue, suggestion, objType, "", line)
	} else if comment == 2 {
		// reportCase(fpath, "PACKAGE in oracle are exported as Schema, please review and edit to match PostgreSQL syntax if required, Package is "+objName, issue, suggestion, objType)
		summaryMap["PACKAGE"].objSet[objName] = true
	} else if comment == 3 {
		reportCase(fpath, "SQLs in file might be unsupported please review and edit to match PostgreSQL syntax if required. ", issue, suggestion, objType, "", line)
	} else if comment == 4 {
		summaryMap[objType].invalidCount += 1
		summaryMap[objType].details["Inherited Types are present which are not supported in PostgreSQL syntax. "] = true
	}

}

//create a json string from the info from summaryMap
func reportSummary() string {

	requiredJson := `"summary": {`
	requiredJson += fmt.Sprintf(`"dbName": "%s",`, source.DBName) +
		fmt.Sprintf(`"schemaName": "%s",`, source.Schema)

	requiredJson += `"databaseObjects": [`
	for _, objType := range sourceObjList {
		if summaryMap[objType].totalCount == 0 {
			continue
		}

		requiredJson += "{"
		requiredJson += fmt.Sprintf(`"objectType": "%s",`, objType)
		requiredJson += fmt.Sprintf(`"totalCount": %d,`, summaryMap[objType].totalCount)
		requiredJson += fmt.Sprintf(`"invalidCount": %d,`, summaryMap[objType].invalidCount)
		requiredJson += fmt.Sprintf(`"objectNames": "%s",`, getMapKeys(summaryMap[objType].objSet))
		requiredJson += fmt.Sprintf(`"details": "%s"`, getMapKeys(summaryMap[objType].details))
		requiredJson += "},"
	}

	//removing last comma(",")
	requiredJson = requiredJson[0 : len(requiredJson)-1]
	requiredJson += "]"

	requiredJson += "},"
	return requiredJson
}

// Checks whether there is gist index
func checkGist(sqlStmtArray []string, fpath string) {
	for _, line := range sqlStmtArray {
		if idx := gistRegex.FindStringSubmatch(line); idx != nil {
			reportCase(fpath, "Schema contains gist index which is not supported.",
				"https://github.com/YugaByte/yugabyte-db/issues/1337", "", "INDEX", idx[2], line)
		} else if idx := brinRegex.FindStringSubmatch(line); idx != nil {
			reportCase(fpath, "index method 'brin' not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1337", "", "INDEX", idx[1], line)
		} else if idx := spgistRegex.FindStringSubmatch(line); idx != nil {
			reportCase(fpath, "index method 'spgist' not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1337", "", "INDEX", idx[1], line)
		} else if idx := rtreeRegex.FindStringSubmatch(line); idx != nil {
			reportCase(fpath, "index method 'rtree' is superceded by 'gist' which is not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1337", "", "INDEX", idx[1], line)
		}
	}
}

// Checks compatibility of views
func checkViews(sqlStmtArray []string, fpath string) {
	for _, line := range sqlStmtArray {
		/*if dropMatViewRegex.MatchString(line) {
			reportCase(fpath, "DROP MATERIALIZED VIEW not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/10102", "")
		} else if view := matViewRegex.FindStringSubmatch(line); view != nil {
			reportCase(fpath, "Schema contains materialized view which is not supported. The view is: "+view[1],
				"https://github.com/yugabyte/yugabyte-db/issues/10102", "")
		} else */
		if view := viewWithCheckRegex.FindStringSubmatch(line); view != nil {
			reportCase(fpath, "Schema containing VIEW WITH CHECK OPTION is not supported yet.", "", "", "VIEW", view[1], line)
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
func checkSql(sqlStmtArray []string, fpath string) {
	for _, line := range sqlStmtArray {
		if rangeRegex.MatchString(line) {
			reportCase(fpath,
				"RANGE with offset PRECEDING/FOLLOWING is not supported for column type numeric and offset type double precision",
				"https://github.com/yugabyte/yugabyte-db/issues/10692", "", "TABLE", "", line)
		} else if stmt := createConvRegex.FindStringSubmatch(line); stmt != nil {
			reportCase(fpath, "CREATE CONVERSION not supported yet", "https://github.com/YugaByte/yugabyte-db/issues/10866", "", "CONVERSION", stmt[2], line)
		} else if alterConvRegex.MatchString(line) {
			reportCase(fpath, "ALTER CONVERSION not supported yet", "https://github.com/YugaByte/yugabyte-db/issues/10866", "", "CONVERSION", stmt[1], line)
		} else if fetchAbsRegex.MatchString(line) {
			reportCase(fpath, "FETCH ABSOLUTE not supported yet", "https://github.com/YugaByte/yugabyte-db/issues/6514", "", "CURSOR", "", line)
		} else if fetchRelativeRegex.MatchString(line) {
			reportCase(fpath, "FETCH RELATIVE not supported yet", "https://github.com/YugaByte/yugabyte-db/issues/6514", "", "CURSOR", "", line)
		} else if fetchRegex.MatchString(line) {
			reportCase(fpath, "FETCH - not supported yet", "https://github.com/YugaByte/yugabyte-db/issues/6514", "", "CURSOR", "", line)
		} else if backwardRegex.MatchString(line) {
			reportCase(fpath, "FETCH BACKWARD not supported yet", "https://github.com/YugaByte/yugabyte-db/issues/6514", "", "CURSOR", "", line)
		} else if stmt := alterAggRegex.FindStringSubmatch(line); stmt != nil {
			reportCase(fpath, "ALTER AGGREGATE not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/2717", "", "AGGREGATE", stmt[1], line)
		} else if dropCollRegex.MatchString(line) {
			reportCase(fpath, "DROP multiple objects not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP COLLATION", line), "COLLATION", "", line)
		} else if dropIdxRegex.MatchString(line) {
			reportCase(fpath, "DROP multiple objects not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP INDEX", line), "INDEX", "", line)
		} else if dropViewRegex.MatchString(line) {
			reportCase(fpath, "DROP multiple objects not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP VIEW", line), "VIEW", "", line)
		} else if dropSeqRegex.MatchString(line) {
			reportCase(fpath, "DROP multiple objects not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP SEQUENCE", line), "SEQUENCE", "", line)
		} else if dropForeignRegex.MatchString(line) {
			reportCase(fpath, "DROP multiple objects not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/880", separateMultiObj("DROP FOREIGN TABLE", line), "FOREIGN TABLE", "", line)
		} else if idx := createIdxConcurRegex.FindStringSubmatch(line); idx != nil {
			reportCase(fpath, "CREATE INDEX CONCURRENTLY not supported yet",
				"https://github.com/yugabyte/yugabyte-db/issues/10799", "", "INDEX", idx[3], line)
		} else if idx := dropIdxConcurRegex.FindStringSubmatch(line); idx != nil {
			reportCase(fpath, "DROP INDEX CONCURRENTLY not supported yet",
				"", "", "INDEX", idx[2], line)
		} else if trig := trigRefRegex.FindStringSubmatch(line); trig != nil {
			reportCase(fpath, "REFERENCING clause (transition tables) not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1668", "", "TRIGGER", trig[1], line)
		} else if trig := constrTrgRegex.FindStringSubmatch(line); trig != nil {
			reportCase(fpath, "CREATE CONSTRAINT TRIGGER not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1709", "", "TRIGGER", trig[1], line)
		} else if currentOfRegex.MatchString(line) {
			reportCase(fpath, "WHERE CURRENT OF not supported yet", "https://github.com/YugaByte/yugabyte-db/issues/737", "", "CURSOR", "", line)
		} else if bulkCollectRegex.MatchString(line) {
			reportCase(fpath, "BULK COLLECT keyword of oracle is not converted into PostgreSQL compatible syntax", "", "", "", "", line)
		}
	}
}

// Checks unsupported DDL statements
func checkDDL(sqlStmtArray []string, fpath string) {

	for _, line := range sqlStmtArray {
		if am := amRegex.FindStringSubmatch(line); am != nil {
			reportCase(fpath, "CREATE ACCESS METHOD is not supported.",
				"https://github.com/yugabyte/yugabyte-db/issues/10693", "", "ACCESS METHOD", am[1], line)
		} else if tbl := idxConcRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "REINDEX CONCURRENTLY is not supported.",
				"https://github.com/yugabyte/yugabyte-db/issues/10694", "", "TABLE", tbl[1], line)
		} else if col := storedRegex.FindStringSubmatch(line); col != nil {
			reportCase(fpath, "Stored generated column is not supported. Column is: "+col[1],
				"https://github.com/yugabyte/yugabyte-db/issues/10695", "", "TABLE", "", line)
		} else if tbl := likeAllRegex.FindStringSubmatch(line); tbl != nil {
			summaryMap["TABLE"].invalidCount++
			reportCase(fpath, "LIKE ALL is not supported yet.",
				"https://github.com/yugabyte/yugabyte-db/issues/10697", "", "TABLE", tbl[2], line)
		} else if tbl := likeRegex.FindStringSubmatch(line); tbl != nil {
			summaryMap["TABLE"].invalidCount++
			reportCase(fpath, "LIKE clause not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1129", "", "TABLE", tbl[2], line)
		} else if tbl := tblPartitionRegex.FindStringSubmatch(line); tbl != nil {
			tblParts[tbl[2]] = tbl[3]
			if filename, ok := primaryCons[tbl[2]]; ok {
				reportAddingPrimaryKey(filename, tbl[2], line)
			}
		} else if tbl := addPrimaryRegex.FindStringSubmatch(line); tbl != nil {
			if _, ok := tblParts[tbl[2]]; ok {
				reportAddingPrimaryKey(fpath, tbl[2], line)
			}
			primaryCons[tbl[2]] = fpath
		} else if tbl := inheritRegex.FindStringSubmatch(line); tbl != nil {
			summaryMap["TABLE"].invalidCount++
			reportCase(fpath, "INHERITS not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1129", "", "TABLE", tbl[3], line)
		} else if tbl := withOidsRegex.FindStringSubmatch(line); tbl != nil {
			summaryMap["TABLE"].invalidCount++
			reportCase(fpath, "OIDs are not supported for user tables.",
				"https://github.com/yugabyte/yugabyte-db/issues/10273", "", "TABLE", tbl[2], line)
		} else if tbl := intvlRegex.FindStringSubmatch(line); tbl != nil {
			summaryMap["TABLE"].invalidCount++
			reportCase(fpath, "PRIMARY KEY containing column of type 'INTERVAL' not yet supported.",
				"https://github.com/YugaByte/yugabyte-db/issues/1397", "", "TABLE", tbl[2], line)
		} else if tbl := alterOfRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE OF not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[2], line)
		} else if tbl := alterSchemaRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE SET SCHEMA not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/3947", "", "TABLE", tbl[2], line)
		} else if createSchemaRegex.MatchString(line) {
			reportCase(fpath, "CREATE SCHEMA with elements not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/10865", "", "SCHEMA", "", line)
		} else if tbl := alterNotOfRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE NOT OF not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", "", line)
		} else if tbl := alterColumnRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE ALTER column not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[3], line)
		} else if tbl := alterConstrRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE ALTER CONSTRAINT not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[3], line)
		} else if tbl := setOidsRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE SET WITH OIDS not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[3], line)
		} else if tbl := withoutClusterRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE SET WITHOUT CLUSTER not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[2], line)
		} else if tbl := clusterRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE CLUSTER not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[2], line)
		} else if tbl := alterSetRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE SET not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[2], line)
		} else if tbl := alterIdxRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE SET not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[1], line)
		} else if tbl := alterResetRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE RESET not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[2], line)
		} else if tbl := alterOptionsRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[3], line)
		} else if typ := dropAttrRegex.FindStringSubmatch(line); typ != nil {
			reportCase(fpath, "ALTER TYPE DROP ATTRIBUTE not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1893", "", "TYPE", typ[1], line)
		} else if typ := alterTypeRegex.FindStringSubmatch(line); typ != nil {
			reportCase(fpath, "ALTER TYPE not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1893", "", "TYPE", typ[1], line)
		} else if tbl := alterInhRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE INHERIT not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[3], line)
		} else if tbl := valConstrRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "ALTER TABLE VALIDATE CONSTRAINT not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1124", "", "TABLE", tbl[3], line)
		} else if tbl := deferRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "DEFERRABLE unique constraints are not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1129", "", "TABLE", tbl[3], line)
		} else if spc := alterTblSpcRegex.FindStringSubmatch(line); spc != nil {
			reportCase(fpath, "ALTER TABLESPACE not supported yet.",
				"https://github.com/YugaByte/yugabyte-db/issues/1153", "", "TABLESPACE", spc[1], line)
		}
	}
}

// check foreign table
func checkForeign(sqlStmtArray []string, fpath string) {
	for _, line := range sqlStmtArray {
		if tbl := primRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "Primary key constraints are not supported on foreign tables.",
				"https://github.com/yugabyte/yugabyte-db/issues/10698", "", "TABLE", tbl[1], line)
		} else if tbl := foreignKeyRegex.FindStringSubmatch(line); tbl != nil {
			reportCase(fpath, "Foreign key constraints are not supported on foreign tables.",
				"https://github.com/yugabyte/yugabyte-db/issues/10699", "", "TABLE", tbl[1], line)
		}
	}
}

//all other cases to check
func checkRemaining(sqlStmtArray []string, fpath string) {
	for _, line := range sqlStmtArray {
		if trig := compoundTrigRegex.FindStringSubmatch(line); trig != nil {
			reportCase(fpath, "Compound Triggers are not supported in YugabyteDB and PostgreSQL yet.",
				"", "", "TRIGGER", trig[2], line)
		}
	}

}

// Checks whether the script, fpath, can be migrated to YB
func checker(sqlStmtArray []string, fpath string) {

	checkViews(sqlStmtArray, fpath)
	checkSql(sqlStmtArray, fpath)
	checkGist(sqlStmtArray, fpath)
	checkDDL(sqlStmtArray, fpath)
	checkForeign(sqlStmtArray, fpath)
	checkRemaining(sqlStmtArray, fpath)

	// if canMigrate {
	// 	log.Println("Schema in " + fpath + " can be migrated to Yugabyte DB")
	// } else {
	// 	log.Println("\n\n" + fpath + " has items to be modified before migration\n")
	// 	canMigrate = true // for checking other scripts
	// }
}

func getMapKeys(receivedMap map[string]bool) string {
	keyString := ""
	for key, _ := range receivedMap {
		keyString += key + ", "
	}

	if keyString != "" {
		keyString = keyString[0 : len(keyString)-2] //popping last comma and space
	}
	return keyString
}

func isCodeBlockPossible(objType string) bool {
	return objType == "PROCEDURE" || objType == "FUNCTION" || objType == "TRIGGER" || objType == "PACKAGE"
}

func invalidSqlComment(line string) int {
	if cmt := unsupportedCommentRegex1.FindStringSubmatch(line); cmt != nil {
		return 1
	} else if cmt := packageSupportCommentRegex.FindStringSubmatch(line); cmt != nil {
		summaryMap["PACKAGE"].totalCount++
		return 2
	} else if cmt := unsupportedCommentRegex2.FindStringSubmatch(line); cmt != nil {
		return 3
	} else if cmt := typeUnsupportedRegex.FindStringSubmatch(line); cmt != nil {
		return 4
	}
	return 0
}

func getCreateObjRegex(objType string) (*regexp.Regexp, int) {
	var createObjRegex *regexp.Regexp
	var objNameIndex int
	if objType == "MVIEW" {
		createObjRegex = regexp.MustCompile("(?i)CREATE (OR REPLACE )?MATERIALIZED VIEW[ ]+([a-zA-Z0-9_.]+)")
		objNameIndex = 2
	} else if objType == "PACKAGE" {
		createObjRegex = regexp.MustCompile("(?i)CREATE SCHEMA (IF NOT EXISTS )?[ ]+([a-zA-Z0-9_.]+)")
		objNameIndex = 2
	} else if objType == "SYNONYM" {
		createObjRegex = regexp.MustCompile("(?i)CREATE (OR REPLACE )?VIEW[ ]+([a-zA-Z0-9_.]+)")
		objNameIndex = 2
	} else if objType == "INDEX" {
		createObjRegex = regexp.MustCompile("(?i)CREATE (UNIQUE )?INDEX[ ]+(IF NOT EXISTS)?[ ]*([a-zA-Z0-9_.]+)")
		objNameIndex = 3
	} else { //TODO: check syntaxes for other objects and add more cases if required
		createObjRegex = regexp.MustCompile(fmt.Sprintf("(?i)CREATE (OR REPLACE )?%s[ ]+(IF NOT EXISTS )?([a-zA-Z0-9_.]+)", objType))
		objNameIndex = 3
	}

	return createObjRegex, objNameIndex
}

func processCollectedSql(fpath string, line *string, objType string, sqlStmtArray *[]string, reportNextSql *int) {
	createObjRegex, objNameIndex := getCreateObjRegex(objType)
	var objName = "" // to extract from sql statement

	//relevant CREATE statement in .sql file will be used
	if stmt := createObjRegex.FindStringSubmatch(*line); stmt != nil {
		objName = stmt[objNameIndex]
		summaryMap[objType].totalCount += 1
		summaryMap[objType].objSet[objName] = true
	}
	*sqlStmtArray = append(*sqlStmtArray, *line)
	if *reportNextSql > 0 {
		reportBasedOnComment(*reportNextSql, fpath, "", "", objName, objType, *line)
		*reportNextSql = 0 //reset flag
	}

	(*line) = ""
}

func createSqlStrArray(path string, objType string) []string {
	// fmt.Printf("Reading %s in dir= %s\n", objType, path)
	var sqlStmtArray []string

	codeBlock := isCodeBlockPossible(objType)
	dollarQuoteFlag := 0 //denotes the code/body part is not started
	reportNextSql := 0

	file, err := os.Open(path)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	line := ""

	// assemble array of lines, each line ends with semicolon
	for scanner.Scan() {
		curr := scanner.Text()
		curr = strings.Trim(curr, " \n\t")

		if len(curr) == 0 {
			continue
		}

		if strings.Contains(curr, "--") { //in case there is a space before '--'
			reportNextSql = invalidSqlComment(curr)
			continue
		}

		line += " " + curr

		if codeBlock {
			// if ';' occurs either before after end of $BODY$ means that sql is complete
			if dollarQuoteFlag == 0 {
				if dollarQuoteRegex.MatchString(curr) {

					dollarQuoteFlag = 1 //denotes start of the code/body part

				} else if strings.Contains(curr, ";") { // in case, there is no body part
					//one liner sql string created, now will check for obj count and report cases
					processCollectedSql(path, &line, objType, &sqlStmtArray, &reportNextSql)
				}
			} else if dollarQuoteFlag == 1 {
				if dollarQuoteRegex.MatchString(curr) {
					dollarQuoteFlag = 2 //denotes end of code/body part
				}
			} else if dollarQuoteFlag == 2 {
				if strings.Contains(curr, ";") {
					processCollectedSql(path, &line, objType, &sqlStmtArray, &reportNextSql)
					dollarQuoteFlag = 0 //resetting for other objects
				}
			}
		} else {
			if strings.Contains(curr, ";") {
				processCollectedSql(path, &line, objType, &sqlStmtArray, &reportNextSql)
			}
		}
	}
	// check whether there was error reading the script
	if scanner.Err() != nil {
		panic(scanner.Err())
	}

	// debugFile, _ := os.OpenFile(exportDir+"/reportSqls.sql", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	// defer debugFile.Close()
	// for _, str := range sqlStmtArray {
	// 	debugFile.WriteString(str + "\n")
	// }

	return sqlStmtArray
}

func initializeSummaryMap() {
	for _, objType := range sourceObjList {
		summaryMap[objType] = &summaryInfo{
			objSet:  make(map[string]bool),
			details: make(map[string]bool),
		}

		//executes only in case of oracle
		if objType == "PACKAGE" {
			summaryMap[objType].details["Packages in oracle are exported as schema, please review and edit them(if needed) to match your requirements"] = true
		} else if objType == "SYNONYM" {
			summaryMap[objType].details["Synonyms in oracle are exported as view, please review and edit them(if needed) to match your requirements"] = true
		}
	}

}

func generateHTMLfromJSONReport(Report utils.Report) string {
	//appending to doc line by line for better readability

	//Broad details
	htmlstring := "<html><body bgcolor='#EFEFEF'><h1>Database Migration Report</h1>"
	htmlstring += "<table><tr><th>Database Name</th><td>" + Report.Summary.DBName + "</td></tr>"
	htmlstring += "<tr><th>Schema Name</th><td>" + Report.Summary.SchemaName + "</td></tr></table>"

	//Summary of report
	htmlstring += "<br><table width='100%' table-layout='fixed'><tr><th>Object</th><th>Total Count</th><th>Auto-Migrated</th><th>Invalid Count</th><th width='40%'>Object Names</th><th width='30%'>Details</th></tr>"
	for i := 0; i < len(Report.Summary.DBObjects); i++ {
		if Report.Summary.DBObjects[i].TotalCount != 0 {
			htmlstring += "<tr><th>" + Report.Summary.DBObjects[i].ObjectType + "</th><td style='text-align: center;'>" + strconv.Itoa(Report.Summary.DBObjects[i].TotalCount) + "</td><td style='text-align: center;'>" + strconv.Itoa(Report.Summary.DBObjects[i].TotalCount-Report.Summary.DBObjects[i].InvalidCount) + "</td><td style='text-align: center;'>" + strconv.Itoa(Report.Summary.DBObjects[i].InvalidCount) + "</td><td width='40%'>" + Report.Summary.DBObjects[i].ObjectNames + "</td><td width='30%'>" + Report.Summary.DBObjects[i].Details + "</td></tr>"
		}
	}
	htmlstring += "</table><br>"

	//Issues/Error messages
	htmlstring += "<ul list-style-type='disc'>"
	for i := 0; i < len(Report.Issues); i++ {
		htmlstring += "<li>Error in Object " + Report.Issues[i].ObjectType + ":</li><ul>"
		htmlstring += "<li>Error Message: " + Report.Issues[i].Reason + "</li>"
		htmlstring += "<li>File Path: " + Report.Issues[i].FilePath + "<a href='" + Report.Issues[i].FilePath + "'> [Preview]</a></li>"
		if Report.Issues[i].GH != "" {
			htmlstring += "<li><a href='" + Report.Issues[i].GH + "'>Github Issue Link</a></li>"
		}
		htmlstring += "</ul>"
	}
	htmlstring += "</ul></body></html>"
	return htmlstring

}

func generateTxtfromJSONReport(Report utils.Report) string {
	txtstring := "Database Migration Report\n"
	txtstring += "Database Name\t" + Report.Summary.DBName + "\n"
	txtstring += "Schema Name\t" + Report.Summary.SchemaName + "\n\n"
	txtstring += "Objects:\n\n"
	//if names for json objects need to be changed make sure to change the tab spaces accordingly as well.
	for i := 0; i < len(Report.Summary.DBObjects); i++ {
		if Report.Summary.DBObjects[i].TotalCount != 0 {
			txtstring += fmt.Sprintf("%-16s", "Object:") + Report.Summary.DBObjects[i].ObjectType + "\n"
			txtstring += fmt.Sprintf("%-16s", "Total Count:") + strconv.Itoa(Report.Summary.DBObjects[i].TotalCount) + "\n"
			txtstring += fmt.Sprintf("%-16s", "Valid Count:") + strconv.Itoa(Report.Summary.DBObjects[i].TotalCount-Report.Summary.DBObjects[i].InvalidCount) + "\n"
			txtstring += fmt.Sprintf("%-16s", "Invalid Count:") + strconv.Itoa(Report.Summary.DBObjects[i].InvalidCount) + "\n"
			txtstring += fmt.Sprintf("%-16s", "Object Names:") + Report.Summary.DBObjects[i].ObjectNames + "\n"
			if Report.Summary.DBObjects[i].Details != "" {
				txtstring += fmt.Sprintf("%-16s", "Details:") + Report.Summary.DBObjects[i].Details + "\n"
			}
			txtstring += "\n"
		}
	}
	if len(Report.Issues) != 0 {
		txtstring += "Issues:\n\n"
	}
	for i := 0; i < len(Report.Issues); i++ {
		txtstring += "Error in Object " + Report.Issues[i].ObjectType + ":\n"
		txtstring += "-Error Message: " + Report.Issues[i].Reason + "\n"
		txtstring += "-File Path: " + Report.Issues[i].FilePath + "\n"
		if Report.Issues[i].GH != "" {
			txtstring += "-Github Issue Link: " + Report.Issues[i].GH + "\n"
		}
		txtstring += "\n"
	}
	return txtstring
}

// The command expects path to the directory containing .sql scripts followed by
// the filename to the summary report
func generateReportHelper() string {
	report = `"issues": [` //initialize this global variable
	var schemaDir string
	if source.GenerateReportMode {
		schemaDir = exportDir + "/temp/schema"
	} else {
		schemaDir = exportDir + "/schema"
	}

	sourceObjList = utils.GetSchemaObjectList(source.DBType)
	sourceObjList = append(sourceObjList, "INDEX")

	initializeSummaryMap()

	for _, objType := range sourceObjList {
		var filePath string
		if objType == "INDEX" {
			filePath = schemaDir + "/tables/INDEXES_table.sql"
		} else {
			filePath = schemaDir + "/" + strings.ToLower(objType) + "s"
			filePath += "/" + strings.ToLower(objType) + ".sql"
		}

		if !utils.FileOrFolderExists(filePath) {
			continue
		}

		sqlStmtArray := createSqlStrArray(filePath, objType)
		checker(sqlStmtArray, filePath)
	}

	summary := reportSummary()
	report += `]`

	finalReport := `{` + summary + report + `}`

	return finalReport
}

func generateReport() {
	reportFile := "report." + outputFormat
	reportPath := exportDir + "/reports/" + reportFile
	reportJsonString := generateReportHelper()

	var finalReport string
	if outputFormat == "html" {
		report := utils.ParseJsonFromString(reportJsonString)
		finalReport = utils.PrettifyHtmlString(generateHTMLfromJSONReport(report))
	} else if outputFormat == "json" {
		finalReport = utils.PrettifyJsonString(reportJsonString)
	} else if outputFormat == "txt" {
		report := utils.ParseJsonFromString(reportJsonString)
		finalReport = generateTxtfromJSONReport(report)
	} else if outputFormat == "xml" {
		report := utils.ParseJsonFromString(reportJsonString)
		byteReport, _ := xml.MarshalIndent(report, "", "\t")
		finalReport = string(byteReport)
	} else {
		//TODO: here, need to implement for other output formats
		finalReport = reportJsonString
	}

	//check & inform if file already exists
	if utils.FileOrFolderExists(reportPath) {
		fmt.Printf("\n%s already exists, overwriting it with a new generated report\n", reportFile)
	}

	file, err := os.OpenFile(reportPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	file.WriteString(finalReport)
	utils.PrintIfTrue(finalReport, source.GenerateReportMode) //don't print in case of export command
	fmt.Printf("\nplease find migration report at: %s\n", reportPath)
}

// generateReportCmd represents the checker command
var generateReportCmd = &cobra.Command{
	Use:   "generateReport",
	Short: "command for checking .sql files under given directory for YB incompatible constructs",
	Long: `Sample command line:
  yb_migrate checker <dir> <path-to-json-output>
where <dir> can have subdirectories containing .sql files

Cobra is a CLI library for Go that empowers applications.
This application is a tool to generate the needed files
to quickly create a Cobra application.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		exportSchemaCmd.PersistentPreRun(exportSchemaCmd, args)

		checkReportOutputFormat()
	},

	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("Note: Generated report will be based on the version - 2.11.3 of YugabyteDB!!")
		migration.PrintSourceDBVersion(&source)

		fmt.Printf("scanning source database to generate report  ")
		waitChannel := make(chan int)
		go utils.Wait(waitChannel)

		source.GenerateReportMode = true //flag to skip info about export schema
		// export schema before generating the report
		exportSchemaCmd.Run(exportSchemaCmd, args)

		waitChannel <- 0

		generateReport()
	},
}

func init() {
	rootCmd.AddCommand(generateReportCmd)

	generateReportCmd.PersistentFlags().StringVarP(&exportDir, "export-dir", "e", ".",
		"export directory (default is current working directory") //default value is current dir

	generateReportCmd.PersistentFlags().StringVar(&source.DBType, "source-db-type", "",
		"source database type (Oracle/PostgreSQL/MySQL)")

	generateReportCmd.PersistentFlags().StringVar(&source.Host, "source-db-host", "localhost",
		"source database server host")

	generateReportCmd.PersistentFlags().StringVar(&source.Port, "source-db-port", "",
		"source database server port number")

	generateReportCmd.PersistentFlags().StringVar(&source.User, "source-db-user", "",
		"connect to source database as specified user")

	// TODO: All sensitive parameters can be taken from the environment variable
	generateReportCmd.PersistentFlags().StringVar(&source.Password, "source-db-password", "",
		"connect to source as specified user")

	generateReportCmd.PersistentFlags().StringVar(&source.DBName, "source-db-name", "",
		"source database name to be migrated to YugabyteDB")

	//out of schema and db-name one should be mandatory(oracle vs others)

	generateReportCmd.PersistentFlags().StringVar(&source.Schema, "source-db-schema", "public",
		"source schema name which needs to be migrated to YugabyteDB")

	// TODO SSL related more args will come. Explore them later.
	generateReportCmd.PersistentFlags().StringVar(&source.SSLCertPath, "source-ssl-cert", "",
		"provide Source SSL Certificate Path")

	generateReportCmd.PersistentFlags().StringVar(&source.SSLMode, "source-ssl-mode", "disable",
		"specify the source SSL mode out of - disable, allow, prefer, require, verify-ca, verify-full")

	generateReportCmd.PersistentFlags().StringVar(&source.Uri, "source-db-uri", "",
		`URI for connecting to the source database
		format:
			1. Oracle:	oracle://User=username;Password=password@hostname:port/SID
			2. MySQL:	mysql://user:password@host:port/database
			3. PostgreSQL:	postgresql://user:password@host:port/dbname?sslmode=mode
		`)

	generateReportCmd.PersistentFlags().StringVar(&outputFormat, "output-format", "html",
		"allowed report formats: html | txt | json")

}

func checkReportOutputFormat() {
	allowedOutputFormats := []string{"html", "json", "txt", "xml"}
	outputFormat = strings.ToLower(outputFormat)

	for i := 0; i < len(allowedOutputFormats); i++ {
		if outputFormat == allowedOutputFormats[i] {
			return
		}
	}

	fmt.Printf("invalid output format: %s!!\n", outputFormat)
	os.Exit(1) // means output format didn't match allowed formats
}
