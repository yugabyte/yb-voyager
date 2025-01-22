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
package constants

const (
	// Database Object types
	TABLE    = "TABLE"
	FUNCTION = "FUNCTION"
	COLUMN   = "COLUMN"

	// Source DB Types
	YUGABYTEDB = "yugabytedb"
	POSTGRESQL = "postgresql"
	ORACLE     = "oracle"
	MYSQL      = "mysql"

	// AssessmentIssue Categoes - used by YugabyteD payload and Migration Complexity Explainability
	// TODO: soon to be renamed as SCHEMA, SCHEMA_PLPGSQL, DML_QUERY, MIGRATION_CAVEAT, "DATATYPE"
	FEATURE           = "feature"
	DATATYPE          = "datatype"
	QUERY_CONSTRUCT   = "query_construct"
	MIGRATION_CAVEATS = "migration_caveats"
	PLPGSQL_OBJECT    = "plpgsql_object"

	// constants for the Impact Buckets
	IMPACT_LEVEL_1 = "LEVEL_1" // Represents minimal impact like only the schema ddl
	IMPACT_LEVEL_2 = "LEVEL_2" // Represents moderate impact like dml queries which might impact a lot of implementation/assumption in app layer
	IMPACT_LEVEL_3 = "LEVEL_3" // Represent significant impact like TABLE INHERITANCE, which doesn't have any simple workaround but can impact multiple objects/apps

	// constants for migration complexity
	MIGRATION_COMPLEXITY_LOW    = "LOW"
	MIGRATION_COMPLEXITY_MEDIUM = "MEDIUM"
	MIGRATION_COMPLEXITY_HIGH   = "HIGH"
)

const (
	OBFUSCATE_STRING = "XXXXX"
)
