INSERT INTO "lt_lc_uc" (id, "column_name", COLUMN_NAME2) VALUES (3, 'data5', 'data6');
UPDATE "lt_lc_uc" SET "column_name" = 'updated_data', COLUMN_NAME2 = 'updated_data' WHERE id = 1;
UPDATE "lt_lc_uc" SET "column_name" = 'updated_data2', id=5, COLUMN_NAME2 = 'updated_data2' WHERE id = 2;
DELETE FROM "lt_lc_uc" WHERE id = 3;


INSERT INTO "lt_rwc" (id, "user", limit, "number", "Column_Name") VALUES (4, 'user4', 'description4', '56789', 'column4');
UPDATE "lt_rwc" SET "user" = 'updated_user', limit = 'updated_description', "number" = 'updated_number', "Column_Name" = 'updated_column', id=3 WHERE id = 1;
UPDATE "lt_rwc" SET limit = 'updated_description', "number" = 'updated_number'WHERE id = 2;


INSERT INTO UT_UC (id, COLUMN_NAME) VALUES (3, 'value3');
DELETE FROM UT_UC WHERE id = 1;
UPDATE UT_UC SET COLUMN_NAME = 'updated_value',id=1 WHERE id = 3;
UPDATE UT_UC SET id=3 WHERE id = 2;


INSERT INTO UT_MC (id, "Column_Name") VALUES (3, 'value3');
UPDATE UT_MC SET "Column_Name" = 'updated_value' WHERE id = 1;
DELETE FROM UT_MC WHERE id = 1;
UPDATE UT_MC SET id=1 WHERE id = 2;


INSERT INTO UT_RWC (id, "user", limit, "Column_Name") VALUES (4, 'user4', 'description4', 'column4');
UPDATE UT_RWC SET limit = 'updated_description' WHERE id = 1;
UPDATE UT_RWC SET "user" = 'updated_user', limit = 'updated_description', "Column_Name" = 'updated_column' WHERE id = 2;
UPDATE UT_RWC SET id=5 WHERE id = 4;
DELETE FROM UT_RWC WHERE id = 2;


INSERT INTO "Mt_Rwc" (id, "user", "column_name", limit) VALUES (3, 'user3', 'column3', 'description3');
UPDATE "Mt_Rwc" SET "user" = 'updated_user', "column_name" = 'updated_column', limit = 'updated_description' WHERE id = 1;
UPDATE "Mt_Rwc" SET "column_name" = 'updated_column' WHERE id = 2;
DELETE FROM "Mt_Rwc" WHERE id = 1;
UPDATE "Mt_Rwc" SET id=1 WHERE id = 2;


INSERT INTO "CASE" (id, "USER", "case", limit) VALUES (3, 'user3', 'case3', 'description3');
INSERT INTO "CASE" (id) VALUES (4);
UPDATE "CASE" SET "USER" = 'updated_user', "case" = 'updated_case', limit = 'updated_description',id=6 WHERE id = 3;
UPDATE "CASE" SET "case" = 'updated_case' WHERE id = 2;
DELETE FROM "CASE" WHERE id = 4;


INSERT INTO "number" (id, "Column_Name", "case", limit) VALUES (3, 'column3', 'case3', 'description3');
INSERT INTO "number" (id, "Column_Name", limit) VALUES (4, 'column4', 'description4');
DELETE FROM "number" WHERE id = 4;
UPDATE "number" SET "case" = 'updated_case',id=6 WHERE id = 10;
UPDATE "number" SET "Column_Name" = 'updated_column', "case" = 'updated_case', limit = 'updated_description' WHERE id = 2;


INSERT INTO limit (id, "Column_Name", "case", limit) VALUES (3, 'column3', 'case3', 'description3');
INSERT INTO limit (id, "Column_Name", "case", limit) VALUES (5, NULL,NULL,NULL);
UPDATE limit SET "Column_Name" = 'updated_column', "case" = 'updated_case', limit = 'updated_description' WHERE id = 1;
UPDATE limit SET limit = 'updated_description', id=8 WHERE id = 3;
DELETE FROM limit WHERE id = 1;


INSERT INTO "RowId" (id, "TABLE") VALUES (3, 'table3');
INSERT INTO "RowId" (id, "User") VALUES (4, 'user4');
INSERT INTO "RowId" (id, "User", "TABLE") VALUES (5, 'user5', 'table5');
UPDATE "RowId" SET "User" = 'updated_user', "TABLE" = 'updated_table' WHERE id = 3;
UPDATE "RowId" SET "User" = 'updated_user', "TABLE" = 'updated_table' WHERE id = 4;
DELETE FROM "RowId" WHERE id > 2;

INSERT INTO cs_pk ("Id", "column_name", COLUMN_NAME2) VALUES (3, 'data5', 'data6');
UPDATE cs_pk SET "column_name" = 'updated_data', COLUMN_NAME2 = 'updated_data' WHERE "Id" = 1;
UPDATE cs_pk SET "column_name" = 'updated_data2', "Id"=5, COLUMN_NAME2 = 'updated_data2' WHERE "Id" = 2;
DELETE FROM cs_pk WHERE "Id" = 3;

INSERT INTO "rw_pk" ("USER", col, COLUMN_NAME2) VALUES (3, 'data5', 'data6');
UPDATE "rw_pk" SET col = 'updated_data', COLUMN_NAME2 = 'updated_data' WHERE "USER" = 1;
UPDATE "rw_pk" SET col = 'updated_data2', "USER"=5, COLUMN_NAME2 = 'updated_data2' WHERE "USER" = 2;
DELETE FROM "rw_pk" WHERE "USER" = 3;