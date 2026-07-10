DROP TABLE IF EXISTS accounts2 CASCADE;
CREATE TABLE accounts2 (
    id int PRIMARY KEY,
    email    text CONSTRAINT accounts2_email_uk UNIQUE,
    username text CONSTRAINT accounts2_username_uk UNIQUE,
    c0 text
);
ALTER TABLE accounts2 REPLICA IDENTITY FULL;
