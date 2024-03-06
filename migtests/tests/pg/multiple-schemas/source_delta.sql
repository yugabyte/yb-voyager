-- insert into ext_test(password) values (crypt('cdcpassword', gen_salt('bf')));

CALL tt_insert_data(5);
DELETE from tt where i < 5;

insert into Recipients(First_name,Last_name,Misc) values ('cdc','test','YES');



---- schema2
set search_path to schema2;

-- insert into ext_test(password) values (crypt('cdcpassword', gen_salt('bf')));

CALL tt_insert_data(5);

insert into Recipients(First_name,Last_name,Misc) values ('cdc','test','YES');
insert into Recipients(First_name,Last_name,Misc) values ('cdc2','test2','YES');
