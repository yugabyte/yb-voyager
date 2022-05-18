#!/usr/bin/env bash

set -e
set -x

export TOP="${PWD}"
export SCRIPTS="${TOP}/migtests/scripts"
export TESTS_DIR="${TOP}/migtests/tests"
export PG_PASSWORD="secret"

source ${SCRIPTS}/functions.sh

main() {
	echo "127.0.0.1:5432:*:postgres:secret" >> ~/.pgpass
	chmod 0600 ~/.pgpass

	echo "TOP=${TOP}"
	for d in `ls ${TESTS_DIR}`
	do
		test_dir="${TESTS_DIR}/${d}"

		source ${test_dir}/env.sh
		echo "===================================================="
		echo "START: ${SOURCE_DB_TYPE}/${SOURCE_DB_NAME}"
		echo "===================================================="

		pushd ${test_dir}
		run_psql postgres "SELECT version();"
		./init-db
		popd
	done
}

main
