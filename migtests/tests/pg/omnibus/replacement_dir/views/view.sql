-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE VIEW agg_ex.my_view AS
 SELECT agg_ex.my_sum(my_table.i) AS my_sum,
    agg_ex.my_avg(my_table.i) AS my_avg
   FROM agg_ex.my_table;


CREATE VIEW composite_type_examples.basic_view AS
 SELECT ordinary_table.basic_,
    ordinary_table._basic,
    ordinary_table.nested,
    ordinary_table._nested
   FROM composite_type_examples.ordinary_table;


CREATE VIEW enum_example._bugs AS
 SELECT bugs.id,
    bugs.status
   FROM enum_example.bugs;


CREATE VIEW extension_example.dependent_view AS
 SELECT each.key,
    each.value
   FROM extension_example.each('""=>"1", "b"=>NULL, "aaa"=>"bq"'::extension_example.hstore) each(key, value);


CREATE VIEW fn_examples.basic_view AS
 SELECT ordinary_table.id
   FROM fn_examples.ordinary_table;


CREATE VIEW public.foreign_db_example AS
 SELECT technically_doesnt_exist.id,
    technically_doesnt_exist.uses_type,
    technically_doesnt_exist._uses_type,
    technically_doesnt_exist.positive_number,
    technically_doesnt_exist._positive_number
   FROM foreign_db_example.technically_doesnt_exist;


CREATE VIEW range_type_example.depends_on_col_using_type AS
 SELECT example_tbl.col
   FROM range_type_example.example_tbl;


CREATE VIEW regress_rls_schema.bv1 WITH (security_barrier='true') AS
 SELECT b1.a,
    b1.b
   FROM regress_rls_schema.b1
  WHERE (b1.a > 0);


CREATE VIEW regress_rls_schema.rec1v AS
 SELECT rec1.x,
    rec1.y
   FROM regress_rls_schema.rec1;


CREATE VIEW regress_rls_schema.rec1v_2 WITH (security_barrier='true') AS
 SELECT rec1.x,
    rec1.y
   FROM regress_rls_schema.rec1;


CREATE VIEW regress_rls_schema.rec2v AS
 SELECT rec2.a,
    rec2.b
   FROM regress_rls_schema.rec2;


CREATE VIEW regress_rls_schema.rec2v_2 WITH (security_barrier='true') AS
 SELECT rec2.a,
    rec2.b
   FROM regress_rls_schema.rec2;


CREATE VIEW regress_rls_schema.rls_sbv WITH (security_barrier='true') AS
 SELECT y1.a,
    y1.b
   FROM regress_rls_schema.y1
  WHERE regress_rls_schema.f_leak(y1.b);


CREATE VIEW regress_rls_schema.rls_sbv_2 WITH (security_barrier='true') AS
 SELECT y1.a,
    y1.b
   FROM regress_rls_schema.y1
  WHERE regress_rls_schema.f_leak(y1.b);


CREATE VIEW regress_rls_schema.rls_view AS
 SELECT rls_tbl.a
   FROM regress_rls_schema.rls_tbl;


CREATE VIEW regress_rls_schema.rls_view_2 AS
 SELECT z1.a,
    z1.b
   FROM regress_rls_schema.z1
  WHERE regress_rls_schema.f_leak(z1.b);


CREATE VIEW regress_rls_schema.rls_view_3 AS
 SELECT z1.a,
    z1.b
   FROM regress_rls_schema.z1
  WHERE regress_rls_schema.f_leak(z1.b);


CREATE VIEW regress_rls_schema.rls_view_4 AS
 SELECT z1.a,
    z1.b
   FROM regress_rls_schema.z1
  WHERE regress_rls_schema.f_leak(z1.b);


CREATE VIEW regress_rls_schema.v2 AS
 SELECT s2.x,
    s2.y
   FROM regress_rls_schema.s2
  WHERE (s2.y ~~ '%af%'::text);


CREATE VIEW trigger_test.accounts_view AS
 SELECT accounts.id,
    accounts.balance
   FROM trigger_test.accounts;


