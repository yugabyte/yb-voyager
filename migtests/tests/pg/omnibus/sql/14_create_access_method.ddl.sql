-- https://www.postgresql.org/docs/current/sql-create-access-method.html
-- adapted from https://raw.githubusercontent.com/postgres/postgres/master/src/test/regress/sql/create_am.sql
-- LICENSE --
-- TODO: copy in postgres license

CREATE USER am_owner;
CREATE SCHEMA am_examples;

CREATE TABLE am_examples.fast_emp4000(
  home_base box
);
-- Make gist2 over gisthandler. In fact, it would be a synonym to gist.
CREATE ACCESS METHOD gist2 TYPE INDEX HANDLER gisthandler;

CREATE OPERATOR FAMILY am_examples.gist2_fam USING gist2;
-- Make operator class for boxes using gist2
CREATE OPERATOR CLASS am_examples.box_ops DEFAULT
	FOR TYPE box USING gist2 AS
	OPERATOR 1	<<,
	OPERATOR 2	&<,
	OPERATOR 3	&&,
	OPERATOR 4	&>,
	OPERATOR 5	>>,
	OPERATOR 6	~=,
	OPERATOR 7	@>,
	OPERATOR 8	<@,
	OPERATOR 9	&<|,
	OPERATOR 10	<<|,
	OPERATOR 11	|>>,
	OPERATOR 12	|&>,
	FUNCTION 1	gist_box_consistent(internal, box, smallint, oid, internal),
	FUNCTION 2	gist_box_union(internal, internal),
	-- don't need compress, decompress, or fetch functions
	FUNCTION 5	gist_box_penalty(internal, internal, internal),
	FUNCTION 6	gist_box_picksplit(internal, internal),
	FUNCTION 7	gist_box_same(box, box, internal);

CREATE INDEX grect2ind2 ON am_examples.fast_emp4000 USING gist2 (home_base);

CREATE INDEX grect2ind3 ON am_examples.fast_emp4000 USING gist2 (home_base am_examples.box_ops);

-- Create a heap2 table am handler with heapam handler
CREATE ACCESS METHOD heap2 TYPE TABLE HANDLER heap_tableam_handler;

-- First create tables employing the new AM using USING

CREATE TABLE am_examples.tableam_tbl_heap2(f1 int) USING heap2;

CREATE TABLE am_examples.tableam_tblas_heap2 USING heap2 AS SELECT * FROM am_examples.tableam_tbl_heap2;

CREATE TABLE am_examples.tableam_parted_heap2 (a text, b int) PARTITION BY list (a);
-- new partitions will inherit from the current default, rather the partition root
-- but the method can be explicitly specified
CREATE TABLE am_examples.tableam_parted_c_heap2 PARTITION OF am_examples.tableam_parted_heap2 FOR VALUES IN ('c') USING heap;
CREATE TABLE am_examples.tableam_parted_d_heap2 PARTITION OF am_examples.tableam_parted_heap2 FOR VALUES IN ('d') USING heap2;

-- ALTER TABLE SET ACCESS METHOD
CREATE TABLE am_examples.heaptable USING heap2 AS
  SELECT a, repeat(a::text, 100) FROM generate_series(1,9) AS a;

-- ALTER TABLE am_examples.heaptable SET ACCESS METHOD heap2;

-- ALTER MATERIALIZED VIEW SET ACCESS METHOD
CREATE MATERIALIZED VIEW am_examples.heapmv USING heap2 AS SELECT * FROM am_examples.heaptable;

-- No support for partitioned tables.
CREATE TABLE am_examples.am_partitioned(x INT, y INT)
  PARTITION BY hash (x);

-- Second, create objects in the new AM by changing the default AM
SET LOCAL default_table_access_method = 'heap2';

-- following tests should all respect the default AM
CREATE TABLE am_examples.tableam_tbl_heapx(f1 int);
CREATE TABLE am_examples.tableam_tblas_heapx AS SELECT * FROM am_examples.tableam_tbl_heapx;
CREATE MATERIALIZED VIEW am_examples.tableam_tblmv_heapx USING heap2 AS SELECT * FROM am_examples.tableam_tbl_heapx;
CREATE TABLE am_examples.tableam_parted_heapx (a text, b int) PARTITION BY list (a);
CREATE TABLE am_examples.tableam_parted_1_heapx PARTITION OF am_examples.tableam_parted_heapx FOR VALUES IN ('a', 'b');

-- but an explicitly set AM overrides it
CREATE TABLE am_examples.tableam_parted_2_heapx PARTITION OF am_examples.tableam_parted_heapx FOR VALUES IN ('c', 'd') USING heap;

-- sequences, views and foreign servers shouldn't have an AM
RESET default_table_access_method;
