CREATE TABLE "lt_lc_uc" (
    id INT PRIMARY KEY,
    "column_name" varchar(1000),
    COLUMN_NAME2 varchar(1000)
);

CREATE TABLE "lt_rwc" (
    id INT PRIMARY KEY,
    "user" varchar(1000),
    limit varchar(1000),
    "number" varchar(1000),
    "Column_Name" varchar(1000)
);

CREATE TABLE UT_UC (
    id INT PRIMARY KEY,
    COLUMN_NAME varchar(1000)
);

CREATE TABLE UT_MC (
    id INT PRIMARY KEY,
    "Column_Name" varchar(1000)
);

CREATE TABLE UT_RWC (
    id INT PRIMARY KEY,
    "user" varchar(1000),
    limit varchar(1000),
    "Column_Name" varchar(1000)
);

CREATE TABLE "Mt_Rwc" (
    id INT PRIMARY KEY,
    "user" varchar(1000),
    "column_name" varchar(1000),
    limit varchar(1000)
);

-- tablename reserved in both yb/oracle/upper case reserved word

CREATE TABLE "CASE" (
    id INT PRIMARY KEY,
    "USER" varchar(1000),
    "case" varchar(1000),
    limit varchar(1000)
);

-- tablename reserved in oracle but not yb

CREATE TABLE "number" (
    id INT PRIMARY KEY,
    "Column_Name" varchar(1000),
    "case" varchar(1000),
    limit varchar(1000)
);

-- tablename reserved in yb but not oracle

CREATE TABLE limit (
    id INT PRIMARY KEY,
    "Column_Name" varchar(1000),
    "case" varchar(1000),
    limit varchar(1000)
);

-- case-sensitive reserved word table/column

CREATE TABLE "RowId" (
    id INT PRIMARY KEY,
    "User" varchar(1000),
    "TABLE" varchar(1000)
);


-- case-sensitive pk

CREATE TABLE cs_pk (
    "Id" INT PRIMARY KEY,
    "column_name" varchar(1000),
    COLUMN_NAME2 varchar(1000)
);

--reserved word pk

CREATE TABLE "rw_pk" (
    "USER" INT PRIMARY KEY,
    col varchar(1000),
    COLUMN_NAME2 varchar(1000)
);