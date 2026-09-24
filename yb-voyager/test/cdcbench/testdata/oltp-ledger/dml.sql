-- Double-entry ledger: append-only journal (unique entry numbers) + hot
-- balance rows (no UK). 5000 x (2 inserts + 2 updates) = 20000 events, zero
-- conflicts.
DO $$
BEGIN
    FOR i IN 1..5000 LOOP
        INSERT INTO journal (id, entry_no, account_id, amount) VALUES (2*i - 1, 'j_' || (2*i - 1), 1 + (i % 100), -10);
        INSERT INTO journal (id, entry_no, account_id, amount) VALUES (2*i,     'j_' || (2*i),     1 + ((i + 50) % 100), 10);
        UPDATE balances SET balance = balance - 10 WHERE id = 1 + (i % 100);
        UPDATE balances SET balance = balance + 10 WHERE id = 1 + ((i + 50) % 100);
    END LOOP;
END $$;
