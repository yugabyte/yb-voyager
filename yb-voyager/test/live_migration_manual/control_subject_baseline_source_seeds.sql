-- Seed rows: **PostgreSQL (source) only.** Do not run on YugabyteDB (target);
-- snapshot / import will populate the target.

\connect schema_drift

BEGIN;

INSERT INTO public.control_t (name, note)
SELECT 'ctl_seed_' || g::text, NULL FROM generate_series(1, 5) AS g;

INSERT INTO public.subject_t (name, note)
SELECT 'sub_seed_' || g::text, NULL FROM generate_series(1, 5) AS g;

COMMIT;
