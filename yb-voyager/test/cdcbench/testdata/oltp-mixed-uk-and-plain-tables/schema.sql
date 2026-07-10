DROP TABLE IF EXISTS accounts, audit_log CASCADE;
CREATE TABLE accounts  (id int PRIMARY KEY, email text CONSTRAINT accounts_email_uk UNIQUE, c0 text);
CREATE TABLE audit_log (id int PRIMARY KEY, actor int, action text, c0 text);
ALTER TABLE accounts  REPLICA IDENTITY FULL;
ALTER TABLE audit_log REPLICA IDENTITY FULL;
