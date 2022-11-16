import os
import sys
from typing import Any, Dict, List
from xmlrpc.client import boolean

import psycopg2


def run_checks(checkFn):
	tgt = new_target_db()
	tgt.connect()
	print("Connected")
	checkFn(tgt)
	tgt.close()
	print("Disconnected")


def new_target_db():
	env = os.environ
	return PostgresDB(
		env.get("TARGET_DB_HOST", "127.0.0.1"),
		env.get("TARGET_DB_PORT", 5433),
		env.get("TARGET_DB_USER", "yugabyte"),
		env.get("TARGET_DB_PASSWORD", ""),
		env["TARGET_DB_NAME"])


class PostgresDB:
	  
	def __init__(self, host, port, user, password, database):
		self.host = host
		self.port = port
		self.user = user
		self.password = password
		self.database = database
	  
	def connect(self):
		self.conn = psycopg2.connect(
			host=self.host,
			port=self.port,
			user=self.user,
			password=self.password,
			database=self.database
		)
	  
	def close(self):
		self.conn.close()

	def table_exists(self, table_name, table_schema="public") -> bool:
		cur = self.conn.cursor()
		cur.execute("SELECT EXISTS(SELECT 1 FROM information_schema.tables WHERE table_name=%s AND table_schema=%s)", (table_name, table_schema))
		result = cur.fetchone()
		return result[0]

	def get_table_names(self, schema="public") -> List[str]:
		cur = self.conn.cursor()
		q = "SELECT table_name FROM information_schema.tables WHERE table_schema=%s AND table_type='BASE TABLE'"
		cur.execute(q, (schema,))
		return [table[0] for table in cur.fetchall()]

	def get_row_count(self, table_name, schema_name="public") -> int:
		cur = self.conn.cursor()
		cur.execute(f'SELECT COUNT(*) FROM {schema_name}."{table_name}"')
		return cur.fetchone()[0]

	def row_count_of_all_tables(self, schema_name="public") -> Dict[str, int]:
		tables = self.get_table_names(schema_name)
		return {table: self.get_row_count(table, schema_name) for table in tables}

	def get_objects_of_type(self, object_type, schema_name="public") -> List[str]:
		object_type = {
			"TABLE": "r",
			"VIEW": "v",
			"INDEX": "i",
			"SEQUENCE": "S",
			"MVIEW": "m",
		}[object_type]
		cur = self.conn.cursor()
		cur.execute(f"select relname from pg_class join pg_namespace on pg_class.relnamespace = pg_namespace.oid"+
			f" where nspname = '{schema_name}' AND relkind = '{object_type}'")
		return [obj[0] for obj in cur.fetchall()]
	
	def get_sum_of_column_of_table(self, table_name, column_name, schema_name="public") -> int:
		cur = self.conn.cursor()
		cur.execute(f"select sum({column_name}) from {schema_name}.{table_name}")
		return cur.fetchone()[0]
		
	def get_count_index_on_table(self, schema_name="public") -> Dict[str,int]:
		cur = self.conn.cursor()
		cur.execute(f"SELECT tablename, count(indexname) FROM pg_indexes WHERE schemaname = '{schema_name}' GROUP  BY tablename;")
		return {tablename: cnt for tablename,cnt in cur.fetchall()}

	def get_distinct_values__of_column_of_table(self, table_name, column_name, schema_name="public") -> List[Any]:
		cur = self.conn.cursor()
		cur.execute(f"select distinct({column_name}) from {schema_name}.{table_name}")
		return [value[0] for value in cur.fetchall()]

	# takes query and error_code and return true id the error_code you believe that query should throw matches
	def run_query_and_chk_error(self, query, error_code) -> boolean:
		cur = self.conn.cursor()
		try:
			cur.execute(f"{query}")
		except Exception as error:
			self.conn.rollback()
			return error_code == int(error.pgcode)
		return False

	def get_functions_count(self, schema_name="public") -> int:
		cur = self.conn.cursor()
		cur.execute(f"SELECT count(routine_name) FROM  information_schema.routines WHERE  routine_type = 'FUNCTION' AND routine_schema = '{schema_name}';")
		return cur.fetchone()[0]

	def execute_function_query(self, query) -> Any:
		cur=self.conn.cursor()
		cur.execute(f"{query}")
		return cur.fetchone()[0]

	def fetch_datatypes_of_all_tables_in_schema(self, schema_name="public") -> Dict[str, List[str]]:
		cur = self.conn.cursor()
		cur.execute(f"SELECT table_name, column_name, data_type FROM information_schema.columns WHERE table_schema = '{schema_name}'")
		tables = {}
		for table_name, column_name, data_type in cur.fetchall():
			if table_name not in tables:
				tables[table_name] = []
			tables[table_name].append(data_type)
		return tables
