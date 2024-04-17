create table x(id int primary key,id2 int);

CREATE TABLE user_table (
    id INT PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    status VARCHAR(50) DEFAULT 'active'
);

create table date_time_types(id int primary key,date_val DATE, time_val timestamp);