
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
	psql "postgresql://postgres:secret@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}" -c "${sql}"
}

grant_user_permission_postgresql() {
	db_name=$1

	echo "SELECT 'GRANT USAGE ON SCHEMA ' || schema_name || ' TO ybvoyager;' FROM information_schema.schemata; \gexec" | psql "postgresql://${SOURCE_DB_USER}:${SOURCE_DB_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}" 
	echo "SELECT 'GRANT SELECT ON ALL TABLES IN SCHEMA ' || schema_name || ' TO ybvoyager;' FROM information_schema.schemata; \gexec" | psql "postgresql://${SOURCE_DB_USER}:${SOURCE_DB_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}" 
	echo "SELECT 'GRANT SELECT ON ALL SEQUENCES IN SCHEMA ' || schema_name || ' TO ybvoyager;' FROM information_schema.schemata; \gexec" | psql "postgresql://${SOURCE_DB_USER}:${SOURCE_DB_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}" 
}

run_pg_restore() {
	db_name=$1
	file_name=$2
	export PGPASSWORD="secret"
	pg_restore --no-password -h ${SOURCE_DB_HOST} -p ${SOURCE_DB_PORT} \
		-U postgres -d ${db_name} ${file_name}
	unset PGPASSWORD
}

run_ysql() {
	db_name=$1
	sql=$2
	psql "postgresql://yugabyte:""@${TARGET_DB_HOST}:${TARGET_DB_PORT}/${db_name}" -c "${sql}"
}

run_mysql() {
	db_name=$1
	sql=$2
	mysql -u root -proot -h ${SOURCE_DB_HOST} -P ${SOURCE_DB_PORT} -D ${db_name} -e "${sql}"
}

grant_user_permission_mysql() {
	db_name=$1
	
	run_mysql ${db_name} "GRANT PROCESS ON *.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"
	run_mysql ${db_name} "GRANT SELECT ON ${SOURCE_DB_NAME}.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"
	run_mysql ${db_name} "GRANT SHOW VIEW ON ${SOURCE_DB_NAME}.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"
	run_mysql ${db_name} "GRANT TRIGGER ON ${SOURCE_DB_NAME}.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"

	# For MySQL >= 8.0.20
	# run_mysql ${db_name} "GRANT SHOW_ROUTINE  ON *.* TO 'ybvoyager'@'${SOURCE_DB_HOST}';"

	# For older versions
	run_mysql ${db_name} "GRANT SELECT ON *.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"

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
	yb-voyager export schema ${args} $*
}

export_data() {
	args="--export-dir ${EXPORT_DIR}
		--source-db-type ${SOURCE_DB_TYPE}
		--source-db-host ${SOURCE_DB_HOST}
		--source-db-port ${SOURCE_DB_PORT}
		--source-db-user ${SOURCE_DB_USER}
		--source-db-password ${SOURCE_DB_PASSWORD}
		--source-db-name ${SOURCE_DB_NAME}
		--disable-pb
		--send-diagnostics=false
	"
	if [ "${SOURCE_DB_SCHEMA}" != "" ]
	then
		args="${args} --source-db-schema ${SOURCE_DB_SCHEMA}"
	fi
	yb-voyager export data ${args} $*
}

analyze_schema() {
	args="--export-dir ${EXPORT_DIR}
		--output-format txt
		--send-diagnostics=false
	"
    yb-voyager analyze-schema ${args} $*
}

import_schema() {
	yb-voyager import schema --export-dir ${EXPORT_DIR} \
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
	yb-voyager import data --export-dir ${EXPORT_DIR} \
		--target-db-host ${TARGET_DB_HOST} \
		--target-db-port ${TARGET_DB_PORT} \
		--target-db-user ${TARGET_DB_USER} \
		--target-db-password ${TARGET_DB_PASSWORD:-''} \
		--target-db-name ${TARGET_DB_NAME} \
		--disable-pb \
		--send-diagnostics=false \
		$*
}

import_data_file() {
	yb-voyager import data file --export-dir ${EXPORT_DIR} \
		--target-db-host ${TARGET_DB_HOST} \
		--target-db-port ${TARGET_DB_PORT} \
		--target-db-user ${TARGET_DB_USER} \
		--target-db-password ${TARGET_DB_PASSWORD:-''} \
		--target-db-name ${TARGET_DB_NAME} \
		--disable-pb \
		--send-diagnostics=false \
		$*
}
