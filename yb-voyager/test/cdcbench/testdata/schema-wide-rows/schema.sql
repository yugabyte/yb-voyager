DROP TABLE IF EXISTS wide_table CASCADE;
CREATE TABLE wide_table (
    id    int PRIMARY KEY,
    email text CONSTRAINT wide_table_email_uk UNIQUE,
    c0 text, c1 text, c2 text, c3 text, c4 text, c5 text, c6 text, c7 text, c8 text, c9 text, c10 text, c11 text, c12 text, c13 text, c14 text, c15 text, c16 text, c17 text, c18 text, c19 text, c20 text, c21 text, c22 text, c23 text, c24 text, c25 text, c26 text, c27 text, c28 text, c29 text, c30 text, c31 text, c32 text, c33 text, c34 text, c35 text, c36 text, c37 text, c38 text, c39 text, c40 text, c41 text, c42 text, c43 text, c44 text, c45 text, c46 text, c47 text, c48 text, c49 text
);
ALTER TABLE wide_table REPLICA IDENTITY FULL;
