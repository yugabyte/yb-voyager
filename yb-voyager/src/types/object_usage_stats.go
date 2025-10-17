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
