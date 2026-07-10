-- Composite-unique-key scan/lookup cost without conflicts (stable expectation
-- across per-column and tuple semantics; the per-column false-positive case
-- lives in canary-composite-per-column). 20000 events.
UPDATE comp_items SET slug = 'slugx_' || id;
