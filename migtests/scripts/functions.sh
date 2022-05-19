
step() {
	echo "==============================================================="
	echo "STEP:" $*
	echo "==============================================================="
}

print_env() {
	echo "==============================================================="
	echo "REPO_ROOT=${REPO_ROOT}"
	echo "EXPORT_DIR=${EXPORT_DIR}"
	echo ""
	echo "SOURCE_DB_TYPE=${SOURCE_DB_TYPE}"
	echo "SOURCE_DB_HOST=${SOURCE_DB_HOST}"
	echo "SOURCE_DB_PORT=${SOURCE_DB_PORT}"
	echo "SOURCE_DB_USER=${SOURCE_DB_USER}"
	echo "SOURCE_DB_PASSWORD=${SOURCE_DB_PASSWORD}"
	echo "SOURCE_DB_NAME=${SOURCE_DB_NAME}"
	echo ""
	echo "TARGET_DB_HOST=${TARGET_DB_HOST}"
	echo "TARGET_DB_PORT=${TARGET_DB_PORT}"
	echo "TARGET_DB_USER=${TARGET_DB_USER}"
	echo "TARGET_DB_PASSWORD=${TARGET_DB_PASSWORD}"
	echo "TARGET_DB_NAME=${TARGET_DB_NAME}"
	echo "==============================================================="
}


run_psql() {
	db_name=$1
	sql=$2
	psql "postgresql://postgres:secret@127.0.0.1:5432/${db_name}" -c "${sql}"
}

run_pg_restore() {
	db_name=$1
	file_name=$2
	pg_restore --no-password -h 127.0.0.1 -p 5432 -U postgres -d ${db_name} ${file_name}
}

run_ysql() {
	db_name=$1
	sql=$2
	docker exec yb-tserver-n1 /home/yugabyte/bin/ysqlsh -h yb-tserver-n1 -d "$db_name" -c "$sql"
}

export_schema() {
	yb_migrate export schema --export-dir ${EXPORT_DIR} \
		--source-db-type ${SOURCE_DB_TYPE} \
		--source-db-host ${SOURCE_DB_HOST} \
		--source-db-port ${SOURCE_DB_PORT} \
		--source-db-user ${SOURCE_DB_USER} \
		--source-db-password ${SOURCE_DB_PASSWORD} \
		--source-db-name ${SOURCE_DB_NAME} \
		$*
}

analyze_schema() {
	yb_migrate analyze-schema --export-dir ${EXPORT_DIR} \
		--source-db-type ${SOURCE_DB_TYPE} \
		--source-db-host ${SOURCE_DB_HOST} \
		--source-db-port ${SOURCE_DB_PORT} \
		--source-db-user ${SOURCE_DB_USER} \
		--source-db-password ${SOURCE_DB_PASSWORD} \
		--source-db-name ${SOURCE_DB_NAME} \
		--output-format txt \
		$*
}

import_schema() {
	yb_migrate import schema --export-dir ${EXPORT_DIR} \
		--target-db-host ${TARGET_DB_HOST} \
		--target-db-port ${TARGET_DB_PORT} \
		--target-db-user ${TARGET_DB_USER} \
		--target-db-password ${TARGET_DB_PASSWORD:-''} \
		--target-db-name ${TARGET_DB_NAME} \
		$*
}
