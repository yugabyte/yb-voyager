-- https://www.postgresql.org/docs/current/sql-createpolicy.html
-- https://github.com/postgres/postgres/blob/master/src/test/regress/sql/equivclass.sql
-- https://github.com/postgres/postgres/blob/master/src/test/regress/sql/rowsecurity.sql
SET client_min_messages TO 'warning';

DROP USER IF EXISTS regress_rls_alice;
DROP USER IF EXISTS regress_rls_bob;
DROP USER IF EXISTS regress_rls_carol;
DROP USER IF EXISTS regress_rls_dave;
DROP USER IF EXISTS regress_rls_exempt_user;
DROP ROLE IF EXISTS regress_rls_group1;
DROP ROLE IF EXISTS regress_rls_group2;

DROP SCHEMA IF EXISTS regress_rls_schema CASCADE;

RESET client_min_messages;


-- initial setup
CREATE USER regress_rls_alice NOLOGIN;
CREATE USER regress_rls_bob NOLOGIN;
CREATE USER regress_rls_carol NOLOGIN;
CREATE USER regress_rls_dave NOLOGIN;
CREATE USER regress_rls_exempt_user BYPASSRLS NOLOGIN;
CREATE ROLE regress_rls_group1 NOLOGIN;
CREATE ROLE regress_rls_group2 NOLOGIN;

GRANT regress_rls_group1 TO regress_rls_bob;
GRANT regress_rls_group2 TO regress_rls_carol;

CREATE SCHEMA regress_rls_schema;
GRANT ALL ON SCHEMA regress_rls_schema to public;
SET search_path = regress_rls_schema;

-- setup of malicious function
CREATE OR REPLACE FUNCTION f_leak(text) RETURNS bool
    COST 0.0000001 LANGUAGE plpgsql
    AS 'BEGIN RAISE NOTICE ''f_leak => %'', $1; RETURN true; END';
GRANT EXECUTE ON FUNCTION f_leak(text) TO public;

-- BASIC Row-Level Security Scenario


CREATE TABLE uaccount (
    pguser      name primary key,
    seclv       int
);
GRANT SELECT ON uaccount TO public;
INSERT INTO uaccount VALUES
    ('regress_rls_alice', 99),
    ('regress_rls_bob', 1),
    ('regress_rls_carol', 2),
    ('regress_rls_dave', 3);

CREATE TABLE category (
    cid        int primary key,
    cname      text
);
GRANT ALL ON category TO public;
INSERT INTO category VALUES
    (11, 'novel'),
    (22, 'science fiction'),
    (33, 'technology'),
    (44, 'manga');

CREATE TABLE document (
    did         int primary key,
    cid         int references category(cid),
    dlevel      int not null,
    dauthor     name,
    dtitle      text
);
GRANT ALL ON document TO public;
INSERT INTO document VALUES
    ( 1, 11, 1, 'regress_rls_bob', 'my first novel'),
    ( 2, 11, 2, 'regress_rls_bob', 'my second novel'),
    ( 3, 22, 2, 'regress_rls_bob', 'my science fiction'),
    ( 4, 44, 1, 'regress_rls_bob', 'my first manga'),
    ( 5, 44, 2, 'regress_rls_bob', 'my second manga'),
    ( 6, 22, 1, 'regress_rls_carol', 'great science fiction'),
    ( 7, 33, 2, 'regress_rls_carol', 'great technology book'),
    ( 8, 44, 1, 'regress_rls_carol', 'great manga'),
    ( 9, 22, 1, 'regress_rls_dave', 'awesome science fiction'),
    (10, 33, 2, 'regress_rls_dave', 'awesome technology book');

ALTER TABLE document ENABLE ROW LEVEL SECURITY;

-- user's security level must be higher than or equal to document's
CREATE POLICY p1 ON document AS PERMISSIVE
    USING (dlevel <= (SELECT seclv FROM uaccount WHERE pguser = current_user));


-- but Dave isn't allowed to anything at cid 50 or above
-- this is to make sure that we sort the policies by name first
-- when applying WITH CHECK, a later INSERT by Dave should fail due
-- to p1r first
CREATE POLICY p2r ON document AS RESTRICTIVE TO regress_rls_dave
    USING (cid <> 44 AND cid < 50);

-- and Dave isn't allowed to see manga documents
CREATE POLICY p1r ON document AS RESTRICTIVE TO regress_rls_dave
    USING (cid <> 44);


SET row_security TO OFF;


SET row_security TO ON;

CREATE TABLE t1 (id int not null primary key, a int, junk1 text, b text);
ALTER TABLE t1 DROP COLUMN junk1;    -- just a disturbing factor
GRANT ALL ON t1 TO public;
CREATE TABLE t2 (c float) INHERITS (t1);
CREATE POLICY p1 ON t1 FOR ALL TO PUBLIC USING (a % 2 = 0); -- be even number
CREATE POLICY p2 ON t2 FOR ALL TO PUBLIC USING (a % 2 = 1); -- be odd number

ALTER TABLE t1 ENABLE ROW LEVEL SECURITY;
ALTER TABLE t2 ENABLE ROW LEVEL SECURITY;




CREATE TABLE part_document (
    did         int,
    cid         int,
    dlevel      int not null,
    dauthor     name,
    dtitle      text
) PARTITION BY RANGE (cid);
GRANT ALL ON part_document TO public;

-- Create partitions for document categories
CREATE TABLE part_document_fiction PARTITION OF part_document FOR VALUES FROM (11) to (12);
CREATE TABLE part_document_satire PARTITION OF part_document FOR VALUES FROM (55) to (56);
CREATE TABLE part_document_nonfiction PARTITION OF part_document FOR VALUES FROM (99) to (100);

GRANT ALL ON part_document_fiction TO public;
GRANT ALL ON part_document_satire TO public;
GRANT ALL ON part_document_nonfiction TO public;

INSERT INTO part_document VALUES
    ( 1, 11, 1, 'regress_rls_bob', 'my first novel'),
    ( 2, 11, 2, 'regress_rls_bob', 'my second novel'),
    ( 3, 99, 2, 'regress_rls_bob', 'my science textbook'),
    ( 4, 55, 1, 'regress_rls_bob', 'my first satire'),
    ( 5, 99, 2, 'regress_rls_bob', 'my history book'),
    ( 6, 11, 1, 'regress_rls_carol', 'great science fiction'),
    ( 7, 99, 2, 'regress_rls_carol', 'great technology book'),
    ( 8, 55, 2, 'regress_rls_carol', 'great satire'),
    ( 9, 11, 1, 'regress_rls_dave', 'awesome science fiction'),
    (10, 99, 2, 'regress_rls_dave', 'awesome technology book');

ALTER TABLE part_document ENABLE ROW LEVEL SECURITY;

-- Create policy on parent
-- user's security level must be higher than or equal to document's
CREATE POLICY pp1 ON part_document AS PERMISSIVE
    USING (dlevel <= (SELECT seclv FROM uaccount WHERE pguser = current_user));

-- Dave is only allowed to see cid < 55
CREATE POLICY pp1r ON part_document AS RESTRICTIVE TO regress_rls_dave
    USING (cid < 55);


SET row_security TO ON;

CREATE TABLE dependee (x integer, y integer);

CREATE TABLE dependent (x integer, y integer);
CREATE POLICY d1 ON dependent FOR ALL
    TO PUBLIC
    USING (x = (SELECT d.x FROM dependee d WHERE d.y = y));


CREATE TABLE rec1 (x integer, y integer);
CREATE POLICY r1 ON rec1 USING (x = (SELECT r.x FROM rec1 r WHERE y = r.y));
ALTER TABLE rec1 ENABLE ROW LEVEL SECURITY;



CREATE TABLE rec2 (a integer, b integer);
CREATE POLICY r1_2 ON rec1 USING (x = (SELECT a FROM rec2 WHERE b = y));
CREATE POLICY r2 ON rec2 USING (a = (SELECT x FROM rec1 WHERE y = b));
ALTER TABLE rec2 ENABLE ROW LEVEL SECURITY;




CREATE VIEW rec1v AS SELECT * FROM rec1;
CREATE VIEW rec2v AS SELECT * FROM rec2;

CREATE POLICY r1_3 ON rec1 USING (x = (SELECT a FROM rec2v WHERE b = y));
CREATE POLICY r2_3 ON rec2 USING (a = (SELECT x FROM rec1v WHERE y = b));


CREATE VIEW rec1v_2 WITH (security_barrier) AS SELECT * FROM rec1;
CREATE VIEW rec2v_2 WITH (security_barrier) AS SELECT * FROM rec2;

CREATE POLICY r1_4 ON rec1 USING (x = (SELECT a FROM rec2v_2 WHERE b = y));
CREATE POLICY r2_2 ON rec2 USING (a = (SELECT x FROM rec1v_2 WHERE y = b));


CREATE TABLE s1 (a int, b text);
INSERT INTO s1 (SELECT x, md5(x::text) FROM generate_series(-10,10) x);

CREATE TABLE s2 (x int, y text);
INSERT INTO s2 (SELECT x, md5(x::text) FROM generate_series(-6,6) x);

GRANT SELECT ON s1, s2 TO regress_rls_bob;

CREATE POLICY p1 ON s1 USING (a in (select x from s2 where y like '%2f%'));
CREATE POLICY p2 ON s2 USING (x in (select a from s1 where b like '%22%'));
CREATE POLICY p3 ON s1 FOR INSERT WITH CHECK (a = (SELECT a FROM s1));

ALTER TABLE s1 ENABLE ROW LEVEL SECURITY;
ALTER TABLE s2 ENABLE ROW LEVEL SECURITY;


CREATE VIEW v2 AS SELECT * FROM s2 WHERE y like '%af%';


CREATE POLICY p1_2 ON s1 USING (a in (select x from v2)); -- using VIEW in RLS policy



CREATE POLICY p2_2 ON s2 USING (x in (select a from s1 where b like '%d2%'));



--
-- S.b. view on top of Row-level security
--

CREATE TABLE b1 (a int, b text);
INSERT INTO b1 (SELECT x, md5(x::text) FROM generate_series(-10,10) x);

CREATE POLICY p1 ON b1 USING (a % 2 = 0);
ALTER TABLE b1 ENABLE ROW LEVEL SECURITY;
GRANT ALL ON b1 TO regress_rls_bob;


CREATE VIEW bv1 WITH (security_barrier) AS SELECT * FROM b1 WHERE a > 0 WITH CHECK OPTION;
GRANT ALL ON bv1 TO regress_rls_carol;





CREATE POLICY p1_3 ON document FOR SELECT USING (true);
CREATE POLICY p2_3 ON document FOR INSERT WITH CHECK (dauthor = current_user);
CREATE POLICY p3_3 ON document FOR UPDATE
  USING (cid = (SELECT cid from category WHERE cname = 'novel'))
  WITH CHECK (dauthor = current_user);

CREATE POLICY p3_with_default ON document FOR UPDATE
  USING (cid = (SELECT cid from category WHERE cname = 'novel'));

--
-- Test ALL policies with ON CONFLICT DO UPDATE (much the same as existing UPDATE
-- tests)
--
CREATE POLICY p3_with_all ON document FOR ALL
  USING (cid = (SELECT cid from category WHERE cname = 'novel'))
  WITH CHECK (dauthor = current_user);


--
-- MERGE
--



ALTER TABLE document ADD COLUMN dnotes text DEFAULT '';
-- all documents are readable
CREATE POLICY p1_4 ON document FOR SELECT USING (true);
-- one may insert documents only authored by them
CREATE POLICY p2_4 ON document FOR INSERT WITH CHECK (dauthor = current_user);
-- one may only update documents in 'novel' category
CREATE POLICY p3_4 ON document FOR UPDATE
  USING (cid = (SELECT cid from category WHERE cname = 'novel'))
  WITH CHECK (dauthor = current_user);
-- one may only delete documents in 'manga' category
CREATE POLICY p4_4 ON document FOR DELETE
  USING (cid = (SELECT cid from category WHERE cname = 'manga'));


--
-- ROLE/GROUP
--

CREATE TABLE z1 (a int, b text);
CREATE TABLE z2 (a int, b text);

GRANT SELECT ON z1,z2 TO regress_rls_group1, regress_rls_group2,
    regress_rls_bob, regress_rls_carol;

INSERT INTO z1 VALUES
    (1, 'aba'),
    (2, 'bbb'),
    (3, 'ccc'),
    (4, 'dad');

CREATE POLICY p1 ON z1 TO regress_rls_group1 USING (a % 2 = 0);
CREATE POLICY p2 ON z1 TO regress_rls_group2 USING (a % 2 = 1);

ALTER TABLE z1 ENABLE ROW LEVEL SECURITY;

--
-- Views should follow policy for view owner.
--
-- View and Table owner are the same.

CREATE VIEW rls_view_4 AS SELECT * FROM z1 WHERE f_leak(b);
GRANT SELECT ON rls_view_4 TO regress_rls_bob;


-- Query as role that is not the owner of the table or view with permissions.

GRANT SELECT ON rls_view_4 TO regress_rls_carol;


-- Policy requiring access to another table.

CREATE TABLE z1_blacklist (a int);
INSERT INTO z1_blacklist VALUES (3), (4);
CREATE POLICY p3 ON z1 AS RESTRICTIVE USING (a NOT IN (SELECT a FROM z1_blacklist));

--
-- Security invoker views should follow policy for current user.
--
-- View and table owner are the same.

CREATE VIEW rls_view_2 AS
  SELECT * FROM z1 WHERE f_leak(b);
GRANT SELECT ON rls_view_2 TO regress_rls_bob;
GRANT SELECT ON rls_view_2 TO regress_rls_carol;



CREATE VIEW rls_view_3 AS
    SELECT * FROM z1 WHERE f_leak(b);
GRANT SELECT ON rls_view_3 TO regress_rls_alice;
GRANT SELECT ON rls_view_3 TO regress_rls_carol;

-- Policy requiring access to another table.

CREATE POLICY p3_2 ON z1 AS RESTRICTIVE USING (a NOT IN (SELECT a FROM z1_blacklist));

-- Query as role that is not the owner of the table or view with permissions.

GRANT SELECT ON z1_blacklist TO regress_rls_carol;



--
-- Command specific
--


CREATE TABLE x1 (a int, b text, c text);
GRANT ALL ON x1 TO PUBLIC;

INSERT INTO x1 VALUES
    (1, 'abc', 'regress_rls_bob'),
    (2, 'bcd', 'regress_rls_bob'),
    (3, 'cde', 'regress_rls_carol'),
    (4, 'def', 'regress_rls_carol'),
    (5, 'efg', 'regress_rls_bob'),
    (6, 'fgh', 'regress_rls_bob'),
    (7, 'fgh', 'regress_rls_carol'),
    (8, 'fgh', 'regress_rls_carol');

CREATE POLICY p0 ON x1 FOR ALL USING (c = current_user);
CREATE POLICY p1 ON x1 FOR SELECT USING (a % 2 = 0);
CREATE POLICY p2 ON x1 FOR INSERT WITH CHECK (a % 2 = 1);
CREATE POLICY p3 ON x1 FOR UPDATE USING (a % 2 = 0);
CREATE POLICY p4 ON x1 FOR DELETE USING (a < 8);

ALTER TABLE x1 ENABLE ROW LEVEL SECURITY;


CREATE TABLE y1 (a int, b text);
CREATE TABLE y2 (a int, b text);

GRANT ALL ON y1, y2 TO regress_rls_bob;

CREATE POLICY p1 ON y1 FOR ALL USING (a % 2 = 0);
CREATE POLICY p2 ON y1 FOR SELECT USING (a > 2);
CREATE POLICY p1_2 ON y1 FOR SELECT USING (a % 2 = 1);  --fail
CREATE POLICY p1 ON y2 FOR ALL USING (a % 2 = 0);  --OK

ALTER TABLE y1 ENABLE ROW LEVEL SECURITY;
ALTER TABLE y2 ENABLE ROW LEVEL SECURITY;

--
-- Expression structure with SBV
--
-- Create view as table owner.  RLS should NOT be applied.

CREATE VIEW rls_sbv WITH (security_barrier) AS
    SELECT * FROM y1 WHERE f_leak(b);


-- Create view as role that does not own table.  RLS should be applied.

CREATE VIEW rls_sbv_2 WITH (security_barrier) AS
  SELECT * FROM y1 WHERE f_leak(b);

--
-- Expression structure
--

INSERT INTO y2 (SELECT x, md5(x::text) FROM generate_series(0,20) x);
CREATE POLICY p2 ON y2 USING (a % 3 = 0);
CREATE POLICY p3 ON y2 USING (a % 4 = 0);




CREATE TABLE test_qual_pushdown (
    abc text
);

INSERT INTO test_qual_pushdown VALUES ('abc'),('def');




CREATE TABLE t1_2 (a integer);

GRANT SELECT ON t1_2 TO regress_rls_bob, regress_rls_carol;

CREATE POLICY p1 ON t1_2 TO regress_rls_bob USING ((a % 2) = 0);
CREATE POLICY p2 ON t1_2 TO regress_rls_carol USING ((a % 4) = 0);

ALTER TABLE t1_2 ENABLE ROW LEVEL SECURITY;



--
-- CTE and RLS
--

CREATE TABLE t1_3 (a integer, b text);
CREATE POLICY p1 ON t1_3 USING (a % 2 = 0);

ALTER TABLE t1_3 ENABLE ROW LEVEL SECURITY;

GRANT ALL ON t1_3 TO regress_rls_bob;

INSERT INTO t1_3 (SELECT x, md5(x::text) FROM generate_series(0,20) x);




--
-- Check INSERT SELECT
--

CREATE TABLE t2_3 (a integer, b text);
GRANT ALL ON t2_3 TO public;

--
-- Table inheritance and RLS policy
--

CREATE TABLE t3_3 (id int not null primary key, c text, b text, a int);

ALTER TABLE t3_3 INHERIT t1_3;
GRANT ALL ON t3_3 TO public;


CREATE POLICY p1_2 ON t1_3 FOR ALL TO PUBLIC USING (a % 2 = 0); -- be even number
CREATE POLICY p2 ON t2_3 FOR ALL TO PUBLIC USING (a % 2 = 1); -- be odd number

ALTER TABLE t1_3 ENABLE ROW LEVEL SECURITY;
ALTER TABLE t2_3 ENABLE ROW LEVEL SECURITY;



CREATE ROLE regress_rls_eve;
CREATE ROLE regress_rls_frank;
CREATE TABLE tbl1 (c) AS VALUES ('bar'::text);
GRANT SELECT ON TABLE tbl1 TO regress_rls_eve;
CREATE POLICY P ON tbl1 TO regress_rls_eve, regress_rls_frank USING (true);

CREATE TABLE t (c int);
CREATE POLICY p ON t USING (c % 2 = 1);
ALTER TABLE t ENABLE ROW LEVEL SECURITY;


-- Non-target relations are only subject to SELECT policies
--

CREATE TABLE r1 (a int primary key);
CREATE TABLE r2 (a int);
INSERT INTO r1 VALUES (10), (20);
INSERT INTO r2 VALUES (10), (20);

GRANT ALL ON r1, r2 TO regress_rls_bob;

CREATE POLICY p1 ON r1 USING (true);
ALTER TABLE r1 ENABLE ROW LEVEL SECURITY;

CREATE POLICY p1 ON r2 FOR SELECT USING (true);
CREATE POLICY p2 ON r2 FOR INSERT WITH CHECK (false);
CREATE POLICY p3 ON r2 FOR UPDATE USING (false);
CREATE POLICY p4 ON r2 FOR DELETE USING (false);
ALTER TABLE r2 ENABLE ROW LEVEL SECURITY;




--
-- FORCE ROW LEVEL SECURITY applies RLS to owners too
--

SET row_security = on;
CREATE TABLE r1_2 (a int);

CREATE POLICY p1 ON r1_2 USING (false);
ALTER TABLE r1_2 ENABLE ROW LEVEL SECURITY;
ALTER TABLE r1_2 FORCE ROW LEVEL SECURITY;



--
-- FORCE ROW LEVEL SECURITY does not break RI
--

SET row_security = on;
CREATE TABLE r1_3 (a int PRIMARY KEY);
CREATE TABLE r2_3 (a int REFERENCES r1); --@
INSERT INTO r1_3 VALUES (10), (20);
INSERT INTO r2_3 VALUES (10), (20);

-- Create policies on r2 which prevent the
-- owner from seeing any rows, but RI should
-- still see them.
CREATE POLICY p1 ON r2_3 USING (false);
ALTER TABLE r2_3 ENABLE ROW LEVEL SECURITY;
ALTER TABLE r2_3 FORCE ROW LEVEL SECURITY;


-- Change r1 to not allow rows to be seen
CREATE POLICY p1 ON r1_3 USING (false);
ALTER TABLE r1_3 ENABLE ROW LEVEL SECURITY;
ALTER TABLE r1_3 FORCE ROW LEVEL SECURITY;


-- Ensure cascaded DELETE works
CREATE TABLE r1_4 (a int PRIMARY KEY);
CREATE TABLE r2_4 (a int REFERENCES r1 ON DELETE CASCADE);
INSERT INTO r1_4 VALUES (10), (20);
INSERT INTO r2_4 VALUES (10), (20);

-- Create policies on r2 which prevent the
-- owner from seeing any rows, but RI should
-- still see them.
CREATE POLICY p1 ON r2_4 USING (false);
ALTER TABLE r2_4 ENABLE ROW LEVEL SECURITY;
ALTER TABLE r2_4 FORCE ROW LEVEL SECURITY;



-- Ensure cascaded UPDATE works
CREATE TABLE r1_5 (a int PRIMARY KEY);
CREATE TABLE r2_5 (a int REFERENCES r1 ON UPDATE CASCADE);
INSERT INTO  r1_5 VALUES (10), (20);
INSERT INTO  r2_5 VALUES (10), (20);

-- Create policies on r2 which prevent the
-- owner from seeing any rows, but RI should
-- still see them.
CREATE POLICY p1 ON r2_5 USING (false);
ALTER TABLE r2_5 ENABLE ROW LEVEL SECURITY;
ALTER TABLE r2_5 FORCE ROW LEVEL SECURITY;


-- DROP OWNED BY testing


CREATE ROLE regress_rls_dob_role1;
CREATE ROLE regress_rls_dob_role2;

CREATE TABLE dob_t1 (c1 int);
CREATE TABLE dob_t2 (c1 int) PARTITION BY RANGE (c1);

CREATE POLICY p1 ON dob_t1 TO regress_rls_dob_role1 USING (true);

CREATE POLICY p1_2 ON dob_t1 TO regress_rls_dob_role1,regress_rls_dob_role2 USING (true);


-- same cases with duplicate polroles entries
CREATE POLICY p1_3 ON dob_t1 TO regress_rls_dob_role1,regress_rls_dob_role1 USING (true);

CREATE POLICY p1_4 ON dob_t1 TO regress_rls_dob_role1,regress_rls_dob_role1,regress_rls_dob_role2 USING (true);
CREATE POLICY p1 ON dob_t2 TO regress_rls_dob_role1,regress_rls_dob_role2 USING (true);

-- Bug #15708: view + table with RLS should check policies as view owner
CREATE TABLE ref_tbl (a int);
INSERT INTO ref_tbl VALUES (1);

CREATE TABLE rls_tbl (a int);
INSERT INTO rls_tbl VALUES (10);
ALTER TABLE rls_tbl ENABLE ROW LEVEL SECURITY;
CREATE POLICY p1 ON rls_tbl USING (EXISTS (SELECT 1 FROM ref_tbl));

CREATE VIEW rls_view AS SELECT * FROM rls_tbl;
ALTER VIEW rls_view OWNER TO regress_rls_bob;
GRANT SELECT ON rls_view TO regress_rls_alice;



CREATE TABLE rls_tbl_2 (a int);
INSERT INTO rls_tbl_2 SELECT x/10 FROM generate_series(1, 100) x;


ALTER TABLE rls_tbl_2 ENABLE ROW LEVEL SECURITY;
GRANT SELECT ON rls_tbl_2 TO regress_rls_alice;


CREATE FUNCTION op_leak(int, int) RETURNS bool
    AS 'BEGIN RAISE NOTICE ''op_leak => %, %'', $1, $2; RETURN $1 < $2; END'
    LANGUAGE plpgsql;
CREATE OPERATOR <<< (procedure = op_leak, leftarg = int, rightarg = int,
                     restrict = scalarltsel);


CREATE TABLE rls_tbl_3 (a int, b int, c int);
CREATE POLICY p1 ON rls_tbl_3 USING (rls_tbl_3 >= ROW(1,1,1));

ALTER TABLE rls_tbl_3 ENABLE ROW LEVEL SECURITY;
ALTER TABLE rls_tbl_3 FORCE ROW LEVEL SECURITY;

INSERT INTO rls_tbl_3 SELECT 10, 20, 30;
RESET search_path;
