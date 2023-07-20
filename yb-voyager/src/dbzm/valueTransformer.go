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
package dbzm

import ()

func TransformValue(value string, colSchema Schema, valueConverterSuite map[string]func(string) (string, error)) (string, error ) {
	logicalType := colSchema.ColDbzSchema.Name
	schemaType := colSchema.ColDbzSchema.Type
	if valueConverterSuite[logicalType] != nil {
		return valueConverterSuite[logicalType](value)
	} else if valueConverterSuite[schemaType] != nil {
		return valueConverterSuite[schemaType](value)
	} else {
		return value, nil
	}
}