import os
import sys

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
        env.get("TARGET_DB_HOST", "localhost"),
        env.get("TARGET_DB_PORT", "1521"),
        env["TARGET_DB_NAME"],
        env.get("TARGET_DB_USER", "ybvoyager"),
        env.get("TARGET_DB_PASSWORD", "password"))
    
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
