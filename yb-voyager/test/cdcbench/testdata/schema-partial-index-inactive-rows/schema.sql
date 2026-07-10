DROP TABLE IF EXISTS soft_users CASCADE;
CREATE TABLE soft_users (id int PRIMARY KEY, email text, deleted boolean, payload text);
CREATE UNIQUE INDEX soft_users_email_uk ON soft_users (email) WHERE NOT deleted;
ALTER TABLE soft_users REPLICA IDENTITY FULL;
