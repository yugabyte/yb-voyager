CREATE TRIGGER transfer_insert
    AFTER INSERT ON transfer
    REFERENCING NEW TABLE AS inserted
    FOR EACH STATEMENT
    EXECUTE FUNCTION check_transfer_balances_to_zero();

CREATE CONSTRAINT TRIGGER some_trig
   AFTER DELETE ON xyz_schema.abc
   DEFERRABLE INITIALLY DEFERRED
   FOR EACH ROW EXECUTE PROCEDURE xyz_schema.some_trig();

-- these two are not PG compatible so we won't be reporting full name (trigger_name ON table_name) 
CREATE TRIGGER emp_trig
	COMPOUND INSERT ON emp FOR EACH ROW
	EXECUTE PROCEDURE trigger_fct_emp_trig();

CREATE TRIGGER test
    INSTEAD OF INSERT on test for each ROW
    EXECUTE PROCEDURE JSON_ARRAYAGG(trunc(b, 2) order by t desc);

CREATE TRIGGER before_insert_or_delete_trigger
BEFORE INSERT OR DELETE ON public.range_columns_partition_test
FOR EACH STATEMENT
EXECUTE FUNCTION handle_insert_or_delete();

CREATE TRIGGER after_insert_or_delete_trigger
AFTER INSERT OR DELETE ON public.range_columns_partition_test
FOR EACH ROW
EXECUTE FUNCTION handle_insert_or_delete();

CREATE TRIGGER before_insert_or_delete_row_trigger
BEFORE INSERT OR DELETE ON public.range_columns_partition_test
FOR EACH ROW
EXECUTE FUNCTION handle_insert_or_delete();


CREATE TRIGGER before_insert_or_delete_row_trigger
BEFORE INSERT OR DELETE ON public.test_non_partition_before
FOR EACH ROW
EXECUTE FUNCTION handle_insert_or_delete();