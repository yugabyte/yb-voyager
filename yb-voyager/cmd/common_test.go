package cmd

import (
	"reflect"
	"testing"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/migassessment"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils"
)

func TestAssessmentReportStruct(t *testing.T) {
	excpectedDBObject := struct {
		ObjectType   string `json:"ObjectType"`
		TotalCount   int    `json:"TotalCount"`
		InvalidCount int    `json:"InvalidCount"`
		ObjectNames  string `json:"ObjectNames"`
		Details      string `json:"Details,omitempty"`
	}{}

	t.Run("Check DBObject structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(utils.DBObject{}), reflect.TypeOf(excpectedDBObject), "DBObject")
	})

	// Define the expected structure for utils.SchemaSummary
	expectedSchemaSummary := struct {
		Description string           `json:"Description"`
		DBName      string           `json:"DbName"`
		SchemaNames []string         `json:"SchemaNames"`
		DBVersion   string           `json:"DbVersion"`
		Notes       []string         `json:"Notes,omitempty"`
		DBObjects   []utils.DBObject `json:"DatabaseObjects"`
	}{}

	t.Run("Check SchemaSummary structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(utils.SchemaSummary{}), reflect.TypeOf(expectedSchemaSummary), "SchemaSummary")
	})

	// Define the expected structure for migassessment.SizingRecommendation
	expectedSizingRecommendation := struct {
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
	}{}

	t.Run("Check SizingRecommendation structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(migassessment.SizingRecommendation{}), reflect.TypeOf(expectedSizingRecommendation), "SizingRecommendation")
	})

	// Define the expected structure for utils.TableColumnsDataTypes
	expectedTableColumnsDataTypes := struct {
		SchemaName string `json:"SchemaName"`
		TableName  string `json:"TableName"`
		ColumnName string `json:"ColumnName"`
		DataType   string `json:"DataType"`
	}{}

	t.Run("Check TableColumnsDataTypes structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(utils.TableColumnsDataTypes{}), reflect.TypeOf(expectedTableColumnsDataTypes), "TableColumnsDataTypes")
	})

	// Define the expected structure for UnsupportedFeature
	expectedUnsupportedFeature := struct {
		FeatureName        string       `json:"FeatureName"`
		Objects            []ObjectInfo `json:"Objects"`
		DisplayDDL         bool         `json:"-"`
		DocsLink           string       `json:"DocsLink,omitempty"`
		FeatureDescription string       `json:"FeatureDescription,omitempty"`
	}{}

	t.Run("Check UnsupportedFeature structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(UnsupportedFeature{}), reflect.TypeOf(expectedUnsupportedFeature), "UnsupportedFeature")
	})

	// Define the expected structure for utils.UnsupportedQueryConstruct
	expectedUnsupportedQueryConstruct := struct {
		ConstructTypeName string
		Query             string
		DocsLink          string
	}{}

	t.Run("Check UnsupportedQueryConstruct structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(utils.UnsupportedQueryConstruct{}), reflect.TypeOf(expectedUnsupportedQueryConstruct), "UnsupportedQueryConstruct")
	})

	// Define the expected structure for migassessment.TableIndexStats
	expectedTableIndexStats := struct {
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
	}{}

	t.Run("Check TableIndexStats structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(migassessment.TableIndexStats{}), reflect.TypeOf(expectedTableIndexStats), "TableIndexStats")
	})

	// Define the expected structure for AssessmentReport
	expectedAssessmentReport := struct {
		VoyagerVersion             string                                `json:"VoyagerVersion"`
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
	}{}

	t.Run("Check AssessmentReport structure", func(t *testing.T) {
		utils.CompareStructAndReport(t, reflect.TypeOf(AssessmentReport{}), reflect.TypeOf(expectedAssessmentReport), "AssessmentReport")
	})
}
