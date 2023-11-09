import os
import sys
from typing import Any, Dict, List
import cx_Oracle

def run_checks(checkFn):
    tgt = new_target_db()
    tgt.connect()
    print("Connected")
    checkFn(tgt)
    tgt.close()
    print("Disconnected")
    
def new_target_db():
    env = os.environ
    return OracleDB(
        env.get("FF_DB_HOST", "localhost"),
        env.get("FF_DB_PORT", "1521"),
        env["FF_DB_NAME"],
        env.get("FF_DB_USER", "FF_SCHEMA"),
        env.get("FF_DB_PASSWORD", "password"))
    
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
            cur = self.conn.cursor()
            cur.execute("SELECT COUNT(*) FROM {}.{}".format(schema_name, table_name))
            row_count = cur.fetchone()[0]
            return row_count
        except cx_Oracle.Error as error:
            print("Error:", error)
            os._exit(1)
            return None

    def row_count_of_all_tables(self, schema_name) -> Dict[str, int]:
        tables = self.get_table_names(schema_name)
        return {table: self.get_row_count(table, schema_name) for table in tables}

    def get_sum_of_column_of_table(self, table_name, column_name, schema_name) -> int:
        cur = self.conn.cursor()
        cur.execute("SELECT SUM({}) FROM {}.{}".format(column_name, schema_name, table_name))
        return cur.fetchone()[0]

    def run_query_and_chk_error(self, query, error_code) -> bool:
        cur = self.conn.cursor()
        try:
            cur.execute(query)
        except cx_Oracle.DatabaseError as error:
            error_code = str(error.code)
            self.conn.rollback()
            return error_code == str(error_code)
        return False

		 