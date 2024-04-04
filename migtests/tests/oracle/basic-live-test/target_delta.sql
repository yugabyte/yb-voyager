insert into test_schema.x values(9,10);
insert into test_schema.x values(11,15);
update test_schema.x set id2=10 where id=9;
delete from test_schema.x where id=5;
update test_schema.x set id2=60 where id=5;
insert into test_schema.x values(100,5);


INSERT into test_schema.user_table (id, email) VALUES (13, 'user13@example.com');
INSERT into test_schema.user_table (id, email) VALUES (14, 'user14@example.com');

-- conflicting DMLs
-- DELETE-INSERT CONFLICT
DELETE FROM test_schema.user_table WHERE id = 7;
INSERT into test_schema.user_table (id, email) VALUES (11, 'user7@example.com');

-- UPDATE-INSERT CONFLICT
UPDATE test_schema.user_table SET email = 'userxyz@example.com' where id = 8;
INSERT into test_schema.user_table (id, email) VALUES (12, 'user8@example.com');

-- DELETE-UPDATE CONFLICT
DELETE FROM test_schema.user_table WHERE id = 9;
UPDATE test_schema.user_table SET email = 'user1@example.com' WHERE id = 13;

-- UPDATE-UPDATE CONFLICT
UPDATE test_schema.user_table SET email = 'user10@example.com' WHERE id = 10;
UPDATE test_schema.user_table SET email = 'user2@example.com' WHERE id = 14;


insert into test_schema.date_time_types values(7,'2020-01-01 12:23:59.213263','2024-12-05 18:45:30.254566');
insert into test_schema.date_time_types values(8,'2020-01-02 14:25:32.214646',CURRENT_TIMESTAMP);
UPDATE test_schema.date_time_types SET date_val = '2023-06-20 22:21:32.989899' WHERE id = 5;
delete from test_schema.date_time_types where id=6;