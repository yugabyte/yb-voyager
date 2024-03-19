insert into ext_test(password) values (schema2.crypt('johnspassword', schema2.gen_salt('bf')));


CALL tt_insert_data(1);
CALL tt_insert_data(2);
CALL tt_insert_data(3);
CALL tt_insert_data(4);


insert into Recipients(First_name,Last_name,Misc) values ('abc','xyz','YES');

insert into "Mixed_Case_Table_Name_Test" (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into "Mixed_Case_Table_Name_Test" (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into "Mixed_Case_Table_Name_Test" (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into "Mixed_Case_Table_Name_Test" (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into "Mixed_Case_Table_Name_Test" (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');


INSERT into "group" values(1, 'abc');
INSERT into "group" values(2, 'abc');
INSERT into "group" values(3, 'abc');
INSERT into "group" values(4, 'abc');
INSERT into "group" values(5, 'abc');
