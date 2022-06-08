
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
	echo "SOURCE_DB_SCHEMA=${SOURCE_DB_SCHEMA}"
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
	psql "postgresql://${SOURCE_DB_USER}:${SOURCE_DB_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}" -c "${sql}"
}

run_pg_restore() {
	db_name=$1
	file_name=$2
	export PGPASSWORD=${SOURCE_DB_PASSWORD}
	pg_restore --no-password -h ${SOURCE_DB_HOST} -p ${SOURCE_DB_PORT} \
		-U ${SOURCE_DB_USER} -d ${db_name} ${file_name}
	unset PGPASSWORD
}

run_ysql() {
	db_name=$1
	sql=$2
	psql "postgresql://yugabyte:${TARGET_DB_PASSWORD}@${TARGET_DB_HOST}:${TARGET_DB_PORT}/${db_name}" -c "${sql}"
}

run_mysql() {
	db_name=$1
	sql=$2
	mysql -u ${SOURCE_DB_USER} -p${SOURCE_DB_PASSWORD} -h ${SOURCE_DB_HOST} -P ${SOURCE_DB_PORT} -D ${db_name} -e "${sql}"
}

export_schema() {
	args="--export-dir ${EXPORT_DIR}
		--source-db-type ${SOURCE_DB_TYPE}
		--source-db-host ${SOURCE_DB_HOST}
		--source-db-port ${SOURCE_DB_PORT}
		--source-db-user ${SOURCE_DB_USER}
		--source-db-password ${SOURCE_DB_PASSWORD}
		--source-db-name ${SOURCE_DB_NAME}
		--send-diagnostics=false
	"
	if [ "${SOURCE_DB_SCHEMA}" != "" ]
	then
		args="${args} --source-db-schema ${SOURCE_DB_SCHEMA}"
	fi
	yb_migrate export schema ${args} $*
}

export_data() {
	args="--export-dir ${EXPORT_DIR}
		--source-db-type ${SOURCE_DB_TYPE}
		--source-db-host ${SOURCE_DB_HOST}
		--source-db-port ${SOURCE_DB_PORT}
		--source-db-user ${SOURCE_DB_USER}
		--source-db-password ${SOURCE_DB_PASSWORD}
		--source-db-name ${SOURCE_DB_NAME}
		--send-diagnostics=false
	"
	if [ "${SOURCE_DB_SCHEMA}" != "" ]
	then
		args="${args} --source-db-schema ${SOURCE_DB_SCHEMA}"
	fi
	yb_migrate export data ${args} $*
}

analyze_schema() {
	args="--export-dir ${EXPORT_DIR}
		--source-db-type ${SOURCE_DB_TYPE}
		--source-db-host ${SOURCE_DB_HOST}
		--source-db-port ${SOURCE_DB_PORT}
		--source-db-user ${SOURCE_DB_USER}
		--source-db-password ${SOURCE_DB_PASSWORD}
		--source-db-name ${SOURCE_DB_NAME}
		--output-format txt
		--send-diagnostics=false
	"
        if [ "${SOURCE_DB_SCHEMA}" != "" ]
        then
                args="${args} --source-db-schema ${SOURCE_DB_SCHEMA}"
        fi
        yb_migrate analyze-schema ${args} $*
}

import_schema() {
	yb_migrate import schema --export-dir ${EXPORT_DIR} \
		--target-db-host ${TARGET_DB_HOST} \
		--target-db-port ${TARGET_DB_PORT} \
		--target-db-user ${TARGET_DB_USER} \
		--target-db-password ${TARGET_DB_PASSWORD:-''} \
		--target-db-name ${TARGET_DB_NAME} \
		--yes \
		--send-diagnostics=false \
		$*
}

import_data() {
	yb_migrate import data --export-dir ${EXPORT_DIR} \
		--target-db-host ${TARGET_DB_HOST} \
		--target-db-port ${TARGET_DB_PORT} \
		--target-db-user ${TARGET_DB_USER} \
		--target-db-password ${TARGET_DB_PASSWORD:-''} \
		--target-db-name ${TARGET_DB_NAME} \
		--disable-pb \
		--send-diagnostics=false \
		$*
}
