
step() {
	echo "==============================================================="
	echo "STEP:" $*
	echo "==============================================================="
}

print_env() {
	echo "==============================================================="
	echo "REPO_ROOT=${REPO_ROOT}"
	echo ""
	echo "SOURCE_DB_TYPE=${SOURCE_DB_TYPE}"
	echo "SOURCE_DB_HOST=${SOURCE_DB_HOST}"
	echo "SOURCE_DB_PORT=${SOURCE_DB_PORT}"
	echo "SOURCE_DB_USER=${SOURCE_DB_USER}"
	echo "SOURCE_DB_PASSWORD=${SOURCE_DB_PASSWORD}"
	echo "SOURCE_DB_NAME=${SOURCE_DB_NAME}"
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
