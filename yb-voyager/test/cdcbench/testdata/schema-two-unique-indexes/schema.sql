DROP TABLE IF EXISTS accounts CASCADE;
CREATE TABLE accounts (
    id       int PRIMARY KEY,
    email    text CONSTRAINT accounts_email_uk UNIQUE,
    username text CONSTRAINT accounts_username_uk UNIQUE,
    c0 text, c1 text, c2 text, c3 text
);
ALTER TABLE accounts REPLICA IDENTITY FULL;
