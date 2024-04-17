CREATE TABLE lt_lc (
    id INT PRIMARY KEY,
    column_name TEXT
);

INSERT INTO lt_lc (id, column_name) VALUES (1, 'value1');
INSERT INTO lt_lc (id, column_name) VALUES (2, 'value2');
INSERT INTO lt_lc (id, column_name) VALUES (3, 'value3');

CREATE TABLE lt_uc (
    id INT PRIMARY KEY,
    COLUMN_NAME TEXT
);

INSERT INTO lt_uc (id, COLUMN_NAME) VALUES (1, 'value1');
INSERT INTO lt_uc (id, COLUMN_NAME) VALUES (2, 'value2');
INSERT INTO lt_uc (id, COLUMN_NAME) VALUES (3, 'value3');

CREATE TABLE lt_mc (
    id INT PRIMARY KEY,
    Column_Name TEXT
);

INSERT INTO lt_mc (id, Column_Name) VALUES (1, 'value1');
INSERT INTO lt_mc (id, Column_Name) VALUES (2, 'value2');
INSERT INTO lt_mc (id, Column_Name) VALUES (3, 'value3');

CREATE TABLE lt_rwc (
    id INT PRIMARY KEY,
    user TEXT,
    `case` TEXT,
    `grouping` TEXT
);

INSERT INTO lt_rwc (id, user, `case`, `grouping`) VALUES (1, 'user1', 'case1', 'group1');
INSERT INTO lt_rwc (id, user, `case`, `grouping`) VALUES (2, 'user2', 'case2', 'group2');
INSERT INTO lt_rwc (id, user, `case`, `grouping`) VALUES (3, 'user3', 'case3', 'group3');

CREATE TABLE UT_LC (
    id INT PRIMARY KEY,
    column_name TEXT
);

INSERT INTO UT_LC (id, column_name) VALUES (1, 'value1');
INSERT INTO UT_LC (id, column_name) VALUES (2, 'value2');
INSERT INTO UT_LC (id, column_name) VALUES (3, 'value3');

CREATE TABLE UT_UC (
    id INT PRIMARY KEY,
    COLUMN_NAME TEXT
);

INSERT INTO UT_UC (id, COLUMN_NAME) VALUES (1, 'value1');
INSERT INTO UT_UC (id, COLUMN_NAME) VALUES (2, 'value2');
INSERT INTO UT_UC (id, COLUMN_NAME) VALUES (3, 'value3');

CREATE TABLE UT_MC (
    id INT PRIMARY KEY,
    Column_Name TEXT
);

INSERT INTO UT_MC (id, Column_Name) VALUES (1, 'value1');
INSERT INTO UT_MC (id, Column_Name) VALUES (2, 'value2');
INSERT INTO UT_MC (id, Column_Name) VALUES (3, 'value3');

CREATE TABLE UT_RWC (
    id INT PRIMARY KEY,
    user TEXT,
    `case` TEXT,
    `grouping` TEXT
);

INSERT INTO UT_RWC (id, user, `case`, `grouping`) VALUES (1, 'user1', 'case1', 'group1');
INSERT INTO UT_RWC (id, user, `case`, `grouping`) VALUES (2, 'user2', 'case2', 'group2');
INSERT INTO UT_RWC (id, user, `case`, `grouping`) VALUES (3, 'user3', 'case3', 'group3');

CREATE TABLE Mt_Lc (
    id INT PRIMARY KEY,
    column_name TEXT
);

INSERT INTO Mt_Lc (id, column_name) VALUES (1, 'value1');
INSERT INTO Mt_Lc (id, column_name) VALUES (2, 'value2');
INSERT INTO Mt_Lc (id, column_name) VALUES (3, 'value3');

CREATE TABLE Mt_Uc (
    id INT PRIMARY KEY,
    COLUMN_NAME TEXT
);

INSERT INTO Mt_Uc (id, COLUMN_NAME) VALUES (1, 'value1');
INSERT INTO Mt_Uc (id, COLUMN_NAME) VALUES (2, 'value2');
INSERT INTO Mt_Uc (id, COLUMN_NAME) VALUES (3, 'value3');

CREATE TABLE Mt_Mc (
    id INT PRIMARY KEY,
    Column_Name TEXT
);

INSERT INTO Mt_Mc (id, Column_Name) VALUES (1, 'value1');
INSERT INTO Mt_Mc (id, Column_Name) VALUES (2, 'value2');
INSERT INTO Mt_Mc (id, Column_Name) VALUES (3, 'value3');

CREATE TABLE Mt_Rwc (
    id INT PRIMARY KEY,
    `user` TEXT,
    `case` TEXT,
    `grouping` TEXT
);

INSERT INTO Mt_Rwc (id, user, `case`, `grouping`) VALUES (1, 'user1', 'case1', 'group1');
INSERT INTO Mt_Rwc (id, user, `case`, `grouping`) VALUES (2, 'user2', 'case2', 'group2');
INSERT INTO Mt_Rwc (id, user, `case`, `grouping`) VALUES (3, 'user3', 'case3', 'group3');

-- tablename reserved in both yb/mysql

CREATE TABLE `case` (
    id INT PRIMARY KEY,
    user TEXT,
    `case` TEXT,
    `grouping` TEXT
);

INSERT INTO `case` (id, user, `case`, `grouping`) VALUES (1, 'user1', 'case1', 'group1');
INSERT INTO `case` (id, user, `case`, `grouping`) VALUES (2, 'user2', 'case2', 'group2');
INSERT INTO `case` (id, user, `case`, `grouping`) VALUES (3, 'user3', 'case3', 'group3');

-- tablename reserved in yb but not mysql

CREATE TABLE user (
    id INT PRIMARY KEY,
    user TEXT,
    `case` TEXT,
    `grouping` TEXT
);

INSERT INTO `user` (id, user, `case`, `grouping`) VALUES (1, 'user1', 'case1', 'group1');
INSERT INTO `user` (id, user, `case`, `grouping`) VALUES (2, 'user2', 'case2', 'group2');
INSERT INTO `user` (id, user, `case`, `grouping`) VALUES (3, 'user3', 'case3', 'group3');

-- tablename reserved in mysql but not yb

CREATE TABLE `grouping` (
    id INT PRIMARY KEY,
    user TEXT,
    `case` TEXT,
    `grouping` TEXT
);

INSERT INTO `grouping` (id, user, `case`, `grouping`) VALUES (1, 'user1', 'case1', 'group1');
INSERT INTO `grouping` (id, user, `case`, `grouping`) VALUES (2, 'user2', 'case2', 'group2');
INSERT INTO `grouping` (id, user, `case`, `grouping`) VALUES (3, 'user3', 'case3', 'group3');
