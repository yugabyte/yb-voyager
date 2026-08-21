#!/usr/bin/env python3
"""
YB Voyager Callhome DDL Extractor

Extracts anonymized DDL statements from YB Voyager callhome payload JSON files
and creates executable SQL files with proper parameter handling and constant replacement.

Features:
- Fixes anonymized session parameters with PostgreSQL defaults
- Replaces anonymized constants in DEFAULT clauses with PostgreSQL-compatible dummy values
- Replaces anonymized constants in datatype parameters with appropriate dummy values
- Handles array types (text[]) with anonymized DEFAULT values
- Comments out problematic nextval() statements with anonymized sequence references
- Generates STUB functions for missing function dependencies (excluded from anonymization)
- Analyzes function usage context to determine appropriate return types for stubs
- Organizes DDL statements in proper dependency execution order
- Handles boolean, numeric, text, UUID, timestamp, varchar, array, and other PostgreSQL data types

Usage:
    python3 extract_anonymized_ddls.py <input_json_file> [output_sql_file]
"""

import json
import sys
import os
import argparse
import re
from pathlib import Path
from datetime import datetime


# PostgreSQL session parameter defaults
PG_PARAMETER_DEFAULTS = {
    'statement_timeout': '0',
    'lock_timeout': '0', 
    'idle_in_transaction_session_timeout': '0',
    'client_encoding': 'UTF8',
    'standard_conforming_strings': 'on',
    'xmloption': 'content',
    'client_min_messages': 'notice',
    'row_security': 'off'
}

# Parameters that don't exist in standard PostgreSQL
PG_UNSUPPORTED_PARAMETERS = {
    'transaction_timeout'  # YugabyteDB specific
}

# DDL execution order based on YB Voyager's proven approach
# This ensures dependencies are handled correctly
VOYAGER_DDL_ORDER = [
    'session_params',    # SET statements and config
    'schemas',          # CREATE SCHEMA
    'extensions',       # CREATE EXTENSION  
    'types',            # CREATE TYPE, DOMAIN
    'stub_functions',   # STUB function definitions for missing dependencies
    'sequences_create', # CREATE SEQUENCE only
    'tables_create',    # CREATE TABLE only
    'indexes',          # CREATE INDEX, CREATE UNIQUE INDEX
    'functions',        # CREATE FUNCTION, PROCEDURE, AGGREGATE
    'views',            # CREATE VIEW, MATERIALIZED VIEW
    'triggers',         # CREATE TRIGGER
    'others',           # RULE, COMMENT, CONVERSION, etc.
    'sequences_alter',  # ALTER SEQUENCE OWNED BY (needs tables first)
    'constraints',      # ALTER TABLE ADD CONSTRAINT (non-FK)
    'foreign_keys',     # ALTER TABLE ... FOREIGN KEY (needs all tables)
]

# PostgreSQL-compatible dummy values for different data types
DUMMY_VALUES_BY_TYPE = {
    'boolean': 'true',
    'bool': 'true',
    'int': '0',
    'int2': '0',
    'int4': '0', 
    'int8': '0',
    'integer': '0',
    'bigint': '0',
    'smallint': '0',
    'numeric': '0.0',
    'decimal': '0.0',
    'real': '0.0',
    'float4': '0.0',
    'float8': '0.0',
    'double precision': '0.0',
    'text': "'dummy_text'",
    'varchar': "'dummy_text'",
    'character varying': "'dummy_text'",
    'char': "'d'",
    'character': "'d'",
    'uuid': "'00000000-0000-0000-0000-000000000000'",
    'timestamp': "'2000-01-01 00:00:00'",
    'timestamp with time zone': "'2000-01-01 00:00:00+00'",
    'timestamp without time zone': "'2000-01-01 00:00:00'",
    'timestamptz': "'2000-01-01 00:00:00+00'",
    'date': "'2000-01-01'",
    'time': "'00:00:00'",
    'time with time zone': "'00:00:00+00'",
    'time without time zone': "'00:00:00'",
    'timetz': "'00:00:00+00'",
    # Array types
    'text[]': "'{}'::text[]"
}

# PostgreSQL-compatible dummy values for datatype parameters
# Maps datatype to list of parameter values [param1, param2, ...]
DATATYPE_PARAM_DEFAULTS = {
    # String types with length parameter
    'varchar': ['255'],
    'character varying': ['255'],
    'char': ['1'],
    'character': ['1'],
    'bpchar': ['1'],
    
    # Numeric types with precision and scale
    'numeric': ['10', '2'],
    'decimal': ['10', '2'],
    
    # Time types with precision
    'timestamp': ['6'],
    'timestamp with time zone': ['6'],
    'timestamp without time zone': ['6'],
    'timestamptz': ['6'],
    'time': ['6'],
    'time with time zone': ['6'],
    'time without time zone': ['6'],
    'timetz': ['6'],
    'interval': ['6'],
    
    # Bit types with length
    'bit': ['1'],
    'varbit': ['64'],
    'bit varying': ['64'],
    
    # Float types with precision (less common but possible)
    'float': ['53'],
    'real': ['24'],
    'double precision': ['53'],
}


def is_builtin_function(func_name):
    """Check if function is a PostgreSQL built-in."""
    builtins = {
        # String functions
        'lower', 'upper', 'trim', 'substr', 'substring', 'length', 'concat',
        'position', 'replace', 'split_part', 'ltrim', 'rtrim', 'lpad', 'rpad',
        'left', 'right', 'reverse', 'translate', 'ascii', 'chr', 'md5',
        
        # Date/Time functions  
        'now', 'current_timestamp', 'current_date', 'current_time',
        'extract', 'date_part', 'date_trunc', 'age', 'clock_timestamp',
        'statement_timestamp', 'transaction_timestamp', 'timeofday',
        'to_timestamp', 'to_date', 'to_char', 'justify_days', 'justify_hours',
        'justify_interval', 'make_date', 'make_time', 'make_timestamp',
        'make_timestamptz', 'make_interval',
        
        # UUID functions
        'gen_random_uuid', 'uuid_generate_v1', 'uuid_generate_v1mc',
        'uuid_generate_v3', 'uuid_generate_v4', 'uuid_generate_v5',
        'uuid_nil', 'uuid_ns_dns', 'uuid_ns_url', 'uuid_ns_oid', 'uuid_ns_x500',
        
        # Array functions
        'array', 'array_append', 'array_prepend', 'array_cat', 'array_ndims',
        'array_dims', 'array_length', 'array_lower', 'array_upper',
        'array_remove', 'array_replace', 'array_to_string', 'string_to_array',
        'array_agg', 'array_position', 'array_positions', 'cardinality',
        'unnest', 'any', 'all',
        
        # Sequence functions
        'nextval', 'currval', 'setval', 'lastval',
        
        # Configuration functions
        'set_config', 'current_setting', 'pg_reload_conf',
        
        # Type casting (common ones)
        'text', 'varchar', 'char', 'int', 'integer', 'bigint', 'smallint',
        'numeric', 'decimal', 'real', 'float', 'double', 'boolean', 'bool',
        'timestamp', 'timestamptz', 'date', 'time', 'timetz', 'interval',
        'uuid', 'json', 'jsonb', 'xml', 'bytea', 'bit', 'varbit',
        
        # Conditional functions
        'coalesce', 'nullif', 'greatest', 'least', 'case', 'when', 'then',
        'else', 'end',
        
        # Math functions
        'abs', 'ceil', 'ceiling', 'floor', 'round', 'trunc', 'sign',
        'sqrt', 'cbrt', 'power', 'exp', 'ln', 'log', 'log10',
        'sin', 'cos', 'tan', 'asin', 'acos', 'atan', 'atan2',
        'sinh', 'cosh', 'tanh', 'asinh', 'acosh', 'atanh',
        'degrees', 'radians', 'pi', 'random', 'setseed',
        'mod', 'div', 'gcd', 'lcm', 'factorial', 'width_bucket',
        
        # Aggregate functions
        'count', 'sum', 'avg', 'min', 'max', 'stddev', 'stddev_pop',
        'stddev_samp', 'variance', 'var_pop', 'var_samp', 'corr',
        'covar_pop', 'covar_samp', 'regr_avgx', 'regr_avgy', 'regr_count',
        'regr_intercept', 'regr_r2', 'regr_slope', 'regr_sxx', 'regr_sxy',
        'regr_syy', 'bool_and', 'bool_or', 'every', 'string_agg',
        'xmlagg', 'json_agg', 'json_object_agg', 'jsonb_agg', 'jsonb_object_agg',
        'bit_and', 'bit_or',
        
        # JSON/JSONB functions
        'json_array_length', 'json_each', 'json_each_text', 'json_extract_path',
        'json_extract_path_text', 'json_object_keys', 'json_populate_record',
        'json_populate_recordset', 'json_array_elements', 'json_array_elements_text',
        'json_typeof', 'json_to_record', 'json_to_recordset', 'json_strip_nulls',
        'jsonb_array_length', 'jsonb_each', 'jsonb_each_text', 'jsonb_extract_path',
        'jsonb_extract_path_text', 'jsonb_object_keys', 'jsonb_populate_record',
        'jsonb_populate_recordset', 'jsonb_array_elements', 'jsonb_array_elements_text',
        'jsonb_typeof', 'jsonb_to_record', 'jsonb_to_recordset', 'jsonb_strip_nulls',
        'jsonb_set', 'jsonb_insert', 'jsonb_pretty', 'jsonb_build_array',
        'jsonb_build_object', 'json_build_array', 'json_build_object',
        
        # System information functions
        'version', 'current_database', 'current_schema', 'current_schemas',
        'current_user', 'session_user', 'user', 'current_role',
        'inet_client_addr', 'inet_client_port', 'inet_server_addr',
        'inet_server_port', 'pg_backend_pid', 'pg_conf_load_time',
        'pg_is_in_recovery', 'pg_postmaster_start_time', 'pg_trigger_depth',
        
        # Other common functions
        'pg_catalog.set_config', 'public.uuid_generate_v4', 'hdb_catalog.gen_hasura_uuid',
        'generate_series', 'regexp_replace', 'regexp_split_to_table',
        'regexp_split_to_array', 'format', 'quote_ident', 'quote_literal',
        'quote_nullable', 'encode', 'decode', 'convert', 'convert_from',
        'convert_to', 'overlay', 'octet_length', 'bit_length', 'char_length',
        'character_length', 'strpos', 'starts_with', 'ends_with',
        'get_bit', 'set_bit', 'get_byte', 'set_byte', 'md5', 'sha224',
        'sha256', 'sha384', 'sha512', 'crypt', 'gen_salt',
        'row_number', 'rank', 'dense_rank', 'percent_rank', 'cume_dist',
        'ntile', 'lag', 'lead', 'first_value', 'last_value', 'nth_value'
    }
    return func_name.lower() in builtins


def extract_function_references(ddls):
    """Extract function references from DEFAULT clauses in DDL statements."""
    function_refs = set()
    
    for ddl in ddls:
        # Skip comments and session parameters
        if ddl.strip().startswith('--') or ddl.strip().upper().startswith('SET '):
            continue
            
        # Only look for function calls in DEFAULT clauses
        # Pattern: DEFAULT followed by optional whitespace, then function call
        default_pattern = r'DEFAULT\s+([^,)]+)'
        default_matches = re.findall(default_pattern, ddl, re.IGNORECASE)
        
        for default_expr in default_matches:
            # Find schema-qualified function calls in DEFAULT expressions
            schema_func_pattern = r'([a-zA-Z_][a-zA-Z0-9_]*)\.([\w_]+)\s*\('
            matches = re.findall(schema_func_pattern, default_expr, re.IGNORECASE)
            for schema, func in matches:
                # Skip built-in schemas and functions
                if schema.lower() in {'pg_catalog', 'information_schema'} and is_builtin_function(func):
                    continue
                if not is_builtin_function(func):
                    function_refs.add((schema, func))
            
            # Remove schema-qualified function calls from the expression before looking for unqualified ones
            expr_without_schema_funcs = re.sub(schema_func_pattern, '', default_expr, flags=re.IGNORECASE)
            
            # Find unqualified function calls in the remaining text
            func_pattern = r'\b([a-zA-Z_][a-zA-Z0-9_]*)\s*\('
            matches = re.findall(func_pattern, expr_without_schema_funcs, re.IGNORECASE)
            for func in matches:
                if not is_builtin_function(func):
                    function_refs.add((None, func))
    
    return function_refs


def analyze_function_usage(ddls, function_refs):
    """Analyze function usage context to determine appropriate return types."""
    function_contexts = {}
    
    for schema, func_name in function_refs:
        contexts = []
        qualified_name = f"{schema}.{func_name}" if schema else func_name
        
        for ddl in ddls:
            if qualified_name in ddl:
                # Check if used in DEFAULT clause
                if re.search(rf'DEFAULT\s+[^,)]*{re.escape(qualified_name)}\s*\([^)]*\)', ddl, re.IGNORECASE):
                    # Try to determine the column type
                    default_match = re.search(rf'(\w+(?:\s+\w+)*)\s+(?:NOT\s+NULL\s+)?DEFAULT\s+[^,)]*{re.escape(qualified_name)}\s*\([^)]*\)', ddl, re.IGNORECASE)
                    if default_match:
                        col_type = default_match.group(1).lower().strip()
                        if 'uuid' in col_type:
                            contexts.append('uuid')
                        elif 'timestamp' in col_type:
                            contexts.append('timestamp')
                        elif 'text' in col_type or 'varchar' in col_type or 'char' in col_type:
                            contexts.append('text')
                        elif 'int' in col_type or 'bigint' in col_type or 'smallint' in col_type:
                            contexts.append('integer')
                        elif 'numeric' in col_type or 'decimal' in col_type:
                            contexts.append('numeric')
                        elif 'boolean' in col_type or 'bool' in col_type:
                            contexts.append('boolean')
                        else:
                            contexts.append('text')  # fallback
                
                # Check if used in CHECK constraint
                elif re.search(rf'CHECK\s*\([^)]*{re.escape(qualified_name)}\s*\([^)]*\)', ddl, re.IGNORECASE):
                    contexts.append('boolean')  # CHECK constraints expect boolean
                
                # Check if used in index expression
                elif re.search(rf'CREATE\s+(?:UNIQUE\s+)?INDEX.*?\([^)]*{re.escape(qualified_name)}\s*\([^)]*\)', ddl, re.IGNORECASE):
                    contexts.append('text')  # Index expressions often return text
        
        # Determine the most appropriate return type
        if contexts:
            # Count occurrences of each context
            context_counts = {}
            for ctx in contexts:
                context_counts[ctx] = context_counts.get(ctx, 0) + 1
            
            # Choose the most common context, with preference for specific types
            if 'uuid' in context_counts:
                function_contexts[(schema, func_name)] = 'uuid'
            elif 'timestamp' in context_counts:
                function_contexts[(schema, func_name)] = 'timestamp'
            elif 'boolean' in context_counts:
                function_contexts[(schema, func_name)] = 'boolean'
            elif 'integer' in context_counts:
                function_contexts[(schema, func_name)] = 'integer'
            elif 'numeric' in context_counts:
                function_contexts[(schema, func_name)] = 'numeric'
            else:
                function_contexts[(schema, func_name)] = 'text'
        else:
            # Default to text if no context found
            function_contexts[(schema, func_name)] = 'text'
    
    return function_contexts


def generate_stub_functions(function_refs, function_contexts):
    """Generate stub function definitions."""
    stubs = []
    
    # Return type mappings
    return_type_map = {
        'uuid': 'uuid',
        'timestamp': 'timestamp',
        'text': 'text',
        'integer': 'integer',
        'numeric': 'numeric(10,2)',
        'boolean': 'boolean'
    }
    
    # Return value mappings
    return_value_map = {
        'uuid': "'00000000-0000-0000-0000-000000000000'::uuid",
        'timestamp': "'2000-01-01 00:00:00'::timestamp",
        'text': "'STUB_FUNCTION_RESULT'::text",
        'integer': "0",
        'numeric': "0.0",
        'boolean': "false"
    }
    
    for schema, func_name in function_refs:
        context = function_contexts.get((schema, func_name), 'text')
        return_type = return_type_map.get(context, 'text')
        return_value = return_value_map.get(context, "'STUB_FUNCTION_RESULT'::text")
        
        if schema:
            # Schema-qualified function
            stub = f"""-- STUB: Missing function definition (excluded from anonymization)
CREATE OR REPLACE FUNCTION {schema}.{func_name}()
RETURNS {return_type}
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT {return_value};
$$;"""
        else:
            # Unqualified function - create in public schema
            stub = f"""-- STUB: Missing function definition (excluded from anonymization)  
CREATE OR REPLACE FUNCTION public.{func_name}()
RETURNS {return_type}
LANGUAGE sql
IMMUTABLE
AS $$
    SELECT {return_value};
$$;"""
        
        stubs.append(stub)
    
    return stubs


def handle_missing_dependencies(ddls):
    """Handle missing function/view/procedure dependencies."""
    # Extract function references
    function_refs = extract_function_references(ddls)
    
    if not function_refs:
        return ddls  # No missing functions found
    
    # Analyze function usage to determine appropriate return types
    function_contexts = analyze_function_usage(ddls, function_refs)
    
    # Generate stub functions
    stub_functions = generate_stub_functions(function_refs, function_contexts)
    
    # Add stubs to the beginning of DDL list (they'll be categorized properly later)
    return stub_functions + ddls


def fix_session_parameter(statement):
    """Fix anonymized session parameters with proper defaults."""
    
    # Handle SET statements
    set_match = re.match(r'^SET\s+(\w+)\s+TO\s+(const_[a-f0-9]+);?$', statement.strip(), re.IGNORECASE)
    if set_match:
        param_name = set_match.group(1).lower()
        
        # Skip unsupported parameters
        if param_name in PG_UNSUPPORTED_PARAMETERS:
            return f"-- SKIPPED (unsupported): {statement}"
        
        # Use default value if we have one
        if param_name in PG_PARAMETER_DEFAULTS:
            default_value = PG_PARAMETER_DEFAULTS[param_name]
            return f"SET {param_name} TO {default_value};"
        
        # For unknown parameters, comment them out
        return f"-- UNKNOWN PARAMETER: {statement}"
    
    # Handle SELECT pg_catalog.set_config statements
    select_match = re.match(r"^SELECT pg_catalog\.set_config\('(const_[a-f0-9]+)', '([^']*)', '[^']*'\);?$", statement.strip(), re.IGNORECASE)
    if select_match:
        # These are typically dynamic parameter settings, comment them out
        return f"-- DYNAMIC CONFIG (anonymized): {statement}"
    
    return statement


def replace_anonymized_constants(statement):
    """Replace anonymized constants in DEFAULT clauses with PostgreSQL-compatible dummy values."""
    
    # Skip non-CREATE TABLE statements for efficiency
    if not statement.strip().upper().startswith('CREATE TABLE'):
        return statement
    
    # Pattern 1: Handle cast expressions like DEFAULT ('const_xxx'::uuid)
    def replace_cast_constants(match):
        cast_type = match.group(1).lower().strip()  # uuid, int4, etc.
        
        # Get dummy value based on cast type
        dummy_value = DUMMY_VALUES_BY_TYPE.get(cast_type, "'dummy_value'")
        
        # For UUID, we can cast the dummy value
        if cast_type == 'uuid':
            return f"DEFAULT ({dummy_value}::{cast_type})"
        else:
            # For other types, just use the dummy value directly
            return f"DEFAULT {dummy_value}"
    
    # Apply cast pattern replacement
    cast_pattern = r"DEFAULT\s+\('const_[a-f0-9]+'::([\w\s]+)\)"
    statement = re.sub(cast_pattern, replace_cast_constants, statement, flags=re.IGNORECASE)
    
    # Pattern 2: Handle direct string constants like DEFAULT 'const_xxx'
    def replace_direct_constants(match):
        before = match.group(1)
        after = match.group(2)
        
        # Try to determine the data type from the preceding column definition
        # Look for the last data type word before DEFAULT
        type_match = re.search(r'\b(boolean|bool|int|int2|int4|int8|integer|bigint|smallint|numeric|decimal|real|float4|float8|double\s+precision|text|varchar|character\s+varying|char|character|uuid|timestamp(?:\s+with(?:out)?\s+time\s+zone)?|timestamptz|date|time(?:\s+with(?:out)?\s+time\s+zone)?|timetz)\b(?:\([^)]*\))?\s*(?:NOT\s+NULL|NULL)?\s*$', before, re.IGNORECASE)
        
        if type_match:
            data_type = type_match.group(1).lower().strip()
            data_type = re.sub(r'\s+', ' ', data_type)
            dummy_value = DUMMY_VALUES_BY_TYPE.get(data_type, "'dummy_value'")
        else:
            dummy_value = "'dummy_value'"
        
        return f"{before} DEFAULT {dummy_value}{after}"
    
    # Apply direct constant pattern replacement
    direct_pattern = r"(.+?)\s+DEFAULT\s+'const_[a-f0-9]+'(\s*.*?)(?=,|\)|$)"
    statement = re.sub(direct_pattern, replace_direct_constants, statement, flags=re.IGNORECASE)
    
    # Pattern 3: Handle empty string defaults for booleans like DEFAULT ''
    def replace_empty_boolean_defaults(match):
        before = match.group(1)
        after = match.group(2)
        
        # Check if this is a boolean column
        if re.search(r'\b(boolean|bool)\b', before, re.IGNORECASE):
            return f"{before} DEFAULT true{after}"
        else:
            # For non-boolean columns, keep empty string but quote it properly
            return f"{before} DEFAULT ''{after}"
    
    # Apply empty string pattern replacement
    empty_pattern = r"(.+?)\s+DEFAULT\s+''(\s*.*?)(?=,|\)|$)"
    statement = re.sub(empty_pattern, replace_empty_boolean_defaults, statement, flags=re.IGNORECASE)
    
    # Pattern 4: Handle array DEFAULT with const values like text[] DEFAULT ('const_xxx'::text[])
    def replace_array_defaults(match):
        array_type = match.group(1)  # e.g., 'text[]'
        cast_type = match.group(2)   # e.g., 'text[]'
        
        # Get the dummy value for this array type
        dummy_value = DUMMY_VALUES_BY_TYPE.get(array_type, "'{}'::text[]")
        return f"{array_type} DEFAULT {dummy_value}"
    
    # Apply array default pattern replacement
    array_pattern = r'\b(text\[\])\s+DEFAULT\s+\(\'const_[a-f0-9]+\'::(text\[\])\)'
    statement = re.sub(array_pattern, replace_array_defaults, statement, flags=re.IGNORECASE)
    
    return statement


def replace_anonymized_datatype_params(statement):
    """Replace anonymized constants in datatype parameters with PostgreSQL-compatible dummy values."""
    
    # Skip non-CREATE TABLE statements for efficiency
    if not statement.strip().upper().startswith('CREATE TABLE'):
        return statement
    
    def get_dummy_params(datatype, param_count):
        """Get appropriate dummy parameters for a datatype."""
        datatype = datatype.lower().strip()
        datatype = re.sub(r'\s+', ' ', datatype)  # normalize whitespace
        
        if datatype in DATATYPE_PARAM_DEFAULTS:
            defaults = DATATYPE_PARAM_DEFAULTS[datatype]
            # Return the appropriate number of parameters
            if param_count == 1:
                return [defaults[0]] if len(defaults) >= 1 else ['10']
            elif param_count == 2:
                return defaults[:2] if len(defaults) >= 2 else ['10', '2']
            else:
                return defaults[:param_count] if len(defaults) >= param_count else ['10'] * param_count
        else:
            # Fallback for unknown datatypes
            return ['10', '2'][:param_count] if param_count <= 2 else ['10'] * param_count
    
    # Pattern 1: Single parameter datatypes like varchar('const_xxx')
    def replace_single_param(match):
        datatype = match.group(1)
        dummy_params = get_dummy_params(datatype, 1)
        return f"{datatype}({dummy_params[0]})"
    
    single_param_pattern = r'\b(varchar|character\s+varying|char|character|bpchar|timestamp|timestamp\s+with\s+time\s+zone|timestamp\s+without\s+time\s+zone|timestamptz|time|time\s+with\s+time\s+zone|time\s+without\s+time\s+zone|timetz|interval|bit|varbit|bit\s+varying|float|real|double\s+precision)\s*\(\s*\'const_[a-f0-9]+\'\s*\)'
    statement = re.sub(single_param_pattern, replace_single_param, statement, flags=re.IGNORECASE)
    
    # Pattern 2: Dual parameter datatypes like numeric('const_xxx', 'const_yyy')
    def replace_dual_param(match):
        datatype = match.group(1)
        dummy_params = get_dummy_params(datatype, 2)
        return f"{datatype}({dummy_params[0]}, {dummy_params[1]})"
    
    dual_param_pattern = r'\b(numeric|decimal)\s*\(\s*\'const_[a-f0-9]+\'\s*,\s*\'const_[a-f0-9]+\'\s*\)'
    statement = re.sub(dual_param_pattern, replace_dual_param, statement, flags=re.IGNORECASE)
    
    return statement


def categorize_ddl(statement):
    """Categorize DDL statement based on YB Voyager's execution order."""
    stmt_upper = statement.strip().upper()
    
    # Session parameters
    if (stmt_upper.startswith('SET ') or 
        stmt_upper.startswith('SELECT PG_CATALOG.SET_CONFIG') or
        stmt_upper.startswith('-- SKIPPED') or 
        stmt_upper.startswith('-- DYNAMIC')):
        return 'session_params'
    
    # Schema creation
    elif stmt_upper.startswith('CREATE SCHEMA'):
        return 'schemas'
    
    # Extensions
    elif stmt_upper.startswith('CREATE EXTENSION'):
        return 'extensions'
    
    # Types and domains
    elif (stmt_upper.startswith('CREATE TYPE') or 
          stmt_upper.startswith('CREATE DOMAIN')):
        return 'types'
    
    # Stub functions (generated by our dependency handler)
    elif (stmt_upper.startswith('-- STUB:') or 
          (stmt_upper.startswith('CREATE OR REPLACE FUNCTION') and 
           'STUB_FUNCTION_RESULT' in stmt_upper)):
        return 'stub_functions'
    
    # Sequences - split into create and alter
    elif stmt_upper.startswith('CREATE SEQUENCE'):
        return 'sequences_create'
    elif ('ALTER SEQUENCE' in stmt_upper and 'OWNED BY' in stmt_upper):
        return 'sequences_alter'
    
    # Tables - only CREATE TABLE
    elif stmt_upper.startswith('CREATE TABLE'):
        return 'tables_create'
    
    # Indexes
    elif (stmt_upper.startswith('CREATE INDEX') or 
          stmt_upper.startswith('CREATE UNIQUE INDEX')):
        return 'indexes'
    
    # Functions, procedures, aggregates
    elif (stmt_upper.startswith('CREATE FUNCTION') or 
          stmt_upper.startswith('CREATE PROCEDURE') or
          stmt_upper.startswith('CREATE AGGREGATE')):
        return 'functions'
    
    # Views
    elif (stmt_upper.startswith('CREATE VIEW') or 
          stmt_upper.startswith('CREATE MATERIALIZED VIEW')):
        return 'views'
    
    # Triggers
    elif stmt_upper.startswith('CREATE TRIGGER'):
        return 'triggers'
    
    # Constraints (non-foreign key)
    elif ('ALTER TABLE' in stmt_upper and 
          'ADD CONSTRAINT' in stmt_upper and 
          'FOREIGN KEY' not in stmt_upper):
        return 'constraints'
    
    # Foreign keys
    elif ('ALTER TABLE' in stmt_upper and 'FOREIGN KEY' in stmt_upper):
        return 'foreign_keys'
    
    # Everything else
    else:
        return 'others'


def load_json(file_path):
    """Load and parse the JSON file."""
    try:
        with open(file_path, 'r', encoding='utf-8') as f:
            return json.load(f)
    except FileNotFoundError:
        print(f"Error: File not found: {file_path}")
        sys.exit(1)
    except json.JSONDecodeError as e:
        print(f"Error: Invalid JSON format: {e}")
        sys.exit(1)


def extract_ddls(payload):
    """Extract anonymized DDLs from the callhome payload."""
    phase_payload_data = payload.get('phase_payload')
    if not phase_payload_data:
        print("Error: No 'phase_payload' field found in JSON")
        sys.exit(1)
    
    # Handle both string and dict formats for phase_payload
    if isinstance(phase_payload_data, str):
        try:
            phase_payload = json.loads(phase_payload_data)
        except json.JSONDecodeError as e:
            print(f"Error: Invalid JSON in phase_payload: {e}")
            sys.exit(1)
    elif isinstance(phase_payload_data, dict):
        phase_payload = phase_payload_data
    else:
        print(f"Error: phase_payload must be a string or dict, got {type(phase_payload_data)}")
        sys.exit(1)
    
    anonymized_ddls = phase_payload.get('anonymized_ddls', [])
    if not anonymized_ddls:
        print("Error: No anonymized_ddls found in payload")
        sys.exit(1)
    
    return anonymized_ddls


def comment_out_anonymized_nextval(statement):
    """
    Comment out ALTER TABLE SET DEFAULT nextval statements that reference anonymized sequence names.
    
    These statements fail because they reference const_xxx sequence names that don't exist.
    The actual sequences are created with seq_xxx names, but the nextval references are anonymized
    as const_xxx by the Go anonymizer's literalNodesProcessor.
    
    This is a known issue in the anonymizer that processes string literals in nextval() calls
    as constants rather than sequence references.
    
    Args:
        statement (str): The DDL statement to process
        
    Returns:
        str: The statement, commented out if it's a problematic nextval reference
    """
    # Pattern for ALTER TABLE SET DEFAULT nextval with anonymized sequence names
    nextval_pattern = r'ALTER\s+TABLE\s+ONLY\s+.*?\s+ALTER\s+COLUMN\s+.*?\s+SET\s+DEFAULT\s+nextval\s*\(\s*\'const_[a-f0-9]+\'\s*::\s*regclass\s*\)'
    
    if re.search(nextval_pattern, statement, re.IGNORECASE):
        return f"-- {statement}"
    
    return statement


def clean_ddl(ddl):
    """Clean and format a DDL statement with parameter fixes and constant replacement."""
    # Fix session parameters first
    ddl_fixed = fix_session_parameter(ddl)
    
    # Replace anonymized constants in DEFAULT clauses
    ddl_with_dummies = replace_anonymized_constants(ddl_fixed)
    
    # Replace anonymized constants in datatype parameters
    ddl_with_fixed_types = replace_anonymized_datatype_params(ddl_with_dummies)
    
    # Comment out problematic nextval statements with anonymized sequence names
    ddl_with_nextval_fix = comment_out_anonymized_nextval(ddl_with_fixed_types)
    
    # Clean and add semicolon if needed (unless it's a comment)
    cleaned = ddl_with_nextval_fix.strip()
    if cleaned and not cleaned.endswith(';') and not cleaned.startswith('--'):
        cleaned += ';'
    return cleaned


def organize_ddls_by_category(ddls):
    """Organize DDL statements by category following Voyager's order."""
    categorized_ddls = {category: [] for category in VOYAGER_DDL_ORDER}
    
    for ddl in ddls:
        category = categorize_ddl(ddl)
        categorized_ddls[category].append(ddl)
    
    return categorized_ddls


def write_sql_file(ddls, output_file):
    """Generate a SQL file from the DDL statements in proper dependency order."""
    try:
        # Handle missing dependencies FIRST
        ddls_with_stubs = handle_missing_dependencies(ddls)
        
        # Organize DDLs by category (including new stub_functions category)
        categorized_ddls = organize_ddls_by_category(ddls_with_stubs)
        
        with open(output_file, 'w', encoding='utf-8') as f:
            # Write header
            f.write("-- YB Voyager Anonymized DDL Statements\n")
            f.write(f"-- Extracted on: {datetime.now().strftime('%Y-%m-%d %H:%M:%S')}\n")
            f.write(f"-- Total DDL statements: {len(ddls_with_stubs)}\n")
            f.write("-- \n")
            f.write("-- This file contains anonymized DDL statements from YB Voyager callhome payload.\n")
            f.write("-- All sensitive information has been anonymized.\n")
            f.write("-- Session parameters have been fixed with PostgreSQL defaults.\n")
            f.write("-- Anonymized constants in DEFAULT clauses have been replaced with dummy values.\n")
            f.write("-- Anonymized constants in datatype parameters have been replaced with dummy values.\n")
            f.write("-- Array types with anonymized DEFAULT values have been replaced with empty arrays.\n")
            f.write("-- Problematic nextval() statements with anonymized sequence names have been commented out.\n")
            f.write("-- STUB functions have been generated for missing function dependencies.\n")
            f.write("-- DDL statements are ordered to handle dependencies correctly.\n")
            f.write("-- \n\n")
            
            # Check if there are any commented nextval statements
            has_commented_nextval = False
            for category_ddls in categorized_ddls.values():
                for ddl in category_ddls:
                    cleaned_ddl = clean_ddl(ddl)
                    if cleaned_ddl.startswith('-- ALTER TABLE ONLY') and 'nextval(' in cleaned_ddl and 'const_' in cleaned_ddl:
                        has_commented_nextval = True
                        break
                if has_commented_nextval:
                    break
            
            # Write DDL statements in dependency order
            statement_number = 1
            for category in VOYAGER_DDL_ORDER:
                category_ddls = categorized_ddls[category]
                if category_ddls:
                    # Add category header for clarity
                    category_name = category.replace('_', ' ').title()
                    f.write(f"-- ========== {category_name} ==========\n\n")
                    
                    # Add special comment for others section if it has commented nextval statements
                    if category == 'others' and has_commented_nextval:
                        f.write("-- Commenting ANONYMIZED SEQUENCE NEXTVAL due to anonymiser bug.\n")
                        f.write("-- These statements reference const_xxx sequence names that don't exist.\n")
                        f.write("-- The actual sequences are created with seq_xxx names above.\n")
                        f.write("-- Uncomment when the anonymizer is fixed to properly map sequence references.\n\n")
                    
                    for ddl in category_ddls:
                        f.write(f"-- DDL Statement {statement_number}\n")
                        f.write(f"{clean_ddl(ddl)}\n\n")
                        statement_number += 1
            
            f.write("-- End of anonymized DDL statements\n")
        
        # Print summary
        print(f"Successfully created SQL file: {output_file}")
        print(f"Extracted {len(ddls)} DDL statements")
        
        # Show stub function summary
        stub_count = len(categorized_ddls.get('stub_functions', []))
        if stub_count > 0:
            print(f"Generated {stub_count} stub functions for missing dependencies")
        
        # Show categorization summary
        non_empty_categories = {cat: len(ddls) for cat, ddls in categorized_ddls.items() if ddls}
        if non_empty_categories:
            print("\nDDL Categories:")
            for category, count in non_empty_categories.items():
                category_display = category.replace('_', ' ').title()
                print(f"  {category_display}: {count} statements")
        
    except Exception as e:
        print(f"Error writing SQL file: {e}")
        sys.exit(1)


def main():
    parser = argparse.ArgumentParser(
        description="Extract anonymized DDLs from YB Voyager callhome payload JSON"
    )
    parser.add_argument('input_file', help='Input JSON file containing callhome payload')
    parser.add_argument('output_file', nargs='?', help='Output SQL file (optional)')
    
    args = parser.parse_args()
    
    # Default output file name
    if not args.output_file:
        input_path = Path(args.input_file)
        args.output_file = f"anonymized_ddls_{input_path.stem}.sql"
    
    # Load JSON and extract DDLs
    payload = load_json(args.input_file)
    ddls = extract_ddls(payload)
    
    # Write SQL file
    write_sql_file(ddls, args.output_file)


if __name__ == "__main__":
    main()