CREATE TABLE lt_lc (
    id INT PRIMARY KEY,
    column_name varchar(1000)
);

CREATE TABLE lt_uc (
    id INT PRIMARY KEY,
    "COLUMN_NAME" varchar(1000)
);

CREATE TABLE lt_mc (
    id INT PRIMARY KEY,
    "Column_Name" varchar(1000)
);

CREATE TABLE lt_rwc (
    id INT PRIMARY KEY,
    "user" varchar(1000),
    "case" varchar(1000)
);

CREATE TABLE "UT_LC" (
    id INT PRIMARY KEY,
    column_name varchar(1000)
);

CREATE TABLE "UT_UC" (
    id INT PRIMARY KEY,
    "COLUMN_NAME" varchar(1000)
);

CREATE TABLE "UT_MC" (
    id INT PRIMARY KEY,
    "Column_Name" varchar(1000)
);

CREATE TABLE "UT_RWC" (
    id INT PRIMARY KEY,
    "user" varchar(1000),
    "case" varchar(1000),
    "table" varchar(1000)
);

CREATE TABLE "Mt_Rwc" (
    id INT PRIMARY KEY,
    "user" varchar(1000),
    "COLUMN_NAME" varchar(1000),
    "Table" varchar(1000)
);

-- tablename reserved in both pg/yb without reserved word columns

CREATE TABLE "integer" (
    id INT PRIMARY KEY,
    "Column_Name" varchar(1000)
);

-- tablename reserved in both pg/yb with reserved word columns

CREATE TABLE "case" (
    id INT PRIMARY KEY,
    "user" varchar(1000),
    "case" varchar(1000),
    "table" varchar(1000)
);

-- case-sensitive reserved word table

CREATE TABLE "Table" (
    id INT PRIMARY KEY,
    "User_name" varchar(1000),
    "table" varchar(1000)
);

-- case-sensitive reserved word column

CREATE TABLE cs_rwc (
    id INT PRIMARY KEY,
    "User" varchar(1000),
    "table" varchar(1000)
);

-- case-sensitive reserved word table/column

CREATE TABLE "USER" (
    id INT PRIMARY KEY,
    "User" varchar(1000)
);

CREATE TABLE "Customers" (id INTEGER, "Statuses" TEXT, "User" NUMERIC, PRIMARY KEY(id, "Statuses", "User")) PARTITION BY LIST("Statuses");

CREATE TABLE cust_active PARTITION OF "Customers" FOR VALUES IN ('ACTIVE', 'RECURRING','REACTIVATED') PARTITION BY RANGE("User");
CREATE TABLE cust_other  PARTITION OF "Customers" DEFAULT;

CREATE TABLE "SCHEMA" PARTITION OF cust_active FOR VALUES FROM (MINVALUE) TO (101) PARTITION BY HASH(id);
CREATE TABLE cust_part11 PARTITION OF "SCHEMA" FOR VALUES WITH (modulus 2, remainder 0);
CREATE TABLE cust_part12 PARTITION OF "SCHEMA" FOR VALUES WITH (modulus 2, remainder 1);

CREATE TABLE cust_user_large PARTITION OF cust_active FOR VALUES FROM (101) TO (MAXVALUE) PARTITION BY HASH(id);
CREATE TABLE cust_part21 PARTITION OF cust_user_large FOR VALUES WITH (modulus 2, remainder 0);
CREATE TABLE "cust_Part22" PARTITION OF cust_user_large FOR VALUES WITH (modulus 2, remainder 1);

