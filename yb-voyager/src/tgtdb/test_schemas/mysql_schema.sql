-- TODO: create user as per User creation steps in docs and use that in tests

CREATE TABLE foo (
    id INT PRIMARY KEY,
    name VARCHAR(255)
);

CREATE TABLE bar (
    id INT PRIMARY KEY,
    name VARCHAR(255)
);

CREATE TABLE unique_table (
    id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(100),
    phone VARCHAR(100),
    address VARCHAR(255),
    UNIQUE KEY unique_email_phone (email, phone)  -- Unique constraint on combination of columns
);

CREATE UNIQUE INDEX unique_address_idx ON unique_table (address); -- Unique Index

CREATE TABLE table1 (
    id INT AUTO_INCREMENT PRIMARY KEY,
    name VARCHAR(100)
);

CREATE TABLE table2 (
    id INT AUTO_INCREMENT PRIMARY KEY,
    email VARCHAR(100)
);

CREATE TABLE non_pk1(
    id INT,
    name VARCHAR(255)
);

CREATE TABLE non_pk2(
    id INT,
    name VARCHAR(255)
);