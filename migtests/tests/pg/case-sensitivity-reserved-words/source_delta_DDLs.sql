-- lt_lc Table:
INSERT INTO lt_lc (id, column_name) VALUES (4, 'Value 1');
UPDATE lt_lc SET column_name = 'Updated Value 1' WHERE id = 1;
UPDATE lt_lc SET column_name = NULL WHERE id = 1;
UPDATE lt_lc SET id=6 WHERE id = 2;
DELETE FROM lt_lc WHERE id = 6;

-- lt_uc Table:
INSERT INTO lt_uc (id, "COLUMN_NAME") VALUES (4, 'Value 1');
UPDATE lt_uc SET "COLUMN_NAME" = 'Updated Value 1' WHERE id = 1;
UPDATE lt_uc SET "COLUMN_NAME" = NULL WHERE id = 4;
UPDATE lt_uc SET id=6 WHERE id = 2;
DELETE FROM lt_uc WHERE id = 6;

-- lt_mc Table:
INSERT INTO lt_mc (id, "Column_Name") VALUES (4, 'Value 1');
UPDATE lt_mc SET "Column_Name" = 'Updated Value 1' WHERE id = 1;
UPDATE lt_mc SET "Column_Name" = NULL WHERE id = 4;
UPDATE lt_mc SET id=6 WHERE id = 2;
DELETE FROM lt_mc WHERE id = 6;

-- lt_rwc Table:
INSERT INTO lt_rwc (id, "user", "case") VALUES (4, 'User 1', 'Case 1');
INSERT INTO lt_rwc (id, "user", "case") VALUES (5, 'User 2', 'Case 2');
UPDATE lt_rwc SET "user" = 'Updated User 1', id=6 WHERE id = 1;
UPDATE lt_rwc SET "user" = NULL WHERE id = 4;
DELETE FROM lt_rwc WHERE id = 4;

-- "UT_LC" Table:
INSERT INTO "UT_LC" (id, column_name) VALUES (4, 'Value 1');
UPDATE "UT_LC" SET column_name = 'Updated Value 1' WHERE id = 1;
UPDATE "UT_LC" SET column_name = NULL WHERE id = 1;
UPDATE "UT_LC" SET id=6 WHERE id = 2;
DELETE FROM "UT_LC" WHERE id = 6;

-- "UT_UC" Table:
INSERT INTO "UT_UC" (id, "COLUMN_NAME") VALUES (4, 'Value 1');
UPDATE "UT_UC" SET "COLUMN_NAME" = 'Updated Value 1' WHERE id = 1;
UPDATE "UT_UC" SET "COLUMN_NAME" = NULL WHERE id = 1;
UPDATE "UT_UC" SET id=6 WHERE id = 2;
DELETE FROM "UT_UC" WHERE id = 6;

-- "UT_MC" Table:
INSERT INTO "UT_MC" (id, "Column_Name") VALUES (4, 'Value 1');
UPDATE "UT_MC" SET "Column_Name" = 'Updated Value 1' WHERE id = 1;
UPDATE "UT_MC" SET "Column_Name" = NULL WHERE id = 3;
UPDATE "UT_MC" SET id=6 WHERE id = 2;
DELETE FROM "UT_MC" WHERE id = 6;

-- "UT_RWC" Table:
INSERT INTO "UT_RWC" (id, "user", "case", "table") VALUES (4, 'User 1', 'Case 1', 'Description 1');
INSERT INTO "UT_RWC" (id, "user", "case", "table") VALUES (5, 'User 2', 'Case 2', 'Description 2');
UPDATE "UT_RWC" SET "user" = 'Updated User 1', "table" = 'Updated description', id=6 WHERE id = 1;
UPDATE "UT_RWC" SET "user" = NULL, "table" = NULL WHERE id = 4;
DELETE FROM "UT_RWC" WHERE id = 4;

-- "Mt_Rwc" Table:
INSERT INTO "Mt_Rwc" (id, "user", "COLUMN_NAME", "Table") VALUES (4, 'User4', 'Value4', 'Description4');
INSERT INTO "Mt_Rwc" (id, "user", "COLUMN_NAME", "Table") VALUES (5, 'User5', 'Value5', 'Description5');
UPDATE "Mt_Rwc" SET "user" = 'UpdatedUser1', "Table" = 'UpdatedDescription2', "COLUMN_NAME" = 'UpdatedValue3' WHERE id = 1;
UPDATE "Mt_Rwc" SET "user" = NULL, "Table" = NULL, "COLUMN_NAME" = NULL WHERE id = 1;
DELETE FROM "Mt_Rwc" WHERE "Table" = 'Description3';

-- "integer" Table:
INSERT INTO "integer" (id, "Column_Name") VALUES (4, 'Value 1');
UPDATE "integer" SET "Column_Name" = 'Updated Value 1' WHERE id = 1;
UPDATE "integer" SET "Column_Name" = NULL WHERE id = 4;
UPDATE "integer" SET id=6 WHERE id = 2;
DELETE FROM "integer" WHERE id = 6;

-- "case" Table:
INSERT INTO "case" (id, "user", "case", "table") VALUES (4, 'User 1', 'Case 1', 'Description 1');
INSERT INTO "case" (id, "user", "case", "table") VALUES (5, 'User 2', 'Case 2', 'Description 2');
UPDATE "case" SET "user" = 'Updated User 1', "table" = 'Updated description', id=6 WHERE id = 1;
UPDATE "case" SET "user" = NULL, "table" = NULL WHERE id = 4;
DELETE FROM "case" WHERE id = 4;

-- case-sensitive reserved word table/column
INSERT INTO "Table" (id, "User_name", "table") VALUES (4, 'User4', 'Description4');
UPDATE "Table" SET "User_name" = 'UpdatedUser1' WHERE id = 1;
UPDATE "Table" SET "table" = 'UpdatedDescription2', id = 5 WHERE id = 2;
UPDATE "Table" SET "table" = NULL, "User_name" = NULL WHERE id = 4;
DELETE FROM "Table" WHERE id = 5;

-- case-sensitive reserved word column
INSERT INTO cs_rwc (id, "User", "table") VALUES (4, 'User4', 'Description4');
UPDATE cs_rwc SET "User" = 'UpdatedUser1' WHERE id = 1;
UPDATE cs_rwc SET "table" = 'UpdatedDescription2', id = 5 WHERE id = 2;
UPDATE cs_rwc SET "table" = NULL, "User" = NULL WHERE id = 5;
DELETE FROM cs_rwc WHERE id = 5;

-- case-sensitive reserved word table/column
INSERT INTO "USER" (id, "User") VALUES (4, 'User4');
DELETE FROM "USER" WHERE id = 3;
UPDATE "USER" SET "User" = 'UpdatedUser1', id = 3 WHERE id = 1;
UPDATE "USER" SET "User" = 'UpdatedUser2' WHERE id = 2;
UPDATE "USER" SET "User" = NULL WHERE id = 3;

-- partitioned table

INSERT INTO "cust_Part22" (id, "Statuses", "User") VALUES (100, 'ACTIVE', 250);
UPDATE "cust_Part22" SET "Statuses" = 'RECURRING' WHERE id = 100;
UPDATE "cust_Part22" SET id = 1012, "User" = 410 WHERE id = 397;

INSERT INTO cust_part21 (id, "Statuses", "User") VALUES (1114, 'RECURRING', 310);
UPDATE cust_part21 SET "User" = 410 WHERE id = 225;

INSERT INTO cust_part12 (id, "Statuses", "User") VALUES (132, 'REACTIVATED', 100);
UPDATE cust_part12 SET "User" = 100, "Statuses" = 'ACTIVE' WHERE id = 90;
DELETE FROM cust_part12 WHERE id = 132;

INSERT INTO cust_part11 (id, "Statuses", "User") VALUES (140, 'REACTIVATED', 50);
UPDATE cust_part11 SET "User" = 100, "Statuses" = 'ACTIVE' WHERE id = 114;

INSERT INTO cust_other (id, "Statuses", "User") VALUES (140, 'INACTIVE', 290);
UPDATE cust_other SET "User" = 340, "Statuses" = 'INACTIVE' WHERE id = 140;
DELETE FROM cust_other WHERE id = 140;

INSERT INTO cust_active (id, "Statuses", "User") VALUES (150, 'ACTIVE', 290);
DELETE FROM cust_active WHERE id = 150;
UPDATE cust_active SET "User" = 340, "Statuses" = 'RECURRING' WHERE id = 16;

INSERT INTO "Customers" (id, "Statuses", "User") VALUES (160, 'ACTIVE', 290);
UPDATE "Customers" SET  "Statuses" = 'RECURRING' where id = 160;
UPDATE "Customers" SET id = 162 WHERE id = 160;