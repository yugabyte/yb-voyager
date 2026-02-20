#!/usr/bin/env bash

set -e

if [ $# -gt 2 ]
then
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
export EXPORT_DIR=${EXPORT_DIR:-"${TEST_DIR}/export-dir"}

export PYTHONPATH="${REPO_ROOT}/migtests/lib"

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
source ${SCRIPTS}/yugabytedb/env.sh

source ${SCRIPTS}/functions.sh

main() {
	echo "Deleting the parent export-dir present in the test directory"
	rm -rf ${EXPORT_DIR}	
	echo "Creating export-dir in the parent test directory"
	mkdir -p ${EXPORT_DIR}
	echo "Assigning permissions to the export-dir to execute init-db, cleanup-db scripts"
	chmod +x ${TEST_DIR}/init-db ${TEST_DIR}/cleanup-db

	step "START: ${TEST_NAME}"
	print_env

	pushd ${TEST_DIR}

	step "Unzip expected and replacement files"
	if [ -f expected_files.zip ]
	then
		unzip -o expected_files.zip
	fi
	
	if [ -f replacement_dir.zip ]
	then
		unzip -o replacement_dir.zip
	fi
	
	step "Initialise source database."
	./init-db

	step "Grant source database user permissions"
	grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA}

	step "Check the Voyager version installed"
	yb-voyager version

    step "Assess Migration"
	assess_migration
	
	step "Validate Assessment Reports"
	# Checking if the assessment reports were created
	if [ -f "${EXPORT_DIR}/assessment/reports/migration_assessment_report.html" ] && [ -f "${EXPORT_DIR}/assessment/reports/migration_assessment_report.json" ]; then
		echo "Assessment reports created successfully."
		echo "Checking for Failures"
		validate_failure_reasoning "${EXPORT_DIR}/assessment/reports/migration_assessment_report.json"
		echo "Comparing Report contents"
        expected_file="${TEST_DIR}/expected_files/expectedAssessmentReport.json"
        actual_file="${EXPORT_DIR}/assessment/reports/migration_assessment_report.json"
	    compare_json_reports ${expected_file} ${actual_file}
	else
		echo "Error: Assessment reports were not created successfully."
		cat_log_file "yb-voyager-assess-migration.log"
		exit 1
	fi

	step "Export schema."
	export_schema

	step "Analyze schema."
	analyze_schema --output-format json
	compare_json_reports "${TEST_DIR}/expected_files/expected_schema_analysis_report.json" "${EXPORT_DIR}/reports/schema_analysis_report.json"

	step "Create target database."
	run_ysql yugabyte "DROP DATABASE IF EXISTS \"${TARGET_DB_NAME};\""
	run_ysql yugabyte "CREATE DATABASE \"${TARGET_DB_NAME}\" with COLOCATION=TRUE"

	step "Import schema."
	import_schema --continue-on-error t
	run_ysql ${TARGET_DB_NAME} "\dt"

	# Extract the major version of YugabyteDB from the version string
	target_major_version=$(run_ysql "${TARGET_DB_NAME}" "SELECT version();" | grep -oE 'YB-([0-9]+\.[0-9]+)' | cut -d '-' -f2)
	echo "Major YugabyteDB Version: $target_major_version"

    if [ -f "${EXPORT_DIR}/schema/failed.sql" ]
    then
		EXPECTED_FAILED_FILE="${TEST_DIR}/expected_files/expected_failed.sql"
		# If version is 2.25 or >= 2025.1, use expected_failed_2025.1.sql if it exists
		EXPECTED_FAILED_FILE_2025_1="${TEST_DIR}/expected_files/expected_failed_2025.1.sql"
		if [ -f "${EXPECTED_FAILED_FILE_2025_1}" ] && [ "$(echo "$target_major_version >= 2025.1" | bc)" -eq 1 ]; then
		    EXPECTED_FAILED_FILE="${EXPECTED_FAILED_FILE_2025_1}"
		fi

		EXPECTED_FAILED_2_25="${TEST_DIR}/expected_files/expected_failed_2.25.sql"
		if [ -f "${EXPECTED_FAILED_2_25}" ] && [ "$target_major_version" == "2.25" ]; then
		    EXPECTED_FAILED_FILE="${EXPECTED_FAILED_2_25}"
		fi

		echo "EXPECTED_FAILED_FILE: $EXPECTED_FAILED_FILE"
        #compare the failed.sql to the expected_failed.sql
        compare_sql_files "${EXPORT_DIR}/schema/failed.sql" "${EXPECTED_FAILED_FILE}" "$target_major_version"
        #rename failed.sql
        mv "${EXPORT_DIR}/schema/failed.sql" "${EXPORT_DIR}/schema/failed.sql.bak"
        #replace_files
        replace_files "${TEST_DIR}/replacement_dir" "${EXPORT_DIR}/schema"
		# --start-clean is required here since we are running the import command for the second time
        import_schema --start-clean t

        if [ -f "${EXPORT_DIR}/schema/failed.sql" ]
        then
            cat "${EXPORT_DIR}/schema/failed.sql"
            exit 1
        fi
    fi

	step "Run validations."
	if [ -x "${TEST_DIR}/validate" ]
	then
		 "${TEST_DIR}/validate"
	fi

	step "End Migration: clearing metainfo about state of migration from everywhere."
	end_migration --yes

	step "Clean up"
	./cleanup-db
	rm -rf "${EXPORT_DIR}"
	run_ysql yugabyte "DROP DATABASE IF EXISTS \"${TARGET_DB_NAME}\";"
}

main
