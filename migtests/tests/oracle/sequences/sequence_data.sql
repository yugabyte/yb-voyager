INSERT INTO identity_demo_generated_always(description)
VALUES('Random description for the test');

INSERT INTO identity_demo_generated_always(description)
VALUES('Random description for the test');

select * from identity_demo_generated_always;

INSERT INTO identity_demo_generated_by_def(description)
VALUES('Random description for the test');

INSERT INTO identity_demo_generated_by_def(id,description)
VALUES(2, 'Random description for the test');

INSERT INTO identity_demo_generated_by_def(id,description)
VALUES(5, 'Random description for the test');

select * from identity_demo_generated_by_def;

INSERT INTO identity_demo_with_null(description)
VALUES('Oracle identity column demo with null');

INSERT INTO identity_demo_with_null
VALUES(null,'Oracle identity column demo with null');

select * from identity_demo_with_null;

INSERT INTO identity_demo_generated_always_start_with(description)
VALUES('Random description for the test');

select * from identity_demo_generated_always_start_with;

INSERT INTO identity_demo_generated_by_def_start_with(description)
VALUES('Random description for the test');

INSERT INTO identity_demo_generated_by_def_start_with(id,description)
VALUES(2, 'Random description for the test');

INSERT INTO identity_demo_generated_by_def_start_with(description)
VALUES('Random description for the test');

select * from identity_demo_generated_by_def_start_with;

INSERT INTO identity_demo_generated_by_def_inc_by(description)
VALUES('Random description for the test');

INSERT INTO identity_demo_generated_by_def_inc_by(id,description)
VALUES(2, 'Random description for the test');

INSERT INTO identity_demo_generated_by_def_inc_by(description)
VALUES('Random description for the test');

select * from identity_demo_generated_by_def_inc_by;

INSERT INTO identity_demo_generated_by_def_st_with_inc_by(description)
VALUES('Random description for the test');

INSERT INTO identity_demo_generated_by_def_st_with_inc_by(id,description)
VALUES(4, 'Random description for the test');

INSERT INTO identity_demo_generated_by_def_st_with_inc_by(description)
VALUES('Random description for the test');

select * from identity_demo_generated_by_def_st_with_inc_by;

commit;