-- Job queue / outbox churn: insert -> claim -> complete -> delete, short row
-- lifetime, unique job keys never reused. 5000 x 4 = 20000 events, zero
-- conflicts.
DO $$
BEGIN
    FOR i IN 1..5000 LOOP
        INSERT INTO jobs (id, job_key, status, c0) VALUES (i, 'job_' || i, 'queued', md5(i::text));
        UPDATE jobs SET status = 'running' WHERE id = i;
        UPDATE jobs SET status = 'done'    WHERE id = i;
        DELETE FROM jobs WHERE id = i;
    END LOOP;
END $$;
