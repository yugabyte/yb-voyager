insert into x values(9,10);
insert into x values(11,15);
update x set id2=10 where id=9;
delete from x where id=5;
update x set id2=60 where id=5;
insert into x values(100,5);


-- DMLs which can possible cause conflicts on import side (DELETE-INSERT, UPDATE-INSERT, DELETE-UPDATE, UPDATE-UPDATE)

-- DELETE-INSERT CONFLICT
DELETE FROM user_table WHERE id = 9;
INSERT INTO user_table (id, email) VALUES (1, 'user1@example.com');

-- UPDATE-INSERT CONFLICT
UPDATE user_table SET email = 'updated_twice_user2@example.com' WHERE id = 2;
INSERT INTO user_table (id, email) VALUES (11, 'updated_user2@example.com');

-- DELETE-UPDATE CONFLICT
DELETE FROM user_table WHERE id = 5;
UPDATE user_table SET email = 'user3@example.com' WHERE id = 7;

-- UPDATE-UPDATE CONFLICT
UPDATE user_table SET email = 'updated_twice_user4@example.com' WHERE id = 6;
UPDATE user_table SET email = 'user4@example.com' WHERE id = 4;

-- events with NULL value for unique key columns
UPDATE user_table SET status = 'active' where id > 0;


-- events for test_partial_unique_index table
-- will uncomment in another PR as part of cdc partitioning strategy changes
-- UPDATE test_partial_unique_index SET most_recent = false WHERE check_id = 1;
-- INSERT INTO test_partial_unique_index (check_id, most_recent) VALUES (1, true);

-- UPDATE test_partial_unique_index SET most_recent = false WHERE check_id = 2;
-- INSERT INTO test_partial_unique_index (check_id, most_recent) VALUES (2, true);
