-- https://www.postgresql.org/docs/current/sql-createindex.html
CREATE SCHEMA idx_ex;
CREATE table idx_ex.films(id integer primary key, title text, director text, rating int4, code text);

CREATE UNIQUE INDEX title_idx_u1 ON idx_ex.films (title);
CREATE UNIQUE INDEX title_idx_u2 ON idx_ex.films (title) INCLUDE (director, rating);
CREATE INDEX title_idx_with_duplicates ON idx_ex.films (title) WITH (deduplicate_items = off);
CREATE INDEX title_idx_lower ON idx_ex.films ((lower(title)));
-- CREATE INDEX title_idx_german ON films (title COLLATE "de_DE");
CREATE INDEX title_idx_nulls_low ON idx_ex.films (title NULLS FIRST);
CREATE UNIQUE INDEX title_idx ON idx_ex.films (title) WITH (fillfactor = 70);
CREATE INDEX gin_idx ON idx_ex.films USING GIN (to_tsvector('english', title)) WITH (fastupdate = off);
