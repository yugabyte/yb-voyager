INSERT INTO lt_lc (id, column_name) VALUES (1, 'value1');
INSERT INTO lt_lc (id, column_name) VALUES (2, NULL);
INSERT INTO lt_lc (id, column_name) VALUES (3, 'value3');


INSERT INTO lt_uc (id, "COLUMN_NAME") VALUES (1, 'value1');
INSERT INTO lt_uc (id, "COLUMN_NAME") VALUES (2, NULL);
INSERT INTO lt_uc (id, "COLUMN_NAME") VALUES (3, 'value3');


INSERT INTO lt_mc (id, "Column_Name") VALUES (1, 'value1');
INSERT INTO lt_mc (id, "Column_Name") VALUES (2, NULL);
INSERT INTO lt_mc (id, "Column_Name") VALUES (3, 'value3');

INSERT INTO lt_rwc (id, "user", "case") VALUES (1, '"user"1', 'case1');
INSERT INTO lt_rwc (id, "user", "case") VALUES (2, '"user"2', NULL);
INSERT INTO lt_rwc (id, "user", "case") VALUES (3, NULL, 'case3');

INSERT INTO "UT_LC" (id, column_name) VALUES (1, 'value1');
INSERT INTO "UT_LC" (id, column_name) VALUES (2, NULL);
INSERT INTO "UT_LC" (id, column_name) VALUES (3, 'value3');


INSERT INTO "UT_UC" (id, "COLUMN_NAME") VALUES (1, 'value1');
INSERT INTO "UT_UC" (id, "COLUMN_NAME") VALUES (2, NULL);
INSERT INTO "UT_UC" (id, "COLUMN_NAME") VALUES (3, 'value3');


INSERT INTO "UT_MC" (id, "Column_Name") VALUES (1, 'value1');
INSERT INTO "UT_MC" (id, "Column_Name") VALUES (2, NULL);
INSERT INTO "UT_MC" (id, "Column_Name") VALUES (3, 'value3');


INSERT INTO "UT_RWC" (id, "user", "case", "table") VALUES (1, '"user"1', 'case1', 'group1');
INSERT INTO "UT_RWC" (id, "user", "case", "table") VALUES (2, '"user"2', NULL, 'group2');
INSERT INTO "UT_RWC" (id, "user", "case", "table") VALUES (3, NULL, 'case3', 'group3');

INSERT INTO "Mt_Rwc" (id, "user", "COLUMN_NAME", "Table") VALUES (1, 'User1', 'Value1', 'Description1');
INSERT INTO "Mt_Rwc" (id, "user", "COLUMN_NAME", "Table") VALUES (2, 'User2', 'Value2', NULL);
INSERT INTO "Mt_Rwc" (id, "user", "COLUMN_NAME", "Table") VALUES (3, NULL, 'Value3', 'Description3');

-- tablename reserved in both pg/yb without reserved word columns

INSERT INTO "integer" (id, "Column_Name") VALUES (1, 'value1');
INSERT INTO "integer" (id, "Column_Name") VALUES (2, NULL);
INSERT INTO "integer" (id, "Column_Name") VALUES (3, 'value3');

-- tablename reserved in both pg/yb with reserved word columns

INSERT INTO "case" (id, "user", "case", "table") VALUES (1, '"user"1', 'case1', 'group1');
INSERT INTO "case" (id, "user", "case", "table") VALUES (2, '"user"2', NULL, 'group2');
INSERT INTO "case" (id, "user", "case", "table") VALUES (3, NULL, 'case3', 'group3');

-- case-sensitive reserved word table

INSERT INTO "Table" (id, "User_name", "table") VALUES (1, 'User1', 'Description1');
INSERT INTO "Table" (id, "User_name", "table") VALUES (2, 'User2', NULL);
INSERT INTO "Table" (id, "User_name", "table") VALUES (3, NULL, 'Description3');

-- case-sensitive reserved word column

INSERT INTO cs_rwc (id, "User", "table") VALUES (1, 'User1', 'Description1');
INSERT INTO cs_rwc (id, "User", "table") VALUES (2, 'User2', NULL);
INSERT INTO cs_rwc (id, "User", "table") VALUES (3, NULL, 'Description3');

-- case-sensitive reserved word table/column

INSERT INTO "USER" (id, "User") VALUES (1, 'User1');
INSERT INTO "USER" (id, "User") VALUES (2, 'User2');
INSERT INTO "USER" (id, "User") VALUES (3, NULL);

-- partitioned table

WITH status_list AS (
        SELECT '{"ACTIVE", "RECURRING", "REACTIVATED", "EXPIRED"}'::TEXT[] "Statuses"
        ), arr_list AS (
            SELECT '{100, 200, 50, 250}'::INT[] "User"
        )
        INSERT INTO "Customers" 
        (id, "Statuses", "User")
            SELECT  n,
                    "Statuses"[1 + mod(n, array_length("Statuses", 1))],
                    "User"[1 + mod(n, array_length("User", 1))]
                        FROM arr_list, generate_series(1,1000) AS n, status_list;