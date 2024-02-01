insert into test_schema.x values(9,10);
insert into test_schema.x values(11,15);
update test_schema.x set id2=10 where id=9;
delete from test_schema.x where id=5;
update test_schema.x set id2=60 where id=5;
insert into test_schema.x values(100,5);