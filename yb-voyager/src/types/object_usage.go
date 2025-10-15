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
package types

import (
	"github.com/yugabyte/yb-voyager/yb-voyager/src/constants"
	"github.com/yugabyte/yb-voyager/yb-voyager/src/utils/sqlname"
)

type ObjectUsageStats struct {
	SchemaName      string
	ObjectName      string
	ObjectType      string
	ParentTableName string
	Reads           int64
	Inserts         int64
	Updates         int64
	Deletes         int64
	//Category based on some logic
	ReadUsage       string
	WriteUsage      string
	Usage           string
}

func (o *ObjectUsageStats) TotalWrites() int64 {
	return o.Inserts + o.Updates + o.Deletes
}

func NewObjectUsageStats(schemaName, objectName, objectType string, parentTableName string, reads, inserts, updates, deletes int64) *ObjectUsageStats {
	return &ObjectUsageStats{
		SchemaName:      schemaName,
		ObjectName:      objectName,
		ObjectType:      objectType,
		ParentTableName: parentTableName,
		Reads:           reads,
		Inserts:         inserts,
		Updates:         updates,
		Deletes:         deletes,
	}
}

func GetCombinedUsageCategory(a, b string) string {
	if a == ObjectUsageFrequent || b == ObjectUsageFrequent {
		return ObjectUsageFrequent
	} else if a == ObjectUsageModerate || b == ObjectUsageModerate {
		return ObjectUsageModerate
	} else if a == ObjectUsageRare || b == ObjectUsageRare {
		return ObjectUsageRare
	}
	return ObjectUsageUnused
}

func (o *ObjectUsageStats) GetObjectName() string {
	if o.ObjectType == "index" {
		//Index object name with qualified table name
		objName := sqlname.NewObjectNameQualifiedWithTableName(constants.POSTGRESQL, "", o.ObjectName, o.SchemaName, o.ParentTableName)
		return objName.Qualified.Unquoted
	}
	//qualified table name
	objName := sqlname.NewObjectName(constants.POSTGRESQL, "", o.SchemaName, o.ObjectName)
	return objName.Qualified.Unquoted

}

func GetUsageCategory(usage int64, maxUsage int64) string {
	//if maxUsage is 0, means
	if maxUsage <= 0 {
		return ObjectUsageUnused
	}

	if usage <= OBJECT_USAGE_THRESHOLD_UNUSED {
		return ObjectUsageUnused
	}

	// Calculate percentage of max usage
	usageRatio := float64(usage) / float64(maxUsage)

	if usageRatio >= OBJECT_USAGE_THRESHOLD_FREQUENT {
		return ObjectUsageFrequent
	} else if usageRatio >= OBJECT_USAGE_THRESHOLD_MODERATE {
		return ObjectUsageModerate
	} else {
		return ObjectUsageRare
	}
}

const (
	ObjectUsageFrequent string = "FREQUENT"
	ObjectUsageModerate string = "MODERATE"
	ObjectUsageRare     string = "RARE"
	ObjectUsageUnused   string = "UNUSED"
)

// Thresholds for object usage categorization
const (
	OBJECT_USAGE_THRESHOLD_FREQUENT = 0.70 // >= 70% of max usage
	OBJECT_USAGE_THRESHOLD_MODERATE = 0.30 // >= 30% of max usage
	OBJECT_USAGE_THRESHOLD_RARE     = 0.01 // >= 1% of max usage (anything above 0 but below moderate)
	OBJECT_USAGE_THRESHOLD_UNUSED   = 0.00 // < 1% of max usage
)
