package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/testutils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/ybversion"
)

func TestAssessmentReportStructs(t *testing.T) {
	tests := []struct {
		name         string
		actualType   reflect.Type
		expectedType interface{}
	}{
		{
			name:       "Validate DBObject Struct Definition",
			actualType: reflect.TypeOf(utils.DBObject{}),
			expectedType: struct {
				ObjectType   string `json:"ObjectType"`
				TotalCount   int    `json:"TotalCount"`
				InvalidCount int    `json:"InvalidCount"`
				ObjectNames  string `json:"ObjectNames"`
				Details      string `json:"Details,omitempty"`
			}{},
		},
		{
			name:       "Validate SchemaSummary Struct Definition",
			actualType: reflect.TypeOf(utils.SchemaSummary{}),
			expectedType: struct {
				Description string           `json:"Description"`
				DBName      string           `json:"DbName"`
				SchemaNames []string         `json:"SchemaNames"`
				DBVersion   string           `json:"DbVersion"`
				Notes       []string         `json:"Notes,omitempty"`
				DBObjects   []utils.DBObject `json:"DatabaseObjects"`
			}{},
		},
		{
			name:       "Validate SizingRecommendation Struct Definition",
			actualType: reflect.TypeOf(migassessment.SizingRecommendation{}),
			expectedType: struct {
				ColocatedTables                 []string
				ColocatedReasoning              string
				ShardedTables                   []string
				NumNodes                        float64
				VCPUsPerInstance                int
				MemoryPerInstance               int
				OptimalSelectConnectionsPerNode int64
				OptimalInsertConnectionsPerNode int64
				EstimatedTimeInMinForImport     float64
				ParallelVoyagerJobs             float64
			}{},
		},
		{
			name:       "Validate TableColumnsDataTypes Struct Definition",
			actualType: reflect.TypeOf(utils.TableColumnsDataTypes{}),
			expectedType: struct {
				SchemaName string `json:"SchemaName"`
				TableName  string `json:"TableName"`
				ColumnName string `json:"ColumnName"`
				DataType   string `json:"DataType"`
			}{},
		},
		{
			name:       "Validate UnsupportedFeature Struct Definition",
			actualType: reflect.TypeOf(UnsupportedFeature{}),
			expectedType: struct {
				FeatureName            string                          `json:"FeatureName"`
				Objects                []ObjectInfo                    `json:"Objects"`
				DisplayDDL             bool                            `json:"-"`
				DocsLink               string                          `json:"DocsLink,omitempty"`
				FeatureDescription     string                          `json:"FeatureDescription,omitempty"`
				MinimumVersionsFixedIn map[string]*ybversion.YBVersion `json:"MinimumVersionsFixedIn"`
			}{},
		},
		{
			name:       "Validate UnsupportedQueryConstruct Struct Definition",
			actualType: reflect.TypeOf(utils.UnsupportedQueryConstruct{}),
			expectedType: struct {
				ConstructTypeName      string
				Query                  string
				DocsLink               string
				MinimumVersionsFixedIn map[string]*ybversion.YBVersion
			}{},
		},
		{
			name:       "Validate TableIndexStats Struct Definition",
			actualType: reflect.TypeOf(migassessment.TableIndexStats{}),
			expectedType: struct {
				SchemaName      string  `json:"SchemaName"`
				ObjectName      string  `json:"ObjectName"`
				RowCount        *int64  `json:"RowCount"` // Pointer to allows null values
				ColumnCount     *int64  `json:"ColumnCount"`
				Reads           *int64  `json:"Reads"`
				Writes          *int64  `json:"Writes"`
				ReadsPerSecond  *int64  `json:"ReadsPerSecond"`
				WritesPerSecond *int64  `json:"WritesPerSecond"`
				IsIndex         bool    `json:"IsIndex"`
				ObjectType      string  `json:"ObjectType"`
				ParentTableName *string `json:"ParentTableName"`
				SizeInBytes     *int64  `json:"SizeInBytes"`
			}{},
		},
		{
			name:       "Validate AssessmentReport Struct Definition",
			actualType: reflect.TypeOf(AssessmentReport{}),
			expectedType: struct {
				VoyagerVersion             string                                `json:"VoyagerVersion"`
				TargetDBVersion            *ybversion.YBVersion                  `json:"TargetDBVersion"`
				MigrationComplexity        string                                `json:"MigrationComplexity"`
				SchemaSummary              utils.SchemaSummary                   `json:"SchemaSummary"`
				Sizing                     *migassessment.SizingAssessmentReport `json:"Sizing"`
				UnsupportedDataTypes       []utils.TableColumnsDataTypes         `json:"UnsupportedDataTypes"`
				UnsupportedDataTypesDesc   string                                `json:"UnsupportedDataTypesDesc"`
				UnsupportedFeatures        []UnsupportedFeature                  `json:"UnsupportedFeatures"`
				UnsupportedFeaturesDesc    string                                `json:"UnsupportedFeaturesDesc"`
				UnsupportedQueryConstructs []utils.UnsupportedQueryConstruct     `json:"UnsupportedQueryConstructs"`
				UnsupportedPlPgSqlObjects  []UnsupportedFeature                  `json:"UnsupportedPlPgSqlObjects"`
				MigrationCaveats           []UnsupportedFeature                  `json:"MigrationCaveats"`
				TableIndexStats            *[]migassessment.TableIndexStats      `json:"TableIndexStats"`
				Notes                      []string                              `json:"Notes"`
			}{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testutils.CompareStructs(t, tt.actualType, reflect.TypeOf(tt.expectedType), tt.name)
		})
	}
}

func TestAssessmentReportJson(t *testing.T) {
	reportDir := filepath.Join(os.TempDir(), "assessment_report_test")
	reportPath := filepath.Join(reportDir, fmt.Sprintf("%s%s", ASSESSMENT_FILE_NAME, JSON_EXTENSION))

	newYbVersion, err := ybversion.NewYBVersion("2024.1.1.1")
	if err != nil {
		t.Fatalf("Failed to create new YBVersion: %v", err)
	}

	assessmentReport = AssessmentReport{
		VoyagerVersion:      "v1.0.0",
		TargetDBVersion:     newYbVersion,
		MigrationComplexity: "High",
		SchemaSummary: utils.SchemaSummary{
			Description: "Test Schema Summary",
			DBName:      "test_db",
			SchemaNames: []string{"public"},
			DBVersion:   "13.3",
			DBObjects: []utils.DBObject{
				{
					ObjectType:   "Table",
					TotalCount:   1,
					InvalidCount: 0,
					ObjectNames:  "test_table",
				},
			},
		},
		Sizing: &migassessment.SizingAssessmentReport{
			SizingRecommendation: migassessment.SizingRecommendation{
				ColocatedTables:                 []string{"test_table"},
				ColocatedReasoning:              "Test reasoning",
				ShardedTables:                   []string{"test_table"},
				NumNodes:                        3,
				VCPUsPerInstance:                4,
				MemoryPerInstance:               16,
				OptimalSelectConnectionsPerNode: 10,
				OptimalInsertConnectionsPerNode: 10,
				EstimatedTimeInMinForImport:     10,
				ParallelVoyagerJobs:             10,
			},
			FailureReasoning: "Test failure reasoning",
		},
		UnsupportedDataTypes: []utils.TableColumnsDataTypes{
			{
				SchemaName: "public",
				TableName:  "test_table",
				ColumnName: "test_column",
				DataType:   "test_type",
			},
		},
		UnsupportedDataTypesDesc: "Test unsupported data types",
		UnsupportedFeatures: []UnsupportedFeature{
			{
				FeatureName: "test_feature",
				Objects: []ObjectInfo{
					{
						ObjectName:   "test_object",
						ObjectType:   "test_type",
						SqlStatement: "test_sql",
					},
				},
				DisplayDDL:             true,
				DocsLink:               "https://test.com",
				FeatureDescription:     "Test feature description",
				MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{"2024.1.1": newYbVersion},
			},
		},
		UnsupportedFeaturesDesc: "Test unsupported features",
		UnsupportedQueryConstructs: []utils.UnsupportedQueryConstruct{
			{
				ConstructTypeName:      "test_construct",
				Query:                  "test_query",
				DocsLink:               "https://test.com",
				MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{"2024.1.1": newYbVersion},
			},
		},
		UnsupportedPlPgSqlObjects: []UnsupportedFeature{
			{
				FeatureName: "test_feature",
				Objects: []ObjectInfo{
					{
						ObjectName:   "test_object",
						ObjectType:   "test_type",
						SqlStatement: "test_sql",
					},
				},
				DisplayDDL:             true,
				DocsLink:               "https://test.com",
				FeatureDescription:     "Test feature description",
				MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{"2024.1.1": newYbVersion},
			},
		},
		MigrationCaveats: []UnsupportedFeature{
			{
				FeatureName: "test_feature",
				Objects: []ObjectInfo{
					{
						ObjectName:   "test_object",
						ObjectType:   "test_type",
						SqlStatement: "test_sql",
					},
				},
				DisplayDDL:             true,
				DocsLink:               "https://test.com",
				FeatureDescription:     "Test feature description",
				MinimumVersionsFixedIn: map[string]*ybversion.YBVersion{"2024.1.1": newYbVersion},
			},
		},
		TableIndexStats: &[]migassessment.TableIndexStats{
			{
				SchemaName:      "public",
				ObjectName:      "test_table",
				RowCount:        Int64Ptr(100),
				ColumnCount:     Int64Ptr(10),
				Reads:           Int64Ptr(100),
				Writes:          Int64Ptr(100),
				ReadsPerSecond:  Int64Ptr(10),
				WritesPerSecond: Int64Ptr(10),
				IsIndex:         true,
				ObjectType:      "Table",
				ParentTableName: StringPtr("parent_table"),
				SizeInBytes:     Int64Ptr(1024),
			},
		},
		Notes: []string{"Test note"},
	}

	// Make the report directory
	err = os.MkdirAll(reportDir, 0755)
	if err != nil {
		t.Fatalf("Failed to create report directory: %v", err)
	}

	// Clean up the report directory
	defer func() {
		err := os.RemoveAll(reportDir)
		if err != nil {
			t.Fatalf("Failed to remove report directory: %v", err)
		}
	}()

	// Write the assessment report to a JSON file
	err = generateAssessmentReportJson(reportDir)
	if err != nil {
		t.Fatalf("Failed to write assessment report to JSON file: %v", err)
	}

	// expected JSON
	expectedJSON := `{
	"VoyagerVersion": "v1.0.0",
	"TargetDBVersion": "2024.1.1.1",
	"MigrationComplexity": "High",
	"SchemaSummary": {
		"Description": "Test Schema Summary",
		"DbName": "test_db",
		"SchemaNames": [
			"public"
		],
		"DbVersion": "13.3",
		"DatabaseObjects": [
			{
				"ObjectType": "Table",
				"TotalCount": 1,
				"InvalidCount": 0,
				"ObjectNames": "test_table"
			}
		]
	},
	"Sizing": {
		"SizingRecommendation": {
			"ColocatedTables": [
				"test_table"
			],
			"ColocatedReasoning": "Test reasoning",
			"ShardedTables": [
				"test_table"
			],
			"NumNodes": 3,
			"VCPUsPerInstance": 4,
			"MemoryPerInstance": 16,
			"OptimalSelectConnectionsPerNode": 10,
			"OptimalInsertConnectionsPerNode": 10,
			"EstimatedTimeInMinForImport": 10,
			"ParallelVoyagerJobs": 10
		},
		"FailureReasoning": "Test failure reasoning"
	},
	"UnsupportedDataTypes": [
		{
			"SchemaName": "public",
			"TableName": "test_table",
			"ColumnName": "test_column",
			"DataType": "test_type"
		}
	],
	"UnsupportedDataTypesDesc": "Test unsupported data types",
	"UnsupportedFeatures": [
		{
			"FeatureName": "test_feature",
			"Objects": [
				{
					"ObjectType": "test_type",
					"ObjectName": "test_object",
					"SqlStatement": "test_sql"
				}
			],
			"DocsLink": "https://test.com",
			"FeatureDescription": "Test feature description",
			"MinimumVersionsFixedIn": {
				"2024.1.1": "2024.1.1.1"
			}
		}
	],
	"UnsupportedFeaturesDesc": "Test unsupported features",
	"UnsupportedQueryConstructs": [
		{
			"ConstructTypeName": "test_construct",
			"Query": "test_query",
			"DocsLink": "https://test.com",
			"MinimumVersionsFixedIn": {
				"2024.1.1": "2024.1.1.1"
			}
		}
	],
	"UnsupportedPlPgSqlObjects": [
		{
			"FeatureName": "test_feature",
			"Objects": [
				{
					"ObjectType": "test_type",
					"ObjectName": "test_object",
					"SqlStatement": "test_sql"
				}
			],
			"DocsLink": "https://test.com",
			"FeatureDescription": "Test feature description",
			"MinimumVersionsFixedIn": {
				"2024.1.1": "2024.1.1.1"
			}
		}
	],
	"MigrationCaveats": [
		{
			"FeatureName": "test_feature",
			"Objects": [
				{
					"ObjectType": "test_type",
					"ObjectName": "test_object",
					"SqlStatement": "test_sql"
				}
			],
			"DocsLink": "https://test.com",
			"FeatureDescription": "Test feature description",
			"MinimumVersionsFixedIn": {
				"2024.1.1": "2024.1.1.1"
			}
		}
	],
	"TableIndexStats": [
		{
			"SchemaName": "public",
			"ObjectName": "test_table",
			"RowCount": 100,
			"ColumnCount": 10,
			"Reads": 100,
			"Writes": 100,
			"ReadsPerSecond": 10,
			"WritesPerSecond": 10,
			"IsIndex": true,
			"ObjectType": "Table",
			"ParentTableName": "parent_table",
			"SizeInBytes": 1024
		}
	],
	"Notes": [
		"Test note"
	]
}`

	testutils.CompareJson(t, reportPath, expectedJSON, reportDir)

}

func Int64Ptr(i int64) *int64 {
	return &i
}

func StringPtr(s string) *string {
	return &s
}
