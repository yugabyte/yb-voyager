-- setting variables for current session
SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;


CREATE TRIGGER check_balance_update BEFORE UPDATE OF balance ON trigger_test.accounts FOR EACH ROW EXECUTE FUNCTION trigger_test.check_account_update();


CREATE TRIGGER check_update BEFORE UPDATE ON trigger_test.accounts FOR EACH ROW EXECUTE FUNCTION trigger_test.check_account_update();


CREATE TRIGGER check_update_when_difft_balance BEFORE UPDATE ON trigger_test.accounts FOR EACH ROW WHEN ((old.balance IS DISTINCT FROM new.balance)) EXECUTE FUNCTION trigger_test.check_account_update();


CREATE TRIGGER log_update AFTER UPDATE ON trigger_test.accounts FOR EACH ROW WHEN ((old.* IS DISTINCT FROM new.*)) EXECUTE FUNCTION trigger_test.log_account_update();


CREATE TRIGGER view_insert INSTEAD OF INSERT ON trigger_test.accounts_view FOR EACH ROW EXECUTE FUNCTION trigger_test.view_insert_row();


