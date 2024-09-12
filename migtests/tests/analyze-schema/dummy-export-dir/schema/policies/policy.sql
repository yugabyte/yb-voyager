CREATE POLICY P ON tbl1 TO regress_rls_eve, regress_rls_frank USING (true);

CREATE POLICY p2 ON t2 FOR ALL TO CURRENT_USER USING (a % 2 = 1); 

CREATE POLICY p1 ON z1 TO regress_rls_group1 USING (a % 2 = 0);

CREATE POLICY r1_2 ON public.rec1 USING ((x = ( SELECT rec2.a
   FROM public.rec2
  WHERE (rec2.b = rec1.y))));


CREATE POLICY p3_2 ON z1 AS RESTRICTIVE USING (a NOT IN (SELECT a FROM z1_blacklist));

CREATE POLICY p3_2 ON schema2.z1 AS RESTRICTIVE USING (a NOT IN (SELECT a FROM z1_blacklist));
