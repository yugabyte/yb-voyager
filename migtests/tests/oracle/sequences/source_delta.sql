INSERT INTO identity_demo_generated_always (description) VALUES ('Event 1');
INSERT INTO identity_demo_generated_always (description) VALUES ('Event 2');

INSERT INTO identity_demo_generated_by_def (description) VALUES ('Event 3');
INSERT INTO identity_demo_generated_by_def (description) VALUES ('Event 4');

INSERT INTO identity_demo_with_null (description) VALUES ('Event 5');
INSERT INTO identity_demo_with_null (description) VALUES ('Event 6');

UPDATE identity_demo_generated_always
SET description = 'Updated Event 1'
WHERE id = 1;

UPDATE identity_demo_generated_by_def
SET description = 'Updated Event 3'
WHERE id = 5;

UPDATE identity_demo_with_null
SET description = 'Updated Event 5'
WHERE id = 3;

DELETE FROM identity_demo_generated_always
WHERE id = 2

DELETE FROM identity_demo_generated_by_def
WHERE description = 'Event 4';

DELETE FROM identity_demo_with_null
WHERE id = 1;

INSERT INTO empty_identity_demo_generated_by_def (description) VALUES ('First Row');
INSERT INTO empty_identity_demo_generated_by_def (description) VALUES ('Second Row');

UPDATE empty_identity_demo_generated_by_def
SET description = 'Updated First Row'
WHERE id = 1;

DELETE FROM empty_identity_demo_generated_by_def
WHERE id = 1;

commit;
