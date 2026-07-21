DROP TABLE IF EXISTS journal, balances CASCADE;
CREATE TABLE journal  (id int PRIMARY KEY, entry_no text CONSTRAINT journal_entry_uk UNIQUE, account_id int, amount int);
CREATE TABLE balances (id int PRIMARY KEY, balance int, c0 text);
ALTER TABLE journal  REPLICA IDENTITY FULL;
ALTER TABLE balances REPLICA IDENTITY FULL;
