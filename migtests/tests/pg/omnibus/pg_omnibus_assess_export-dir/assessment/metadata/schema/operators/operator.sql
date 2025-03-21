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


CREATE OPERATOR public.<% (
    FUNCTION = base_type_examples.fake_op,
    LEFTARG = point,
    RIGHTARG = base_type_examples.int42,
    COMMUTATOR = OPERATOR(public.>%),
    NEGATOR = OPERATOR(public.>=%)
);


CREATE OPERATOR regress_rls_schema.<<< (
    FUNCTION = regress_rls_schema.op_leak,
    LEFTARG = integer,
    RIGHTARG = integer,
    RESTRICT = scalarltsel
);


CREATE OPERATOR FAMILY am_examples.box_ops USING gist2;
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


CREATE OPERATOR CLASS am_examples.box_ops
    DEFAULT FOR TYPE box USING gist2 FAMILY am_examples.box_ops AS
    FUNCTION 1 (box, box) gist_box_consistent(internal,box,smallint,oid,internal) ,
    FUNCTION 2 (box, box) gist_box_union(internal,internal) ,
    FUNCTION 5 (box, box) gist_box_penalty(internal,internal,internal) ,
    FUNCTION 6 (box, box) gist_box_picksplit(internal,internal) ,
    FUNCTION 7 (box, box) gist_box_same(box,box,internal);


CREATE OPERATOR FAMILY am_examples.gist2_fam USING gist2;


