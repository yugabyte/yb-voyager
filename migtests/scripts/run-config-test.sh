#!/usr/bin/env bash

set -e

if [ $# -gt 2 ]; then
	echo "Usage: $0 TEST_NAME [env.sh]"
	exit 1
fi

set -x

export YB_VOYAGER_SEND_DIAGNOSTICS=false
export TEST_NAME=$1

export REPO_ROOT="${PWD}"
export SCRIPTS="${REPO_ROOT}/migtests/scripts"
export TESTS_DIR="${REPO_ROOT}/migtests/tests"
export TEST_DIR="${TESTS_DIR}/${TEST_NAME}"
export PYTHONPATH="${REPO_ROOT}/migtests/lib"

CONFIG_TEMPLATE="${SCRIPTS}/config-templates/offline-migration.yaml"
GENERATED_CONFIG="${TEST_DIR}/generated-config.yaml"
CONFIG_GEN_SCRIPT="${SCRIPTS}/generate_config.py"

# Order of env.sh import matters.
if [ $2 != "" ] #if env.sh is passed as an argument, source it
then
	if [ ! -f "${TEST_DIR}/$2" ]
	then
		echo "$2 file not found in the test directory"
		exit 1
	fi
	source ${TEST_DIR}/$2
else
	source ${TEST_DIR}/env.sh
fi

source ${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh


source ${SCRIPTS}/functions.sh

normalize_and_export_vars "offline"

source ${SCRIPTS}/yugabytedb/env.sh


main() {
	echo "Deleting the parent export-dir present in the test directory"
	rm -rf ${EXPORT_DIR}	
	echo "Creating export-dir in the parent test directory"
	mkdir -p ${EXPORT_DIR}
	echo "Assigning permissions to the export-dir to execute init-db, cleanup-db scripts"
	chmod +x ${TEST_DIR}/init-db ${TEST_DIR}/cleanup-db

	step "START: ${TEST_NAME}"
	print_env

	# Generate the config dynamically
	python3 "$CONFIG_GEN_SCRIPT" --template "$CONFIG_TEMPLATE" --output "$GENERATED_CONFIG"

	pushd ${TEST_DIR}

	step "Initialise source database."
	if [[ "${SKIP_DB_CREATION}" != "true" ]]; then
		if [[ "${SOURCE_DB_TYPE}" == "postgresql" || "${SOURCE_DB_TYPE}" == "mysql" ]]; then
			create_source_db "${SOURCE_DB_NAME}"
		elif [[ "${SOURCE_DB_TYPE}" == "oracle" ]]; then
			create_source_db "${SOURCE_DB_SCHEMA}"
		else
			echo "ERROR: Unsupported SOURCE_DB_TYPE: ${SOURCE_DB_TYPE}"
			exit 1
		fi
	else
		echo "Skipping database creation as SKIP_DB_CREATION is set to true."
	fi
	./init-db

	step "Grant source database user permissions"
	grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA}

	step "Check the Voyager version installed"
	yb-voyager version

	step "Assess Migration"
	if [ "${SOURCE_DB_TYPE}" = "postgresql" ] || [ "${SOURCE_DB_TYPE}" == "oracle" ]; then
		yb-voyager assess-migration -c "${GENERATED_CONFIG}" --yes || {
			cat_log_file "yb-voyager-assess-migration.log"
			cat_file ${EXPORT_DIR}/assessment/metadata/yb-voyager-assessment.log
		}

		step "Validate Assessment Reports"
		# Checking if the assessment reports were created
		if [ -f "${EXPORT_DIR}/assessment/reports/migration_assessment_report.html" ] && [ -f "${EXPORT_DIR}/assessment/reports/migration_assessment_report.json" ]; then
			echo "Assessment reports created successfully."
			validate_failure_reasoning "${EXPORT_DIR}/assessment/reports/migration_assessment_report.json"
		else
			echo "Error: Assessment reports were not created successfully."
			cat_log_file "yb-voyager-assess-migration.log"
			exit 1
		fi

		post_assess_migration
	fi

	step "Export schema."
	yb-voyager export schema -c "${GENERATED_CONFIG}" --yes
	find ${EXPORT_DIR}/schema -name '*.sql' -printf "'%p'\n"| xargs grep -wh CREATE

	step "Analyze schema."
	yb-voyager analyze-schema -c "${GENERATED_CONFIG}" --yes
	tail -20 ${EXPORT_DIR}/reports/schema_analysis_report.json

	step "Fix schema."
	if [ -x "${TEST_DIR}/fix-schema" ]
	then
		"${TEST_DIR}/fix-schema"
	fi

	step "Analyze schema."
	yb-voyager analyze-schema -c "${GENERATED_CONFIG}" --yes
	tail -20 ${EXPORT_DIR}/reports/schema_analysis_report.json

	step "Export data."
	# false if exit code of export_data is non-zero
	yb-voyager export data -c "${GENERATED_CONFIG}" --yes || { 
		cat_log_file "yb-voyager-export-data.log"
		cat_log_file "debezium-source_db_exporter.log"
		exit 1
	}

	ls -R ${EXPORT_DIR}/data | sed 's/:$//' | sed -e 's/[^-][^\/]*\//--/g' -e 's/^/   /' -e 's/-/|/'

	cat ${EXPORT_DIR}/data/export_status.json || echo "No export_status.json found."
	cat ${EXPORT_DIR}/metainfo/dataFileDescriptor.json

	step "Verify the pg_dump version being used"
	if [ "${SOURCE_DB_TYPE}" = "postgresql" ] && { [ -z "${BETA_FAST_DATA_EXPORT}" ] || [ "${BETA_FAST_DATA_EXPORT}" = "0" ]; }; then
		if ! grep "Dumped by pg_dump version:" "${EXPORT_DIR}/logs/yb-voyager-export-data.log"; then
			echo "Error: pg_dump version not found in the log file." >&2
		fi
	fi

	step "Fix data."
	if [ -x "${TEST_DIR}/fix-data" ]
	then
		"${TEST_DIR}/fix-data" ${EXPORT_DIR}
	fi

	step "Create target database."
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
	if [ "${SOURCE_DB_TYPE}" = "postgresql" ] || [ "${SOURCE_DB_TYPE}" = "oracle" ]; then
		run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME} with COLOCATION=TRUE"
	else
		run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME}"
	fi

	step "Import schema."
	yb-voyager import schema -c "${GENERATED_CONFIG}" --yes
	run_ysql ${TARGET_DB_NAME} "\dt"

	step "Run Schema validations."
	if [ -x "${TEST_DIR}/validate-schema" ]
	then
		"${TEST_DIR}/validate-schema"
	fi

	step "Import data."
	yb-voyager import data -c "${GENERATED_CONFIG}" --yes
	
	step "Import remaining schema (FK, index, and trigger) and Refreshing MViews if present."
	yb-voyager finalize-schema-post-data-import -c "${GENERATED_CONFIG}" --yes
	
	run_ysql ${TARGET_DB_NAME} "\di"
	run_ysql ${TARGET_DB_NAME} "\dft" 

	step "Run validations."
	if [[ "${EXPORT_TABLE_LIST}" != "" && -x "${TEST_DIR}/validate-with-table-list" ]]; then
		"${TEST_DIR}/validate-with-table-list"
	elif [[ -x "${TEST_DIR}/validate" ]]; then
		"${TEST_DIR}/validate"
	fi

	step "Run export-data-status"
	yb-voyager export data status -c "${GENERATED_CONFIG}" --yes --output-format json

	expected_file="${TEST_DIR}/export_data_status-report.json"
	actual_file="${EXPORT_DIR}/reports/export-data-status-report.json"

	if [ "${EXPORT_TABLE_LIST}" != "" ]
	then
		expected_file="${TEST_DIR}/export-data-status-with-table-list-report.json"
	fi

	step "Verify export-data-status report"
	verify_report ${expected_file} ${actual_file}

	step "Run import-data-status"
	yb-voyager import data status -c "${GENERATED_CONFIG}" --yes --output-format json

	expected_file="${TEST_DIR}/import_data_status-report.json"
	actual_file="${EXPORT_DIR}/reports/import-data-status-report.json"

	if [ "${EXPORT_TABLE_LIST}" != "" ]
	then
		expected_file="${TEST_DIR}/import-data-status-with-table-list-report.json"
	fi

	verify_report ${expected_file} ${actual_file}

	step "End Migration: clearing metainfo about state of migration from everywhere."
	yb-voyager end migration -c "${GENERATED_CONFIG}" --yes

	step "Clean up"
	./cleanup-db
	rm -rf "${EXPORT_DIR}"
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
	rm -rf "${GENERATED_CONFIG}"
}

main
