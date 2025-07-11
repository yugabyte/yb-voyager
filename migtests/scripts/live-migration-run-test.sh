#!/usr/bin/env bash

set -e
set -m

if [ $# -gt 2 ]; then
    echo "Usage: $0 TEST_NAME [--run-via-config-file]"
    exit 1
fi

set -x

export VOYAGER_WORKFLOW="live-migration"
export YB_VOYAGER_SEND_DIAGNOSTICS=false
export TEST_NAME=$1

export REPO_ROOT="${PWD}"
export SCRIPTS="${REPO_ROOT}/migtests/scripts"
export TESTS_DIR="${REPO_ROOT}/migtests/tests"
export TEST_DIR="${TESTS_DIR}/${TEST_NAME}"
export QUEUE_SEGMENT_MAX_BYTES=400

export PYTHONPATH="${REPO_ROOT}/migtests/lib"

run_via_config_file=false

# Parse optional second argument
if [ $# -eq 2 ]; then
    if [ "$2" = "--run-via-config-file" ]; then
        run_via_config_file=true
    else
        echo "Unknown option: $2"
        echo "Usage: $0 TEST_NAME [--run-via-config-file]"
        exit 1
    fi
fi

# Order of env.sh import matters.
if [ -f "${TEST_DIR}/live_env.sh" ]; then
    source "${TEST_DIR}/live_env.sh"
else
    source "${TEST_DIR}/env.sh"
fi

if [ "${SOURCE_DB_TYPE}" = "oracle" ]
then
	source ${SCRIPTS}/${SOURCE_DB_TYPE}/live_env.sh 
else
	source ${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh
fi

source ${SCRIPTS}/functions.sh

normalize_and_export_vars "live"

source ${SCRIPTS}/yugabytedb/env.sh

# Handling for config generation
if [ "${run_via_config_file}" = true ]; then
	CONFIG_TEMPLATE="${SCRIPTS}/config-templates/live-migration.yaml"
	generate_voyager_config "$CONFIG_TEMPLATE"
fi

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

	step "Initialise source database."
	if [ "${SOURCE_DB_TYPE}" = "oracle" ]
	then
		create_source_db ${SOURCE_DB_SCHEMA}
	fi
	./init-db

	step "Grant source database user permissions for live migration"
	grant_permissions_for_live_migration

	step "Check the Voyager version installed"
	yb-voyager version

	step "Assess Migration"
	if [ "${SOURCE_DB_TYPE}" = "postgresql" ] || [ "${SOURCE_DB_TYPE}" == "oracle" ]; then
		assess_migration || {
			cat_log_file "yb-voyager-assess-migration.log"
			cat_file ${EXPORT_DIR}/assessment/metadata/yb-voyager-assessment.log
		}

		step "Validate Assessment Reports"
		# Checking if the assessment reports were created
		if [ -f "${EXPORT_DIR}/assessment/reports/migration_assessment_report.html" ] && [ -f "${EXPORT_DIR}/assessment/reports/migration_assessment_report.json" ]; then
			echo "Assessment reports created successfully."
			validate_failure_reasoning "${EXPORT_DIR}/assessment/reports/migration_assessment_report.json"
			#TODO: Further validation to be added
		else
			echo "Error: Assessment reports were not created successfully."
			cat_log_file "yb-voyager-assess-migration.log"
			exit 1
		fi

		post_assess_migration
	fi

	step "Export schema."
	export_schema
	find ${EXPORT_DIR}/schema -name '*.sql' -printf "'%p'\n"| xargs grep -wh CREATE

	step "Analyze schema."
	analyze_schema
	tail -20 ${EXPORT_DIR}/reports/schema_analysis_report.json

	step "Fix schema."
	if [ -x "${TEST_DIR}/fix-schema" ]
	then
		 "${TEST_DIR}/fix-schema"
	fi

	step "Analyze schema."
	analyze_schema
	tail -20 ${EXPORT_DIR}/reports/schema_analysis_report.json

	step "Create target database."
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
	if [ "${SOURCE_DB_TYPE}" = "postgresql" ] || [ "${SOURCE_DB_TYPE}" = "oracle" ]; then
		run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME} with COLOCATION=TRUE"
	else
		run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME}"
	fi

	step "Import schema."
	import_schema
	run_ysql ${TARGET_DB_NAME} "\dt"

	step "Run Schema validations."
	if [ -x "${TEST_DIR}/validate-schema" ]
	then
		 "${TEST_DIR}/validate-schema"
	fi

	step "Export data."
	# false if exit code of export_data is non-zero
	export_data || { 
		tail_log_file "yb-voyager-export-data.log"
		tail_log_file "debezium-source_db_exporter.log"
		exit 1
	} &

	# Storing the pid for the export data command
	exp_pid=$!

	# Killing the export process in case of failure
	trap "kill_process -${exp_pid} ; exit 1" SIGINT SIGTERM EXIT SIGSEGV SIGHUP

	# Waiting for snapshot to complete
	timeout 100 bash -c -- 'while [ ! -f ${EXPORT_DIR}/metainfo/dataFileDescriptor.json ]; do sleep 3; done'

	ls -R ${EXPORT_DIR}/data | sed 's/:$//' | sed -e 's/[^-][^\/]*\//--/g' -e 's/^/   /' -e 's/-/|/'

	cat ${EXPORT_DIR}/data/export_status.json || echo "No export_status.json found."
	cat ${EXPORT_DIR}/metainfo/dataFileDescriptor.json

	step "Import data."
	import_data || { 
		tail_log_file "yb-voyager-import-data.log"
		exit 1
	} &

	# Storing the pid for the import data command
	imp_pid=$!

	# Updating the trap command to include the importer
	trap "kill_process -${exp_pid} ; kill_process -${imp_pid} ; exit 1" SIGINT SIGTERM EXIT SIGSEGV SIGHUP

	step "Archive Changes."
	archive_changes &

	sleep 60 

	step "Import remaining schema (FK, index, and trigger) and Refreshing MViews if present."
	finalize_schema_post_data_import

	step "Run snapshot validations."
	"${TEST_DIR}/validate" --live_migration 'true' --ff_enabled 'false' --fb_enabled 'false'

	step "Inserting new events"
	run_sql_file source_delta.sql

	sleep 120

	# Resetting the trap command
	trap - SIGINT SIGTERM EXIT SIGSEGV SIGHUP

	step "Initiating cutover"
	cutover_to_target

	for ((i = 0; i < 10; i++)); do
    if [ "$(yb-voyager cutover status --export-dir "${EXPORT_DIR}" | grep "cutover to target status" | cut -d ':'  -f 2 | tr -d '[:blank:]')" != "COMPLETED" ]; then
        echo "Waiting for cutover to be COMPLETED..."
        sleep 20
        if [ "$i" -eq 9 ]; then
            tail_log_file "yb-voyager-export-data.log"
            tail_log_file "yb-voyager-import-data.log"
			tail_log_file "debezium-source_db_exporter.log"		
			exit 1
        fi
    else
        break
    fi
	done


	step "Run final validations."
	if [ -x "${TEST_DIR}/validateAfterChanges" ]
	then
	"${TEST_DIR}/validateAfterChanges" --ff_enabled 'false' --fb_enabled 'false'
	fi

	step "Run get data-migration-report"
	get_data_migration_report

	expected_file="${TEST_DIR}/data-migration-report-live-migration.json"
	actual_file="${EXPORT_DIR}/reports/data-migration-report.json"

	step "Verify data-migration-report report"
	verify_report ${expected_file} ${actual_file}

	step "End Migration: clearing metainfo about state of migration from everywhere"
	end_migration --yes

	step "Clean up"
	./cleanup-db
	rm -rf "${EXPORT_DIR}"
	if [ "${run_via_config_file}" = true ]; then
	rm -f "${GENERATED_CONFIG}"
	fi
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
}

main
