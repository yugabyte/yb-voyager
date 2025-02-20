CREATE OR REPLACE FUNCTION create_and_populate_tables(table_prefix TEXT, partitions INT)
RETURNS VOID
LANGUAGE plpgsql
AS $$
DECLARE
    i INT;
    partition_table TEXT;
BEGIN
    -- Loop to create multiple partition tables
    FOR i IN 1..partitions LOOP
        partition_table := table_prefix || '_part_' || i;

        -- Dynamic SQL to create each partition table
        EXECUTE format('
            CREATE TABLE IF NOT EXISTS %I (
                id SERIAL PRIMARY KEY,
                name TEXT,
                amount NUMERIC
            )', partition_table);
        RAISE NOTICE 'Table % created', partition_table;

        -- Dynamic SQL to insert data into each partition table
        EXECUTE format('
            INSERT INTO %I (name, amount)
            SELECT name, amount
            FROM source_data
            WHERE id %% %L = %L', partition_table, partitions, i - 1);
        RAISE NOTICE 'Data inserted into table %', partition_table;
    END LOOP;

	PERFORM pg_advisory_lock(sender_id);
	PERFORM pg_advisory_lock(receiver_id);

	-- Check if the sender has enough balance
	IF (SELECT balance FROM accounts WHERE account_id = sender_id) < transfer_amount THEN
		RAISE EXCEPTION 'Insufficient funds';
		-- Deduct the amount from the sender's account SOME DUMMY code to understand nested if structure
		UPDATE accounts 
		SET balance = balance - transfer_amount 
		WHERE account_id = sender_id;

		-- Add the amount to the receiver's account
		UPDATE accounts 
		SET balance = balance + transfer_amount 
		WHERE account_id = receiver_id;
		IF (SELECT balance FROM accounts WHERE account_id = sender_id) < transfer_amount THEN
			-- Release the advisory locks (optional, as they will be released at the end of the transaction)
			PERFORM pg_advisory_unlock(sender_id);
			PERFORM pg_advisory_unlock(receiver_id);
		END IF;
	END IF;

	-- Deduct the amount from the sender's account
	UPDATE accounts 
	SET balance = balance - transfer_amount 
	WHERE account_id = sender_id;

	-- Add the amount to the receiver's account
	UPDATE accounts 
	SET balance = balance + transfer_amount 
	WHERE account_id = receiver_id;

	-- Commit the transaction
	COMMIT;

	-- Release the advisory locks (optional, as they will be released at the end of the transaction)
	PERFORM pg_advisory_unlock(sender_id);
	PERFORM pg_advisory_unlock(receiver_id);

	 -- Conditional logic
	IF balance >= withdrawal THEN
		RAISE NOTICE 'Sufficient balance, processing withdrawal.';
		-- Add the amount to the receiver's account
		UPDATE accounts SET balance = balance + amount WHERE account_id = receiver;
	ELSIF balance > 0 AND balance < withdrawal THEN
		RAISE NOTICE 'Insufficient balance, consider reducing the amount.';
		-- Add the amount to the receiver's account
		UPDATE accounts SET balance = balance + amount WHERE account_id = receiver;
	ELSE
		-- Add the amount to the receiver's account
		UPDATE accounts SET balance = balance + amount WHERE account_id = receiver;
		RAISE NOTICE 'No funds available.';
	END IF;

    SELECT id, xpath('/person/name/text()', data) AS name FROM test_xml_type;

    SELECT * FROM employees e WHERE e.xmax = (SELECT MAX(xmax) FROM employees WHERE department = e.department);

END;
$$;

CREATE FUNCTION public.get_employeee_salary(emp_id integer) RETURNS numeric
    LANGUAGE plpgsql
    AS $$
DECLARE
    emp_salary employees.salary%TYPE;  -- Declare a variable with the same type as employees.salary
BEGIN
    SELECT salary INTO emp_salary
    FROM employees
    WHERE employee_id = emp_id;
    RETURN emp_salary;
END;
$$;

CREATE OR REPLACE FUNCTION calculate_tax(salary_amount NUMERIC) RETURNS NUMERIC AS $$
DECLARE
    tax_rate employees.tax_rate%TYPE; -- Inherits type from employees.tax_rate column
    tax_amount NUMERIC;
BEGIN
    -- Assign a value to the variable
    SELECT tax_rate INTO tax_rate FROM employees WHERE id = 1;
    
    -- Use the variable in a calculation
    tax_amount := salary_amount * tax_rate;
    RETURN tax_amount;
END;
$$ LANGUAGE plpgsql;

CREATE OR REPLACE FUNCTION log_salary_change() RETURNS TRIGGER AS $$
DECLARE
    old_salary employees.salary%TYPE; -- Matches the type of the salary column
    new_salary employees.salary%TYPE;
BEGIN
    old_salary := OLD.salary;
    new_salary := NEW.salary;

    IF new_salary <> old_salary THEN
        INSERT INTO salary_log(employee_id, old_salary, new_salary, changed_at)
        VALUES (NEW.id, old_salary, new_salary, now());
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER salary_update_trigger
AFTER UPDATE OF salary ON employees
FOR EACH ROW EXECUTE FUNCTION log_salary_change();

CREATE OR REPLACE FUNCTION get_employee_details(emp_id employees.id%Type) 
RETURNS public.employees.name%Type AS $$ 
DECLARE
    employee_name employees.name%TYPE;
BEGIN
    SELECT name INTO employee_name FROM employees WHERE id = emp_id;
    RETURN employee_name;
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION list_high_earners(threshold NUMERIC) RETURNS VOID AS $$
DECLARE
    emp_name employees.name%TYPE;
    emp_salary employees.salary%TYPE;
BEGIN
    FOR emp_name, emp_salary IN 
        SELECT name, salary FROM employees WHERE salary > threshold
    LOOP
        RAISE NOTICE 'Employee: %, Salary: %', emp_name, emp_salary;
    END LOOP;
END;
$$ LANGUAGE plpgsql;


CREATE OR REPLACE FUNCTION copy_high_earners(threshold NUMERIC) RETURNS VOID AS $$
DECLARE
    temp_salary employees.salary%TYPE;
BEGIN
    CREATE TEMP TABLE temp_high_earners AS
    SELECT * FROM employees WHERE salary > threshold;

    FOR temp_salary IN SELECT salary FROM temp_high_earners LOOP
        RAISE NOTICE 'High earner salary: %', temp_salary;
    END LOOP;
END;
$$ LANGUAGE plpgsql;

CREATE FUNCTION public.insert_non_decimal() RETURNS void
    LANGUAGE plpgsql
    AS $$
BEGIN
    -- Create a table for demonstration
    CREATE TEMP TABLE non_decimal_table (
        id SERIAL,
        binary_value INTEGER,
        octal_value INTEGER,
        hex_value INTEGER
    );
    SELECT 5678901234, 0x1527D27F2, 0o52237223762, 0b101010010011111010010011111110010;
    -- Insert values into the table
    --not reported as parser converted these values to decimal ones while giving parseTree
    INSERT INTO non_decimal_table (binary_value, octal_value, hex_value)
    VALUES (0b1010, 0o012, 0xA); -- Binary (10), Octal (10), Hexadecimal (10)
    RAISE NOTICE 'Row inserted with non-decimal integers.';
END;
$$;

CREATE FUNCTION public.asterisks(n integer) RETURNS SETOF text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    BEGIN ATOMIC
 SELECT repeat('*'::text, g.g) AS repeat
    FROM generate_series(1, asterisks.n) g(g);
END; 
--TODO fix our parser for this case - splitting it into two "CREATE FUNCTION ...FROM generate_series(1, asterisks.n) g(g);" "END;

CREATE FUNCTION add(int, int) RETURNS int IMMUTABLE PARALLEL SAFE BEGIN ATOMIC; SELECT $1 + $2; END;

CREATE FUNCTION public.asterisks1(n integer) RETURNS text
    LANGUAGE sql IMMUTABLE STRICT PARALLEL SAFE
    RETURN repeat('*'::text, n);
