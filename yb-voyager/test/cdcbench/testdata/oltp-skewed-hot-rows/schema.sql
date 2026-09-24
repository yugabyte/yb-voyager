DROP TABLE IF EXISTS uk_table CASCADE;
CREATE TABLE uk_table (
    id    int PRIMARY KEY,
    email text CONSTRAINT uk_table_email_uk UNIQUE,
    c0 text, c1 text, c2 text, c3 text, c4 text, c5 text,
    c6 text, c7 text, c8 text, c9 text, c10 text, c11 text
);
ALTER TABLE uk_table REPLICA IDENTITY FULL;
