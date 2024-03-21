
CALL tt_insert_data(10);
DELETE from tt where i = 5;

-- export from target fails for tables having enums+domains
/*
insert into Recipients(First_name,Last_name,Misc) values ('cdc','test','YES');
*/

---- schema2
set search_path to schema2;

CALL tt_insert_data(10);

-- export from target fails for tables having enums+domains
/*
insert into Recipients(First_name,Last_name,Misc) values ('cdc','test','YES');
insert into Recipients(First_name,Last_name,Misc) values ('cdc2','test2','YES');
*/