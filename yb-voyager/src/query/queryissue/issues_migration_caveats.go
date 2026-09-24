/*
Copyright (c) YugabyteDB, Inc.

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

package queryissue

import (
	"fmt"
	"strings"

	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/issue"
)

var pointDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_POINT,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_POINT_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewPointDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := pointDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var lineDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LINE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LINE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewLineDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := lineDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var lsegDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LSEG,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LSEG_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewLsegDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := lsegDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var boxDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_BOX,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_BOX_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewBoxDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := boxDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var pathDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_PATH,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_PATH_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewPathDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := pathDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var polygonDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_POLYGON,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_POLYGON_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewPolygonDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := polygonDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var circleDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_CIRCLE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_CIRCLE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewCircleDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := circleDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var vectorDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_VECTOR,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_VECTOR_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "",
	DocsLink:    "https://docs.yugabyte.com/stable/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewVectorDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := vectorDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var xmlLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_XML,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_XML_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewXMLLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := xmlLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var timetzDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TIMETZ,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TIMETZ_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewTimetzDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := timetzDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var geometryLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_GEOMETRY,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_GEOMETRY_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewGeometryLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := geometryLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var geographyLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_GEOGRAPHY,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_GEOGRAPHY_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewGeographyLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := geographyLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var box2DLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_BOX2D,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_BOX2D_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewBox2DLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := box2DLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var box3DLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_BOX3D,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_BOX3D_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewBox3DLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := box3DLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var topogeometryLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TOPOGEOMETRY,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TOPOGEOMETRY_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewTopogeometryLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := topogeometryLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var rasterLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_RASTER,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_RASTER_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewRasterLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := rasterLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var pgLsnLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_PG_LSN,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_PG_LSN_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewPgLsnLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := pgLsnLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var txidSnapshotLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TXID_SNAPSHOT,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TXID_SNAPSHOT_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewTxidSnapshotLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := txidSnapshotLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var lOLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LARGE_OBJECT,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_LARGE_OBJECT_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewLOLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := lOLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var int4MultiRangeLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_INT4MULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_INT4MULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewInt4MultiRangeLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := int4MultiRangeLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var int8MultiRangeLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_INT8MULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_INT8MULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewInt8MultiRangeLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := int8MultiRangeLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var numMultiRangeLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_NUMMULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_NUMMULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewNumMultiRangeLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := numMultiRangeLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var tSMultiRangeLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TSMULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TSMULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewTSMultiRangeLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := tSMultiRangeLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var tSTZMultiRangeLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TSTZMULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_TSTZMULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewTSTZMultiRangeLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := tSTZMultiRangeLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var dateMultiRangeLiveMigrationDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_DATEMULTIRANGE,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_DATEMULTIRANGE_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewDateMultiRangeLiveMigrationDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := dateMultiRangeLiveMigrationDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var userDefinedDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_USER_DEFINED,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_USER_DEFINED_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_USER_DEFINED_DATATYPE_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewUserDefinedDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := userDefinedDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}

var tsQueryDatatypeIssue = issue.Issue{
	Type:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_TSQUERY,
	Name:        UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_TSQUERY_ISSUE_NAME,
	Impact:      constants.IMPACT_LEVEL_1,
	Description: UNSUPPORTED_DATATYPE_LIVE_MIGRATION_WITH_FF_FB_ISSUE_DESCRIPTION,
	GH:          "https://github.com/yugabyte/yb-voyager/issues/1731",
	DocsLink:    "https://docs.yugabyte.com/preview/yugabyte-voyager/known-issues/postgresql/#unsupported-datatypes-by-voyager-during-live-migration",
}

func NewTsQueryDatatypeIssue(objectType string, objectName string, sqlStatement string, typeName string, colName string) QueryIssue {
	issue := tsQueryDatatypeIssue
	typeName = strings.ToUpper(typeName)
	issue.Description = fmt.Sprintf(issue.Description, typeName, colName)
	return newQueryIssue(issue, objectType, objectName, sqlStatement, map[string]interface{}{}, map[string]interface{}{})
}
