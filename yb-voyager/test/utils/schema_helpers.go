package testutils

import "fmt"

// AllTypesSchemaDDL returns the DDLs needed to create the multi-datatype table with the given name.
func AllTypesSchemaDDL(tableName string) string {
	return fmt.Sprintf(`
CREATE TYPE week AS ENUM ('Mon','Tue','Wed','Thu','Fri','Sat','Sun');
-- Enable required extensions
CREATE EXTENSION IF NOT EXISTS hstore; -- for hstore
CREATE EXTENSION IF NOT EXISTS pgcrypto; -- for digest

CREATE TABLE %s (
    id              SERIAL PRIMARY KEY,
    v1_smallint     SMALLINT,
    v2_int          INTEGER,
    v3_bigint       BIGINT,
    v4_decimal      DECIMAL(6,3),
    v5_numeric      NUMERIC,
    v6_money        MONEY,
    v7_bigprec1     NUMERIC(108,9),
    v8_bigprec2     NUMERIC(19,2),
    v9_bool         BOOLEAN DEFAULT true,
    v10_char1       CHAR(1),
    v11_varchar     VARCHAR(100),
    v12_text        TEXT,
    v13_bytea       BYTEA,
    v14_enum        week,
    v15_date        DATE,
    v16_time        TIME,
    v17_ts          TIMESTAMP,
    v18_ts_no_tz    TIMESTAMP WITHOUT TIME ZONE DEFAULT CURRENT_TIMESTAMP(0),
    v19_json        JSON,
    v20_bit10       BIT(10),
    v21_int_arr     INT[],
    v22_text_2d     TEXT[][],
    v23_bit_var     BIT VARYING,
    v24_def_bool    BOOLEAN DEFAULT false,
    v25_def_int     INTEGER DEFAULT 10,
    v26_def_varchar VARCHAR DEFAULT 'testdefault',
    v27_hstore      HSTORE
);`, tableName)
}

// AllTypesInsertSQL returns an INSERT…SELECT that populates `rowCount` rows
// in `tableName` with randomized data for every supported PostgreSQL datatype.
func AllTypesInsertSQL(tableName string, rowCount int) string {
	return fmt.Sprintf(`
INSERT INTO %s (
    v1_smallint,
    v2_int,
    v3_bigint,
    v4_decimal,
    v5_numeric,
    v6_money,
    v7_bigprec1,
    v8_bigprec2,
    v9_bool,
    v10_char1,
    v11_varchar,
    v12_text,
    v13_bytea,
    v14_enum,
    v15_date,
    v16_time,
    v17_ts,
    v19_json,
    v20_bit10,
    v21_int_arr,
    v22_text_2d,
    v23_bit_var,
    v27_hstore
)
SELECT
    -- v1_smallint: small integer
    (random()*32767)::SMALLINT,

    -- v2_int: standard integer
    (random()*100000)::INT,

    -- v3_bigint: large integer
    (random()*1000000000)::BIGINT,

    -- v4_decimal: fixed-scale numeric
    round((random()*1000)::numeric,3),

    -- v5_numeric: arbitrary-precision numeric
    random()*1000,

    -- v6_money: locale-aware money
    concat('$', (random()*100)::numeric)::MONEY,

    -- v7_bigprec1: high-precision numeric (108,9)
    (random()*999999999)::NUMERIC(108,9),

    -- v8_bigprec2: high-precision numeric (19,2)
    (random()*9999)::NUMERIC(19,2),

    -- v9_bool: boolean
    (random()>0.5),

    -- v10_char1: fixed-width char
    chr(65 + trunc(random()*25)::INT),

    -- v11_varchar: variable-length string
    md5(random()::text),

    -- v12_text: unconstrained text
    repeat(md5(random()::text),3),

    -- v13_bytea: binary data via SHA-1 digest
    encode(
      digest(random()::text::bytea,'sha1'),
      'hex'
    )::bytea,

    -- v14_enum: one of your ENUM values
    (ARRAY['Mon','Tue','Wed','Thu','Fri','Sat','Sun']::week[])[(1+floor(random()*7))::INT],

    -- v15_date: calendar date
    date '2025-01-01' + (random()*30)::INT,

    -- v16_time: time of day
    time '00:00' + (((random()*86400)::INT)||' seconds')::interval,

    -- v17_ts: timestamp with time zone
    now() - (((random()*10000)::INT)||' seconds')::interval,

    -- v19_json: JSON object
    json_build_object('k',md5(random()::text)),

    -- v20_bit10: fixed 10-bit string
    lpad('',10,'1')::bit(10),

    -- v21_int_arr: one-dimensional int array
    ARRAY[(random()*10)::INT,(random()*10)::INT,(random()*10)::INT],

    -- v22_text_2d: two-dimensional text array
    ARRAY[ARRAY['a','b'],ARRAY['c','d']],

    -- v23_bit_var: varying-length bit string
    lpad('1010',16,'0')::bit varying,

    -- v27_hstore: key→value mapping
    hstore(ARRAY[['foo','bar'],['baz','qux']])
FROM generate_series(1,%d);
`, tableName, rowCount)
}
