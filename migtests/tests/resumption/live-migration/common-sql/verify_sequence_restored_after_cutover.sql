-- Honest insert: relies on cutover_table's SERIAL default (nextval) instead of
-- supplying an id. If the cutover did not restore the sequence past the migrated
-- rows, nextval returns an already-used id and this fails with a duplicate-key
-- error, surfacing the missed sequence bump. Run against the DB just cut over to.
INSERT INTO public.cutover_table(status)
VALUES ('verify sequence restored after cutover');
