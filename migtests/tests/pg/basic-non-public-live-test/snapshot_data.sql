set search_path to non_public;

insert into x values(1,2);
insert into x values(2,3);
insert into x values(3,4);
INSERT INTO user_table (email) VALUES 
    ('user1@example.com'), 
    ('user2@example.com'), 
    ('user3@example.com'),
    ('user4@example.com'),
    ('user5@example.com'),
    ('user6@example.com'),
    ('user7@example.com'),
    ('user8@example.com');

INSERT INTO test_enum values(1, 'duplicate_payment_method');
INSERT INTO test_enum values(2, 'server_failure');
INSERT INTO test_enum values(3, 'server_failure');
INSERT INTO test_enum values(4, 'duplicate_payment_method');
