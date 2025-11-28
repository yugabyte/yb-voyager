create table x(id int primary key,id2 int);

CREATE TABLE user_table (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    status VARCHAR(50) DEFAULT 'active'
);


CREATE TABLE test_partial_unique_index (
    id SERIAL PRIMARY KEY,
    check_id int,
    most_recent boolean
);

CREATE UNIQUE INDEX idx_test_partial_unique_index ON test_partial_unique_index (check_id) WHERE most_recent;