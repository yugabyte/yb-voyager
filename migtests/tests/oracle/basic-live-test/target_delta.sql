insert into test_schema.x values(9,10);
insert into test_schema.x values(11,15);
update test_schema.x set id2=10 where id=9;
delete from test_schema.x where id=5;
update test_schema.x set id2=60 where id=5;
insert into test_schema.x values(100,5);

-- DMLs which can possible cause conflicts on import side (DELETE-INSERT, UPDATE-INSERT, DELETE-UPDATE, UPDATE-UPDATE)

-- DELETE-INSERT CONFLICT
-- Delete a row (id=1) and then insert with the same unique key value (id=9)
DELETE FROM user_table WHERE id = 1;
INSERT INTO user_table (id, email) VALUES (9, 'user1@example.com');

-- UPDATE-INSERT CONFLICT
-- Update a row (id=2) and then insert with the same unique key value (id=10)
UPDATE user_table SET email = 'updated_user2@example.com' WHERE id = 2;
INSERT INTO user_table (id, email) VALUES (10, 'user2@example.com');

-- DELETE-UPDATE CONFLICT
-- Delete a row (id=3) and then update another row with the same unique key value (id=5)
DELETE FROM user_table WHERE id = 3;
UPDATE user_table SET email = 'user3@example.com' WHERE id = 5;

-- UPDATE-UPDATE CONFLICT
-- Update a row (id=4) and then update another row with the same unique key value (id=6)
UPDATE user_table SET email = 'updated_user4@example.com' WHERE id = 4;
UPDATE user_table SET email = 'user4@example.com' WHERE id = 6;
