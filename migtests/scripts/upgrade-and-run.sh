#!/usr/bin/env bash

set -e
set -x

if [ $# -gt 3 ]; then
    echo "Usage: $0 TEST_TYPE TEST_NAME [env.sh]"
    exit 1
fi

export YB_VOYAGER_SEND_DIAGNOSTICS=false
export TEST_TYPE=$1
export TEST_NAME=$2

export REPO_ROOT="${PWD}"
export SCRIPTS="${REPO_ROOT}/migtests/scripts"
export TESTS_DIR="${REPO_ROOT}/migtests/tests"
export TEST_TYPE_DIR="${TESTS_DIR}/upgrade-tests/${TEST_TYPE}"
export TEST_DIR="${TESTS_DIR}/${TEST_NAME}"
export EXPORT_DIR=${EXPORT_DIR:-"${TEST_DIR}/export-dir"}
export PYTHONPATH="${REPO_ROOT}/migtests/lib"
export LAST_BREAKING_RELEASE=${LAST_BREAKING_RELEASE:-"1.8.5-0"}

# Load environment configurations
if [ $3 != "" ]; then
    if [ ! -f "${TEST_DIR}/$3" ]; then
        echo "$3 file not found in the test directory"
        exit 1
    fi
    source ${TEST_DIR}/$3
else
    source ${TEST_DIR}/env.sh
fi
source ${SCRIPTS}/${SOURCE_DB_TYPE}/env.sh
source ${SCRIPTS}/yugabytedb/env.sh
source ${SCRIPTS}/functions.sh

run_script() {
    local script_file=$1
    if [ -f "${script_file}" ]; then
        step "Running script: ${script_file}"
        source "${script_file}"
    else
        echo "Script ${script_file} not found, skipping."
    fi
}

main() {
    echo ${REPO_ROOT}
    echo "Deleting and recreating export-dir."
    rm -rf ${EXPORT_DIR}
    mkdir -p ${EXPORT_DIR}
    chmod +x ${TEST_DIR}/init-db ${TEST_DIR}/cleanup-db

    step "START: ${TEST_NAME}"
    print_env

    pushd ${TEST_DIR}

    step "Initialize source database."
    ./init-db

    step "Create target database."
	run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
	if [ "${SOURCE_DB_TYPE}" = "postgresql" ] || [ "${SOURCE_DB_TYPE}" = "oracle" ]; then
		run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME} with COLOCATION=TRUE"
	else
		run_ysql yugabyte "CREATE DATABASE ${TARGET_DB_NAME}"
	fi


    sudo rm -rf $(which yb-voyager)
    # Install older version of yb-voyager
    step "Installing Voyager version ${LAST_BREAKING_RELEASE}."
    wget https://s3.us-west-2.amazonaws.com/downloads.yugabyte.com/repos/reporpms/yb-apt-repo_1.0.0_all.deb
    sudo apt-get install ./yb-apt-repo_1.0.0_all.deb -y
    sudo apt-get clean
    sudo apt-get update
    sudo apt install -y postgresql-common
    echo | sudo /usr/share/postgresql-common/pgdg/apt.postgresql.org.sh
    sudo apt-get install ora2pg=23.2-yb.2 -y
    sudo apt-get remove -y "yb-voyager=${LAST_BREAKING_RELEASE}"
    install_output=$(sudo apt-get install -y "yb-voyager=${LAST_BREAKING_RELEASE}" 2>&1) || {
        echo "Initial installation failed. Parsing errors to resolve dependencies..."
        resolve_and_install_dependencies "$install_output"
    echo "Retrying Installation"
    sudo apt-get install -y "yb-voyager=${LAST_BREAKING_RELEASE}"
    }
    source ~/.bashrc
    step "Check Voyager version."
    yb-voyager version

    step "Grant permissions."
    grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA}

    # Run before steps
    run_script "${TEST_TYPE_DIR}/before"

    # Upgrade yb-voyager
    # Can only test once the release is done. Testing with local build until then
    # step "Upgrading Voyager."
    # sudo apt-get upgrade -y yb-voyager

    sudo rm -rf $(which yb-voyager)
    cd ${REPO_ROOT}/yb-voyager
    go build
    sudo mv yb-voyager /usr/bin/yb-voyager
    source ~/.bashrc

    pushd ${TEST_DIR}

    step "Check Voyager version after upgrade."
    yb-voyager version

    # Run after steps
    run_script "${TEST_TYPE_DIR}/after"

    # Clean up
    step "Cleaning up."
    ./cleanup-db
    rm -rf "${EXPORT_DIR}"
    run_ysql yugabyte "DROP DATABASE IF EXISTS ${TARGET_DB_NAME};"
}

main

