-- Interleaved 60% insert / 30% update / 10% delete on the UK table,
-- 20k events, no conflicts. Per iteration: 6 inserts (fresh ids/emails),
-- 3 updates (ids 1..6000, fresh emails), 1 delete (ids 6001..8000, disjoint
-- from the updated ids). All email namespaces are disjoint.
DO $$
BEGIN
    FOR i IN 1..2000 LOOP
        INSERT INTO uk_table
        SELECT 20000 + (i-1)*6 + j,
               'mix_ins_' || ((i-1)*6 + j),
               md5((i*100 + j)::text || 'a'), md5((i*100 + j)::text || 'b'), md5((i*100 + j)::text || 'c'),
               md5((i*100 + j)::text || 'd'), md5((i*100 + j)::text || 'e'), md5((i*100 + j)::text || 'f'),
               md5((i*100 + j)::text || 'g'), md5((i*100 + j)::text || 'h'), md5((i*100 + j)::text || 'i'),
               md5((i*100 + j)::text || 'j'), md5((i*100 + j)::text || 'k'), md5((i*100 + j)::text || 'l')
        FROM generate_series(1, 6) j;

        UPDATE uk_table
        SET email = 'mix_upd_' || id, c0 = md5(id::text || 'mix')
        WHERE id IN ((i-1)*3 + 1, (i-1)*3 + 2, (i-1)*3 + 3);

        DELETE FROM uk_table WHERE id = 6000 + i;
    END LOOP;
END $$;
