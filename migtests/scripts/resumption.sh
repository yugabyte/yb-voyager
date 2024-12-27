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

source ${SCRIPTS}/yugabytedb/env.sh

source ${SCRIPTS}/functions.sh

main() {
	echo "Deleting the parent export-dir present in the test directory"
	rm -rf ${EXPORT_DIR}	
	echo "Creating export-dir in the parent test directory"
	mkdir -p ${EXPORT_DIR}
	echo "Assigning permissions to the export-dir to execute init-db script"
	chmod +x ${TEST_DIR}/init-target-db

	if [ -f "${TEST_DIR}/generate_config.py" ]; then
	  chmod +x "${TEST_DIR}/generate_config.py"	  
	fi

	step "START: ${TEST_NAME}"
	print_env

	pushd ${TEST_DIR}

	step "Check the Voyager version installed"
	yb-voyager version

	step "Initialise target database."
	./init-target-db

	step "Generate the YAML file"
	if [ -f "${TEST_DIR}/generate_config.py" ]; then
	  ./generate_config.py
	fi

	step "Run import with resumptions"

	${SCRIPTS}/resumption.py config.yaml

	step "Clean up"
	rm -rf "${EXPORT_DIR}"
	if [ -f "${TEST_DIR}/generate_config.py" ]; then
	  rm config.yaml
	fi
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
}

main
