INSERT INTO identity_demo_generated_always (description) VALUES ('Event 1');
INSERT INTO identity_demo_generated_always (description) VALUES ('Event 2');

INSERT INTO identity_demo_generated_by_def (description) VALUES ('Event 3');
INSERT INTO identity_demo_generated_by_def (description) VALUES ('Event 4');
INSERT INTO identity_demo_generated_by_def (description) VALUES ('Event 4');

INSERT INTO identity_demo_with_null (description) VALUES ('Event 5');
INSERT INTO identity_demo_with_null (description) VALUES ('Event 6');

UPDATE identity_demo_generated_always
SET description = 'Updated Event 3'
WHERE id = 3;

UPDATE identity_demo_generated_by_def
SET description = 'Updated Event 6'
WHERE id = 6;

UPDATE identity_demo_with_null
SET description = 'Updated Event 3'
WHERE id = 3;

DELETE FROM identity_demo_generated_always
WHERE id = 1;

DELETE FROM identity_demo_generated_by_def
WHERE id = 2;

DELETE FROM identity_demo_with_null
WHERE id = 3;

INSERT INTO empty_identity_def (description) VALUES ('Some value');

DELETE FROM empty_identity_def
WHERE id = 2;

INSERT INTO empty_identity_always (description) VALUES ('First Row');
INSERT INTO empty_identity_always (description) VALUES ('Second Row');

UPDATE empty_identity_always
SET description = 'Updated First Row'
WHERE id = 1;

UPDATE empty_identity_always
SET description = 'Updated Second Row'
WHERE id = 2;

DELETE FROM empty_identity_always
WHERE id = 2;