/*
ERROR: server "p10" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/pgtbrus/export-dir/schema/tables/foreign_table.sql
*/
CREATE FOREIGN TABLE public.f_c (
    i integer NOT NULL,
    t integer,
    x text
)
SERVER p10
OPTIONS (
    table_name 'c'
);

/*
ERROR: server "p10" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/pgtbrus/export-dir/schema/tables/foreign_table.sql
*/
CREATE FOREIGN TABLE public.f_t (
    i integer NOT NULL,
    ts timestamp(0) with time zone DEFAULT now(),
    j json,
    t text,
    e public.myenum,
    c public.mycomposit
)
SERVER p10
OPTIONS (
    table_name 't'
);

/*
ERROR: relation "public.f_c" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/pgtbrus/export-dir/schema/tables/table.sql
*/
ALTER FOREIGN TABLE ONLY public.f_c ALTER COLUMN i SET DEFAULT nextval('public.f_c_i_seq'::regclass);

/*
ERROR: relation "public.f_t" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/pgtbrus/export-dir/schema/tables/table.sql
*/
ALTER FOREIGN TABLE ONLY public.f_t ALTER COLUMN i SET DEFAULT nextval('public.f_t_i_seq'::regclass);

/*
ERROR: relation "public.f_c" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/pgtbrus/export-dir/schema/sequences/sequence.sql
*/
ALTER SEQUENCE public.f_c_i_seq OWNED BY public.f_c.i;

/*
ERROR: relation "public.f_t" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/pgtbrus/export-dir/schema/sequences/sequence.sql
*/
ALTER SEQUENCE public.f_t_i_seq OWNED BY public.f_t.i;

