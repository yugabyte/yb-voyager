-- Exercises all four conflict types documented in conflictDetectionCache.go:
-- UPDATE-INSERT, UPDATE-UPDATE, DELETE-INSERT, DELETE-UPDATE.
-- 2500 blocks x 8 events = 20000 events; block b uses seeded rows 6b-5..6b.
DO $$
BEGIN
    FOR b IN 1..2500 LOOP
        -- UPDATE-INSERT: update frees the email, insert takes it
        UPDATE uk_table SET email = 'docx_' || (6*b - 5) WHERE id = 6*b - 5;
        INSERT INTO uk_table (id, email, c0) VALUES (20000 + 2*b - 1, 'doc_' || (6*b - 5), 'x');
        -- UPDATE-UPDATE: one row takes the email another row just released
        UPDATE uk_table SET email = 'docx_' || (6*b - 4) WHERE id = 6*b - 4;
        UPDATE uk_table SET email = 'doc_'  || (6*b - 4) WHERE id = 6*b - 3;
        -- DELETE-INSERT: delete frees the email, insert takes it
        DELETE FROM uk_table WHERE id = 6*b - 2;
        INSERT INTO uk_table (id, email, c0) VALUES (20000 + 2*b, 'doc_' || (6*b - 2), 'y');
        -- DELETE-UPDATE: delete frees the email, another row's update takes it
        DELETE FROM uk_table WHERE id = 6*b - 1;
        UPDATE uk_table SET email = 'doc_' || (6*b - 1) WHERE id = 6*b;
    END LOOP;
END $$;
