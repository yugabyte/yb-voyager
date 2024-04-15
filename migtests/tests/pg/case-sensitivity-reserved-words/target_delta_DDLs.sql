-- rw_pk Table:
INSERT INTO rw_pk (id, column_name) VALUES (5, 'Value 2');
INSERT INTO rw_pk (id, column_name) VALUES (6, 'Value 3');
DELETE FROM rw_pk WHERE id = 6;
UPDATE rw_pk SET column_name = NULL WHERE id = 5;
UPDATE rw_pk SET id = 6 WHERE id = 1;

-- lt_uc Table:
INSERT INTO lt_uc (id, "COLUMN_NAME") VALUES (5, 'Value 2');
INSERT INTO lt_uc (id, "COLUMN_NAME") VALUES (6, 'Value 3');
DELETE FROM lt_uc WHERE id = 6;
UPDATE lt_uc SET "COLUMN_NAME" = 'Updated Value 1 again',id=6 WHERE id = 1;
UPDATE lt_uc SET "COLUMN_NAME" = NULL WHERE id = 5;

-- lt_mc Table:
INSERT INTO lt_mc (id, "Column_Name") VALUES (5, 'Value 2');
INSERT INTO lt_mc (id, "Column_Name") VALUES (6, 'Value 3');
DELETE FROM lt_mc WHERE id = 6;
UPDATE lt_mc SET "Column_Name" = 'Updated Value 1 again', id=6 WHERE id = 1;
UPDATE lt_mc SET "Column_Name" = NULL WHERE id = 6;

-- lt_rwc Table:
INSERT INTO lt_rwc (id, "user", "case") VALUES (7, 'User 3', 'Case 3');
UPDATE lt_rwc SET "user" = 'Updated User 5', id=4 WHERE id = 5;
DELETE FROM lt_rwc WHERE "case" = 'Case 2';
UPDATE lt_rwc SET "case" = NULL, id=4 WHERE id = 7;

-- "UT_LC" Table:
INSERT INTO "UT_LC" (id, column_name) VALUES (5, 'Value 2');
INSERT INTO "UT_LC" (id, column_name) VALUES (6, 'Value 3');
UPDATE "UT_LC" SET column_name = NULL WHERE id = 5;
DELETE FROM "UT_LC" WHERE id = 6;
UPDATE "UT_LC" SET id = 6 WHERE id = 1;

-- "UT_UC" Table:
INSERT INTO "UT_UC" (id, "COLUMN_NAME") VALUES (5, 'Value 2');
INSERT INTO "UT_UC" (id, "COLUMN_NAME") VALUES (6, 'Value 3');
UPDATE "UT_UC" SET "COLUMN_NAME" = NULL WHERE id = 6;
DELETE FROM "UT_UC" WHERE id = 6;
UPDATE "UT_UC" SET "COLUMN_NAME" = 'Updated Value 1 again',id=6 WHERE id = 1;

-- "UT_MC" Table:
INSERT INTO "UT_MC" (id, "Column_Name") VALUES (5, 'Value 2');
INSERT INTO "UT_MC" (id, "Column_Name") VALUES (6, 'Value 3');
UPDATE "UT_MC" SET "Column_Name" = NULL WHERE id = 6;
DELETE FROM "UT_MC" WHERE id = 6;
UPDATE "UT_MC" SET "Column_Name" = 'Updated Value 1 again', id=6 WHERE id = 1;

-- "UT_RWC" Table:
INSERT INTO "UT_RWC" (id, "user", "case", "table") VALUES (7, 'User 3', 'Case 3', 'Description 3');
UPDATE "UT_RWC" SET "user" = 'Updated User 5', "table" = 'Updated description', id=4 WHERE id = 5;
UPDATE "UT_RWC" SET "case" = NULL, "table" = NULL WHERE id = 7;
DELETE FROM "UT_RWC" WHERE "table" = 'Updated description';

-- "Mt_Rwc" Table:
INSERT INTO "Mt_Rwc" (id, "user", "COLUMN_NAME", "Table") VALUES (6, 'User6', 'Value6', 'Description6');
UPDATE "Mt_Rwc" SET id = 3, "user" = 'UpdatedUser1', "Table" = 'UpdatedDescription2', "COLUMN_NAME" = 'UpdatedValue3' WHERE id = 5;
DELETE FROM "Mt_Rwc" WHERE id = 1;
UPDATE "Mt_Rwc" SET "user" = NULL, "Table" = NULL, "COLUMN_NAME" = NULL,id=1 WHERE id = 6;

-- "integer" Table:
INSERT INTO "integer" (id, "Column_Name") VALUES (5, 'Value 2');
INSERT INTO "integer" (id, "Column_Name") VALUES (6, 'Value 3');
UPDATE "integer" SET "Column_Name" = NULL WHERE id = 6;
DELETE FROM "integer" WHERE id = 6;
UPDATE "integer" SET "Column_Name" = 'Updated Value 1 again', id=6 WHERE id = 1;

-- "case" Table:
INSERT INTO "case" (id, "user", "case", "table") VALUES (7, 'User 3', 'Case 3', 'Description 3');
UPDATE "case" SET "user" = 'Updated User 5', "table" = 'Updated description', id=4 WHERE id = 5;
DELETE FROM "case" WHERE "table" = 'Updated description';
UPDATE "case" SET "user" = NULL, "table" = NULL WHERE id = 7;

-- case-sensitive reserved word table/column
INSERT INTO "Table" (id, "User_name", "table") VALUES (5, 'User5', 'Description5');
INSERT INTO "Table" (id,"table") VALUES (6, 'Description6');
UPDATE "Table" SET "User_name" = 'UpdatedUser', "table" = 'UpdatedDescription' WHERE id = 5;
DELETE FROM "Table" WHERE id = 3;
UPDATE "Table" SET "table" = NULL, "User_name" = NULL WHERE id = 6;

-- case-sensitive reserved word column
INSERT INTO cs_rwc (id, "User", "table") VALUES (5, 'User5', 'Description5');
INSERT INTO cs_rwc (id,"table") VALUES (6, 'Description6');
UPDATE cs_rwc SET "User" = 'UpdatedUser', "table" = 'UpdatedDescription' WHERE id = 5;
UPDATE cs_rwc SET "table" = NULL, "User" = NULL WHERE id = 5;
DELETE FROM cs_rwc WHERE id = 3;

-- case-sensitive reserved word table/column
INSERT INTO "USER" (id, "User") VALUES (6, 'User6');
UPDATE "USER" SET "User" = 'UpdatedUser2 again' WHERE id = 2;
DELETE FROM "USER" WHERE id = 6;
UPDATE "USER" SET "User" = NULL WHERE id = 4;

-- partitioned table

DELETE FROM "cust_Part22" WHERE id = 525;
INSERT INTO "cust_Part22" (id, "Statuses", "User") VALUES (525, 'ACTIVE', 340);
UPDATE "cust_Part22" SET id = 397, "Statuses" = 'RECURRING' WHERE id = 1012;

DELETE FROM cust_part21 WHERE id = 225;
INSERT INTO cust_part21 (id, "Statuses", "User") VALUES (1115, 'RECURRING', 310);
UPDATE cust_part21 SET id = 225 WHERE id = 1115;

INSERT INTO cust_part12 (id, "Statuses", "User") VALUES (132, 'REACTIVATED', 100);
UPDATE cust_part12 SET "User" = 100, "Statuses" = 'ACTIVE' WHERE id = 122;
UPDATE cust_part12 SET id = 132 WHERE id = 196;

UPDATE cust_part11 SET "User" = 50, "Statuses" = 'REACTIVATED', id=136 WHERE id = 114;
UPDATE cust_part11 SET "User" = 100, "Statuses" = 'ACTIVE' WHERE id = 142;

INSERT INTO cust_other (id, "Statuses", "User") VALUES (141, 'PENDING', 340);
UPDATE cust_other SET "User" = 340, "Statuses" = 'INACTIVE' WHERE id = 141;
DELETE FROM cust_other WHERE id = 141;

INSERT INTO cust_active (id, "Statuses", "User") VALUES (151, 'RECURRING', 340);
UPDATE cust_active SET "User" = 340, "Statuses" = 'RECURRING' WHERE id = 12;
DELETE FROM cust_active WHERE "User" = 340;

INSERT INTO "Customers" (id, "Statuses", "User") VALUES (161, 'RECURRING', 340);
UPDATE "Customers" SET  "User" = 290 where id = 163;
UPDATE "Customers" SET  "User" = 410, "Statuses" = 'RECURRING' where id = 164;
DELETE FROM "Customers" WHERE id = 162;

-- case-sensitive pk

INSERT INTO cs_pk ("Id", column_name) VALUES (5, 'Value 2');
INSERT INTO cs_pk ("Id", column_name) VALUES (6, 'Value 3');
DELETE FROM cs_pk WHERE "Id" = 6;
UPDATE cs_pk SET column_name = NULL WHERE "Id" = 5;
UPDATE cs_pk SET "Id" = 6 WHERE "Id" = 1;

-- reserved word pk

INSERT INTO rw_pk ("user", column_name) VALUES (5, 'Value 2');
INSERT INTO rw_pk ("user", column_name) VALUES (6, 'Value 3');
DELETE FROM rw_pk WHERE "user" = 6;
UPDATE rw_pk SET column_name = NULL WHERE "user" = 5;
UPDATE rw_pk SET "user" = 6 WHERE "user" = 1;