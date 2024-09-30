import os
import sys
from typing import Any, Dict, List
import cx_Oracle

def run_checks(checkFn, db_type):
    if db_type == "source":
        tgt = new_source_db()
    elif db_type == "source_replica":
        tgt = new_source_replica_db()
    else:
        raise ValueError("Invalid database type. Use 'source' or 'source_replica'.")
    tgt.connect()
    print("Connected")
    checkFn(tgt)
    tgt.close()
    print("Disconnected")
    
def new_source_replica_db():
    env = os.environ
    return OracleDB(
        env.get("SOURCE_REPLICA_DB_HOST", "localhost"),
        env.get("SOURCE_REPLICA_DB_PORT", "1521"),
        env["SOURCE_REPLICA_DB_NAME"],
        env.get("SOURCE_REPLICA_DB_SCHEMA", "FF_SCHEMA"),
        env.get("SOURCE_REPLICA_DB_PASSWORD", "password"))

def new_source_db():
    env = os.environ
    return OracleDB(
        env.get("SOURCE_DB_HOST", "localhost"),
        env.get("SOURCE_DB_PORT", "1521"),
        env["SOURCE_DB_NAME"],
        env.get("SOURCE_DB_SCHEMA", "TEST_SCHEMA"),
        env.get("SOURCE_DB_PASSWORD", "password"))
    
class OracleDB:
    
    def __init__(self, host, port, dbname, username, password):
        self.host = host
        self.port = port
        self.dbname = dbname
        self.username = username
        self.password = password
        self.conn = None
        
    def connect(self):
        try:
            dsn = cx_Oracle.makedsn(self.host, self.port, service_name=self.dbname)
            self.conn = cx_Oracle.connect(self.username, self.password, dsn)
        except cx_Oracle.Error as error:
            print("Error:", error)
            os._exit(1)
            
    def close(self):
        if self.conn:
            self.conn.close()

    def get_table_names(self, schema_name) -> List[str]:
        try:
            cur = self.conn.cursor()
            cur.execute("SELECT table_name FROM all_tables WHERE owner = '{}'".format(schema_name))
            return [table[0].lower() for table in cur.fetchall()]
        except cx_Oracle.Error as error:
            print("Error:", error)
            os._exit(1)
            return None

    def get_row_count(self, table_name, schema_name) -> int:
        try:
            #handle case sensitive table names
            if table_name[0] == '"' :
                table_name = table_name[1:-1]
            else:
                table_name = table_name.upper()
            cur = self.conn.cursor()
            cur.execute('SELECT COUNT(*) FROM {}."{}"'.format(schema_name, table_name))
            row_count = cur.fetchone()[0]
            return row_count
        except cx_Oracle.Error as error:
            print("Error:", error)
            os._exit(1)
            return None

    def row_count_of_all_tables(self, schema_name) -> Dict[str, int]:
        cur = self.conn.cursor()
        #for handling the case sensitive table names in oracle
        #quote all tables coming from DB
        cur.execute("SELECT table_name FROM all_tables WHERE owner = '{}'".format(schema_name))
        tables = ['"{}"'.format(table[0]) for table in cur.fetchall()]
        #return tables in lower case to match EXPECTED values Todo handled it properly later
        return {table[1:-1].lower(): self.get_row_count(table, schema_name) for table in tables}
    
    def get_distinct_values_of_column_of_table(self, table_name, column_name, schema_name) -> List[Any]:
        cur = self.conn.cursor()
        cur.execute(f"select distinct({column_name}) from {schema_name}.{table_name}")
        return [value[0] for value in cur.fetchall()]

    def get_sum_of_column_of_table(self, table_name, column_name, schema_name) -> int:
        cur = self.conn.cursor()
        cur.execute('SELECT SUM({}) FROM {}.{}'.format(column_name, schema_name, table_name))
        return cur.fetchone()[0]

    def run_query_and_chk_error(self, query, error_code) -> bool:
        cur = self.conn.cursor()
        try:
            cur.execute(query)
        except cx_Oracle.DatabaseError as error:
            code = str(error.args[0].code)
            self.conn.rollback()
            print(error)
            return error_code == str(code)
        return False
    
    def get_identity_type_columns(self, type_name, table_name, schema_name) -> List[str]:
        if table_name[0] == '"' : #case sensitive
            table_name = table_name[1:-1]
        else:
            table_name = table_name.upper()
        cur = self.conn.cursor()
        query = f"Select COLUMN_NAME from ALL_TAB_IDENTITY_COLS where OWNER = '{schema_name}' AND TABLE_NAME = '{table_name}' AND GENERATION_TYPE='{type_name}'"
        cur.execute(query)
        return [column[0].lower() for column in cur.fetchall()]