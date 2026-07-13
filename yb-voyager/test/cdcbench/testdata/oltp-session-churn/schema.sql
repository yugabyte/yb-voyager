DROP TABLE IF EXISTS sessions CASCADE;
CREATE TABLE sessions (
    id    int PRIMARY KEY,
    token text CONSTRAINT sessions_token_uk UNIQUE,
    c0 text, c1 text
);
ALTER TABLE sessions REPLICA IDENTITY FULL;
