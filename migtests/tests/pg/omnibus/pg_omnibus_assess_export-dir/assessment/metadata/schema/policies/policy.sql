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


CREATE POLICY d1 ON regress_rls_schema.dependent USING ((x = ( SELECT d.x
   FROM regress_rls_schema.dependee d
  WHERE (d.y = d.y))));


CREATE POLICY p ON regress_rls_schema.t USING (((c % 2) = 1));


CREATE POLICY p ON regress_rls_schema.tbl1 TO regress_rls_eve, regress_rls_frank USING (true);


CREATE POLICY p0 ON regress_rls_schema.x1 USING ((c = CURRENT_USER));


CREATE POLICY p1 ON regress_rls_schema.b1 USING (((a % 2) = 0));


CREATE POLICY p1 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1 USING (true);


CREATE POLICY p1 ON regress_rls_schema.dob_t2 TO regress_rls_dob_role1, regress_rls_dob_role2 USING (true);


CREATE POLICY p1 ON regress_rls_schema.document USING ((dlevel <= ( SELECT uaccount.seclv
   FROM regress_rls_schema.uaccount
  WHERE (uaccount.pguser = CURRENT_USER))));


CREATE POLICY p1 ON regress_rls_schema.r1 USING (true);


CREATE POLICY p1 ON regress_rls_schema.r1_2 USING (false);


CREATE POLICY p1 ON regress_rls_schema.r1_3 USING (false);


CREATE POLICY p1 ON regress_rls_schema.r2 FOR SELECT USING (true);


CREATE POLICY p1 ON regress_rls_schema.r2_3 USING (false);


CREATE POLICY p1 ON regress_rls_schema.r2_4 USING (false);


CREATE POLICY p1 ON regress_rls_schema.r2_5 USING (false);


CREATE POLICY p1 ON regress_rls_schema.rls_tbl USING ((EXISTS ( SELECT 1
   FROM regress_rls_schema.ref_tbl)));


CREATE POLICY p1 ON regress_rls_schema.rls_tbl_3 USING ((rls_tbl_3.* >= ROW(1, 1, 1)));


CREATE POLICY p1 ON regress_rls_schema.s1 USING ((a IN ( SELECT s2.x
   FROM regress_rls_schema.s2
  WHERE (s2.y ~~ '%2f%'::text))));


CREATE POLICY p1 ON regress_rls_schema.t1 USING (((a % 2) = 0));


CREATE POLICY p1 ON regress_rls_schema.t1_2 TO regress_rls_bob USING (((a % 2) = 0));


CREATE POLICY p1 ON regress_rls_schema.t1_3 USING (((a % 2) = 0));


CREATE POLICY p1 ON regress_rls_schema.x1 FOR SELECT USING (((a % 2) = 0));


CREATE POLICY p1 ON regress_rls_schema.y1 USING (((a % 2) = 0));


CREATE POLICY p1 ON regress_rls_schema.y2 USING (((a % 2) = 0));


CREATE POLICY p1 ON regress_rls_schema.z1 TO regress_rls_group1 USING (((a % 2) = 0));


CREATE POLICY p1_2 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1, regress_rls_dob_role2 USING (true);


CREATE POLICY p1_2 ON regress_rls_schema.s1 USING ((a IN ( SELECT v2.x
   FROM regress_rls_schema.v2)));


CREATE POLICY p1_2 ON regress_rls_schema.t1_3 USING (((a % 2) = 0));


CREATE POLICY p1_2 ON regress_rls_schema.y1 FOR SELECT USING (((a % 2) = 1));


CREATE POLICY p1_3 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1 USING (true);


CREATE POLICY p1_3 ON regress_rls_schema.document FOR SELECT USING (true);


CREATE POLICY p1_4 ON regress_rls_schema.dob_t1 TO regress_rls_dob_role1, regress_rls_dob_role2 USING (true);


CREATE POLICY p1_4 ON regress_rls_schema.document FOR SELECT USING (true);


CREATE POLICY p1r ON regress_rls_schema.document AS RESTRICTIVE TO regress_rls_dave USING ((cid <> 44));


CREATE POLICY p2 ON regress_rls_schema.r2 FOR INSERT WITH CHECK (false);


CREATE POLICY p2 ON regress_rls_schema.s2 USING ((x IN ( SELECT s1.a
   FROM regress_rls_schema.s1
  WHERE (s1.b ~~ '%22%'::text))));


CREATE POLICY p2 ON regress_rls_schema.t1_2 TO regress_rls_carol USING (((a % 4) = 0));


CREATE POLICY p2 ON regress_rls_schema.t2 USING (((a % 2) = 1));


CREATE POLICY p2 ON regress_rls_schema.t2_3 USING (((a % 2) = 1));


CREATE POLICY p2 ON regress_rls_schema.x1 FOR INSERT WITH CHECK (((a % 2) = 1));


CREATE POLICY p2 ON regress_rls_schema.y1 FOR SELECT USING ((a > 2));


CREATE POLICY p2 ON regress_rls_schema.y2 USING (((a % 3) = 0));


CREATE POLICY p2 ON regress_rls_schema.z1 TO regress_rls_group2 USING (((a % 2) = 1));


CREATE POLICY p2_2 ON regress_rls_schema.s2 USING ((x IN ( SELECT s1.a
   FROM regress_rls_schema.s1
  WHERE (s1.b ~~ '%d2%'::text))));


CREATE POLICY p2_3 ON regress_rls_schema.document FOR INSERT WITH CHECK ((dauthor = CURRENT_USER));


CREATE POLICY p2_4 ON regress_rls_schema.document FOR INSERT WITH CHECK ((dauthor = CURRENT_USER));


CREATE POLICY p2r ON regress_rls_schema.document AS RESTRICTIVE TO regress_rls_dave USING (((cid <> 44) AND (cid < 50)));


CREATE POLICY p3 ON regress_rls_schema.r2 FOR UPDATE USING (false);


CREATE POLICY p3 ON regress_rls_schema.s1 FOR INSERT WITH CHECK ((a = ( SELECT s1_1.a
   FROM regress_rls_schema.s1 s1_1)));


CREATE POLICY p3 ON regress_rls_schema.x1 FOR UPDATE USING (((a % 2) = 0));


CREATE POLICY p3 ON regress_rls_schema.y2 USING (((a % 4) = 0));


CREATE POLICY p3 ON regress_rls_schema.z1 AS RESTRICTIVE USING ((NOT (a IN ( SELECT z1_blacklist.a
   FROM regress_rls_schema.z1_blacklist))));


CREATE POLICY p3_2 ON regress_rls_schema.z1 AS RESTRICTIVE USING ((NOT (a IN ( SELECT z1_blacklist.a
   FROM regress_rls_schema.z1_blacklist))));


CREATE POLICY p3_3 ON regress_rls_schema.document FOR UPDATE USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'novel'::text)))) WITH CHECK ((dauthor = CURRENT_USER));


CREATE POLICY p3_4 ON regress_rls_schema.document FOR UPDATE USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'novel'::text)))) WITH CHECK ((dauthor = CURRENT_USER));


CREATE POLICY p3_with_all ON regress_rls_schema.document USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'novel'::text)))) WITH CHECK ((dauthor = CURRENT_USER));


CREATE POLICY p3_with_default ON regress_rls_schema.document FOR UPDATE USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'novel'::text))));


CREATE POLICY p4 ON regress_rls_schema.r2 FOR DELETE USING (false);


CREATE POLICY p4 ON regress_rls_schema.x1 FOR DELETE USING ((a < 8));


CREATE POLICY p4_4 ON regress_rls_schema.document FOR DELETE USING ((cid = ( SELECT category.cid
   FROM regress_rls_schema.category
  WHERE (category.cname = 'manga'::text))));


CREATE POLICY pp1 ON regress_rls_schema.part_document USING ((dlevel <= ( SELECT uaccount.seclv
   FROM regress_rls_schema.uaccount
  WHERE (uaccount.pguser = CURRENT_USER))));


CREATE POLICY pp1r ON regress_rls_schema.part_document AS RESTRICTIVE TO regress_rls_dave USING ((cid < 55));


CREATE POLICY r1 ON regress_rls_schema.rec1 USING ((x = ( SELECT r.x
   FROM regress_rls_schema.rec1 r
  WHERE (r.y = r.y))));


CREATE POLICY r1_2 ON regress_rls_schema.rec1 USING ((x = ( SELECT rec2.a
   FROM regress_rls_schema.rec2
  WHERE (rec2.b = rec1.y))));


CREATE POLICY r1_3 ON regress_rls_schema.rec1 USING ((x = ( SELECT rec2v.a
   FROM regress_rls_schema.rec2v
  WHERE (rec2v.b = rec1.y))));


CREATE POLICY r1_4 ON regress_rls_schema.rec1 USING ((x = ( SELECT rec2v_2.a
   FROM regress_rls_schema.rec2v_2
  WHERE (rec2v_2.b = rec1.y))));


CREATE POLICY r2 ON regress_rls_schema.rec2 USING ((a = ( SELECT rec1.x
   FROM regress_rls_schema.rec1
  WHERE (rec1.y = rec2.b))));


CREATE POLICY r2_2 ON regress_rls_schema.rec2 USING ((a = ( SELECT rec1v_2.x
   FROM regress_rls_schema.rec1v_2
  WHERE (rec1v_2.y = rec2.b))));


CREATE POLICY r2_3 ON regress_rls_schema.rec2 USING ((a = ( SELECT rec1v.x
   FROM regress_rls_schema.rec1v
  WHERE (rec1v.y = rec2.b))));


