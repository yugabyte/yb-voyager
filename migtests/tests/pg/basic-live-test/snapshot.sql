create table x(id int primary key,id2 int);
insert into x values(1,2);
insert into x values(2,3);
insert into x values(3,4);


-- CREATE TABLE
CREATE TABLE user_table (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE,
    status VARCHAR(50) DEFAULT 'active'
);

-- INSERT INITIAL DATA
INSERT INTO user_table (email) VALUES 
    ('user1@example.com'), 
    ('user2@example.com'), 
    ('user3@example.com'),
    ('user4@example.com'),
    ('user5@example.com'),
    ('user6@example.com'),
    ('user7@example.com'),
    ('user8@example.com');