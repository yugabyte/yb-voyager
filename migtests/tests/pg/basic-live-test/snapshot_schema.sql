create table x(id int primary key,id2 int);
ALTER TABLE user_table REPLICA IDENTITY FULL;

CREATE TABLE user_table (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    status VARCHAR(50) DEFAULT 'active'
);
ALTER TABLE user_table REPLICA IDENTITY FULL; 
