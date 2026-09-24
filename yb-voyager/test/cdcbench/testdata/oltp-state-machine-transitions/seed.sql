-- every payment starts with an initial transition holding most_recent
TRUNCATE payment_transitions;
INSERT INTO payment_transitions
SELECT i, i, 0, 'created', md5(i::text), true FROM generate_series(1, 2000) i;
