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