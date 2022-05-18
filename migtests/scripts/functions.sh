
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
