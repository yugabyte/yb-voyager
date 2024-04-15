set search_path to test_schema2;

INSERT INTO lt_lc_uc (id, "column_name", COLUMN_NAME2) VALUES (4, 'data7', 'data8');
DELETE FROM lt_lc_uc WHERE id = 1;
UPDATE lt_lc_uc SET "column_name" = 'updated_data', COLUMN_NAME2 = 'updated_data', id=1 WHERE id = 5;


DELETE FROM lt_rwc WHERE id = 3;
INSERT INTO lt_rwc (id, "user", "limit", number, column_name) VALUES (3, 'user3', 'description3', '98765', 'column3');
DELETE FROM lt_rwc WHERE id = 4;
UPDATE lt_rwc SET "user" = 'updated_user2', "limit" = 'updated_description2', number = 'updated_number2', column_name = 'updated_column2', id=4 WHERE id = 2;


INSERT INTO UT_UC (id, COLUMN_NAME) VALUES (4, 'value4');
UPDATE UT_UC SET COLUMN_NAME = 'updated_value2',id=2 WHERE id = 3;
DELETE FROM UT_UC WHERE id = 4;


INSERT INTO UT_MC (id, column_name) VALUES (4, 'value4');
DELETE FROM UT_MC WHERE id = 4;
UPDATE UT_MC SET column_name = 'updated_value',id=4 WHERE id = 1;


INSERT INTO UT_RWC (id, "user", "limit", column_name) VALUES (4, 'user4', 'description4', 'column4');
UPDATE UT_RWC SET "user" = 'updated_user' WHERE id = 4;
DELETE FROM UT_RWC WHERE id = 4;
UPDATE UT_RWC SET "user" = 'updated_user', "limit" = 'updated_description', column_name = 'updated_column' WHERE id = 5;
DELETE FROM UT_RWC WHERE "limit" = 'updated_description';


INSERT INTO mt_rwc (id, "user", "column_name", "limit") VALUES (4, 'user4', 'column4', 'description4');
UPDATE mt_rwc SET "user" = 'updated_user', "column_name" = 'updated_column', "limit" = 'updated_description' WHERE id = 4;
UPDATE mt_rwc SET "user" = 'updated_user', "limit" = 'updated_description' WHERE id = 1;
DELETE FROM mt_rwc WHERE "user" = 'updated_user';


INSERT INTO "case" (id, "user") VALUES (3, 'user3');
INSERT INTO "case" (id, "user", "case", "limit") VALUES (4, 'user4', 'case4', 'description4');
UPDATE "case" SET "user" = NULL, "case" = NULL, "limit" = NULL WHERE id = 6;
UPDATE "case" SET "user" = 'updated_user', "case" = 'updated_case_again', "limit" = 'updated_description' WHERE id = 2;
DELETE FROM "case" WHERE "case" = 'updated_case_again';


INSERT INTO number (id, column_name, "case", "limit") VALUES (5, NULL,NULL,NULL);
INSERT INTO number (id, column_name, "case", "limit") VALUES (1, 'column4', 'case4', 'description4');
DELETE FROM number WHERE column_name = 'column4';
UPDATE number SET column_name = 'updated_column', "case" = 'updated_case', "limit" = 'updated_description' WHERE id = 5;


DELETE FROM "limit" WHERE "case" IS NULL;
INSERT INTO "limit" (id, column_name, "case", "limit") VALUES (5, 'column4', 'case4', 'description4');
UPDATE "limit" SET column_name = 'updated_column', "limit" = 'updated_description',id=3 WHERE id = 8;
UPDATE "limit" SET column_name = 'updated_column', "case" = 'updated_case', "limit" = 'updated_description' WHERE id = 2;
INSERT INTO "limit" (id, column_name, "case", "limit") VALUES (15, NULL,'case',NULL);


INSERT INTO rowid (id, "table") VALUES (3, 'table3');
INSERT INTO rowid (id, "user") VALUES (4, 'user4');
INSERT INTO rowid (id, "user", "table") VALUES (5, 'user5', 'table5');
UPDATE rowid SET "user" = 'updated_user', "table" = 'updated_table' WHERE id = 3;
UPDATE rowid SET "user" = 'updated_user', "table" = 'updated_table' WHERE id = 4;
DELETE FROM rowid WHERE id > 4;

INSERT INTO cs_pk (id, "column_name", COLUMN_NAME2) VALUES (4, 'data7', 'data8');
DELETE FROM cs_pk WHERE id = 1;
UPDATE cs_pk SET "column_name" = 'updated_data', COLUMN_NAME2 = 'updated_data', id=1 WHERE id = 5;

INSERT INTO rw_pk ("user", col, COLUMN_NAME2) VALUES (4, 'data7', 'data8');
DELETE FROM rw_pk WHERE "user" = 1;
UPDATE rw_pk SET col = 'updated_data', COLUMN_NAME2 = 'updated_data', "user"=1 WHERE "user" = 5;


