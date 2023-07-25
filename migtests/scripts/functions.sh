
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
	conn_string="postgresql://${SOURCE_DB_ADMIN_USER}:${SOURCE_DB_ADMIN_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}"
	psql "${conn_string}" -c "${sql}"
}

psql_import_file() {
	db_name=$1
	file=$2
	conn_string="postgresql://${SOURCE_DB_ADMIN_USER}:${SOURCE_DB_ADMIN_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}"
	psql "${conn_string}" -f "${file}"
}

grant_user_permission_postgresql() {
	db_name=$1
	conn_string="postgresql://${SOURCE_DB_ADMIN_USER}:${SOURCE_DB_ADMIN_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}" 
	commands=(
		"SELECT 'GRANT USAGE ON SCHEMA '"
		"SELECT 'GRANT SELECT ON ALL TABLES IN SCHEMA '" 
		"SELECT 'GRANT SELECT ON ALL SEQUENCES IN SCHEMA '"
		)
	for command in "${commands[@]}"; do
		echo "${command} || schema_name || ' TO ${SOURCE_DB_USER};' FROM information_schema.schemata; \gexec" | psql "${conn_string}" 
	done
}

run_pg_restore() {
	db_name=$1
	file_name=$2
	export PGPASSWORD=${SOURCE_DB_ADMIN_PASSWORD}
	pg_restore --no-password -h ${SOURCE_DB_HOST} -p ${SOURCE_DB_PORT} \
		-U ${SOURCE_DB_ADMIN_USER} -d ${db_name} ${file_name}
	unset PGPASSWORD
}

run_ysql() {
	db_name=$1
	sql=$2
	psql "postgresql://${TARGET_DB_ADMIN_USER}:${TARGET_DB_ADMIN_PASSWORD}@${TARGET_DB_HOST}:${TARGET_DB_PORT}/${db_name}" -c "${sql}"
}

ysql_import_file() {
	db_name=$1
	file=$2
	conn_string="postgresql://${TARGET_DB_ADMIN_USER}:${TARGET_DB_ADMIN_PASSWORD}@${TARGET_DB_HOST}:${TARGET_DB_PORT}/${db_name}"
	psql "${conn_string}" -f "${file}"
}

run_mysql() {
	db_name=$1
	sql=$2
	mysql -u ${SOURCE_DB_ADMIN_USER} -p${SOURCE_DB_ADMIN_PASSWORD} -h ${SOURCE_DB_HOST} -P ${SOURCE_DB_PORT} -D ${db_name} -e "${sql}"
}

grant_user_permission_mysql() {
	db_name=$1
	
	commands=(
		"GRANT PROCESS ON *.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"
		"GRANT SELECT ON ${SOURCE_DB_NAME}.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"
		"GRANT SHOW VIEW ON ${SOURCE_DB_NAME}.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"
		"GRANT TRIGGER ON ${SOURCE_DB_NAME}.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"
    # Extra steps required to enable Debezium export 
		"GRANT FLUSH_TABLES ON *.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"
		"GRANT REPLICATION CLIENT ON *.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"
	)

	for command in "${commands[@]}"; do
		run_mysql ${db_name} "${command}"
	done

	# For MySQL >= 8.0.20
	# run_mysql ${db_name} "GRANT SHOW_ROUTINE  ON *.* TO 'ybvoyager'@'${SOURCE_DB_HOST}';"

	# For older versions
	run_mysql ${db_name} "GRANT SELECT ON *.* TO '${SOURCE_DB_USER}'@'${SOURCE_DB_HOST}';"

}

grant_user_permission_oracle(){
	db_name=$1
	db_schema=$2

	cat > oracle-inputs.sql << EOF
	GRANT CONNECT TO ybvoyager;
	GRANT SELECT_CATALOG_ROLE TO ybvoyager;
	GRANT SELECT ANY DICTIONARY TO ybvoyager;
	GRANT SELECT ON SYS.ARGUMENT$ TO ybvoyager;
	BEGIN
		FOR R IN (SELECT owner, object_name FROM all_objects WHERE owner=UPPER('${db_schema}') and object_type = 'TYPE') LOOP
	   		EXECUTE IMMEDIATE 'grant execute on '||R.owner||'."'||R.object_name||'" to ybvoyager';
		END LOOP;
	END;
	/
	BEGIN
		FOR R IN (SELECT owner, object_name FROM all_objects WHERE owner=UPPER('${db_schema}') and object_type in ('VIEW','SEQUENCE','TABLE PARTITION','SYNONYM','MATERIALIZED VIEW')) LOOP
	    	EXECUTE IMMEDIATE 'grant select on '||R.owner||'."'||R.object_name||'" to ybvoyager';
		END LOOP;
	END;
	/
	BEGIN
		FOR R IN (SELECT owner, object_name FROM all_objects WHERE owner=UPPER('${db_schema}') and object_type ='TABLE' MINUS SELECT owner, table_name from all_nested_tables where owner = UPPER('${db_schema}')) LOOP
			EXECUTE IMMEDIATE 'grant select on '||R.owner||'."'||R.object_name||'" to  ybvoyager';
		END LOOP;
	END;
	/
	/*
	Extra steps required to enable Debezium export
	*/
	GRANT FLASHBACK ANY TABLE TO ybvoyager;
EOF
	run_sqlplus_as_sys ${db_name} "oracle-inputs.sql"	

}

grant_permissions_for_live_migration_oracle() {
	cdb_name=$1
	pdb_name=$2

	cat > create-pdb-tablespace.sql << EOF
		CREATE TABLESPACE logminer_tbs DATAFILE '/opt/oracle/oradata/ORCLCDB/ORCLPDB1/logminer_tbs.dbf'
    	SIZE 25M REUSE AUTOEXTEND ON MAXSIZE UNLIMITED;
EOF
	run_sqlplus_as_sys ${pdb_name} "create-pdb-tablespace.sql"
	cp ${SCRIPTS}/oracle/live-grants.sql oracle-inputs.sql
	run_sqlplus_as_sys ${cdb_name} "oracle-inputs.sql"
}

grant_permissions() {
	db_name=$1
	db_type=$2
	db_schema=$3
	case ${db_type} in
		postgresql)
			grant_user_permission_postgresql ${db_name}
			;;
		mysql)
			grant_user_permission_mysql ${db_name}
			;;
		oracle)
			grant_user_permission_oracle ${db_name} ${db_schema}
			;;
		*)
			echo "ERROR: grant_permissions not implemented for ${SOURCE_DB_TYPE}"
			exit 1
			;;
	esac
}


run_sqlplus_as_sys() {
	db_name=$1
    local file_name=$2
    conn_string="${SOURCE_DB_USER_SYS}/${SOURCE_DB_USER_SYS_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name} as SYSDBA"
	echo exit | sqlplus -f "${conn_string}" @"${file_name}"
}


run_sqlplus_as_schema_owner() {
    db_name=$1
    sql=$2
    conn_string="${SOURCE_DB_USER_SCHEMA_OWNER}/${SOURCE_DB_USER_SCHEMA_OWNER_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}"
    echo exit | sqlplus -f "${conn_string}" @"${sql}"
}

export_schema() {
	args="--export-dir ${EXPORT_DIR}
		--source-db-type ${SOURCE_DB_TYPE}
		--source-db-host ${SOURCE_DB_HOST}
		--source-db-port ${SOURCE_DB_PORT}
		--source-db-user ${SOURCE_DB_USER}
		--source-db-password ${SOURCE_DB_PASSWORD}
		--source-db-name ${SOURCE_DB_NAME}
		--send-diagnostics=false --yes
		--start-clean
	"
	if [ "${SOURCE_DB_SCHEMA}" != "" ]
	then
		args="${args} --source-db-schema ${SOURCE_DB_SCHEMA}"
	fi
	if [ "${SOURCE_DB_SSL_MODE}" != "" ]
	then
		args="${args} --source-ssl-mode ${SOURCE_DB_SSL_MODE}"
	fi

	if [ "${SOURCE_DB_SSL_CERT}" != "" ]
	then
		args="${args} --source-ssl-cert ${SOURCE_DB_SSL_CERT}"
	fi

	if [ "${SOURCE_DB_SSL_KEY}" != "" ]
	then
		args="${args} --source-ssl-key ${SOURCE_DB_SSL_KEY}"
	fi

	if [ "${SOURCE_DB_SSL_ROOT_CERT}" != "" ]
	then
		args="${args} --source-ssl-root-cert ${SOURCE_DB_SSL_ROOT_CERT}"
	fi
	
	yb-voyager export schema ${args} $*
}

export_data() {
	args="--export-dir ${EXPORT_DIR}
		--source-db-type ${SOURCE_DB_TYPE}
		--source-db-user ${SOURCE_DB_USER}
		--source-db-password ${SOURCE_DB_PASSWORD}
		--source-db-name ${SOURCE_DB_NAME}
		--disable-pb
		--send-diagnostics=false
		--yes
		--start-clean
	"
	if [ "${SOURCE_DB_ORACLE_TNS_ALIAS}" != "" ]
	then
		args="${args} --oracle-tns-alias ${SOURCE_DB_ORACLE_TNS_ALIAS}"
	else
		args="${args} --source-db-host ${SOURCE_DB_HOST} --source-db-port ${SOURCE_DB_PORT}"
	fi

	if [ "${SOURCE_DB_SCHEMA}" != "" ]
	then
		args="${args} --source-db-schema ${SOURCE_DB_SCHEMA}"
	fi

	if [ "${SOURCE_DB_SSL_MODE}" != "" ]
	then
		args="${args} --source-ssl-mode ${SOURCE_DB_SSL_MODE}"
	fi

	if [ "${SOURCE_DB_SSL_CERT}" != "" ]
	then
		args="${args} --source-ssl-cert ${SOURCE_DB_SSL_CERT}"
	fi

	if [ "${SOURCE_DB_SSL_KEY}" != "" ]
	then
		args="${args} --source-ssl-key ${SOURCE_DB_SSL_KEY}"
	fi

	if [ "${SOURCE_DB_SSL_ROOT_CERT}" != "" ]
	then
		args="${args} --source-ssl-root-cert ${SOURCE_DB_SSL_ROOT_CERT}"
	fi

	if [ "${ORACLE_CDB_NAME}" != "" ]
	then
		args="${args} --oracle-cdb-name ${ORACLE_CDB_NAME}"
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
	args="--export-dir ${EXPORT_DIR} 
		--target-db-host ${TARGET_DB_HOST} 
		--target-db-port ${TARGET_DB_PORT} 
		--target-db-user ${TARGET_DB_USER} 
		--target-db-password ${TARGET_DB_PASSWORD:-''} 
		--target-db-name ${TARGET_DB_NAME} 
		--target-db-schema ${TARGET_DB_SCHEMA} 
		--yes
		--send-diagnostics=false
		"
		yb-voyager import schema ${args} $*
}

import_data() {
	args="
	 --export-dir ${EXPORT_DIR} 
		--target-db-host ${TARGET_DB_HOST} 
		--target-db-port ${TARGET_DB_PORT} 
		--target-db-user ${TARGET_DB_USER} 
		--target-db-password ${TARGET_DB_PASSWORD:-''} 
		--target-db-name ${TARGET_DB_NAME} 
		--target-db-schema ${TARGET_DB_SCHEMA} 
		--disable-pb
		--send-diagnostics=false 
		--start-clean
		"
		yb-voyager import data ${args} $*
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
		--parallel-jobs 3 \
		$* || {
			cat ${EXPORT_DIR}/metainfo/dataFileDescriptor.json
			exit 1
		}
}

tail_log_file() {
	log_file_name=$1
	if [ -f "${EXPORT_DIR}/logs/${log_file_name}" ]
	then
		echo "Printing ${log_file_name} file"
		tail -n 100 "${EXPORT_DIR}/logs/${log_file_name}"
	else
		echo "No ${log_file_name} found."
	fi	
}

kill_process() {
	to_be_killed=$1
	kill -15 ${to_be_killed}
}

run_sql_file() {
	file_name=$1
	if [ "${SOURCE_DB_TYPE}" = "mysql" ]
	then
		run_mysql ${SOURCE_DB_NAME} "SOURCE ${file_name};"
	elif [ "${SOURCE_DB_TYPE}" = "postgresql" ]
	then
		run_psql ${SOURCE_DB_NAME} "\i ${file_name};"
	elif [ "${SOURCE_DB_TYPE}" = "oracle" ]
	then
		run_sqlplus_as_schema_owner ${SOURCE_DB_NAME} ${file_name}
	else
		echo "Invalid source database."
	fi
}