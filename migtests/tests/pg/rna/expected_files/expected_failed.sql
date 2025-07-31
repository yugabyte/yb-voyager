/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p10_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p10_deleted_check CHECK (dbid = 10 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p10_not_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p10_not_deleted_check CHECK (dbid = 10 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p11_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p11_deleted_check CHECK (dbid = 11 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p11_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p11_not_deleted_check CHECK (dbid = 11 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p12_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p12_deleted_check CHECK (dbid = 12 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p12_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p12_not_deleted_check CHECK (dbid = 12 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p13_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p13_deleted_check CHECK (dbid = 13 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p13_not_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p13_not_deleted_check CHECK (dbid = 13 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p14_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p14_deleted_check CHECK (dbid = 14 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p14_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p14_not_deleted_check CHECK (dbid = 14 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p15_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p15_deleted_check CHECK (dbid = 15 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p15_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p15_not_deleted_check CHECK (dbid = 15 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p16_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p16_deleted_check CHECK (dbid = 16 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p16_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p16_not_deleted_check CHECK (dbid = 16 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p17_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p17_deleted_check CHECK (dbid = 17 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p17_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p17_not_deleted_check CHECK (dbid = 17 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p18_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p18_deleted_check CHECK (dbid = 18 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p18_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p18_not_deleted_check CHECK (dbid = 18 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p19_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p19_deleted_check CHECK (dbid = 19 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p19_not_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p19_not_deleted_check CHECK (dbid = 19 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p1_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p1_deleted_check CHECK (dbid = 1 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p1_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p1_not_deleted_check CHECK (dbid = 1 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p20_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p20_deleted_check CHECK (dbid = 20 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p20_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p20_not_deleted_check CHECK (dbid = 20 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p21_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p21_deleted_check CHECK (dbid = 21 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p21_not_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p21_not_deleted_check CHECK (dbid = 21 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p22_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p22_deleted_check CHECK (dbid = 22 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p22_not_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p22_not_deleted_check CHECK (dbid = 22 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p23_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p23_deleted_check CHECK (dbid = 23 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p23_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p23_not_deleted_check CHECK (dbid = 23 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p24_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p24_deleted_check CHECK (dbid = 24 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p24_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p24_not_deleted_check CHECK (dbid = 24 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p25_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p25_deleted_check CHECK (dbid = 25 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p25_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p25_not_deleted_check CHECK (dbid = 25 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p26_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p26_deleted_check CHECK (dbid = 26 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p26_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p26_not_deleted_check CHECK (dbid = 26 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p27_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p27_deleted_check CHECK (dbid = 27 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p27_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p27_not_deleted_check CHECK (dbid = 27 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p28_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p28_deleted_check CHECK (dbid = 28 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p28_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p28_not_deleted_check CHECK (dbid = 28 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p29_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p29_deleted_check CHECK (dbid = 29 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p29_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p29_not_deleted_check CHECK (dbid = 29 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p2_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p2_deleted_check CHECK (dbid = 2 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p2_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p2_not_deleted_check CHECK (dbid = 2 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p30_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p30_deleted_check CHECK (dbid = 30 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p30_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p30_not_deleted_check CHECK (dbid = 30 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p31_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p31_deleted_check CHECK (dbid = 31 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p31_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p31_not_deleted_check CHECK (dbid = 31 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p32_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p32_deleted_check CHECK (dbid = 32 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p32_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p32_not_deleted_check CHECK (dbid = 32 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p33_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p33_deleted_check CHECK (dbid = 33 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p33_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p33_not_deleted_check CHECK (dbid = 33 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p34_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p34_deleted_check CHECK (dbid = 34 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p34_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p34_not_deleted_check CHECK (dbid = 34 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p35_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p35_deleted_check CHECK (dbid = 35 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p35_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p35_not_deleted_check CHECK (dbid = 35 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p36_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p36_deleted_check CHECK (dbid = 36 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p36_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p36_not_deleted_check CHECK (dbid = 36 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p37_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p37_deleted_check CHECK (dbid = 37 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p37_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p37_not_deleted_check CHECK (dbid = 37 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p38_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p38_deleted_check CHECK (dbid = 38 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p38_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p38_not_deleted_check CHECK (dbid = 38 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p39_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p39_deleted_check CHECK (dbid = 39 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p39_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p39_not_deleted_check CHECK (dbid = 39 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p3_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p3_deleted_check CHECK (dbid = 3 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p3_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p3_not_deleted_check CHECK (dbid = 3 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p40_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p40_deleted_check CHECK (dbid = 40 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p40_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p40_not_deleted_check CHECK (dbid = 40 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p41_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p41_deleted_check CHECK (dbid = 41 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p41_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p41_not_deleted_check CHECK (dbid = 41 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p42_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p42_deleted_check CHECK (dbid = 42 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p42_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p42_not_deleted_check CHECK (dbid = 42 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p43_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p43_deleted_check CHECK (dbid = 43 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p43_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p43_not_deleted_check CHECK (dbid = 43 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p44_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p44_deleted_check CHECK (dbid = 44 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p44_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p44_not_deleted_check CHECK (dbid = 44 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p45_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p45_deleted_check CHECK (dbid = 45 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p45_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p45_not_deleted_check CHECK (dbid = 45 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p46_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p46_deleted_check CHECK (dbid = 46 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p46_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p46_not_deleted_check CHECK (dbid = 46 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p47_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p47_deleted_check CHECK (dbid = 47 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p47_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p47_not_deleted_check CHECK (dbid = 47 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p48_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p48_deleted_check CHECK (dbid = 48 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p48_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p48_not_deleted_check CHECK (dbid = 48 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p49_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p49_deleted_check CHECK (dbid = 49 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p49_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p49_not_deleted_check CHECK (dbid = 49 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p4_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p4_deleted_check CHECK (dbid = 4 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p4_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p4_not_deleted_check CHECK (dbid = 4 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p50_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p50_deleted_check CHECK (dbid = 50 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p50_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p50_not_deleted_check CHECK (dbid = 50 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p51_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p51_deleted_check CHECK (dbid = 51 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p51_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p51_not_deleted_check CHECK (dbid = 51 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p52_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p52_deleted_check CHECK (dbid = 52 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p52_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p52_not_deleted_check CHECK (dbid = 52 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p53_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p53_deleted_check CHECK (dbid = 53 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p53_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p53_not_deleted_check CHECK (dbid = 53 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p54_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint NOT NULL, CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p54_deleted_check CHECK (dbid = 54 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p54_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint NOT NULL, CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p54_not_deleted_check CHECK (dbid = 54 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p55_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p55_deleted_check CHECK (dbid = 55 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p55_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p55_not_deleted_check CHECK (dbid = 55 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p5_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p5_deleted_check CHECK (dbid = 5 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p5_not_deleted (CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p5_not_deleted_check CHECK (dbid = 5 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p6_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p6_deleted_check CHECK (dbid = 6 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p6_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p6_not_deleted_check CHECK (dbid = 6 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p7_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p7_deleted_check CHECK (dbid = 7 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p7_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p7_not_deleted_check CHECK (dbid = 7 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p8_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p8_deleted_check CHECK (dbid = 8 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p8_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p8_not_deleted_check CHECK (dbid = 8 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p9_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p9_deleted_check CHECK (dbid = 9 AND deleted = 'Y'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: INHERITS not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
CREATE TABLE rnacen.xref_p9_not_deleted (dbid smallint, created int, last int, upi varchar(26), version_i int, deleted char(1), "timestamp" timestamp DEFAULT ('now'::text::timestamp), userstamp varchar(20) DEFAULT ('USER'::varchar), ac varchar(300), version int, taxid bigint, id bigint DEFAULT nextval('rnacen.xref_pk_seq'::regclass), CONSTRAINT "ck_xref$deleted" CHECK (deleted = ANY(ARRAY['Y'::bpchar, 'N'::bpchar])), CONSTRAINT xref_p9_not_deleted_check CHECK (dbid = 9 AND deleted = 'N'::bpchar)) INHERITS (rnacen.xref) WITH (colocation=false);

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_not_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_not_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_not_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_not_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_not_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p54_deleted_id_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_p54_not_deleted_id_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_not_deleted ALTER COLUMN "timestamp" SET DEFAULT 'now'::text::timestamp;

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_not_deleted ALTER COLUMN userstamp SET DEFAULT 'USER'::varchar;

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_not_deleted ALTER COLUMN id SET DEFAULT nextval('rnacen.xref_pk_seq'::regclass);

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p10_deleted__upi_taxid ON rnacen.xref_p10_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p10_not_deleted__upi_taxid ON rnacen.xref_p10_not_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p13_deleted__upi_taxid ON rnacen.xref_p13_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p13_not_deleted__upi_taxid ON rnacen.xref_p13_not_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p19_deleted__upi_taxid ON rnacen.xref_p19_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p19_not_deleted__upi_taxid ON rnacen.xref_p19_not_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p21_deleted__upi_taxid ON rnacen.xref_p21_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p21_not_deleted__upi_taxid ON rnacen.xref_p21_not_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p22_deleted__upi_taxid ON rnacen.xref_p22_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p22_not_deleted__upi_taxid ON rnacen.xref_p22_not_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p27_deleted__upi_taxid ON rnacen.xref_p27_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p27_not_deleted__upi_taxid ON rnacen.xref_p27_not_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p28_deleted__upi_taxid ON rnacen.xref_p28_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p28_not_deleted__upi_taxid ON rnacen.xref_p28_not_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p29_deleted__upi_taxid ON rnacen.xref_p29_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p29_not_deleted__upi_taxid ON rnacen.xref_p29_not_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p5_deleted__upi_taxid ON rnacen.xref_p5_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX ix_xref_p5_not_deleted__upi_taxid ON rnacen.xref_p5_not_deleted USING btree (upi ASC, taxid);

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_deleted$ac" ON rnacen.xref_p10_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_deleted$created" ON rnacen.xref_p10_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_deleted$dbid" ON rnacen.xref_p10_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_deleted$last" ON rnacen.xref_p10_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_deleted$taxid" ON rnacen.xref_p10_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_deleted$upi" ON rnacen.xref_p10_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_not_deleted$ac" ON rnacen.xref_p10_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_not_deleted$created" ON rnacen.xref_p10_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_not_deleted$dbid" ON rnacen.xref_p10_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_not_deleted$last" ON rnacen.xref_p10_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_not_deleted$taxid" ON rnacen.xref_p10_not_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p10_not_deleted$upi" ON rnacen.xref_p10_not_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_deleted$ac" ON rnacen.xref_p11_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_deleted$created" ON rnacen.xref_p11_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_deleted$dbid" ON rnacen.xref_p11_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_deleted$last" ON rnacen.xref_p11_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_deleted$taxid" ON rnacen.xref_p11_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_deleted$upi" ON rnacen.xref_p11_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_not_deleted$ac" ON rnacen.xref_p11_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_not_deleted$created" ON rnacen.xref_p11_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_not_deleted$dbid" ON rnacen.xref_p11_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_not_deleted$last" ON rnacen.xref_p11_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_not_deleted$taxid" ON rnacen.xref_p11_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p11_not_deleted$upi" ON rnacen.xref_p11_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_deleted$ac" ON rnacen.xref_p12_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_deleted$created" ON rnacen.xref_p12_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_deleted$dbid" ON rnacen.xref_p12_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/  
CREATE INDEX "xref_p12_deleted$last" ON rnacen.xref_p12_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_deleted$taxid" ON rnacen.xref_p12_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_deleted$upi" ON rnacen.xref_p12_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_not_deleted$ac" ON rnacen.xref_p12_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_not_deleted$created" ON rnacen.xref_p12_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_not_deleted$dbid" ON rnacen.xref_p12_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_not_deleted$last" ON rnacen.xref_p12_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_not_deleted$taxid" ON rnacen.xref_p12_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p12_not_deleted$upi" ON rnacen.xref_p12_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_deleted$ac" ON rnacen.xref_p13_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_deleted$created" ON rnacen.xref_p13_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_deleted$dbid" ON rnacen.xref_p13_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_deleted$last" ON rnacen.xref_p13_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_deleted$taxid" ON rnacen.xref_p13_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_deleted$upi" ON rnacen.xref_p13_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_not_deleted$ac" ON rnacen.xref_p13_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_not_deleted$created" ON rnacen.xref_p13_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_not_deleted$dbid" ON rnacen.xref_p13_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_not_deleted$last" ON rnacen.xref_p13_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_not_deleted$taxid" ON rnacen.xref_p13_not_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p13_not_deleted$upi" ON rnacen.xref_p13_not_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_deleted$ac" ON rnacen.xref_p14_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_deleted$created" ON rnacen.xref_p14_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_deleted$dbid" ON rnacen.xref_p14_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_deleted$last" ON rnacen.xref_p14_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_deleted$taxid" ON rnacen.xref_p14_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_deleted$upi" ON rnacen.xref_p14_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_not_deleted$ac" ON rnacen.xref_p14_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_not_deleted$created" ON rnacen.xref_p14_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_not_deleted$dbid" ON rnacen.xref_p14_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_not_deleted$last" ON rnacen.xref_p14_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_not_deleted$taxid" ON rnacen.xref_p14_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p14_not_deleted$upi" ON rnacen.xref_p14_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_deleted$ac" ON rnacen.xref_p15_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_deleted$created" ON rnacen.xref_p15_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_deleted$dbid" ON rnacen.xref_p15_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_deleted$last" ON rnacen.xref_p15_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_deleted$taxid" ON rnacen.xref_p15_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_deleted$upi" ON rnacen.xref_p15_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_not_deleted$ac" ON rnacen.xref_p15_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_not_deleted$created" ON rnacen.xref_p15_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_not_deleted$dbid" ON rnacen.xref_p15_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_not_deleted$last" ON rnacen.xref_p15_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_not_deleted$taxid" ON rnacen.xref_p15_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p15_not_deleted$upi" ON rnacen.xref_p15_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_deleted$ac" ON rnacen.xref_p16_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_deleted$created" ON rnacen.xref_p16_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_deleted$dbid" ON rnacen.xref_p16_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_deleted$last" ON rnacen.xref_p16_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_deleted$taxid" ON rnacen.xref_p16_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_deleted$upi" ON rnacen.xref_p16_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_not_deleted$ac" ON rnacen.xref_p16_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_not_deleted$created" ON rnacen.xref_p16_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_not_deleted$dbid" ON rnacen.xref_p16_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_not_deleted$last" ON rnacen.xref_p16_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_not_deleted$taxid" ON rnacen.xref_p16_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p16_not_deleted$upi" ON rnacen.xref_p16_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_deleted$ac" ON rnacen.xref_p17_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_deleted$created" ON rnacen.xref_p17_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_deleted$dbid" ON rnacen.xref_p17_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_deleted$last" ON rnacen.xref_p17_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_deleted$taxid" ON rnacen.xref_p17_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_deleted$upi" ON rnacen.xref_p17_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_not_deleted$ac" ON rnacen.xref_p17_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_not_deleted$created" ON rnacen.xref_p17_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_not_deleted$dbid" ON rnacen.xref_p17_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_not_deleted$last" ON rnacen.xref_p17_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_not_deleted$taxid" ON rnacen.xref_p17_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p17_not_deleted$upi" ON rnacen.xref_p17_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_deleted$ac" ON rnacen.xref_p18_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_deleted$created" ON rnacen.xref_p18_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_deleted$dbid" ON rnacen.xref_p18_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_deleted$last" ON rnacen.xref_p18_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_deleted$taxid" ON rnacen.xref_p18_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_deleted$upi" ON rnacen.xref_p18_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_not_deleted$ac" ON rnacen.xref_p18_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_not_deleted$created" ON rnacen.xref_p18_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_not_deleted$dbid" ON rnacen.xref_p18_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_not_deleted$last" ON rnacen.xref_p18_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_not_deleted$taxid" ON rnacen.xref_p18_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p18_not_deleted$upi" ON rnacen.xref_p18_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_deleted$ac" ON rnacen.xref_p19_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_deleted$created" ON rnacen.xref_p19_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_deleted$dbid" ON rnacen.xref_p19_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_deleted$last" ON rnacen.xref_p19_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_deleted$taxid" ON rnacen.xref_p19_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_deleted$upi" ON rnacen.xref_p19_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_not_deleted$ac" ON rnacen.xref_p19_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_not_deleted$created" ON rnacen.xref_p19_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_not_deleted$dbid" ON rnacen.xref_p19_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_not_deleted$last" ON rnacen.xref_p19_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_not_deleted$taxid" ON rnacen.xref_p19_not_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p19_not_deleted$upi" ON rnacen.xref_p19_not_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_deleted$ac" ON rnacen.xref_p1_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_deleted$created" ON rnacen.xref_p1_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_deleted$dbid" ON rnacen.xref_p1_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_deleted$last" ON rnacen.xref_p1_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_deleted$taxid" ON rnacen.xref_p1_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_deleted$upi" ON rnacen.xref_p1_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_not_deleted$ac" ON rnacen.xref_p1_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_not_deleted$created" ON rnacen.xref_p1_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_not_deleted$dbid" ON rnacen.xref_p1_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_not_deleted$last" ON rnacen.xref_p1_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_not_deleted$taxid" ON rnacen.xref_p1_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p1_not_deleted$upi" ON rnacen.xref_p1_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_deleted$ac" ON rnacen.xref_p20_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_deleted$created" ON rnacen.xref_p20_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_deleted$dbid" ON rnacen.xref_p20_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_deleted$last" ON rnacen.xref_p20_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_deleted$taxid" ON rnacen.xref_p20_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_deleted$upi" ON rnacen.xref_p20_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_not_deleted$ac" ON rnacen.xref_p20_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_not_deleted$created" ON rnacen.xref_p20_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_not_deleted$dbid" ON rnacen.xref_p20_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_not_deleted$last" ON rnacen.xref_p20_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_not_deleted$taxid" ON rnacen.xref_p20_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p20_not_deleted$upi" ON rnacen.xref_p20_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_deleted$ac" ON rnacen.xref_p21_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_deleted$created" ON rnacen.xref_p21_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_deleted$dbid" ON rnacen.xref_p21_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_deleted$last" ON rnacen.xref_p21_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_deleted$taxid" ON rnacen.xref_p21_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_deleted$upi" ON rnacen.xref_p21_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_not_deleted$ac" ON rnacen.xref_p21_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_not_deleted$created" ON rnacen.xref_p21_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_not_deleted$dbid" ON rnacen.xref_p21_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_not_deleted$last" ON rnacen.xref_p21_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_not_deleted$taxid" ON rnacen.xref_p21_not_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p21_not_deleted$upi" ON rnacen.xref_p21_not_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_deleted$ac" ON rnacen.xref_p22_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_deleted$created" ON rnacen.xref_p22_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_deleted$dbid" ON rnacen.xref_p22_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_deleted$last" ON rnacen.xref_p22_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_deleted$taxid" ON rnacen.xref_p22_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_deleted$upi" ON rnacen.xref_p22_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_not_deleted$ac" ON rnacen.xref_p22_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_not_deleted$created" ON rnacen.xref_p22_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_not_deleted$dbid" ON rnacen.xref_p22_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_not_deleted$last" ON rnacen.xref_p22_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_not_deleted$taxid" ON rnacen.xref_p22_not_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p22_not_deleted$upi" ON rnacen.xref_p22_not_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_deleted$ac" ON rnacen.xref_p23_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_deleted$created" ON rnacen.xref_p23_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_deleted$dbid" ON rnacen.xref_p23_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_deleted$last" ON rnacen.xref_p23_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_deleted$taxid" ON rnacen.xref_p23_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_deleted$upi" ON rnacen.xref_p23_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_not_deleted$ac" ON rnacen.xref_p23_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_not_deleted$created" ON rnacen.xref_p23_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_not_deleted$dbid" ON rnacen.xref_p23_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_not_deleted$last" ON rnacen.xref_p23_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_not_deleted$taxid" ON rnacen.xref_p23_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p23_not_deleted$upi" ON rnacen.xref_p23_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_deleted$ac" ON rnacen.xref_p24_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_deleted$created" ON rnacen.xref_p24_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_deleted$dbid" ON rnacen.xref_p24_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_deleted$last" ON rnacen.xref_p24_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_deleted$taxid" ON rnacen.xref_p24_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_deleted$upi" ON rnacen.xref_p24_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_not_deleted$ac" ON rnacen.xref_p24_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_not_deleted$created" ON rnacen.xref_p24_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_not_deleted$dbid" ON rnacen.xref_p24_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_not_deleted$last" ON rnacen.xref_p24_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_not_deleted$taxid" ON rnacen.xref_p24_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p24_not_deleted$upi" ON rnacen.xref_p24_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_deleted$ac" ON rnacen.xref_p25_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_deleted$created" ON rnacen.xref_p25_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_deleted$dbid" ON rnacen.xref_p25_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_deleted$last" ON rnacen.xref_p25_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_deleted$taxid" ON rnacen.xref_p25_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_deleted$upi" ON rnacen.xref_p25_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_not_deleted$ac" ON rnacen.xref_p25_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_not_deleted$created" ON rnacen.xref_p25_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_not_deleted$dbid" ON rnacen.xref_p25_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_not_deleted$last" ON rnacen.xref_p25_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_not_deleted$taxid" ON rnacen.xref_p25_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p25_not_deleted$upi" ON rnacen.xref_p25_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_deleted$ac" ON rnacen.xref_p26_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_deleted$created" ON rnacen.xref_p26_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_deleted$dbid" ON rnacen.xref_p26_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_deleted$last" ON rnacen.xref_p26_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_deleted$taxid" ON rnacen.xref_p26_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_deleted$upi" ON rnacen.xref_p26_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_not_deleted$ac" ON rnacen.xref_p26_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_not_deleted$created" ON rnacen.xref_p26_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_not_deleted$dbid" ON rnacen.xref_p26_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_not_deleted$last" ON rnacen.xref_p26_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_not_deleted$taxid" ON rnacen.xref_p26_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p26_not_deleted$upi" ON rnacen.xref_p26_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_deleted$ac" ON rnacen.xref_p27_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_deleted$created" ON rnacen.xref_p27_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_deleted$dbid" ON rnacen.xref_p27_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_deleted$last" ON rnacen.xref_p27_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_deleted$taxid" ON rnacen.xref_p27_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_deleted$upi" ON rnacen.xref_p27_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_not_deleted$ac" ON rnacen.xref_p27_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_not_deleted$created" ON rnacen.xref_p27_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_not_deleted$dbid" ON rnacen.xref_p27_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_not_deleted$last" ON rnacen.xref_p27_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_not_deleted$taxid" ON rnacen.xref_p27_not_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p27_not_deleted$upi" ON rnacen.xref_p27_not_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_deleted$ac" ON rnacen.xref_p28_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_deleted$created" ON rnacen.xref_p28_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_deleted$dbid" ON rnacen.xref_p28_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_deleted$last" ON rnacen.xref_p28_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_deleted$taxid" ON rnacen.xref_p28_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_deleted$upi" ON rnacen.xref_p28_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_not_deleted$ac" ON rnacen.xref_p28_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_not_deleted$created" ON rnacen.xref_p28_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_not_deleted$dbid" ON rnacen.xref_p28_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_not_deleted$last" ON rnacen.xref_p28_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_not_deleted$taxid" ON rnacen.xref_p28_not_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p28_not_deleted$upi" ON rnacen.xref_p28_not_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_deleted$ac" ON rnacen.xref_p29_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_deleted$created" ON rnacen.xref_p29_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_deleted$dbid" ON rnacen.xref_p29_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_deleted$last" ON rnacen.xref_p29_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_deleted$taxid" ON rnacen.xref_p29_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_deleted$upi" ON rnacen.xref_p29_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_not_deleted$ac" ON rnacen.xref_p29_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_not_deleted$created" ON rnacen.xref_p29_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_not_deleted$dbid" ON rnacen.xref_p29_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_not_deleted$last" ON rnacen.xref_p29_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_not_deleted$taxid" ON rnacen.xref_p29_not_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p29_not_deleted$upi" ON rnacen.xref_p29_not_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_deleted$ac" ON rnacen.xref_p2_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_deleted$created" ON rnacen.xref_p2_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_deleted$dbid" ON rnacen.xref_p2_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_deleted$last" ON rnacen.xref_p2_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_deleted$taxid" ON rnacen.xref_p2_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_deleted$upi" ON rnacen.xref_p2_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_not_deleted$ac" ON rnacen.xref_p2_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_not_deleted$created" ON rnacen.xref_p2_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_not_deleted$dbid" ON rnacen.xref_p2_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_not_deleted$last" ON rnacen.xref_p2_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_not_deleted$taxid" ON rnacen.xref_p2_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p2_not_deleted$upi" ON rnacen.xref_p2_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_deleted$ac" ON rnacen.xref_p30_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_deleted$created" ON rnacen.xref_p30_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_deleted$dbid" ON rnacen.xref_p30_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_deleted$last" ON rnacen.xref_p30_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_deleted$taxid" ON rnacen.xref_p30_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_deleted$upi" ON rnacen.xref_p30_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_not_deleted$ac" ON rnacen.xref_p30_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_not_deleted$created" ON rnacen.xref_p30_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_not_deleted$dbid" ON rnacen.xref_p30_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_not_deleted$last" ON rnacen.xref_p30_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_not_deleted$taxid" ON rnacen.xref_p30_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p30_not_deleted$upi" ON rnacen.xref_p30_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_deleted$ac" ON rnacen.xref_p31_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_deleted$created" ON rnacen.xref_p31_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_deleted$dbid" ON rnacen.xref_p31_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_deleted$last" ON rnacen.xref_p31_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_deleted$taxid" ON rnacen.xref_p31_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_deleted$upi" ON rnacen.xref_p31_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_not_deleted$ac" ON rnacen.xref_p31_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_not_deleted$created" ON rnacen.xref_p31_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_not_deleted$dbid" ON rnacen.xref_p31_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_not_deleted$last" ON rnacen.xref_p31_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_not_deleted$taxid" ON rnacen.xref_p31_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p31_not_deleted$upi" ON rnacen.xref_p31_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_deleted$ac" ON rnacen.xref_p32_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_deleted$created" ON rnacen.xref_p32_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_deleted$dbid" ON rnacen.xref_p32_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_deleted$last" ON rnacen.xref_p32_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_deleted$taxid" ON rnacen.xref_p32_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_deleted$upi" ON rnacen.xref_p32_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_not_deleted$ac" ON rnacen.xref_p32_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_not_deleted$created" ON rnacen.xref_p32_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_not_deleted$dbid" ON rnacen.xref_p32_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_not_deleted$last" ON rnacen.xref_p32_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_not_deleted$taxid" ON rnacen.xref_p32_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p32_not_deleted$upi" ON rnacen.xref_p32_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_deleted$ac" ON rnacen.xref_p33_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_deleted$created" ON rnacen.xref_p33_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_deleted$dbid" ON rnacen.xref_p33_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_deleted$last" ON rnacen.xref_p33_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_deleted$taxid" ON rnacen.xref_p33_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_deleted$upi" ON rnacen.xref_p33_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_not_deleted$ac" ON rnacen.xref_p33_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_not_deleted$created" ON rnacen.xref_p33_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_not_deleted$dbid" ON rnacen.xref_p33_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_not_deleted$last" ON rnacen.xref_p33_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_not_deleted$taxid" ON rnacen.xref_p33_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p33_not_deleted$upi" ON rnacen.xref_p33_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_deleted$ac" ON rnacen.xref_p34_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_deleted$created" ON rnacen.xref_p34_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_deleted$dbid" ON rnacen.xref_p34_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_deleted$last" ON rnacen.xref_p34_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_deleted$taxid" ON rnacen.xref_p34_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_deleted$upi" ON rnacen.xref_p34_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_not_deleted$ac" ON rnacen.xref_p34_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_not_deleted$created" ON rnacen.xref_p34_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_not_deleted$dbid" ON rnacen.xref_p34_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_not_deleted$last" ON rnacen.xref_p34_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_not_deleted$taxid" ON rnacen.xref_p34_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p34_not_deleted$upi" ON rnacen.xref_p34_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_deleted$ac" ON rnacen.xref_p35_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_deleted$created" ON rnacen.xref_p35_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_deleted$dbid" ON rnacen.xref_p35_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_deleted$last" ON rnacen.xref_p35_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_deleted$taxid" ON rnacen.xref_p35_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_deleted$upi" ON rnacen.xref_p35_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_not_deleted$ac" ON rnacen.xref_p35_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_not_deleted$created" ON rnacen.xref_p35_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_not_deleted$dbid" ON rnacen.xref_p35_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_not_deleted$last" ON rnacen.xref_p35_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_not_deleted$taxid" ON rnacen.xref_p35_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p35_not_deleted$upi" ON rnacen.xref_p35_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_deleted$ac" ON rnacen.xref_p36_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_deleted$created" ON rnacen.xref_p36_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_deleted$dbid" ON rnacen.xref_p36_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_deleted$last" ON rnacen.xref_p36_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_deleted$taxid" ON rnacen.xref_p36_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_deleted$upi" ON rnacen.xref_p36_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_not_deleted$ac" ON rnacen.xref_p36_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_not_deleted$created" ON rnacen.xref_p36_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_not_deleted$dbid" ON rnacen.xref_p36_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_not_deleted$last" ON rnacen.xref_p36_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_not_deleted$taxid" ON rnacen.xref_p36_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p36_not_deleted$upi" ON rnacen.xref_p36_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_deleted$ac" ON rnacen.xref_p37_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_deleted$created" ON rnacen.xref_p37_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_deleted$dbid" ON rnacen.xref_p37_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_deleted$last" ON rnacen.xref_p37_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_deleted$taxid" ON rnacen.xref_p37_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_deleted$upi" ON rnacen.xref_p37_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_not_deleted$ac" ON rnacen.xref_p37_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_not_deleted$created" ON rnacen.xref_p37_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_not_deleted$dbid" ON rnacen.xref_p37_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_not_deleted$last" ON rnacen.xref_p37_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_not_deleted$taxid" ON rnacen.xref_p37_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p37_not_deleted$upi" ON rnacen.xref_p37_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_deleted$ac" ON rnacen.xref_p38_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_deleted$created" ON rnacen.xref_p38_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_deleted$dbid" ON rnacen.xref_p38_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_deleted$last" ON rnacen.xref_p38_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_deleted$taxid" ON rnacen.xref_p38_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_deleted$upi" ON rnacen.xref_p38_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_not_deleted$ac" ON rnacen.xref_p38_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_not_deleted$created" ON rnacen.xref_p38_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_not_deleted$dbid" ON rnacen.xref_p38_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_not_deleted$last" ON rnacen.xref_p38_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_not_deleted$taxid" ON rnacen.xref_p38_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p38_not_deleted$upi" ON rnacen.xref_p38_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_deleted$ac" ON rnacen.xref_p39_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_deleted$created" ON rnacen.xref_p39_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_deleted$dbid" ON rnacen.xref_p39_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_deleted$last" ON rnacen.xref_p39_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_deleted$taxid" ON rnacen.xref_p39_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_deleted$upi" ON rnacen.xref_p39_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_not_deleted$ac" ON rnacen.xref_p39_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_not_deleted$created" ON rnacen.xref_p39_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_not_deleted$dbid" ON rnacen.xref_p39_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_not_deleted$last" ON rnacen.xref_p39_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_not_deleted$taxid" ON rnacen.xref_p39_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p39_not_deleted$upi" ON rnacen.xref_p39_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_deleted$ac" ON rnacen.xref_p3_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_deleted$created" ON rnacen.xref_p3_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_deleted$dbid" ON rnacen.xref_p3_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_deleted$last" ON rnacen.xref_p3_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_deleted$taxid" ON rnacen.xref_p3_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_deleted$upi" ON rnacen.xref_p3_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_not_deleted$ac" ON rnacen.xref_p3_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_not_deleted$created" ON rnacen.xref_p3_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_not_deleted$dbid" ON rnacen.xref_p3_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_not_deleted$last" ON rnacen.xref_p3_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_not_deleted$taxid" ON rnacen.xref_p3_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p3_not_deleted$upi" ON rnacen.xref_p3_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_deleted$ac" ON rnacen.xref_p40_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_deleted$created" ON rnacen.xref_p40_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_deleted$dbid" ON rnacen.xref_p40_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_deleted$last" ON rnacen.xref_p40_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_deleted$taxid" ON rnacen.xref_p40_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_deleted$upi" ON rnacen.xref_p40_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_not_deleted$ac" ON rnacen.xref_p40_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_not_deleted$created" ON rnacen.xref_p40_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_not_deleted$dbid" ON rnacen.xref_p40_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_not_deleted$last" ON rnacen.xref_p40_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_not_deleted$taxid" ON rnacen.xref_p40_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p40_not_deleted$upi" ON rnacen.xref_p40_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_deleted$ac" ON rnacen.xref_p41_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_deleted$created" ON rnacen.xref_p41_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_deleted$dbid" ON rnacen.xref_p41_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_deleted$last" ON rnacen.xref_p41_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_deleted$taxid" ON rnacen.xref_p41_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_deleted$upi" ON rnacen.xref_p41_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_not_deleted$ac" ON rnacen.xref_p41_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_not_deleted$created" ON rnacen.xref_p41_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_not_deleted$dbid" ON rnacen.xref_p41_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_not_deleted$last" ON rnacen.xref_p41_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_not_deleted$taxid" ON rnacen.xref_p41_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p41_not_deleted$upi" ON rnacen.xref_p41_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_deleted$ac" ON rnacen.xref_p42_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_deleted$created" ON rnacen.xref_p42_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_deleted$dbid" ON rnacen.xref_p42_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_deleted$last" ON rnacen.xref_p42_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_deleted$taxid" ON rnacen.xref_p42_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_deleted$upi" ON rnacen.xref_p42_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_not_deleted$ac" ON rnacen.xref_p42_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_not_deleted$created" ON rnacen.xref_p42_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_not_deleted$dbid" ON rnacen.xref_p42_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_not_deleted$last" ON rnacen.xref_p42_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_not_deleted$taxid" ON rnacen.xref_p42_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p42_not_deleted$upi" ON rnacen.xref_p42_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_deleted$ac" ON rnacen.xref_p43_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_deleted$created" ON rnacen.xref_p43_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_deleted$dbid" ON rnacen.xref_p43_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_deleted$last" ON rnacen.xref_p43_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_deleted$taxid" ON rnacen.xref_p43_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_deleted$upi" ON rnacen.xref_p43_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_not_deleted$ac" ON rnacen.xref_p43_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_not_deleted$created" ON rnacen.xref_p43_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_not_deleted$dbid" ON rnacen.xref_p43_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_not_deleted$last" ON rnacen.xref_p43_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_not_deleted$taxid" ON rnacen.xref_p43_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p43_not_deleted$upi" ON rnacen.xref_p43_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_deleted$ac" ON rnacen.xref_p44_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_deleted$created" ON rnacen.xref_p44_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_deleted$dbid" ON rnacen.xref_p44_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_deleted$last" ON rnacen.xref_p44_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_deleted$taxid" ON rnacen.xref_p44_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_deleted$upi" ON rnacen.xref_p44_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_not_deleted$ac" ON rnacen.xref_p44_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_not_deleted$created" ON rnacen.xref_p44_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_not_deleted$dbid" ON rnacen.xref_p44_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_not_deleted$last" ON rnacen.xref_p44_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_not_deleted$taxid" ON rnacen.xref_p44_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p44_not_deleted$upi" ON rnacen.xref_p44_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_deleted$ac" ON rnacen.xref_p45_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_deleted$created" ON rnacen.xref_p45_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_deleted$dbid" ON rnacen.xref_p45_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_deleted$last" ON rnacen.xref_p45_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_deleted$taxid" ON rnacen.xref_p45_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_deleted$upi" ON rnacen.xref_p45_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_not_deleted$ac" ON rnacen.xref_p45_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_not_deleted$created" ON rnacen.xref_p45_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_not_deleted$dbid" ON rnacen.xref_p45_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_not_deleted$last" ON rnacen.xref_p45_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_not_deleted$taxid" ON rnacen.xref_p45_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p45_not_deleted$upi" ON rnacen.xref_p45_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_deleted$ac" ON rnacen.xref_p46_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_deleted$created" ON rnacen.xref_p46_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_deleted$dbid" ON rnacen.xref_p46_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_deleted$last" ON rnacen.xref_p46_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_deleted$taxid" ON rnacen.xref_p46_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_deleted$upi" ON rnacen.xref_p46_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_not_deleted$ac" ON rnacen.xref_p46_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_not_deleted$created" ON rnacen.xref_p46_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_not_deleted$dbid" ON rnacen.xref_p46_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_not_deleted$last" ON rnacen.xref_p46_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_not_deleted$taxid" ON rnacen.xref_p46_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p46_not_deleted$upi" ON rnacen.xref_p46_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_deleted$ac" ON rnacen.xref_p47_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_deleted$created" ON rnacen.xref_p47_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_deleted$dbid" ON rnacen.xref_p47_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_deleted$last" ON rnacen.xref_p47_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_deleted$taxid" ON rnacen.xref_p47_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_deleted$upi" ON rnacen.xref_p47_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_not_deleted$ac" ON rnacen.xref_p47_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_not_deleted$created" ON rnacen.xref_p47_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_not_deleted$dbid" ON rnacen.xref_p47_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_not_deleted$last" ON rnacen.xref_p47_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_not_deleted$taxid" ON rnacen.xref_p47_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p47_not_deleted$upi" ON rnacen.xref_p47_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_deleted$ac" ON rnacen.xref_p48_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_deleted$created" ON rnacen.xref_p48_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_deleted$dbid" ON rnacen.xref_p48_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_deleted$last" ON rnacen.xref_p48_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_deleted$taxid" ON rnacen.xref_p48_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_deleted$upi" ON rnacen.xref_p48_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_not_deleted$ac" ON rnacen.xref_p48_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_not_deleted$created" ON rnacen.xref_p48_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_not_deleted$dbid" ON rnacen.xref_p48_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_not_deleted$last" ON rnacen.xref_p48_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_not_deleted$taxid" ON rnacen.xref_p48_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p48_not_deleted$upi" ON rnacen.xref_p48_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_deleted$ac" ON rnacen.xref_p49_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_deleted$created" ON rnacen.xref_p49_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_deleted$dbid" ON rnacen.xref_p49_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_deleted$last" ON rnacen.xref_p49_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_deleted$taxid" ON rnacen.xref_p49_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_deleted$upi" ON rnacen.xref_p49_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_not_deleted$ac" ON rnacen.xref_p49_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_not_deleted$created" ON rnacen.xref_p49_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_not_deleted$dbid" ON rnacen.xref_p49_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_not_deleted$last" ON rnacen.xref_p49_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_not_deleted$taxid" ON rnacen.xref_p49_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p49_not_deleted$upi" ON rnacen.xref_p49_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_deleted$ac" ON rnacen.xref_p4_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_deleted$created" ON rnacen.xref_p4_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_deleted$dbid" ON rnacen.xref_p4_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_deleted$last" ON rnacen.xref_p4_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_deleted$taxid" ON rnacen.xref_p4_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_deleted$upi" ON rnacen.xref_p4_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_not_deleted$ac" ON rnacen.xref_p4_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_not_deleted$created" ON rnacen.xref_p4_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_not_deleted$dbid" ON rnacen.xref_p4_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_not_deleted$last" ON rnacen.xref_p4_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_not_deleted$taxid" ON rnacen.xref_p4_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p4_not_deleted$upi" ON rnacen.xref_p4_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_deleted$ac" ON rnacen.xref_p50_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_deleted$created" ON rnacen.xref_p50_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_deleted$dbid" ON rnacen.xref_p50_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_deleted$last" ON rnacen.xref_p50_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_deleted$taxid" ON rnacen.xref_p50_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_deleted$upi" ON rnacen.xref_p50_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_not_deleted$ac" ON rnacen.xref_p50_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_not_deleted$created" ON rnacen.xref_p50_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_not_deleted$dbid" ON rnacen.xref_p50_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_not_deleted$last" ON rnacen.xref_p50_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_not_deleted$taxid" ON rnacen.xref_p50_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p50_not_deleted$upi" ON rnacen.xref_p50_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_deleted$ac" ON rnacen.xref_p51_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_deleted$created" ON rnacen.xref_p51_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_deleted$dbid" ON rnacen.xref_p51_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_deleted$last" ON rnacen.xref_p51_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_deleted$taxid" ON rnacen.xref_p51_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_deleted$upi" ON rnacen.xref_p51_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_not_deleted$ac" ON rnacen.xref_p51_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_not_deleted$created" ON rnacen.xref_p51_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_not_deleted$dbid" ON rnacen.xref_p51_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_not_deleted$last" ON rnacen.xref_p51_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_not_deleted$taxid" ON rnacen.xref_p51_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p51_not_deleted$upi" ON rnacen.xref_p51_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_deleted$ac" ON rnacen.xref_p52_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_deleted$created" ON rnacen.xref_p52_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_deleted$dbid" ON rnacen.xref_p52_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_deleted$last" ON rnacen.xref_p52_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_deleted$taxid" ON rnacen.xref_p52_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_deleted$upi" ON rnacen.xref_p52_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_not_deleted$ac" ON rnacen.xref_p52_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_not_deleted$created" ON rnacen.xref_p52_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_not_deleted$dbid" ON rnacen.xref_p52_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_not_deleted$last" ON rnacen.xref_p52_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_not_deleted$taxid" ON rnacen.xref_p52_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p52_not_deleted$upi" ON rnacen.xref_p52_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_deleted$ac" ON rnacen.xref_p53_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_deleted$created" ON rnacen.xref_p53_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_deleted$dbid" ON rnacen.xref_p53_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_deleted$last" ON rnacen.xref_p53_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_deleted$taxid" ON rnacen.xref_p53_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_deleted$upi" ON rnacen.xref_p53_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_not_deleted$ac" ON rnacen.xref_p53_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_not_deleted$created" ON rnacen.xref_p53_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_not_deleted$dbid" ON rnacen.xref_p53_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_not_deleted$last" ON rnacen.xref_p53_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_not_deleted$taxid" ON rnacen.xref_p53_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p53_not_deleted$upi" ON rnacen.xref_p53_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_deleted$ac" ON rnacen.xref_p54_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_deleted$created" ON rnacen.xref_p54_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_deleted$dbid" ON rnacen.xref_p54_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_deleted$last" ON rnacen.xref_p54_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_deleted$taxid" ON rnacen.xref_p54_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_deleted$upi" ON rnacen.xref_p54_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_not_deleted$ac" ON rnacen.xref_p54_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_not_deleted$created" ON rnacen.xref_p54_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_not_deleted$dbid" ON rnacen.xref_p54_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_not_deleted$last" ON rnacen.xref_p54_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_not_deleted$taxid" ON rnacen.xref_p54_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p54_not_deleted$upi" ON rnacen.xref_p54_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_deleted$ac" ON rnacen.xref_p55_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_deleted$created" ON rnacen.xref_p55_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_deleted$dbid" ON rnacen.xref_p55_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_deleted$last" ON rnacen.xref_p55_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_deleted$taxid" ON rnacen.xref_p55_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_deleted$upi" ON rnacen.xref_p55_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_not_deleted$ac" ON rnacen.xref_p55_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_not_deleted$created" ON rnacen.xref_p55_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_not_deleted$dbid" ON rnacen.xref_p55_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_not_deleted$last" ON rnacen.xref_p55_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_not_deleted$taxid" ON rnacen.xref_p55_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p55_not_deleted$upi" ON rnacen.xref_p55_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_deleted$ac" ON rnacen.xref_p5_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_deleted$created" ON rnacen.xref_p5_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_deleted$dbid" ON rnacen.xref_p5_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_deleted$last" ON rnacen.xref_p5_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_deleted$taxid" ON rnacen.xref_p5_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_deleted$upi" ON rnacen.xref_p5_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_not_deleted$ac" ON rnacen.xref_p5_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_not_deleted$created" ON rnacen.xref_p5_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_not_deleted$dbid" ON rnacen.xref_p5_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_not_deleted$last" ON rnacen.xref_p5_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_not_deleted$taxid" ON rnacen.xref_p5_not_deleted USING btree (taxid);

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p5_not_deleted$upi" ON rnacen.xref_p5_not_deleted USING btree (upi);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_deleted$ac" ON rnacen.xref_p6_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_deleted$created" ON rnacen.xref_p6_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_deleted$dbid" ON rnacen.xref_p6_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_deleted$last" ON rnacen.xref_p6_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_deleted$taxid" ON rnacen.xref_p6_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_deleted$upi" ON rnacen.xref_p6_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_not_deleted$ac" ON rnacen.xref_p6_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_not_deleted$created" ON rnacen.xref_p6_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_not_deleted$dbid" ON rnacen.xref_p6_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_not_deleted$last" ON rnacen.xref_p6_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_not_deleted$taxid" ON rnacen.xref_p6_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p6_not_deleted$upi" ON rnacen.xref_p6_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_deleted$ac" ON rnacen.xref_p7_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_deleted$created" ON rnacen.xref_p7_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_deleted$dbid" ON rnacen.xref_p7_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_deleted$last" ON rnacen.xref_p7_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_deleted$taxid" ON rnacen.xref_p7_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_deleted$upi" ON rnacen.xref_p7_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_not_deleted$ac" ON rnacen.xref_p7_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_not_deleted$created" ON rnacen.xref_p7_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_not_deleted$dbid" ON rnacen.xref_p7_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_not_deleted$last" ON rnacen.xref_p7_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_not_deleted$taxid" ON rnacen.xref_p7_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p7_not_deleted$upi" ON rnacen.xref_p7_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_deleted$ac" ON rnacen.xref_p8_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_deleted$created" ON rnacen.xref_p8_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_deleted$dbid" ON rnacen.xref_p8_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_deleted$last" ON rnacen.xref_p8_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_deleted$taxid" ON rnacen.xref_p8_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_deleted$upi" ON rnacen.xref_p8_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_not_deleted$ac" ON rnacen.xref_p8_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_not_deleted$created" ON rnacen.xref_p8_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_not_deleted$dbid" ON rnacen.xref_p8_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_not_deleted$last" ON rnacen.xref_p8_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_not_deleted$taxid" ON rnacen.xref_p8_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p8_not_deleted$upi" ON rnacen.xref_p8_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_deleted$ac" ON rnacen.xref_p9_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_deleted$created" ON rnacen.xref_p9_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_deleted$dbid" ON rnacen.xref_p9_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_deleted$last" ON rnacen.xref_p9_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_deleted$taxid" ON rnacen.xref_p9_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_deleted$upi" ON rnacen.xref_p9_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_not_deleted$ac" ON rnacen.xref_p9_not_deleted USING btree (ac ASC);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_not_deleted$created" ON rnacen.xref_p9_not_deleted USING btree (created ASC);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_not_deleted$dbid" ON rnacen.xref_p9_not_deleted USING btree (dbid ASC);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_not_deleted$last" ON rnacen.xref_p9_not_deleted USING btree (last ASC);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_not_deleted$taxid" ON rnacen.xref_p9_not_deleted USING btree (taxid ASC);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX "xref_p9_not_deleted$upi" ON rnacen.xref_p9_not_deleted USING btree (upi ASC);

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p10_deleted$id" ON rnacen.xref_p10_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p10_not_deleted$id" ON rnacen.xref_p10_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p11_deleted$id" ON rnacen.xref_p11_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p11_not_deleted$id" ON rnacen.xref_p11_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p12_deleted$id" ON rnacen.xref_p12_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p12_not_deleted$id" ON rnacen.xref_p12_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p13_deleted$id" ON rnacen.xref_p13_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p13_not_deleted$id" ON rnacen.xref_p13_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p14_deleted$id" ON rnacen.xref_p14_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p14_not_deleted$id" ON rnacen.xref_p14_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p15_deleted$id" ON rnacen.xref_p15_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p15_not_deleted$id" ON rnacen.xref_p15_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p16_deleted$id" ON rnacen.xref_p16_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p16_not_deleted$id" ON rnacen.xref_p16_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p17_deleted$id" ON rnacen.xref_p17_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p17_not_deleted$id" ON rnacen.xref_p17_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p18_deleted$id" ON rnacen.xref_p18_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p18_not_deleted$id" ON rnacen.xref_p18_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p19_deleted$id" ON rnacen.xref_p19_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p19_not_deleted$id" ON rnacen.xref_p19_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p1_deleted$id" ON rnacen.xref_p1_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p1_not_deleted$id" ON rnacen.xref_p1_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p20_deleted$id" ON rnacen.xref_p20_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p20_not_deleted$id" ON rnacen.xref_p20_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p22_deleted$id" ON rnacen.xref_p22_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p22_not_deleted$id" ON rnacen.xref_p22_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p23_deleted$id" ON rnacen.xref_p23_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p23_not_deleted$id" ON rnacen.xref_p23_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p24_deleted$id" ON rnacen.xref_p24_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p24_not_deleted$id" ON rnacen.xref_p24_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p25_deleted$id" ON rnacen.xref_p25_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p25_not_deleted$id" ON rnacen.xref_p25_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p26_deleted$id" ON rnacen.xref_p26_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p26_not_deleted$id" ON rnacen.xref_p26_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p27_deleted$id" ON rnacen.xref_p27_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p27_not_deleted$id" ON rnacen.xref_p27_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p28_deleted$id" ON rnacen.xref_p28_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p28_not_deleted$id" ON rnacen.xref_p28_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p29_deleted$id" ON rnacen.xref_p29_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p29_not_deleted$id" ON rnacen.xref_p29_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p2_deleted$id" ON rnacen.xref_p2_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p2_not_deleted$id" ON rnacen.xref_p2_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p30_deleted$id" ON rnacen.xref_p30_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p30_not_deleted$id" ON rnacen.xref_p30_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p31_deleted$id" ON rnacen.xref_p31_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p31_not_deleted$id" ON rnacen.xref_p31_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p32_deleted$id" ON rnacen.xref_p32_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p32_not_deleted$id" ON rnacen.xref_p32_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p33_deleted$id" ON rnacen.xref_p33_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p33_not_deleted$id" ON rnacen.xref_p33_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p34_deleted$id" ON rnacen.xref_p34_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p34_not_deleted$id" ON rnacen.xref_p34_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p35_deleted$id" ON rnacen.xref_p35_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p35_not_deleted$id" ON rnacen.xref_p35_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p36_deleted$id" ON rnacen.xref_p36_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p36_not_deleted$id" ON rnacen.xref_p36_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p37_deleted$id" ON rnacen.xref_p37_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p37_not_deleted$id" ON rnacen.xref_p37_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p38_deleted$id" ON rnacen.xref_p38_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p38_not_deleted$id" ON rnacen.xref_p38_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p39_deleted$id" ON rnacen.xref_p39_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p39_not_deleted$id" ON rnacen.xref_p39_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p3_deleted$id" ON rnacen.xref_p3_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p3_not_deleted$id" ON rnacen.xref_p3_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p40_deleted$id" ON rnacen.xref_p40_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p40_not_deleted$id" ON rnacen.xref_p40_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p41_deleted$id" ON rnacen.xref_p41_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p41_not_deleted$id" ON rnacen.xref_p41_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p42_deleted$id" ON rnacen.xref_p42_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p42_not_deleted$id" ON rnacen.xref_p42_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p43_deleted$id" ON rnacen.xref_p43_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p43_not_deleted$id" ON rnacen.xref_p43_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p44_deleted$id" ON rnacen.xref_p44_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p44_not_deleted$id" ON rnacen.xref_p44_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p45_deleted$id" ON rnacen.xref_p45_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p45_not_deleted$id" ON rnacen.xref_p45_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p46_deleted$id" ON rnacen.xref_p46_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p46_not_deleted$id" ON rnacen.xref_p46_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p47_deleted$id" ON rnacen.xref_p47_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p47_not_deleted$id" ON rnacen.xref_p47_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p48_deleted$id" ON rnacen.xref_p48_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p48_not_deleted$id" ON rnacen.xref_p48_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p49_deleted$id" ON rnacen.xref_p49_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p49_not_deleted$id" ON rnacen.xref_p49_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p4_deleted$id" ON rnacen.xref_p4_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p4_not_deleted$id" ON rnacen.xref_p4_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p50_deleted$id" ON rnacen.xref_p50_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p50_not_deleted$id" ON rnacen.xref_p50_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p51_deleted$id" ON rnacen.xref_p51_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p51_not_deleted$id" ON rnacen.xref_p51_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p52_deleted$id" ON rnacen.xref_p52_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p52_not_deleted$id" ON rnacen.xref_p52_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p53_deleted$id" ON rnacen.xref_p53_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p53_not_deleted$id" ON rnacen.xref_p53_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p54_deleted$id" ON rnacen.xref_p54_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p54_not_deleted$id" ON rnacen.xref_p54_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p55_deleted$id" ON rnacen.xref_p55_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p55_not_deleted$id" ON rnacen.xref_p55_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p5_deleted$id" ON rnacen.xref_p5_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p5_not_deleted$id" ON rnacen.xref_p5_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p6_deleted$id" ON rnacen.xref_p6_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p6_not_deleted$id" ON rnacen.xref_p6_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p7_deleted$id" ON rnacen.xref_p7_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p7_not_deleted$id" ON rnacen.xref_p7_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p8_deleted$id" ON rnacen.xref_p8_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p8_not_deleted$id" ON rnacen.xref_p8_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p9_deleted$id" ON rnacen.xref_p9_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE UNIQUE INDEX "xref_p9_not_deleted$id" ON rnacen.xref_p9_not_deleted USING btree (id ASC);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/sequences/sequence.sql
*/
ALTER SEQUENCE rnacen.xref_p54_deleted_id_seq OWNED BY rnacen.xref_p54_deleted.id;

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/sequences/sequence.sql
*/
ALTER SEQUENCE rnacen.xref_p54_not_deleted_id_seq OWNED BY rnacen.xref_p54_not_deleted.id;

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_deleted ADD CONSTRAINT xref_p10_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_deleted ADD CONSTRAINT xref_p10_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_deleted ADD CONSTRAINT xref_p10_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p10_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_deleted ADD CONSTRAINT xref_p10_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_not_deleted ADD CONSTRAINT xref_p10_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_not_deleted ADD CONSTRAINT xref_p10_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_not_deleted ADD CONSTRAINT xref_p10_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p10_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p10_not_deleted ADD CONSTRAINT xref_p10_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p11_deleted ADD CONSTRAINT xref_p11_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p11_deleted ADD CONSTRAINT xref_p11_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p11_deleted ADD CONSTRAINT xref_p11_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p11_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p11_deleted ADD CONSTRAINT xref_p11_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p11_not_deleted ADD CONSTRAINT xref_p11_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p11_not_deleted ADD CONSTRAINT xref_p11_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p11_not_deleted ADD CONSTRAINT xref_p11_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p11_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p11_not_deleted ADD CONSTRAINT xref_p11_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p12_deleted ADD CONSTRAINT xref_p12_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p12_deleted ADD CONSTRAINT xref_p12_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p12_deleted ADD CONSTRAINT xref_p12_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p12_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p12_deleted ADD CONSTRAINT xref_p12_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p12_not_deleted ADD CONSTRAINT xref_p12_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p12_not_deleted ADD CONSTRAINT xref_p12_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p12_not_deleted ADD CONSTRAINT xref_p12_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p12_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p12_not_deleted ADD CONSTRAINT xref_p12_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_deleted ADD CONSTRAINT xref_p13_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_deleted ADD CONSTRAINT xref_p13_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_deleted ADD CONSTRAINT xref_p13_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p13_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_deleted ADD CONSTRAINT xref_p13_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_not_deleted ADD CONSTRAINT xref_p13_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_not_deleted ADD CONSTRAINT xref_p13_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_not_deleted ADD CONSTRAINT xref_p13_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p13_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p13_not_deleted ADD CONSTRAINT xref_p13_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p14_deleted ADD CONSTRAINT xref_p14_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p14_deleted ADD CONSTRAINT xref_p14_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p14_deleted ADD CONSTRAINT xref_p14_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p14_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p14_deleted ADD CONSTRAINT xref_p14_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p14_not_deleted ADD CONSTRAINT xref_p14_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p14_not_deleted ADD CONSTRAINT xref_p14_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p14_not_deleted ADD CONSTRAINT xref_p14_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p14_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p14_not_deleted ADD CONSTRAINT xref_p14_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p15_deleted ADD CONSTRAINT xref_p15_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p15_deleted ADD CONSTRAINT xref_p15_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p15_deleted ADD CONSTRAINT xref_p15_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p15_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p15_deleted ADD CONSTRAINT xref_p15_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p15_not_deleted ADD CONSTRAINT xref_p15_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p15_not_deleted ADD CONSTRAINT xref_p15_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p15_not_deleted ADD CONSTRAINT xref_p15_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p15_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p15_not_deleted ADD CONSTRAINT xref_p15_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p16_deleted ADD CONSTRAINT xref_p16_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p16_deleted ADD CONSTRAINT xref_p16_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p16_deleted ADD CONSTRAINT xref_p16_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p16_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p16_deleted ADD CONSTRAINT xref_p16_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p16_not_deleted ADD CONSTRAINT xref_p16_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p16_not_deleted ADD CONSTRAINT xref_p16_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p16_not_deleted ADD CONSTRAINT xref_p16_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p16_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p16_not_deleted ADD CONSTRAINT xref_p16_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p17_deleted ADD CONSTRAINT xref_p17_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p17_deleted ADD CONSTRAINT xref_p17_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p17_deleted ADD CONSTRAINT xref_p17_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p17_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p17_deleted ADD CONSTRAINT xref_p17_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p17_not_deleted ADD CONSTRAINT xref_p17_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p17_not_deleted ADD CONSTRAINT xref_p17_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p17_not_deleted ADD CONSTRAINT xref_p17_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p17_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p17_not_deleted ADD CONSTRAINT xref_p17_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p18_deleted ADD CONSTRAINT xref_p18_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p18_deleted ADD CONSTRAINT xref_p18_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p18_deleted ADD CONSTRAINT xref_p18_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p18_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p18_deleted ADD CONSTRAINT xref_p18_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p18_not_deleted ADD CONSTRAINT xref_p18_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p18_not_deleted ADD CONSTRAINT xref_p18_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p18_not_deleted ADD CONSTRAINT xref_p18_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p18_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p18_not_deleted ADD CONSTRAINT xref_p18_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_deleted ADD CONSTRAINT xref_p19_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_deleted ADD CONSTRAINT xref_p19_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_deleted ADD CONSTRAINT xref_p19_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p19_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_deleted ADD CONSTRAINT xref_p19_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_not_deleted ADD CONSTRAINT xref_p19_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_not_deleted ADD CONSTRAINT xref_p19_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_not_deleted ADD CONSTRAINT xref_p19_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p19_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p19_not_deleted ADD CONSTRAINT xref_p19_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p1_deleted ADD CONSTRAINT xref_p1_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p1_deleted ADD CONSTRAINT xref_p1_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p1_deleted ADD CONSTRAINT xref_p1_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p1_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p1_deleted ADD CONSTRAINT xref_p1_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p1_not_deleted ADD CONSTRAINT xref_p1_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p1_not_deleted ADD CONSTRAINT xref_p1_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p1_not_deleted ADD CONSTRAINT xref_p1_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p1_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p1_not_deleted ADD CONSTRAINT xref_p1_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p20_deleted ADD CONSTRAINT xref_p20_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p20_deleted ADD CONSTRAINT xref_p20_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p20_deleted ADD CONSTRAINT xref_p20_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p20_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p20_deleted ADD CONSTRAINT xref_p20_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p20_not_deleted ADD CONSTRAINT xref_p20_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p20_not_deleted ADD CONSTRAINT xref_p20_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p20_not_deleted ADD CONSTRAINT xref_p20_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p20_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p20_not_deleted ADD CONSTRAINT xref_p20_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_deleted ADD CONSTRAINT xref_p21_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_deleted ADD CONSTRAINT xref_p21_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_deleted ADD CONSTRAINT xref_p21_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p21_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_deleted ADD CONSTRAINT xref_p21_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_not_deleted ADD CONSTRAINT xref_p21_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_not_deleted ADD CONSTRAINT xref_p21_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_not_deleted ADD CONSTRAINT xref_p21_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p21_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p21_not_deleted ADD CONSTRAINT xref_p21_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_deleted ADD CONSTRAINT xref_p22_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_deleted ADD CONSTRAINT xref_p22_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_deleted ADD CONSTRAINT xref_p22_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p22_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_deleted ADD CONSTRAINT xref_p22_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_not_deleted ADD CONSTRAINT xref_p22_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_not_deleted ADD CONSTRAINT xref_p22_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_not_deleted ADD CONSTRAINT xref_p22_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p22_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p22_not_deleted ADD CONSTRAINT xref_p22_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p23_deleted ADD CONSTRAINT xref_p23_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p23_deleted ADD CONSTRAINT xref_p23_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p23_deleted ADD CONSTRAINT xref_p23_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p23_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p23_deleted ADD CONSTRAINT xref_p23_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p23_not_deleted ADD CONSTRAINT xref_p23_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p23_not_deleted ADD CONSTRAINT xref_p23_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p23_not_deleted ADD CONSTRAINT xref_p23_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p23_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p23_not_deleted ADD CONSTRAINT xref_p23_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p24_deleted ADD CONSTRAINT xref_p24_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p24_deleted ADD CONSTRAINT xref_p24_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p24_deleted ADD CONSTRAINT xref_p24_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p24_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p24_deleted ADD CONSTRAINT xref_p24_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p24_not_deleted ADD CONSTRAINT xref_p24_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p24_not_deleted ADD CONSTRAINT xref_p24_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p24_not_deleted ADD CONSTRAINT xref_p24_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p24_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p24_not_deleted ADD CONSTRAINT xref_p24_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p25_deleted ADD CONSTRAINT xref_p25_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p25_deleted ADD CONSTRAINT xref_p25_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p25_deleted ADD CONSTRAINT xref_p25_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p25_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p25_deleted ADD CONSTRAINT xref_p25_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p25_not_deleted ADD CONSTRAINT xref_p25_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p25_not_deleted ADD CONSTRAINT xref_p25_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p25_not_deleted ADD CONSTRAINT xref_p25_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p25_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p25_not_deleted ADD CONSTRAINT xref_p25_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p26_deleted ADD CONSTRAINT xref_p26_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p26_deleted ADD CONSTRAINT xref_p26_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p26_deleted ADD CONSTRAINT xref_p26_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p26_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p26_deleted ADD CONSTRAINT xref_p26_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p26_not_deleted ADD CONSTRAINT xref_p26_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p26_not_deleted ADD CONSTRAINT xref_p26_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p26_not_deleted ADD CONSTRAINT xref_p26_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p26_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p26_not_deleted ADD CONSTRAINT xref_p26_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p27_deleted ADD CONSTRAINT xref_p27_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p27_deleted ADD CONSTRAINT xref_p27_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p27_deleted ADD CONSTRAINT xref_p27_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p27_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p27_deleted ADD CONSTRAINT xref_p27_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p27_not_deleted ADD CONSTRAINT xref_p27_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p27_not_deleted ADD CONSTRAINT xref_p27_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p27_not_deleted ADD CONSTRAINT xref_p27_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p27_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p27_not_deleted ADD CONSTRAINT xref_p27_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p28_deleted ADD CONSTRAINT xref_p28_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p28_deleted ADD CONSTRAINT xref_p28_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p28_deleted ADD CONSTRAINT xref_p28_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p28_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p28_deleted ADD CONSTRAINT xref_p28_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p28_not_deleted ADD CONSTRAINT xref_p28_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p28_not_deleted ADD CONSTRAINT xref_p28_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p28_not_deleted ADD CONSTRAINT xref_p28_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p28_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p28_not_deleted ADD CONSTRAINT xref_p28_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p29_deleted ADD CONSTRAINT xref_p29_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p29_deleted ADD CONSTRAINT xref_p29_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p29_deleted ADD CONSTRAINT xref_p29_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p29_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p29_deleted ADD CONSTRAINT xref_p29_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p29_not_deleted ADD CONSTRAINT xref_p29_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p29_not_deleted ADD CONSTRAINT xref_p29_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p29_not_deleted ADD CONSTRAINT xref_p29_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p29_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p29_not_deleted ADD CONSTRAINT xref_p29_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p2_deleted ADD CONSTRAINT xref_p2_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p2_deleted ADD CONSTRAINT xref_p2_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p2_deleted ADD CONSTRAINT xref_p2_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p2_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p2_deleted ADD CONSTRAINT xref_p2_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p2_not_deleted ADD CONSTRAINT xref_p2_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p2_not_deleted ADD CONSTRAINT xref_p2_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p2_not_deleted ADD CONSTRAINT xref_p2_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p2_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p2_not_deleted ADD CONSTRAINT xref_p2_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p30_deleted ADD CONSTRAINT xref_p30_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p30_deleted ADD CONSTRAINT xref_p30_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p30_deleted ADD CONSTRAINT xref_p30_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p30_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p30_deleted ADD CONSTRAINT xref_p30_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p30_not_deleted ADD CONSTRAINT xref_p30_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p30_not_deleted ADD CONSTRAINT xref_p30_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p30_not_deleted ADD CONSTRAINT xref_p30_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p30_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p30_not_deleted ADD CONSTRAINT xref_p30_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p31_deleted ADD CONSTRAINT xref_p31_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p31_deleted ADD CONSTRAINT xref_p31_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p31_deleted ADD CONSTRAINT xref_p31_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p31_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p31_deleted ADD CONSTRAINT xref_p31_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p31_not_deleted ADD CONSTRAINT xref_p31_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p31_not_deleted ADD CONSTRAINT xref_p31_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p31_not_deleted ADD CONSTRAINT xref_p31_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p31_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p31_not_deleted ADD CONSTRAINT xref_p31_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p32_deleted ADD CONSTRAINT xref_p32_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p32_deleted ADD CONSTRAINT xref_p32_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p32_deleted ADD CONSTRAINT xref_p32_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p32_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p32_deleted ADD CONSTRAINT xref_p32_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p32_not_deleted ADD CONSTRAINT xref_p32_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p32_not_deleted ADD CONSTRAINT xref_p32_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p32_not_deleted ADD CONSTRAINT xref_p32_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p32_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p32_not_deleted ADD CONSTRAINT xref_p32_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p33_deleted ADD CONSTRAINT xref_p33_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p33_deleted ADD CONSTRAINT xref_p33_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p33_deleted ADD CONSTRAINT xref_p33_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p33_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p33_deleted ADD CONSTRAINT xref_p33_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p33_not_deleted ADD CONSTRAINT xref_p33_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p33_not_deleted ADD CONSTRAINT xref_p33_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p33_not_deleted ADD CONSTRAINT xref_p33_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p33_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p33_not_deleted ADD CONSTRAINT xref_p33_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p34_deleted ADD CONSTRAINT xref_p34_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p34_deleted ADD CONSTRAINT xref_p34_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p34_deleted ADD CONSTRAINT xref_p34_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p34_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p34_deleted ADD CONSTRAINT xref_p34_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p34_not_deleted ADD CONSTRAINT xref_p34_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p34_not_deleted ADD CONSTRAINT xref_p34_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p34_not_deleted ADD CONSTRAINT xref_p34_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p34_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p34_not_deleted ADD CONSTRAINT xref_p34_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p35_deleted ADD CONSTRAINT xref_p35_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p35_deleted ADD CONSTRAINT xref_p35_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p35_deleted ADD CONSTRAINT xref_p35_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p35_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p35_deleted ADD CONSTRAINT xref_p35_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p35_not_deleted ADD CONSTRAINT xref_p35_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p35_not_deleted ADD CONSTRAINT xref_p35_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p35_not_deleted ADD CONSTRAINT xref_p35_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p35_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p35_not_deleted ADD CONSTRAINT xref_p35_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p36_deleted ADD CONSTRAINT xref_p36_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p36_deleted ADD CONSTRAINT xref_p36_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p36_deleted ADD CONSTRAINT xref_p36_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p36_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p36_deleted ADD CONSTRAINT xref_p36_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p36_not_deleted ADD CONSTRAINT xref_p36_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p36_not_deleted ADD CONSTRAINT xref_p36_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p36_not_deleted ADD CONSTRAINT xref_p36_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p36_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p36_not_deleted ADD CONSTRAINT xref_p36_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p37_deleted ADD CONSTRAINT xref_p37_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p37_deleted ADD CONSTRAINT xref_p37_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p37_deleted ADD CONSTRAINT xref_p37_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p37_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p37_deleted ADD CONSTRAINT xref_p37_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p37_not_deleted ADD CONSTRAINT xref_p37_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p37_not_deleted ADD CONSTRAINT xref_p37_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p37_not_deleted ADD CONSTRAINT xref_p37_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p37_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p37_not_deleted ADD CONSTRAINT xref_p37_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p38_deleted ADD CONSTRAINT xref_p38_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p38_deleted ADD CONSTRAINT xref_p38_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p38_deleted ADD CONSTRAINT xref_p38_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p38_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p38_deleted ADD CONSTRAINT xref_p38_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p38_not_deleted ADD CONSTRAINT xref_p38_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p38_not_deleted ADD CONSTRAINT xref_p38_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p38_not_deleted ADD CONSTRAINT xref_p38_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p38_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p38_not_deleted ADD CONSTRAINT xref_p38_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p39_deleted ADD CONSTRAINT xref_p39_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p39_deleted ADD CONSTRAINT xref_p39_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p39_deleted ADD CONSTRAINT xref_p39_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p39_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p39_deleted ADD CONSTRAINT xref_p39_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p39_not_deleted ADD CONSTRAINT xref_p39_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p39_not_deleted ADD CONSTRAINT xref_p39_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p39_not_deleted ADD CONSTRAINT xref_p39_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p39_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p39_not_deleted ADD CONSTRAINT xref_p39_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p3_deleted ADD CONSTRAINT xref_p3_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p3_deleted ADD CONSTRAINT xref_p3_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p3_deleted ADD CONSTRAINT xref_p3_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p3_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p3_deleted ADD CONSTRAINT xref_p3_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p3_not_deleted ADD CONSTRAINT xref_p3_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p3_not_deleted ADD CONSTRAINT xref_p3_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p3_not_deleted ADD CONSTRAINT xref_p3_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p3_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p3_not_deleted ADD CONSTRAINT xref_p3_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p40_deleted ADD CONSTRAINT xref_p40_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p40_deleted ADD CONSTRAINT xref_p40_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p40_deleted ADD CONSTRAINT xref_p40_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p40_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p40_deleted ADD CONSTRAINT xref_p40_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p40_not_deleted ADD CONSTRAINT xref_p40_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p40_not_deleted ADD CONSTRAINT xref_p40_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p40_not_deleted ADD CONSTRAINT xref_p40_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p40_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p40_not_deleted ADD CONSTRAINT xref_p40_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p41_deleted ADD CONSTRAINT xref_p41_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p41_deleted ADD CONSTRAINT xref_p41_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p41_deleted ADD CONSTRAINT xref_p41_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p41_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p41_deleted ADD CONSTRAINT xref_p41_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p41_not_deleted ADD CONSTRAINT xref_p41_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p41_not_deleted ADD CONSTRAINT xref_p41_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p41_not_deleted ADD CONSTRAINT xref_p41_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p41_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p41_not_deleted ADD CONSTRAINT xref_p41_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p42_deleted ADD CONSTRAINT xref_p42_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p42_deleted ADD CONSTRAINT xref_p42_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p42_deleted ADD CONSTRAINT xref_p42_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p42_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p42_deleted ADD CONSTRAINT xref_p42_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p42_not_deleted ADD CONSTRAINT xref_p42_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p42_not_deleted ADD CONSTRAINT xref_p42_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p42_not_deleted ADD CONSTRAINT xref_p42_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p42_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p42_not_deleted ADD CONSTRAINT xref_p42_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p43_deleted ADD CONSTRAINT xref_p43_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p43_deleted ADD CONSTRAINT xref_p43_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p43_deleted ADD CONSTRAINT xref_p43_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p43_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p43_deleted ADD CONSTRAINT xref_p43_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p43_not_deleted ADD CONSTRAINT xref_p43_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p43_not_deleted ADD CONSTRAINT xref_p43_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p43_not_deleted ADD CONSTRAINT xref_p43_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p43_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p43_not_deleted ADD CONSTRAINT xref_p43_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p44_deleted ADD CONSTRAINT xref_p44_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p44_deleted ADD CONSTRAINT xref_p44_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p44_deleted ADD CONSTRAINT xref_p44_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p44_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p44_deleted ADD CONSTRAINT xref_p44_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p44_not_deleted ADD CONSTRAINT xref_p44_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p44_not_deleted ADD CONSTRAINT xref_p44_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p44_not_deleted ADD CONSTRAINT xref_p44_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p44_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p44_not_deleted ADD CONSTRAINT xref_p44_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p45_deleted ADD CONSTRAINT xref_p45_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p45_deleted ADD CONSTRAINT xref_p45_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p45_deleted ADD CONSTRAINT xref_p45_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p45_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p45_deleted ADD CONSTRAINT xref_p45_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p45_not_deleted ADD CONSTRAINT xref_p45_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p45_not_deleted ADD CONSTRAINT xref_p45_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p45_not_deleted ADD CONSTRAINT xref_p45_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p45_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p45_not_deleted ADD CONSTRAINT xref_p45_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p46_deleted ADD CONSTRAINT xref_p46_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p46_deleted ADD CONSTRAINT xref_p46_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p46_deleted ADD CONSTRAINT xref_p46_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p46_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p46_deleted ADD CONSTRAINT xref_p46_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p46_not_deleted ADD CONSTRAINT xref_p46_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p46_not_deleted ADD CONSTRAINT xref_p46_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p46_not_deleted ADD CONSTRAINT xref_p46_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p46_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p46_not_deleted ADD CONSTRAINT xref_p46_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p47_deleted ADD CONSTRAINT xref_p47_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p47_deleted ADD CONSTRAINT xref_p47_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p47_deleted ADD CONSTRAINT xref_p47_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p47_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p47_deleted ADD CONSTRAINT xref_p47_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p47_not_deleted ADD CONSTRAINT xref_p47_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p47_not_deleted ADD CONSTRAINT xref_p47_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p47_not_deleted ADD CONSTRAINT xref_p47_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p47_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p47_not_deleted ADD CONSTRAINT xref_p47_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p48_deleted ADD CONSTRAINT xref_p48_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p48_deleted ADD CONSTRAINT xref_p48_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p48_deleted ADD CONSTRAINT xref_p48_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p48_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p48_deleted ADD CONSTRAINT xref_p48_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p48_not_deleted ADD CONSTRAINT xref_p48_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p48_not_deleted ADD CONSTRAINT xref_p48_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p48_not_deleted ADD CONSTRAINT xref_p48_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p48_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p48_not_deleted ADD CONSTRAINT xref_p48_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p49_deleted ADD CONSTRAINT xref_p49_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p49_deleted ADD CONSTRAINT xref_p49_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p49_deleted ADD CONSTRAINT xref_p49_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p49_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p49_deleted ADD CONSTRAINT xref_p49_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p49_not_deleted ADD CONSTRAINT xref_p49_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p49_not_deleted ADD CONSTRAINT xref_p49_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p49_not_deleted ADD CONSTRAINT xref_p49_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p49_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p49_not_deleted ADD CONSTRAINT xref_p49_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p4_deleted ADD CONSTRAINT xref_p4_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p4_deleted ADD CONSTRAINT xref_p4_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p4_deleted ADD CONSTRAINT xref_p4_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p4_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p4_deleted ADD CONSTRAINT xref_p4_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p4_not_deleted ADD CONSTRAINT xref_p4_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p4_not_deleted ADD CONSTRAINT xref_p4_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p4_not_deleted ADD CONSTRAINT xref_p4_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p4_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p4_not_deleted ADD CONSTRAINT xref_p4_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p50_deleted ADD CONSTRAINT xref_p50_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p50_deleted ADD CONSTRAINT xref_p50_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p50_deleted ADD CONSTRAINT xref_p50_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p50_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p50_deleted ADD CONSTRAINT xref_p50_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p50_not_deleted ADD CONSTRAINT xref_p50_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p50_not_deleted ADD CONSTRAINT xref_p50_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p50_not_deleted ADD CONSTRAINT xref_p50_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p50_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p50_not_deleted ADD CONSTRAINT xref_p50_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p51_deleted ADD CONSTRAINT xref_p51_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p51_deleted ADD CONSTRAINT xref_p51_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p51_deleted ADD CONSTRAINT xref_p51_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p51_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p51_deleted ADD CONSTRAINT xref_p51_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p51_not_deleted ADD CONSTRAINT xref_p51_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p51_not_deleted ADD CONSTRAINT xref_p51_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p51_not_deleted ADD CONSTRAINT xref_p51_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p51_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p51_not_deleted ADD CONSTRAINT xref_p51_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p52_deleted ADD CONSTRAINT xref_p52_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p52_deleted ADD CONSTRAINT xref_p52_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p52_deleted ADD CONSTRAINT xref_p52_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p52_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p52_deleted ADD CONSTRAINT xref_p52_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p52_not_deleted ADD CONSTRAINT xref_p52_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p52_not_deleted ADD CONSTRAINT xref_p52_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p52_not_deleted ADD CONSTRAINT xref_p52_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p52_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p52_not_deleted ADD CONSTRAINT xref_p52_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p53_deleted ADD CONSTRAINT xref_p53_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p53_deleted ADD CONSTRAINT xref_p53_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p53_deleted ADD CONSTRAINT xref_p53_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p53_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p53_deleted ADD CONSTRAINT xref_p53_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p53_not_deleted ADD CONSTRAINT xref_p53_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p53_not_deleted ADD CONSTRAINT xref_p53_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p53_not_deleted ADD CONSTRAINT xref_p53_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p53_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p53_not_deleted ADD CONSTRAINT xref_p53_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_deleted ADD CONSTRAINT xref_p54_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_deleted ADD CONSTRAINT xref_p54_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_deleted ADD CONSTRAINT xref_p54_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p54_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_deleted ADD CONSTRAINT xref_p54_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_not_deleted ADD CONSTRAINT xref_p54_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_not_deleted ADD CONSTRAINT xref_p54_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_not_deleted ADD CONSTRAINT xref_p54_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p54_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p54_not_deleted ADD CONSTRAINT xref_p54_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p55_deleted ADD CONSTRAINT xref_p55_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p55_deleted ADD CONSTRAINT xref_p55_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p55_deleted ADD CONSTRAINT xref_p55_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p55_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p55_deleted ADD CONSTRAINT xref_p55_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p55_not_deleted ADD CONSTRAINT xref_p55_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p55_not_deleted ADD CONSTRAINT xref_p55_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p55_not_deleted ADD CONSTRAINT xref_p55_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p55_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p55_not_deleted ADD CONSTRAINT xref_p55_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_deleted ADD CONSTRAINT xref_p5_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_deleted ADD CONSTRAINT xref_p5_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_deleted ADD CONSTRAINT xref_p5_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p5_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_deleted ADD CONSTRAINT xref_p5_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_not_deleted ADD CONSTRAINT xref_p5_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_not_deleted ADD CONSTRAINT xref_p5_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_not_deleted ADD CONSTRAINT xref_p5_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p5_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p5_not_deleted ADD CONSTRAINT xref_p5_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi) DEFERRABLE;

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p6_deleted ADD CONSTRAINT xref_p6_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p6_deleted ADD CONSTRAINT xref_p6_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p6_deleted ADD CONSTRAINT xref_p6_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p6_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p6_deleted ADD CONSTRAINT xref_p6_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p6_not_deleted ADD CONSTRAINT xref_p6_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p6_not_deleted ADD CONSTRAINT xref_p6_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p6_not_deleted ADD CONSTRAINT xref_p6_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p6_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p6_not_deleted ADD CONSTRAINT xref_p6_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p7_deleted ADD CONSTRAINT xref_p7_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p7_deleted ADD CONSTRAINT xref_p7_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p7_deleted ADD CONSTRAINT xref_p7_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p7_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p7_deleted ADD CONSTRAINT xref_p7_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p7_not_deleted ADD CONSTRAINT xref_p7_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p7_not_deleted ADD CONSTRAINT xref_p7_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p7_not_deleted ADD CONSTRAINT xref_p7_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p7_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p7_not_deleted ADD CONSTRAINT xref_p7_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p8_deleted ADD CONSTRAINT xref_p8_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p8_deleted ADD CONSTRAINT xref_p8_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p8_deleted ADD CONSTRAINT xref_p8_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p8_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p8_deleted ADD CONSTRAINT xref_p8_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p8_not_deleted ADD CONSTRAINT xref_p8_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p8_not_deleted ADD CONSTRAINT xref_p8_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p8_not_deleted ADD CONSTRAINT xref_p8_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p8_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p8_not_deleted ADD CONSTRAINT xref_p8_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p9_deleted ADD CONSTRAINT xref_p9_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p9_deleted ADD CONSTRAINT xref_p9_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p9_deleted ADD CONSTRAINT xref_p9_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p9_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p9_deleted ADD CONSTRAINT xref_p9_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p9_not_deleted ADD CONSTRAINT xref_p9_not_deleted_fk1 FOREIGN KEY (created) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p9_not_deleted ADD CONSTRAINT xref_p9_not_deleted_fk2 FOREIGN KEY (dbid) REFERENCES rnacen.rnc_database (id);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p9_not_deleted ADD CONSTRAINT xref_p9_not_deleted_fk3 FOREIGN KEY (last) REFERENCES rnacen.rnc_release (id);

/*
ERROR: relation "rnacen.xref_p9_not_deleted" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/rna/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY rnacen.xref_p9_not_deleted ADD CONSTRAINT xref_p9_not_deleted_fk4 FOREIGN KEY (upi) REFERENCES rnacen.rna (upi);

