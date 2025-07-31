/*
ERROR: INDEX on column of type 'user_defined_type' not yet supported (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX idx_1 ON composite_type_examples.ordinary_table USING btree (basic_);

/*
ERROR: index method "gist" not supported yet (SQLSTATE XX000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX hidx ON extension_example.testhstore USING gist (h extension_example.gist_hstore_ops(siglen='32'));

/*
ERROR: ybgin indexes do not support reloption fastupdate (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX gin_idx ON idx_ex.films USING gin (to_tsvector('english'::regconfig, title)) WITH (fastupdate=off);

/*
ERROR: SQL function cannot accept shell type base_type_examples.int42 (SQLSTATE 42P13)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/functions/function.sql
*/
CREATE FUNCTION base_type_examples.fake_op(point, base_type_examples.int42) RETURNS boolean
    LANGUAGE sql IMMUTABLE
    AS $$ select true $$;

/*
ERROR: VIEW WITH CASCADED CHECK OPTION not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/views/view.sql
*/
CREATE VIEW regress_rls_schema.bv1 WITH (security_barrier='true') AS
 SELECT a,
    b
   FROM regress_rls_schema.b1
  WHERE (a > 0)
  WITH CASCADED CHECK OPTION;

/*
ERROR: CREATE CONVERSION not supported yet (SQLSTATE 0A000)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/conversions/conversion.sql
*/
CREATE CONVERSION conversion_example.myconv FOR 'LATIN1' TO 'UTF8' FROM iso8859_1_to_utf8;

/*
ERROR: type "base_type_examples.int42" is only a shell (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/operators/operator.sql
*/
CREATE OPERATOR public.<% (
    FUNCTION = base_type_examples.fake_op,
    LEFTARG = point,
    RIGHTARG = base_type_examples.int42,
    COMMUTATOR = OPERATOR(public.>%),
    NEGATOR = OPERATOR(public.>=%)
);

/*
ERROR: access method "gist2" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/tables/INDEXES_table.sql
*/
CREATE INDEX grect2ind2 ON am_examples.fast_emp4000 USING gist2 (home_base);

/*
ERROR: relation "foreign_db_example.technically_doesnt_exist" does not exist (SQLSTATE 42P01)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/views/view.sql
*/
CREATE VIEW public.foreign_db_example AS
 SELECT id,
    uses_type,
    _uses_type,
    positive_number,
    _positive_number
   FROM foreign_db_example.technically_doesnt_exist;

/*
ERROR: server "technically_this_server" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/tables/foreign_table.sql
*/
CREATE FOREIGN TABLE foreign_db_example.technically_doesnt_exist (
    id integer,
    uses_type foreign_db_example.example_type,
    _uses_type foreign_db_example.example_type GENERATED ALWAYS AS (uses_type) STORED,
    positive_number foreign_db_example.positive_number,
    _positive_number foreign_db_example.positive_number GENERATED ALWAYS AS (positive_number) STORED,
    CONSTRAINT imaginary_table_id_gt_1 CHECK ((id > 1))
)
SERVER technically_this_server;

/*
ERROR: role "regress_rls_eve" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p ON regress_rls_schema.tbl1 TO regress_rls_eve, regress_rls_frank USING (true);

/*
ERROR: role "regress_rls_dob_role1" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p1 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1 USING (true);

/*
ERROR: role "regress_rls_dob_role1" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p1 ON regress_rls_schema.dob_t2 TO regress_rls_dob_role1, regress_rls_dob_role2 USING (true);

/*
ERROR: role "regress_rls_bob" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p1 ON regress_rls_schema.t1_2 TO regress_rls_bob USING (((a % 2) = 0));

/*
ERROR: role "regress_rls_group1" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p1 ON regress_rls_schema.z1 TO regress_rls_group1 USING (((a % 2) = 0));

/*
ERROR: role "regress_rls_dob_role1" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p1_2 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1, regress_rls_dob_role2 USING (true);

/*
ERROR: role "regress_rls_dob_role1" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p1_3 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1 USING (true);

/*
ERROR: role "regress_rls_dob_role1" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p1_4 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1, regress_rls_dob_role2 USING (true);

/*
ERROR: role "regress_rls_dave" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p1r ON regress_rls_schema.document AS RESTRICTIVE TO regress_rls_dave USING ((cid <> 44));

/*
ERROR: role "regress_rls_carol" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p2 ON regress_rls_schema.t1_2 TO regress_rls_carol USING (((a % 4) = 0));

/*
ERROR: role "regress_rls_group2" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p2 ON regress_rls_schema.z1 TO regress_rls_group2 USING (((a % 2) = 1));

/*
ERROR: role "regress_rls_dave" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY p2r ON regress_rls_schema.document AS RESTRICTIVE TO regress_rls_dave USING (((cid <> 44) AND (cid < 50)));

/*
ERROR: role "regress_rls_dave" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/policies/policy.sql
*/
CREATE POLICY pp1r ON regress_rls_schema.part_document AS RESTRICTIVE TO regress_rls_dave USING ((cid < 55));

/*
ERROR: access method "gist2" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/operators/operator.sql
*/
CREATE OPERATOR FAMILY am_examples.box_ops USING gist2;

/*
ERROR: access method "gist2" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/operators/operator.sql
*/
ALTER OPERATOR FAMILY am_examples.box_ops USING gist2 ADD
    OPERATOR 1 <<(box,box) ,
    OPERATOR 2 &<(box,box) ,
    OPERATOR 3 &&(box,box) ,
    OPERATOR 4 &>(box,box) ,
    OPERATOR 5 >>(box,box) ,
    OPERATOR 6 ~=(box,box) ,
    OPERATOR 7 @>(box,box) ,
    OPERATOR 8 <@(box,box) ,
    OPERATOR 9 &<|(box,box) ,
    OPERATOR 10 <<|(box,box) ,
    OPERATOR 11 |>>(box,box) ,
    OPERATOR 12 |&>(box,box);

/*
ERROR: access method "gist2" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/operators/operator.sql
*/
CREATE OPERATOR CLASS am_examples.box_ops
    DEFAULT FOR TYPE box USING gist2 FAMILY am_examples.box_ops AS
    FUNCTION 1 (box, box) gist_box_consistent(internal,box,smallint,oid,internal) ,
    FUNCTION 2 (box, box) gist_box_union(internal,internal) ,
    FUNCTION 5 (box, box) gist_box_penalty(internal,internal,internal) ,
    FUNCTION 6 (box, box) gist_box_picksplit(internal,internal) ,
    FUNCTION 7 (box, box) gist_box_same(box,box,internal);

/*
ERROR: access method "gist2" does not exist (SQLSTATE 42704)
File :/home/ubuntu/yb-voyager/migtests/tests/pg/omnibus/export-dir/schema/operators/operator.sql
*/
CREATE OPERATOR FAMILY am_examples.gist2_fam USING gist2;

