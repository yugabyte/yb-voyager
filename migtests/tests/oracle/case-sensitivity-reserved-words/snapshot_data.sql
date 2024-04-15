INSERT INTO "lt_lc_uc" (id, "column_name", COLUMN_NAME2) VALUES (1, 'data1', 'data2');
INSERT INTO "lt_lc_uc" (id, "column_name", COLUMN_NAME2) VALUES (2, 'data3', NULL);

INSERT INTO "lt_rwc" (id, "user", limit, "number", "Column_Name") VALUES (1, 'user1', NULL, '12345', 'column1');
INSERT INTO "lt_rwc" (id, "user", limit, "number", "Column_Name") VALUES (2, 'user2', 'description2', NULL, 'column2');

INSERT INTO UT_UC (id, COLUMN_NAME) VALUES (1, 'value1');
INSERT INTO UT_UC (id, COLUMN_NAME) VALUES (2, 'value2');

INSERT INTO UT_MC (id, "Column_Name") VALUES (1, 'value1');
INSERT INTO UT_MC (id, "Column_Name") VALUES (2, NULL);

INSERT INTO UT_RWC (id, "user", limit, "Column_Name") VALUES (1, 'user1', 'description1', 'column1');
INSERT INTO UT_RWC (id, "user", limit, "Column_Name") VALUES (2, NULL, 'description2', 'column2');

INSERT INTO "Mt_Rwc" (id, "user", "column_name", limit) VALUES (1, 'user1', 'column1', 'description1');
INSERT INTO "Mt_Rwc" (id, "user", "column_name", limit) VALUES (2, 'user2', 'column2', 'description2');

INSERT INTO "CASE" (id, "USER", "case", limit) VALUES (1, 'user1', 'case1', 'description1');
INSERT INTO "CASE" (id, "USER", "case", limit) VALUES (2, NULL, 'case2', 'description2');

INSERT INTO "number" (id, "Column_Name", "case", limit) VALUES (10, 'column1', NULL, 'description1');
INSERT INTO "number" (id, "Column_Name", "case", limit) VALUES (2, 'column2', 'case2', 'description2');

INSERT INTO limit (id, "Column_Name", "case", limit) VALUES (1, 'column1', 'case1', 'description1');
INSERT INTO limit (id, "Column_Name", "case", limit) VALUES (2, 'column2', 'case2', 'description2');

INSERT INTO "RowId" (id, "User", "TABLE") VALUES (1, 'user1', NULL);
INSERT INTO "RowId" (id, "User", "TABLE") VALUES (2, 'user2', 'table2');

INSERT INTO cs_pk ("Id", "column_name", COLUMN_NAME2) VALUES (1, 'data1', 'data2');
INSERT INTO cs_pk ("Id", "column_name", COLUMN_NAME2) VALUES (2, 'data3', NULL);

INSERT INTO "rw_pk" ("USER", col, COLUMN_NAME2) VALUES (1, 'data1', 'data2');
INSERT INTO "rw_pk" ("USER", col, COLUMN_NAME2) VALUES (2, 'data3', NULL);
