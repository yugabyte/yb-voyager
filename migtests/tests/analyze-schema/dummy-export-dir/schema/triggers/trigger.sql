CREATE TRIGGER transfer_insert
    AFTER INSERT ON transfer
    REFERENCING NEW TABLE AS inserted
    FOR EACH STATEMENT
    EXECUTE FUNCTION check_transfer_balances_to_zero();

CREATE CONSTRAINT TRIGGER some_trig
   AFTER DELETE ON xyz_schema.abc
   DEFERRABLE INITIALLY DEFERRED
   FOR EACH ROW EXECUTE PROCEDURE xyz_schema.some_trig();

CREATE TRIGGER emp_trig
	COMPOUND INSERT ON emp FOR EACH ROW
	EXECUTE PROCEDURE trigger_fct_emp_trig();

CREATE TRIGGER test
    INSERT on test for each ROW
    EXECUTE PROCEDURE JSON_ARRAYAGG(trunc(b, 2) order by t desc);