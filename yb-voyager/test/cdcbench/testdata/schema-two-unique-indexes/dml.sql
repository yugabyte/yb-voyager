-- Table with TWO unique indexes (email, username): half the updates change
-- email, half change username, all to globally fresh values. Zero conflicts;
-- measures the per-index scan cost multiplier.
-- 20000 events.
UPDATE accounts SET email    = 'emx_' || id WHERE id <= 10000;
UPDATE accounts SET username = 'unx_' || id WHERE id >  10000;
