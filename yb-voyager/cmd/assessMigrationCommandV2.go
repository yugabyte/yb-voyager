package cmd

import (
	"fmt"
	"strings"

	"github.com/samber/lo"
	log "github.com/sirupsen/logrus"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryissue"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/query/queryparser"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
	"golang.org/x/exp/slices"
)

// TODO: content of this function will move to generateAssessmentReport()
// For now this function will fetch all the issues which will be used by the migration complexity determination logic
func fetchAllAssessmentIssues() ([]AssessmentIssue, error) {
	var assessmentIssues []AssessmentIssue

	assessmentIssues = append(assessmentIssues, getAssessmentReportContentFromAnalyzeSchemaV2()...)

	issues, err := fetchUnsupportedObjectTypesV2()
	if err != nil {
		return assessmentIssues, fmt.Errorf("failed to fetch unsupported object type issues: %w", err)
	}
	assessmentIssues = append(assessmentIssues, issues...)

	if utils.GetEnvAsBool("REPORT_UNSUPPORTED_QUERY_CONSTRUCTS", true) {
		issues, err := fetchUnsupportedQueryConstructsV2()
		if err != nil {
			return assessmentIssues, fmt.Errorf("failed to fetch unsupported queries on YugabyteDB: %w", err)
		}
		assessmentIssues = append(assessmentIssues, issues...)
	}

	unsupportedDataTypes, unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB, err := fetchColumnsWithUnsupportedDataTypes()
	if err != nil {
		return assessmentIssues, fmt.Errorf("failed to fetch columns with unsupported data types: %w", err)
	}

	assessmentIssues = append(assessmentIssues, getAssessmentIssuesForUnsupportedDatatypes(unsupportedDataTypes)...)

	assessmentIssues = append(assessmentIssues, fetchMigrationCaveatAssessmentIssues(unsupportedDataTypesForLiveMigration, unsupportedDataTypesForLiveMigrationWithFForFB)...)

	return assessmentIssues, nil
}

// TODO: replacement of getAssessmentReportContentFromAnalyzeSchema()
func getAssessmentReportContentFromAnalyzeSchemaV2() []AssessmentIssue {
	/*
		Here we are generating analyze schema report which converts issue instance to analyze schema issue
		Then in assessment codepath we extract the required information from analyze schema issue which could have been done directly from issue instance(TODO)

		But current Limitation is analyze schema currently uses regexp etc to detect some issues(not using parser).
	*/
	schemaAnalysisReport := analyzeSchemaInternal(&source, true)
	assessmentReport.MigrationComplexity = schemaAnalysisReport.MigrationComplexity
	assessmentReport.SchemaSummary = schemaAnalysisReport.SchemaSummary
	assessmentReport.SchemaSummary.Description = lo.Ternary(source.DBType == ORACLE, SCHEMA_SUMMARY_DESCRIPTION_ORACLE, SCHEMA_SUMMARY_DESCRIPTION)

	var assessmentIssues []AssessmentIssue
	switch source.DBType {
	case ORACLE:
		assessmentIssues = append(assessmentIssues, fetchUnsupportedOracleFeaturesFromSchemaReportV2(schemaAnalysisReport)...)
	case POSTGRESQL:
		assessmentIssues = append(assessmentIssues, fetchUnsupportedPGFeaturesFromSchemaReportV2(schemaAnalysisReport)...)
	default:
		panic(fmt.Sprintf("unsupported source db type %q", source.DBType))
	}

	// Ques: Do we still need this and REPORT_UNSUPPORTED_QUERY_CONSTRUCTS env var
	if utils.GetEnvAsBool("REPORT_UNSUPPORTED_PLPGSQL_OBJECTS", true) {
		assessmentIssues = append(assessmentIssues, fetchUnsupportedPlPgSQLObjectsV2(schemaAnalysisReport)...)
	}

	return assessmentIssues
}

// TODO: will replace fetchUnsupportedOracleFeaturesFromSchemaReport()
func fetchUnsupportedOracleFeaturesFromSchemaReportV2(schemaAnalysisReport utils.SchemaReport) []AssessmentIssue {
	log.Infof("fetching assessment issues of feature category for Oracle...")
	var assessmentIssues []AssessmentIssue
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, "", COMPOUND_TRIGGER_ISSUE_REASON, schemaAnalysisReport, "")...)
	return assessmentIssues
}

// TODO: will replace fetchUnsupportedPGFeaturesFromSchemaReport()
func fetchUnsupportedPGFeaturesFromSchemaReportV2(schemaAnalysisReport utils.SchemaReport) []AssessmentIssue {
	log.Infof("fetching assessment issues of feature category for PG...")

	var assessmentIssues []AssessmentIssue
	for _, indexMethod := range queryissue.UnsupportedIndexMethods {
		displayIndexMethod := strings.ToUpper(indexMethod)
		reason := fmt.Sprintf(INDEX_METHOD_ISSUE_REASON, displayIndexMethod)
		assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, "", reason, schemaAnalysisReport, "")...)
	}

	assessmentIssues = append(assessmentIssues, getIndexesOnComplexTypeUnsupportedFeatureV2(schemaAnalysisReport, queryissue.UnsupportedIndexDatatypes)...)

	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.CONSTRAINT_TRIGGER, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.INHERITANCE, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.STORED_GENERATED_COLUMNS, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, "", CONVERSION_ISSUE_REASON, schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.MULTI_COLUMN_GIN_INDEX, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.ALTER_TABLE_SET_COLUMN_ATTRIBUTE, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.ALTER_TABLE_DISABLE_RULE, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.ALTER_TABLE_CLUSTER_ON, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.STORAGE_PARAMETER, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, "", UNSUPPORTED_EXTENSION_ISSUE, schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.EXCLUSION_CONSTRAINTS, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.DEFERRABLE_CONSTRAINTS, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, "", VIEW_CHECK_OPTION_ISSUE, schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.PK_UK_ON_COMPLEX_DATATYPE, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.UNLOGGED_TABLE, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.REFERENCING_CLAUSE_IN_TRIGGER, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.BEFORE_ROW_TRIGGER_ON_PARTITIONED_TABLE, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.ADVISORY_LOCKS, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.XML_FUNCTIONS, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.SYSTEM_COLUMNS, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.LARGE_OBJECT_FUNCTIONS, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.REGEX_FUNCTIONS, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.FETCH_WITH_TIES, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.JSON_QUERY_FUNCTION, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.JSON_CONSTRUCTOR_FUNCTION, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.AGGREGATE_FUNCTION, "", schemaAnalysisReport, "")...)
	assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, queryissue.SECURITY_INVOKER_VIEWS, "", schemaAnalysisReport, "")...)

	return assessmentIssues
}

func getIndexesOnComplexTypeUnsupportedFeatureV2(schemaAnalysisReport utils.SchemaReport, unsupportedIndexDatatypes []string) []AssessmentIssue {
	var assessmentIssues []AssessmentIssue
	// TODO: include MinimumVersionsFixedIn

	unsupportedIndexDatatypes = append(unsupportedIndexDatatypes, "array")             // adding it here only as we know issue form analyze will come with type
	unsupportedIndexDatatypes = append(unsupportedIndexDatatypes, "user_defined_type") // adding it here as we UDTs will come with this type.
	for _, unsupportedType := range unsupportedIndexDatatypes {
		// formattedObject.ObjectName = fmt.Sprintf("%s: %s", strings.ToUpper(unsupportedType), object.ObjectName)
		issueReason := fmt.Sprintf(ISSUE_INDEX_WITH_COMPLEX_DATATYPES, unsupportedType)
		assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.FEATURE, "", issueReason, schemaAnalysisReport, "")...)
	}
	return assessmentIssues
}

// TODO: replacement of fetchUnsupportedObjectTypes()
func fetchUnsupportedObjectTypesV2() ([]AssessmentIssue, error) {
	if source.DBType != ORACLE {
		return nil, nil
	}

	query := fmt.Sprintf(`SELECT schema_name, object_name, object_type FROM %s`, migassessment.OBJECT_TYPE_MAPPING)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying-%s: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching object type mapping metadata: %v", err)
		}
	}()

	var assessmentIssues []AssessmentIssue
	for rows.Next() {
		var schemaName, objectName, objectType string
		err = rows.Scan(&schemaName, &objectName, &objectType)
		if err != nil {
			return nil, fmt.Errorf("error scanning rows:%w", err)
		}

		switch {
		case slices.Contains(OracleUnsupportedIndexTypes, objectType):
			assessmentIssues = append(assessmentIssues, AssessmentIssue{
				Category:   constants.FEATURE,
				Type:       "", // TODO
				Name:       UNSUPPORTED_INDEXES_FEATURE,
				Impact:     "", // TODO
				ObjectType: "INDEX",
				ObjectName: fmt.Sprintf("Index Name: %s, Index Type=%s", objectName, objectType),
			})
		case objectType == VIRTUAL_COLUMN:
			assessmentIssues = append(assessmentIssues, AssessmentIssue{
				Category:   constants.FEATURE,
				Type:       "", // TODO
				Name:       VIRTUAL_COLUMNS_FEATURE,
				Impact:     "", // TODO
				ObjectName: objectName,
			})
		case objectType == INHERITED_TYPE:
			assessmentIssues = append(assessmentIssues, AssessmentIssue{
				Category:   constants.FEATURE,
				Type:       "", // TODO
				Name:       INHERITED_TYPES_FEATURE,
				Impact:     "", // TODO
				ObjectName: objectName,
			})
		case objectType == REFERENCE_PARTITION || objectType == SYSTEM_PARTITION:
			referenceOrTablePartitionPresent = true
			assessmentIssues = append(assessmentIssues, AssessmentIssue{
				Category:   constants.FEATURE,
				Type:       "", // TODO
				Name:       UNSUPPORTED_PARTITIONING_METHODS_FEATURE,
				Impact:     "", // TODO
				ObjectType: "TABLE",
				ObjectName: fmt.Sprintf("Table Name: %s, Partition Method: %s", objectName, objectType),
			})
		}
	}

	return assessmentIssues, nil
}

// // TODO: replacement of fetchUnsupportedPlPgSQLObjects()
func fetchUnsupportedPlPgSQLObjectsV2(schemaAnalysisReport utils.SchemaReport) []AssessmentIssue {
	if source.DBType != POSTGRESQL {
		return nil
	}

	plpgsqlIssues := lo.Filter(schemaAnalysisReport.Issues, func(issue utils.AnalyzeSchemaIssue, _ int) bool {
		return issue.IssueType == UNSUPPORTED_PLPGSQL_OBJECTS
	})
	groupedPlpgsqlIssuesByReason := lo.GroupBy(plpgsqlIssues, func(issue utils.AnalyzeSchemaIssue) string {
		return issue.Reason
	})

	var assessmentIssues []AssessmentIssue
	for reason, issues := range groupedPlpgsqlIssuesByReason {
		var minVersionsFixedIn map[string]*ybversion.YBVersion
		var minVersionsFixedInSet bool

		for _, issue := range issues {
			if !minVersionsFixedInSet {
				minVersionsFixedIn = issue.MinimumVersionsFixedIn
				minVersionsFixedInSet = true
			}
			if !areMinVersionsFixedInEqual(minVersionsFixedIn, issue.MinimumVersionsFixedIn) {
				utils.ErrExit("Issues belonging to UnsupportedFeature %s have different minimum versions fixed in: %v, %v", reason, minVersionsFixedIn, issue.MinimumVersionsFixedIn)
			}

			assessmentIssues = append(assessmentIssues, AssessmentIssue{
				Category:              constants.PLPGSQL_OBJECT,
				Type:                  issue.Type,
				Name:                  reason,
				Impact:                issue.Impact, // TODO: verify(expected already there since underlying issues are assigned)
				ObjectType:            issue.ObjectType,
				ObjectName:            issue.ObjectName,
				SqlStatement:          issue.SqlStatement,
				DocsLink:              issue.DocsLink,
				MinimumVersionFixedIn: issue.MinimumVersionsFixedIn,
			})
		}
	}

	return assessmentIssues
}

// Q: do we no need of displayDDLInHTML in this approach? DDL can always be there for issues in the table.
func getAssessmentIssuesFromSchemaAnalysisReport(category string, issueType string, issueReason string, schemaAnalysisReport utils.SchemaReport, issueDescription string) []AssessmentIssue {
	log.Infof("filtering issues for type: %s", issueType)
	var issues []AssessmentIssue
	var minVersionsFixedIn map[string]*ybversion.YBVersion
	var minVersionsFixedInSet bool
	for _, analyzeIssue := range schemaAnalysisReport.Issues {
		if !slices.Contains([]string{UNSUPPORTED_FEATURES, MIGRATION_CAVEATS}, analyzeIssue.IssueType) {
			continue
		}

		issueMatched := lo.Ternary[bool](issueType != "", issueType == analyzeIssue.Type, strings.Contains(analyzeIssue.Reason, issueReason))
		if issueMatched {
			if !minVersionsFixedInSet {
				minVersionsFixedIn = analyzeIssue.MinimumVersionsFixedIn
				minVersionsFixedInSet = true
			}
			if !areMinVersionsFixedInEqual(minVersionsFixedIn, analyzeIssue.MinimumVersionsFixedIn) {
				utils.ErrExit("Issues belonging to type %s have different minimum versions fixed in: %v, %v", analyzeIssue.Type, minVersionsFixedIn, analyzeIssue.MinimumVersionsFixedIn)
			}

			issues = append(issues, AssessmentIssue{
				Category:              category,
				CategoryDescription:   GetCategoryDescription(category),
				Type:                  analyzeIssue.Type,
				Name:                  analyzeIssue.Reason, // in convertIssueInstanceToAnalyzeIssue() we assign IssueType to Reason field
				Description:           issueDescription,    // TODO: verify
				Impact:                analyzeIssue.Impact,
				ObjectType:            analyzeIssue.ObjectType,
				ObjectName:            analyzeIssue.ObjectName,
				SqlStatement:          analyzeIssue.SqlStatement,
				DocsLink:              analyzeIssue.DocsLink,
				MinimumVersionFixedIn: minVersionsFixedIn,
			})
		}
	}

	return issues
}

// TODO: soon to replace fetchUnsupportedQueryConstructs()
func fetchUnsupportedQueryConstructsV2() ([]AssessmentIssue, error) {
	if source.DBType != POSTGRESQL {
		return nil, nil
	}

	query := fmt.Sprintf("SELECT DISTINCT query from %s", migassessment.DB_QUERIES_SUMMARY)
	rows, err := assessmentDB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying=%s on assessmentDB: %w", query, err)
	}
	defer func() {
		closeErr := rows.Close()
		if closeErr != nil {
			log.Warnf("error closing rows while fetching database queries summary metadata: %v", err)
		}
	}()

	var executedQueries []string
	for rows.Next() {
		var executedQuery string
		err := rows.Scan(&executedQuery)
		if err != nil {
			return nil, fmt.Errorf("error scanning rows: %w", err)
		}
		executedQueries = append(executedQueries, executedQuery)
	}

	if len(executedQueries) == 0 {
		log.Infof("queries info not present in the assessment metadata for detecting unsupported query constructs")
		return nil, nil
	}

	var assessmentIssues []AssessmentIssue
	for i := 0; i < len(executedQueries); i++ {
		query := executedQueries[i]
		log.Debugf("fetching unsupported query constructs for query - [%s]", query)
		collectedSchemaList, err := queryparser.GetSchemaUsed(query)
		if err != nil { // no need to error out if failed to get schemas for a query
			log.Errorf("failed to get schemas used for query [%s]: %v", query, err)
			continue
		}

		log.Infof("collected schema list %v(len=%d) for query [%s]", collectedSchemaList, len(collectedSchemaList), query)
		if !considerQueryForIssueDetection(collectedSchemaList) {
			log.Infof("ignoring query due to difference in collected schema list %v(len=%d) vs source schema list %v(len=%d)",
				collectedSchemaList, len(collectedSchemaList), source.GetSchemaList(), len(source.GetSchemaList()))
			continue
		}

		issues, err := parserIssueDetector.GetDMLIssues(query, targetDbVersion)
		if err != nil {
			log.Errorf("failed while trying to fetch query issues in query - [%s]: %v",
				query, err)
		}

		for _, issue := range issues {
			issue := AssessmentIssue{
				Category:              constants.QUERY_CONSTRUCT,
				Type:                  issue.Type,
				Name:                  issue.Name,
				Impact:                issue.Impact,
				SqlStatement:          issue.SqlStatement,
				DocsLink:              issue.DocsLink,
				MinimumVersionFixedIn: issue.MinimumVersionsFixedIn,
			}
			assessmentIssues = append(assessmentIssues, issue)
		}
	}

	return assessmentIssues, nil
}

func getAssessmentIssuesForUnsupportedDatatypes(unsupportedDatatypes []utils.TableColumnsDataTypes) []AssessmentIssue {
	var assessmentIssues []AssessmentIssue
	for _, colInfo := range unsupportedDatatypes {
		qualifiedColName := fmt.Sprintf("%s.%s.%s", colInfo.SchemaName, colInfo.TableName, colInfo.ColumnName)
		issue := AssessmentIssue{
			Category:              constants.DATATYPE,
			CategoryDescription:   GetCategoryDescription(constants.DATATYPE),
			Type:                  colInfo.DataType, // TODO: maybe name it like "unsupported datatype - geometry"
			Name:                  colInfo.DataType, // TODO: maybe name it like "unsupported datatype - geometry"
			Impact:                "",               // TODO
			ObjectType:            constants.COLUMN,
			ObjectName:            qualifiedColName,
			DocsLink:              "",  // TODO
			MinimumVersionFixedIn: nil, // TODO
		}
		assessmentIssues = append(assessmentIssues, issue)
	}

	return assessmentIssues
}

// TODO: soon to replace addMigrationCaveatsToAssessmentReport()
func fetchMigrationCaveatAssessmentIssues(unsupportedDataTypesForLiveMigration []utils.TableColumnsDataTypes, unsupportedDataTypesForLiveMigrationWithFForFB []utils.TableColumnsDataTypes) []AssessmentIssue {
	var assessmentIssues []AssessmentIssue
	switch source.DBType {
	case POSTGRESQL:
		log.Infof("fetching migration caveat category assessment issues")
		assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.MIGRATION_CAVEATS, queryissue.ALTER_TABLE_ADD_PK_ON_PARTITIONED_TABLE, "", schemaAnalysisReport, DESCRIPTION_ADD_PK_TO_PARTITION_TABLE)...)
		assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.MIGRATION_CAVEATS, queryissue.FOREIGN_TABLE, "", schemaAnalysisReport, DESCRIPTION_FOREIGN_TABLES)...)
		assessmentIssues = append(assessmentIssues, getAssessmentIssuesFromSchemaAnalysisReport(constants.MIGRATION_CAVEATS, queryissue.POLICY_WITH_ROLES, "", schemaAnalysisReport, DESCRIPTION_POLICY_ROLE_DESCRIPTION)...)

		if len(unsupportedDataTypesForLiveMigration) > 0 {
			for _, colInfo := range unsupportedDataTypesForLiveMigration {
				issue := AssessmentIssue{
					Category:            constants.MIGRATION_CAVEATS,
					CategoryDescription: "",                                        // TODO
					Type:                UNSUPPORTED_DATATYPES_LIVE_CAVEAT_FEATURE, // TODO add object type in type name
					Name:                "",                                        // TODO
					Impact:              "",                                        // TODO
					Description:         UNSUPPORTED_DATATYPES_FOR_LIVE_MIGRATION_DESCRIPTION,
					ObjectType:          constants.COLUMN,
					ObjectName:          fmt.Sprintf("%s.%s.%s", colInfo.SchemaName, colInfo.TableName, colInfo.ColumnName),
					DocsLink:            UNSUPPORTED_DATATYPE_LIVE_MIGRATION_DOC_LINK,
				}
				assessmentIssues = append(assessmentIssues, issue)
			}
		}

		if len(unsupportedDataTypesForLiveMigrationWithFForFB) > 0 {
			for _, colInfo := range unsupportedDataTypesForLiveMigrationWithFForFB {
				issue := AssessmentIssue{
					Category:            constants.MIGRATION_CAVEATS,
					CategoryDescription: "",                                                   // TODO
					Type:                UNSUPPORTED_DATATYPES_LIVE_WITH_FF_FB_CAVEAT_FEATURE, // TODO add object type in type name
					Name:                "",                                                   // TODO
					Impact:              "",                                                   // TODO
					Description:         UNSUPPORTED_DATATYPES_FOR_LIVE_MIGRATION_WITH_FF_FB_DESCRIPTION,
					ObjectType:          constants.COLUMN,
					ObjectName:          fmt.Sprintf("%s.%s.%s", colInfo.SchemaName, colInfo.TableName, colInfo.ColumnName),
					DocsLink:            UNSUPPORTED_DATATYPE_LIVE_MIGRATION_DOC_LINK,
				}
				assessmentIssues = append(assessmentIssues, issue)
			}
		}
	}

	return assessmentIssues
}
