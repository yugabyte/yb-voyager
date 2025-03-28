create schema trigger_test;
-- https://www.postgresql.org/docs/current/sql-createtrigger.html#SQL-CREATETRIGGER-EXAMPLES
create table trigger_test.accounts (
  id INTEGER PRIMARY KEY
  , balance REAL
);

CREATE VIEW trigger_test.accounts_view AS (select * from trigger_test.accounts);

create table trigger_test.update_log (
  timestamp timestamp default now()
  , account_id INTEGER REFERENCES trigger_test.accounts(id)
  , CONSTRAINT update_log_pk PRIMARY KEY (timestamp, account_id)
);

CREATE OR REPLACE FUNCTION trigger_test.check_account_update()
  RETURNS trigger
  LANGUAGE plpgsql
  AS $$
    BEGIN
      RAISE NOTICE 'trigger_func(%) called: action = %, when = %, level = %, old = %, new = %',
                        TG_ARGV[0], TG_OP, TG_WHEN, TG_LEVEL, OLD, NEW;
      RETURN NULL;
    END;
  $$;

CREATE FUNCTION trigger_test.log_account_update()
  RETURNS trigger
  LANGUAGE plpgsql
  AS $$
    BEGIN
      INSERT INTO trigger_test.update_log(account_id) VALUES (1);
      RETURN NULL;
    END;
  $$;

CREATE FUNCTION trigger_test.view_insert_row()
  RETURNS trigger
  LANGUAGE plpgsql
  AS $$
    BEGIN
      INSERT INTO trigger_test.accounts(id, balance) VALUES (NEW.id, NEW.balance);
      RETURN NEW;
    END;
  $$;


CREATE TRIGGER check_update
    BEFORE UPDATE ON trigger_test.accounts
    FOR EACH ROW
    EXECUTE FUNCTION trigger_test.check_account_update();

CREATE TRIGGER check_balance_update
    BEFORE UPDATE OF balance ON trigger_test.accounts
    FOR EACH ROW
    EXECUTE FUNCTION trigger_test.check_account_update();

CREATE TRIGGER check_update_when_difft_balance
    BEFORE UPDATE ON trigger_test.accounts
    FOR EACH ROW
    WHEN (OLD.balance IS DISTINCT FROM NEW.balance)
    EXECUTE FUNCTION trigger_test.check_account_update();

CREATE TRIGGER log_update
    AFTER UPDATE ON trigger_test.accounts
    FOR EACH ROW
    WHEN (OLD.* IS DISTINCT FROM NEW.*)
    EXECUTE FUNCTION trigger_test.log_account_update();

CREATE TRIGGER view_insert
    INSTEAD OF INSERT ON trigger_test.accounts_view
    FOR EACH ROW
    EXECUTE FUNCTION trigger_test.view_insert_row();

-- CREATE TRIGGER paired_items_update
--     AFTER UPDATE ON paired_items
--     REFERENCING NEW TABLE AS newtab OLD TABLE AS oldtab
--     FOR EACH ROW
--     EXECUTE FUNCTION check_matching_pairs();
