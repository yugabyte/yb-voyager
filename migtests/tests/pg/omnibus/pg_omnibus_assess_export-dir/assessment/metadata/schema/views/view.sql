-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE VIEW agg_ex.my_view AS
 SELECT agg_ex.my_sum(i) AS my_sum,
    agg_ex.my_avg(i) AS my_avg
   FROM agg_ex.my_table;


CREATE VIEW composite_type_examples.basic_view AS
 SELECT basic_,
    _basic,
    nested,
    _nested
   FROM composite_type_examples.ordinary_table;


CREATE VIEW enum_example._bugs AS
 SELECT id,
    status
   FROM enum_example.bugs;


CREATE VIEW extension_example.dependent_view AS
 SELECT key,
    value
   FROM extension_example.each('""=>"1", "b"=>NULL, "aaa"=>"bq"'::extension_example.hstore) each(key, value);


CREATE VIEW fn_examples.basic_view AS
 SELECT id
   FROM fn_examples.ordinary_table;


CREATE VIEW public.foreign_db_example AS
 SELECT id,
    uses_type,
    _uses_type,
    positive_number,
    _positive_number
   FROM foreign_db_example.technically_doesnt_exist;


CREATE VIEW range_type_example.depends_on_col_using_type AS
 SELECT col
   FROM range_type_example.example_tbl;


CREATE VIEW regress_rls_schema.bv1 WITH (security_barrier='true') AS
 SELECT a,
    b
   FROM regress_rls_schema.b1
  WHERE (a > 0)
  WITH CASCADED CHECK OPTION;


CREATE VIEW regress_rls_schema.rec1v AS
 SELECT x,
    y
   FROM regress_rls_schema.rec1;


CREATE VIEW regress_rls_schema.rec1v_2 WITH (security_barrier='true') AS
 SELECT x,
    y
   FROM regress_rls_schema.rec1;


CREATE VIEW regress_rls_schema.rec2v AS
 SELECT a,
    b
   FROM regress_rls_schema.rec2;


CREATE VIEW regress_rls_schema.rec2v_2 WITH (security_barrier='true') AS
 SELECT a,
    b
   FROM regress_rls_schema.rec2;


CREATE VIEW regress_rls_schema.rls_sbv WITH (security_barrier='true') AS
 SELECT a,
    b
   FROM regress_rls_schema.y1
  WHERE regress_rls_schema.f_leak(b);


CREATE VIEW regress_rls_schema.rls_sbv_2 WITH (security_barrier='true') AS
 SELECT a,
    b
   FROM regress_rls_schema.y1
  WHERE regress_rls_schema.f_leak(b);


CREATE VIEW regress_rls_schema.rls_view AS
 SELECT a
   FROM regress_rls_schema.rls_tbl;


CREATE VIEW regress_rls_schema.rls_view_2 AS
 SELECT a,
    b
   FROM regress_rls_schema.z1
  WHERE regress_rls_schema.f_leak(b);


CREATE VIEW regress_rls_schema.rls_view_3 AS
 SELECT a,
    b
   FROM regress_rls_schema.z1
  WHERE regress_rls_schema.f_leak(b);


CREATE VIEW regress_rls_schema.rls_view_4 AS
 SELECT a,
    b
   FROM regress_rls_schema.z1
  WHERE regress_rls_schema.f_leak(b);


CREATE VIEW regress_rls_schema.v2 AS
 SELECT x,
    y
   FROM regress_rls_schema.s2
  WHERE (y ~~ '%af%'::text);


CREATE VIEW trigger_test.accounts_view AS
 SELECT id,
    balance
   FROM trigger_test.accounts;


