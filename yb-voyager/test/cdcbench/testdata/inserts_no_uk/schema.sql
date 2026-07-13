-- Control table WITHOUT any unique index: the conflict-detection machinery
-- never engages, measuring the pipeline ceiling.
DROP TABLE IF EXISTS no_uk_table CASCADE;
CREATE TABLE no_uk_table (
    id int PRIMARY KEY,
    c0 text, c1 text, c2 text, c3 text, c4 text, c5 text,
    c6 text, c7 text, c8 text, c9 text, c10 text, c11 text
);
ALTER TABLE no_uk_table REPLICA IDENTITY FULL;
