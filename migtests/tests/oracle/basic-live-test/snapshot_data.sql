insert into x values(1,2);
insert into x values(2,3);
insert into x values(3,4);

-- INSERT INITIAL DATA
INSERT INTO user_table VALUES (1, 'user1@example.com', 'active');
INSERT INTO user_table VALUES (2, 'user2@example.com', 'active');
INSERT INTO user_table VALUES (3, 'user3@example.com', 'active');
INSERT INTO user_table VALUES (4, 'user4@example.com', 'active');
INSERT INTO user_table VALUES (5, 'user5@example.com', 'active');
INSERT INTO user_table VALUES (6, 'user6@example.com', 'active');
INSERT INTO user_table VALUES (7, 'user7@example.com', 'active');
INSERT INTO user_table VALUES (8, 'user8@example.com', 'active');

insert into date_time_types(id,date_val,time_val) values(1,DATE'2020-01-01',TIMESTAMP'2020-05-05 11:45:30.54560');
insert into date_time_types(id,date_val,time_val) values(2,DATE'2020-01-02',CURRENT_TIMESTAMP);
insert into date_time_types(id,date_val,time_val) values(3,TIMESTAMP'2020-01-03 03:12:54',TO_TIMESTAMP('10-Sep-02 14:10:10.123000', 'DD-Mon-RR HH24:MI:SS.FF'));
insert into date_time_types(id,date_val,time_val) values(4,TO_TIMESTAMP('18-Sep-02 14:10:10.878940','DD-Mon-RR HH24:MI:SS.FF'),TO_TIMESTAMP('10-Dec-02 16:10:10.123990', 'DD-Mon-RR HH24:MI:SS.FF'));