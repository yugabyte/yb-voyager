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

/*
ObjectUsage - refers to the usage of a specific object in the database under any of these buckets
- frequent - if the object is used more than 70% of the workload
- moderate - if the object is used more than 10% of the workload and less than 70% of the workload
- rare - if the object is used less than 10% of the workload
- unused - if the object is not used at all

Usage category for an object if usage and max usage are given
FREQUENT : usage >= 70% of max_usage
MODERATE :  usage >=10 % of max_usage and usage < 70% of max_usage
RARE : usage >0 and usage <10% of max_usage
UNUSED : usage = 0

Overall category of the object
FREQUENT- if any of the read or write is FREQUENT
MODERATE - if any of the read or write is MODERATE
RARE - if any of the read or write is RARE
UNUSED if both read and write are UNUSED


*/

const (
	ObjectUsageFrequent string = "FREQUENT"
	ObjectUsageModerate string = "MODERATE"
	ObjectUsageRare     string = "RARE"
	ObjectUsageUnused   string = "UNUSED"

	// Thresholds for object usage categorization
	OBJECT_USAGE_THRESHOLD_FREQUENT = 0.70 // >= 70% of max usage
	OBJECT_USAGE_THRESHOLD_MODERATE = 0.10 // >= 10% of max usage and < 70% of max usage
	//anything between 0 and 10% is considered as rare
	OBJECT_USAGE_THRESHOLD_UNUSED   = 0.00 // 0% is unused
)

type ObjectUsage struct {
	ObjectUsageStats
	//Category based on some logic
	ReadUsage  string
	WriteUsage string
	Usage      string
}

func NewObjectUsage(schemaName, objectName, objectType string, parentTableName string, scans, inserts, updates, deletes int64) *ObjectUsage {
	return &ObjectUsage{
		ObjectUsageStats: ObjectUsageStats{
			SchemaName:      schemaName,
			ObjectName:      objectName,
			ObjectType:      objectType,
			ParentTableName: parentTableName,
			Scans:           scans,
			Inserts:         inserts,
			Updates:         updates,
			Deletes:         deletes,
		},
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

func GetUsageCategory(usage int64, maxUsage int64) string {
	//if maxUsage is 0 itself, all objects are unused
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
