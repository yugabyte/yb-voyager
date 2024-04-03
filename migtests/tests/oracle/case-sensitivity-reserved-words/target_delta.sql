INSERT INTO "lt_lc_uc" (id, "column_name", COLUMN_NAME2) VALUES (4, 'data7', 'data8');
DELETE FROM "lt_lc_uc" WHERE id = 1;
UPDATE "lt_lc_uc" SET "column_name" = 'updated_data', COLUMN_NAME2 = 'updated_data', id=1 WHERE id = 5;


DELETE FROM "lt_rwc" WHERE id = 3;
INSERT INTO "lt_rwc" (id, "user", limit, "number", "Column_Name") VALUES (3, 'user3', 'description3', '98765', 'column3');
DELETE FROM "lt_rwc" WHERE id = 4;
UPDATE "lt_rwc" SET "user" = 'updated_user2', limit = 'updated_description2', "number" = 'updated_number2', "Column_Name" = 'updated_column2', id=4 WHERE id = 2;


INSERT INTO UT_UC (id, COLUMN_NAME) VALUES (4, 'value4');
UPDATE UT_UC SET COLUMN_NAME = 'updated_value2',id=2 WHERE id = 3;
DELETE FROM UT_UC WHERE id = 4;


INSERT INTO UT_MC (id, "Column_Name") VALUES (4, 'value4');
DELETE FROM UT_MC WHERE id = 4;
UPDATE UT_MC SET "Column_Name" = 'updated_value',id=4 WHERE id = 1;


INSERT INTO UT_RWC (id, "user", limit, "Column_Name") VALUES (4, 'user4', 'description4', 'column4');
UPDATE UT_RWC SET "user" = 'updated_user' WHERE id = 4;
DELETE FROM UT_RWC WHERE id = 4;
UPDATE UT_RWC SET "user" = 'updated_user', limit = 'updated_description', "Column_Name" = 'updated_column' WHERE id = 5;
DELETE FROM UT_RWC WHERE limit = 'updated_description';


INSERT INTO "Mt_Rwc" (id, "user", "column_name", limit) VALUES (4, 'user4', 'column4', 'description4');
UPDATE "Mt_Rwc" SET "user" = 'updated_user', "column_name" = 'updated_column', limit = 'updated_description' WHERE id = 4;
UPDATE "Mt_Rwc" SET "user" = 'updated_user', limit = 'updated_description' WHERE id = 1;
DELETE FROM "Mt_Rwc" WHERE "user" = 'updated_user';


INSERT INTO "CASE" (id, "USER") VALUES (3, 'user3');
INSERT INTO "CASE" (id, "USER", "case", limit) VALUES (4, 'user4', 'case4', 'description4');
UPDATE "CASE" SET "USER" = NULL, "case" = NULL, limit = NULL WHERE id = 6;
UPDATE "CASE" SET "USER" = 'updated_user', "case" = 'updated_case_again', limit = 'updated_description' WHERE id = 2;
DELETE FROM "CASE" WHERE "case" = 'updated_case_again';


INSERT INTO "number" (id, "Column_Name", "case", limit) VALUES (5, NULL,NULL,NULL);
INSERT INTO "number" (id, "Column_Name", "case", limit) VALUES (1, 'column4', 'case4', 'description4');
DELETE FROM "number" WHERE "Column_Name" = 'column4';
UPDATE "number" SET "Column_Name" = 'updated_column', "case" = 'updated_case', limit = 'updated_description' WHERE id = 5;


DELETE FROM limit WHERE "case" IS NULL;
INSERT INTO limit (id, "Column_Name", "case", limit) VALUES (5, 'column4', 'case4', 'description4');
UPDATE limit SET "Column_Name" = 'updated_column', limit = 'updated_description',id=3 WHERE id = 8;
UPDATE limit SET "Column_Name" = 'updated_column', "case" = 'updated_case', limit = 'updated_description' WHERE id = 2;
INSERT INTO limit (id, "Column_Name", "case", limit) VALUES (15, NULL,'case',NULL);


INSERT INTO "RowId" (id, "TABLE") VALUES (3, 'table3');
INSERT INTO "RowId" (id, "User") VALUES (4, 'user4');
INSERT INTO "RowId" (id, "User", "TABLE") VALUES (5, 'user5', 'table5');
UPDATE "RowId" SET "User" = 'updated_user', "TABLE" = 'updated_table' WHERE id = 3;
UPDATE "RowId" SET "User" = 'updated_user', "TABLE" = 'updated_table' WHERE id = 4;
DELETE FROM "RowId" WHERE id > 4;
