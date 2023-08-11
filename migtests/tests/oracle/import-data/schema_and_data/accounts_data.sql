-- Generate and insert 1 million rows into the ACCOUNTS table
DECLARE
    v_counter NUMBER := 1;
BEGIN
    FOR v_counter IN 1..100000 LOOP
        INSERT INTO ACCOUNTS (
            block,
            address,
            dc_balance,
            dc_nonce,
            security_balance,
            security_nonce,
            balance,
            nonce,
            staked_balance
        ) VALUES (
            v_counter,                       -- block
            'Address ' || v_counter,         -- address
            DBMS_RANDOM.VALUE,               -- dc_balance
            DBMS_RANDOM.VALUE,               -- dc_nonce
            DBMS_RANDOM.VALUE,               -- security_balance
            DBMS_RANDOM.VALUE,               -- security_nonce
            DBMS_RANDOM.VALUE,               -- balance
            DBMS_RANDOM.VALUE,               -- nonce
            DBMS_RANDOM.VALUE * 10           -- staked_balance
        );
    END LOOP;
    
    COMMIT; -- Committing after the loop to finalize the changes
END;
/
