
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
	db_schema=$2
	conn_string="postgresql://${SOURCE_DB_ADMIN_USER}:${SOURCE_DB_ADMIN_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}"
	psql "${conn_string}" -v voyager_user="${SOURCE_DB_USER}" -v schema_list="${db_schema}" -v is_live_migration=0 -f /opt/yb-voyager/guardrails-scripts/yb-voyager-pg-grant-migration-permissions.sql

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

grant_permissions_for_live_migration_pg() {
	db_name=$1
	db_schema=$2
	conn_string="postgresql://${SOURCE_DB_ADMIN_USER}:${SOURCE_DB_ADMIN_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}"
	psql "${conn_string}" -v voyager_user="${SOURCE_DB_USER}" -v schema_list="${db_schema}" -v replication_group='replication_group' -v is_live_migration=1 -v is_live_migration_fall_back=0 -f /opt/yb-voyager/guardrails-scripts/yb-voyager-pg-grant-migration-permissions.sql
}

grant_permissions() {
	db_name=$1
	db_type=$2
	db_schema=$3
	case ${db_type} in
		postgresql)
			grant_user_permission_postgresql ${db_name} ${db_schema}
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

run_sqlplus() {
	db_name=$1
   	db_schema=$2
   	db_password=$3
   	sql=$4
   	conn_string="${db_schema}/${db_password}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${db_name}"

	echo exit | sqlplus -f "${conn_string}" @"${sql}"
}

# Sample invocation without default values
# export_schema export_dir "${MY_EXPORT_DIR}" source_db_schema ${MY_SOURCE_DB_SCHEMA}

export_schema() {
    # Default values
    export_dir="${EXPORT_DIR}"
    source_db_schema="${SOURCE_DB_SCHEMA}"

    # Process arguments
    while [ $# -gt 0 ]; do
        case "$1" in
            export_dir)
                export_dir="$2"
                shift 2
                ;;
            source_db_schema)
                source_db_schema="$2"
                shift 2
                ;;
            *)
                break
                ;;
        esac
    done

    args="--export-dir ${export_dir}
 		--source-db-type ${SOURCE_DB_TYPE}
 		--source-db-user ${SOURCE_DB_USER}
 		--source-db-password ${SOURCE_DB_PASSWORD}
		--source-db-name ${SOURCE_DB_NAME}
		--send-diagnostics=false --yes
	"
	if [ "${source_db_schema}" != "" ]
	then
		args="${args} --source-db-schema ${source_db_schema}"
	fi
	if [ "${SOURCE_DB_ORACLE_TNS_ALIAS}" != "" ]
	then
		args="${args} --oracle-tns-alias ${SOURCE_DB_ORACLE_TNS_ALIAS}"
	else
		args="${args} --source-db-host ${SOURCE_DB_HOST} --source-db-port ${SOURCE_DB_PORT}"
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
		--disable-pb=true
		--send-diagnostics=false
		--yes
	"
	if [ "${TABLE_LIST}" != "" ]
	then
		args="${args} --table-list ${TABLE_LIST}"
	fi
	if [ "${SOURCE_DB_ORACLE_CDB_TNS_ALIAS}" != "" ]
	then
	    args="${args} --oracle-cdb-tns-alias ${SOURCE_DB_ORACLE_CDB_TNS_ALIAS}"
	fi

	if [ "${SOURCE_DB_ORACLE_TNS_ALIAS}" != "" ]
	then
	    args="${args} --oracle-tns-alias ${SOURCE_DB_ORACLE_TNS_ALIAS}"
	fi

	if [ "${SOURCE_DB_ORACLE_CDB_TNS_ALIAS}" = "" ] && [ "${SOURCE_DB_ORACLE_TNS_ALIAS}" = "" ]
	then
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

	if [ "${EXPORT_TABLE_LIST}" != "" ]
	then
		args="${args} --table-list ${EXPORT_TABLE_LIST}"
	fi

	if [ "${EXPORT_EX_TABLE_LIST}" != "" ]
	then
		args="${args} --exclude-table-list ${EXPORT_EX_TABLE_LIST}"
	fi

	if [ "${EXPORT_TABLE_LIST_FILE_PATH}" != "" ]
	then
		args="${args} --table-list-file-path ${EXPORT_TABLE_LIST_FILE_PATH}"
	fi

	if [ "${EXPORT_EX_TABLE_LIST_FILE_PATH}" != "" ]
	then
		args="${args} --exclude-table-list-file-path ${EXPORT_EX_TABLE_LIST_FILE_PATH}"
	fi

	yb-voyager export data ${args} $*

}

analyze_schema() {
	args="--export-dir ${EXPORT_DIR}
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
		--yes
		--send-diagnostics=false
		"

		if [ "${SOURCE_DB_TYPE}" != "postgresql" ]
		then
			args="${args} --target-db-schema ${TARGET_DB_SCHEMA}"
		fi

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
		--disable-pb true
		--send-diagnostics=false 
		--truncate-splits true
		--max-retries 1
		"

		if [ "${SOURCE_DB_TYPE}" != "postgresql" ]
		then
			args="${args} --target-db-schema ${TARGET_DB_SCHEMA}"
		fi

		if [ "${IMPORT_TABLE_LIST}" != "" ]
		then
			args="${args} --table-list ${IMPORT_TABLE_LIST}"
		fi

		if [ "${IMPORT_EX_TABLE_LIST}" != "" ]
		then
			args="${args} --exclude-table-list ${IMPORT_EX_TABLE_LIST}"
		fi

		if [ "${IMPORT_TABLE_LIST_FILE_PATH}" != "" ]
		then
			args="${args} --table-list-file-path ${IMPORT_TABLE_LIST_FILE_PATH}"
		fi

		if [ "${IMPORT_EX_TABLE_LIST_FILE_PATH}" != "" ]
		then
			args="${args} --exclude-table-list-file-path ${IMPORT_EX_TABLE_LIST_FILE_PATH}"
		fi

		# Check if RUN_WITHOUT_ADAPTIVE_PARALLELISM is true
    	if [ "${RUN_WITHOUT_ADAPTIVE_PARALLELISM}" = "true" ]; then
    	    args="${args} --enable-adaptive-parallelism false"
    	fi

		yb-voyager import data ${args} $*
}

import_data_to_source_replica() {
	args="
	--export-dir ${EXPORT_DIR}
	--source-replica-db-user ${SOURCE_REPLICA_DB_USER} 
	--source-replica-db-name ${SOURCE_REPLICA_DB_NAME} 
	--source-replica-db-password ${SOURCE_REPLICA_DB_PASSWORD} 
	--disable-pb true
	--send-diagnostics=false
	--parallel-jobs 3
	--max-retries 1 
	"
	if [ "${SOURCE_REPLICA_DB_SCHEMA}" != "" ]
	then
		args="${args} --source-replica-db-schema ${SOURCE_REPLICA_DB_SCHEMA}"
	fi
	if [ "${SOURCE_REPLICA_DB_ORACLE_TNS_ALIAS}" != "" ]
	then
		args="${args} --oracle-tns-alias ${SOURCE_REPLICA_DB_ORACLE_TNS_ALIAS}"
	else
		args="${args} --source-replica-db-host ${SOURCE_REPLICA_DB_HOST}"
	fi
	yb-voyager import data to source-replica ${args} $*
}

import_data_file() {
    args="
    --export-dir ${EXPORT_DIR}
    --target-db-host ${TARGET_DB_HOST}
    --target-db-port ${TARGET_DB_PORT}
    --target-db-user ${TARGET_DB_USER}
    --target-db-password ${TARGET_DB_PASSWORD:-''}
    --target-db-schema ${TARGET_DB_SCHEMA:-''}
    --target-db-name ${TARGET_DB_NAME}
    --disable-pb true
    --send-diagnostics=false
    "

    # Check if RUN_WITHOUT_ADAPTIVE_PARALLELISM is true
    if [ "${RUN_WITHOUT_ADAPTIVE_PARALLELISM}" = "true" ]; then
        args="${args} --enable-adaptive-parallelism false"
    fi

    yb-voyager import data file ${args} $*
}


archive_changes() {
	ENABLE=$(shuf -i 0-1 -n 1)
	echo "archive changes ENABLE=${ENABLE}"
	if [[ ${ENABLE} -eq 1 ]];
	then
		ARCHIVE_DIR=${EXPORT_DIR}/archive-dir
		mkdir ${ARCHIVE_DIR}  # temporary place to store the archive files

		yb-voyager archive changes --move-to ${ARCHIVE_DIR} \
		--export-dir ${EXPORT_DIR} \
		--fs-utilization-threshold 0
	fi
}

end_migration() {
	BACKUP_DIR=${EXPORT_DIR}/backup-dir
	mkdir ${BACKUP_DIR}  # temporary place to store the backup

	# setting env vars for passwords to be used for saving reports
	export SOURCE_DB_PASSWORD=${SOURCE_DB_PASSWORD}
	export TARGET_DB_PASSWORD=${TARGET_DB_PASSWORD}
	export SOURCE_REPLICA_DB_PASSWORD=${SOURCE_REPLICA_DB_PASSWORD}

	# TODO: TABLENAME reenable --save-migration-reports
	yb-voyager end migration --export-dir ${EXPORT_DIR} \
	--backup-dir ${BACKUP_DIR} --backup-schema-files true \
	--backup-data-files true --backup-log-files true \
	--save-migration-reports true $* || { 
		cat ${EXPORT_DIR}/logs/yb-voyager-end-migration.log
		exit 1
	}
}

export_data_status(){
	yb-voyager export data status --export-dir ${EXPORT_DIR} \
								  --output-format json
}

import_data_status(){
	yb-voyager import data status --export-dir ${EXPORT_DIR} \
								  --output-format json
}

get_data_migration_report(){

	# setting env vars for passwords to be used for saving reports
	export SOURCE_DB_PASSWORD=${SOURCE_DB_PASSWORD}
	export TARGET_DB_PASSWORD=${TARGET_DB_PASSWORD}
	export SOURCE_REPLICA_DB_PASSWORD=${SOURCE_REPLICA_DB_PASSWORD}

	yb-voyager get data-migration-report --export-dir ${EXPORT_DIR} \
										--output-format json
}

verify_report() {
    expected_report=$1
    actual_report=$2

    if [ -f "${actual_report}" ]; then        
        # Parse and sort JSON data
        actual_data=$(jq -c '.' "${actual_report}" | jq -S 'sort_by(.table_name)')
        
        if [ -f "${expected_report}" ]; then
            expected_data=$(jq -c '.' "${expected_report}" | jq -S 'sort_by(.table_name)')
            
            # Save the sorted JSON data to temporary files
            temp_actual=$(mktemp)
            temp_expected=$(mktemp)
            echo "$actual_data" > "$temp_actual"
            echo "$expected_data" > "$temp_expected"

            compare_files "$temp_actual" "$temp_expected"
            
            # Clean up temporary files
            rm "$temp_actual" "$temp_expected"
            
            # If files do not match, exit
            if [ $? -ne 0 ]; then
                exit 1
            fi
        else
            echo "No ${expected_report} found."
        fi
    else
        echo "No ${actual_report} found."
        exit 1
    fi
}


tail_log_file() {
	log_file_name=$1
	if [ -f "${EXPORT_DIR}/logs/${log_file_name}" ]
	then
		echo "Printing ${log_file_name} file"
		tail -n 150 "${EXPORT_DIR}/logs/${log_file_name}"
	else
		echo "No ${log_file_name} found." 
	fi	
}

cat_log_file() {
	log_file_name=$1
	if [ -f "${EXPORT_DIR}/logs/${log_file_name}" ]
	then
		echo "Printing ${log_file_name} file"
		cat "${EXPORT_DIR}/logs/${log_file_name}"
	else
		echo "No ${log_file_name} found."
	fi	
}

cat_file() {
	file_path=$1
	if [ -f "$file_path" ]
	then
		echo "Printing ${file_path} file"
		cat "$file_path"
	else
		echo "No $file_path found."
	fi
}

kill_process() {
	to_be_killed=$1
	kill -15 ${to_be_killed}
	sleep 1m
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

get_value_from_msr(){
  jq_filter=$1
  val=`sqlite3 ${EXPORT_DIR}/metainfo/meta.db "select json_text from json_objects where key='migration_status';" | jq $jq_filter`
  echo $val
}

create_ff_schema(){
	db_name=$1

	cat > create-ff-schema.sql << EOF
	CREATE USER FF_SCHEMA IDENTIFIED BY "password";
	GRANT all privileges to FF_SCHEMA;
EOF
	run_sqlplus_as_sys ${db_name} "create-ff-schema.sql"
}

set_replica_identity(){
	db_schema=$1
    cat > alter_replica_identity.sql <<EOF
    DO \$CUSTOM\$ 
    DECLARE
		r record;
    BEGIN
        FOR r IN (SELECT table_schema,table_name FROM information_schema.tables WHERE table_schema = '${db_schema}' AND table_type = 'BASE TABLE') 
        LOOP
            EXECUTE 'ALTER TABLE ' || r.table_schema || '."' || r.table_name || '" REPLICA IDENTITY FULL';
        END LOOP;
    END \$CUSTOM\$;
EOF
    run_psql ${SOURCE_DB_NAME} "$(cat alter_replica_identity.sql)"
}

grant_permissions_for_live_migration() {
    if [ "${SOURCE_DB_TYPE}" = "mysql" ]; then
        grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${SOURCE_DB_SCHEMA}
    elif [ "${SOURCE_DB_TYPE}" = "postgresql" ]; then
		for schema_name in $(echo ${SOURCE_DB_SCHEMA} | tr "," "\n")
		do
			set_replica_identity ${schema_name}
			grant_permissions ${SOURCE_DB_NAME} ${SOURCE_DB_TYPE} ${schema_name}
			grant_permissions_for_live_migration_pg ${SOURCE_DB_NAME} ${schema_name}
		done
        
    elif [ "${SOURCE_DB_TYPE}" = "oracle" ]; then
        grant_permissions_for_live_migration_oracle ${ORACLE_CDB_NAME} ${SOURCE_DB_NAME}
        if [ -n "${SOURCE_REPLICA_DB_NAME}" ]; then
            run_sqlplus_as_sys ${SOURCE_REPLICA_DB_NAME} ${SCRIPTS}/oracle/fall_forward_prep.sql
        fi
    else
        echo "Invalid source database."
    fi
}

setup_fallback_environment() {
	if [ "${SOURCE_DB_TYPE}" = "oracle" ]; then
		run_sqlplus_as_sys ${SOURCE_DB_NAME} ${SCRIPTS}/oracle/create_metadata_tables.sql
		run_sqlplus_as_sys ${SOURCE_DB_NAME} ${SCRIPTS}/oracle/fall_back_prep.sql
		elif [ "${SOURCE_DB_TYPE}" = "postgresql" ]; then
		conn_string="postgresql://${SOURCE_DB_ADMIN_USER}:${SOURCE_DB_ADMIN_PASSWORD}@${SOURCE_DB_HOST}:${SOURCE_DB_PORT}/${SOURCE_DB_NAME}"
		psql "${conn_string}" -v voyager_user="${SOURCE_DB_USER}" -v schema_list="${SOURCE_DB_SCHEMA}" -v replication_group='replication_group' -v is_live_migration=1 -v is_live_migration_fall_back=1 -f /opt/yb-voyager/guardrails-scripts/yb-voyager-pg-grant-migration-permissions.sql

		disable_triggers_sql=$(mktemp)
        drop_constraints_sql=$(mktemp)
		formatted_schema_list=$(echo "${SOURCE_DB_SCHEMA}" | sed "s/,/','/g")

		# Disabling Triggers
		cat <<EOF > "${disable_triggers_sql}"
DO \$\$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT table_schema, '"' || table_name || '"' AS t_name
        FROM information_schema.tables
        WHERE table_type = 'BASE TABLE'
        AND table_schema IN ('${formatted_schema_list}')
    LOOP
        EXECUTE 'ALTER TABLE ' || r.table_schema || '.' || r.t_name || ' DISABLE TRIGGER ALL';
    END LOOP;
END \$\$;
EOF

        # Dropping Fkeys
        cat <<EOF > "${drop_constraints_sql}"
DO \$\$
DECLARE
    fk RECORD;
BEGIN
    FOR fk IN
        SELECT conname, conrelid::regclass AS table_name
        FROM pg_constraint
        JOIN pg_class ON conrelid = pg_class.oid
        JOIN pg_namespace ON pg_namespace.oid = pg_class.relnamespace
        WHERE contype = 'f'
        AND pg_namespace.nspname IN ('${formatted_schema_list}')
    LOOP
        EXECUTE 'ALTER TABLE ' || fk.table_name || ' DROP CONSTRAINT ' || fk.conname;
    END LOOP;
END \$\$;
EOF

        psql_import_file "${SOURCE_DB_NAME}" "${disable_triggers_sql}"
        psql_import_file "${SOURCE_DB_NAME}" "${drop_constraints_sql}"

        rm -f "${disable_triggers_sql}" "${drop_constraints_sql}"
	fi
}

reenable_triggers_fkeys() {
	if [ "${SOURCE_DB_TYPE}" = "postgresql" ]; then
		enable_triggers_sql=$(mktemp)
		formatted_schema_list=$(echo "${SOURCE_DB_SCHEMA}" | sed "s/,/','/g")

		cat <<EOF > "${enable_triggers_sql}"
DO \$\$
DECLARE
    r RECORD;
BEGIN
    FOR r IN
        SELECT table_schema, '"' || table_name || '"' AS t_name
        FROM information_schema.tables
        WHERE table_type = 'BASE TABLE'
        AND table_schema IN ('${formatted_schema_list}')
    LOOP
        EXECUTE 'ALTER TABLE ' || r.table_schema || '.' || r.t_name || ' ENABLE TRIGGER ALL';
    END LOOP;
END \$\$;
EOF
		psql_import_file "${SOURCE_DB_NAME}" "${enable_triggers_sql}"
	fi
#TODO: Add re-creating FKs
}

assess_migration() {
	args="--export-dir ${EXPORT_DIR}
		--source-db-type ${SOURCE_DB_TYPE}
		--source-db-user ${SOURCE_DB_USER}
		--source-db-password ${SOURCE_DB_PASSWORD}
		--source-db-name ${SOURCE_DB_NAME}
		--send-diagnostics=false --yes
		--iops-capture-interval 0
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

	# flag enabling oracle ssl tests --oracle-tns-alias
	if [ "${SOURCE_DB_ORACLE_TNS_ALIAS}" != "" ]
	then
		args="${args} --oracle-tns-alias ${SOURCE_DB_ORACLE_TNS_ALIAS}"
	else
		args="${args} --source-db-host ${SOURCE_DB_HOST} --source-db-port ${SOURCE_DB_PORT}"
	fi
	
	yb-voyager assess-migration ${args} $*
}

validate_failure_reasoning() {
    assessment_report="$1"

    # Check if FailureReasoning is empty or not
    failure_reasoning=$(jq -r '.Sizing.FailureReasoning' "$assessment_report")
    if [ -z "$failure_reasoning" ]; then
        echo "FailureReasoning is empty. Assessment passed."
    else
        echo "Error: FailureReasoning is not empty. Assessment failed."
        echo "FailureReasoning: $failure_reasoning"
        cat_log_file "yb-voyager-assess-migration.log"
        exit 1
    fi
}

post_assess_migration() {
    json_file="$EXPORT_DIR/assessment/reports/migration_assessment_report.json"
    sharded_tables=$(fetch_sharded_tables "$json_file")
    echo "Sharded Tables: $sharded_tables"
    colocated_tables=$(fetch_colocated_tables "$json_file")
    echo "Colocated Tables: $colocated_tables"

    move_tables "$json_file" 30

    updated_sharded_tables=$(fetch_sharded_tables "$json_file")
    echo "Updated Sharded Tables: $updated_sharded_tables"
    updated_colocated_tables=$(fetch_colocated_tables "$json_file")
    echo "Updated Colocated Tables: $updated_colocated_tables"
}

fetch_sharded_tables() {
    local json_file=$1
    jq '.Sizing.SizingRecommendation.ShardedTables // []' "$json_file"
}

fetch_colocated_tables() {
    local json_file=$1
    jq '.Sizing.SizingRecommendation.ColocatedTables // []' "$json_file"
}

# Function to move a specified percentage of tables from colocated to sharded
move_tables() {
    local json_file=$1
    local percentage=$2

    local total_tables=$(jq '[.Sizing.SizingRecommendation.ShardedTables // [], .Sizing.SizingRecommendation.ColocatedTables // []] | flatten | length' "$json_file")
    local sharded_tables_count=$(jq '.Sizing.SizingRecommendation.ShardedTables | length // 0' "$json_file")
    local target_sharded_tables=$((total_tables * percentage / 100))

    if [ "$sharded_tables_count" -ge "$target_sharded_tables" ]; then
        echo "Sharded tables are already 30% or more of the total tables. No need to move tables."
        return
    fi

    local tables_to_move=$((target_sharded_tables - sharded_tables_count))

    if [ "$tables_to_move" -le 0 ]; then
        echo "No tables need to be moved."
        return
    fi

    echo "Moving $tables_to_move tables from colocated to sharded."

    colocated_tables=$(fetch_colocated_tables "$json_file")
    # Select random tables to move
    new_sharded_tables=$(echo "$colocated_tables" | jq --argjson count "$tables_to_move" 'to_entries | .[:$count] | map(.value) | flatten')
    remaining_colocated_tables=$(echo "$colocated_tables" | jq --argjson new_sharded "$new_sharded_tables" '. - $new_sharded')

    # Convert arrays to JSON
    new_sharded_tables_json=$(echo "$new_sharded_tables" | jq -s 'flatten')
    remaining_colocated_tables_json=$(echo "$remaining_colocated_tables" | jq -s 'flatten')

    # Update the JSON file with the new lists of sharded and colocated tables
    jq --indent 4 --argjson new_sharded_tables "$new_sharded_tables_json" --argjson remaining_colocated_tables "$remaining_colocated_tables_json" \
       '.Sizing.SizingRecommendation.ShardedTables += $new_sharded_tables | .Sizing.SizingRecommendation.ColocatedTables = $remaining_colocated_tables' "$json_file" > tmp.json && mv tmp.json "$json_file"
}

normalize_json() {
    local input_file="$1"
    local output_file="$2"
    local temp_file="/tmp/temp_file.json"

    # Normalize JSON with jq; use --sort-keys to avoid the need to keep the same sequence of keys in expected vs actual json
    jq --sort-keys 'walk(
        if type == "object" then
            .ObjectNames? |= (
				if type == "string" then
					split(", ") | sort | join(", ")
				else
					.
				end
			) |
            .VoyagerVersion? = "IGNORED" |
			.TargetDBVersion? = "IGNORED" |
            .DbVersion? = "IGNORED" |
            .FilePath? = "IGNORED" |
            .OptimalSelectConnectionsPerNode? = "IGNORED" |
            .OptimalInsertConnectionsPerNode? = "IGNORED" |
            .RowCount? = "IGNORED" |
            # Replace newline characters in SqlStatement with spaces
			.SqlStatement? |= (
				if type == "string" then
					gsub("\\n"; " ")
				else
					.
				end
			)
        elif type == "array" then
			sort_by(tostring)
		else
            .
        end
    )' "$input_file" > "$temp_file"

    # Remove unwanted lines
    sed -i '/Review and manually import.*uncategorized.sql/d' "$temp_file"

    # Move cleaned file to output
    mv "$temp_file" "$output_file"
}


compare_sql_files() {
    sql_file1="$1"
    sql_file2="$2"

    # Normalize the files by removing lines that start with "File :"
    normalized_file1=$(mktemp)
    normalized_file2=$(mktemp)
    
    grep -v '^File :' "$sql_file1" > "$normalized_file1"
    grep -v '^File :' "$sql_file2" > "$normalized_file2"

    sed -i -E 's#could not open extension control file ".*/(postgis\.control)"#could not open extension control file "PATH_PLACEHOLDER/\1"#g' "$normalized_file1"
    sed -i -E 's#could not open extension control file ".*/(postgis\.control)"#could not open extension control file "PATH_PLACEHOLDER/\1"#g' "$normalized_file2"

    # Compare the normalized files
    compare_files "$normalized_file1" "$normalized_file2"
    
    # Clean up temporary files
    rm "$normalized_file1" "$normalized_file2"
}


compare_files() {
    file1="$1"
    file2="$2"

    if cmp -s "$file1" "$file2"; then
        echo "Data matches expected report."
        return 0
    else
        echo "Data does not match expected report."
        diff_output=$(diff --context "$file1" "$file2")
        echo "$diff_output"
        return 1
    fi
}

compare_json_reports() {
    local file1="$1"
    local file2="$2"

    local temp_file1=$(mktemp)
    local temp_file2=$(mktemp)

    normalize_json "$file1" "$temp_file1"
    normalize_json "$file2" "$temp_file2"

    compare_files "$temp_file1" "$temp_file2"
    compare_status=$?

    # Clean up temporary files
    rm "$temp_file1" "$temp_file2"

    # Exit with the status from compare_files if there are differences
    if [ $compare_status -ne 0 ]; then
        exit $compare_status
    fi

    echo "Proceeding with further steps..."
}

replace_files() {
    replacement_dir="$1"
    export_dir="$2"

    find "$replacement_dir" -type f | while read -r replacement_file; do
        # Get the relative path of the file in the replacement_dir
        relative_path="${replacement_file#$replacement_dir/}"

        # Construct the corresponding path in the export_dir/schema
        target_file="$export_dir/$relative_path"

        # Check if the target file exists in the export_dir/schema
        if [ -f "$target_file" ]; then
            echo "Replacing $target_file with $replacement_file"
            cp "$replacement_file" "$target_file"
        else
            echo "Target file $target_file does not exist. Skipping."
        fi
    done
}

bulk_assessment() {
	yb-voyager assess-migration-bulk --bulk-assessment-dir "${BULK_ASSESSMENT_DIR}" \
	--fleet-config-file "${TEST_DIR}"/fleet-config-file.csv
}

fix_config_file() {
  local file="$1"
  awk -F, 'NR==3 {$8="password"}1' OFS=, "$file" > tmp && mv tmp "$file"
}

compare_and_validate_reports() {
    local html_file="$1"
    local json_file="$2"
    local expected_file="$3"
    local log_file="$4"

    if [ -f "${html_file}" ] && [ -f "${json_file}" ]; then
        echo "Assessment reports created successfully."
        echo "Comparing Report contents"
        compare_json_reports "${expected_file}" "${json_file}"
    else
        echo "Error: Assessment reports were not created successfully."
        cat_file "${log_file}"
        exit 1
    fi
}

validate_import_data_state_batch_files() {
	# Iterate over files in the directory
	DIR="$1"
	shift # Shift to access array elements (after directory)
	expected_files=("$@")
	for file in "$DIR"/**/*
	do
		# Check if the current file is a symlink, skip if true
		if [ -L "$file" ]; then
			continue
		fi
		# Get just the file name from the full path
		filename=$(basename "$file")

		# Check if the filename is in the expected list
		if [[ " ${expected_files[@]} " =~ " ${filename} " ]]; then
		    echo "Batch file matches"
		else
			echo "Batch file doesn't match ${filename} with expected ${expected_files[@]}"  
			exit 1
		fi
	done
}

cutover_to_target() {
	args="
	--export-dir ${EXPORT_DIR} --yes
	"    
    if [ "${USE_YB_LOGICAL_REPLICATION_CONNECTOR}" = true ]; then
        args="${args} --use-yb-grpc-connector false"
    fi
    
    yb-voyager initiate cutover to target ${args} $*
}
