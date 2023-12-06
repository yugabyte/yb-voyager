-- single table with case sensitive table name 
-- Note: do not add sequences or any other tables to this test, as the bug occurs only when there is a single table. 
drop table if exists "Mixed_Case_Table_Name_Test";

create table "Mixed_Case_Table_Name_Test" (
	id int primary key,
	name VARCHAR(50)
);

-- aut int NOT NULL GENERATED ALWAYS AS ((id*9)+1) stored

insert into "Mixed_Case_Table_Name_Test" (id, name) values (1, 'Modestine');
insert into "Mixed_Case_Table_Name_Test" (id, name) values (2, 'Genna');
insert into "Mixed_Case_Table_Name_Test" (id, name) values (3, 'Tess');
insert into "Mixed_Case_Table_Name_Test" (id, name) values (4, 'Magnum');
insert into "Mixed_Case_Table_Name_Test" (id, name) values (5, 'Mitzi');
