-- insert into ext_test(password) values (crypt('cdcpassword', gen_salt('bf')));

CALL tt_insert_data(10);
DELETE from tt where i = 5;

---- schema2
set search_path to schema2;

-- insert into ext_test(password) values (crypt('cdcpassword', gen_salt('bf')));

CALL tt_insert_data(10);
