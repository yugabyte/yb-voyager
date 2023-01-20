-- test for schema objects: VIEW, MVIEW, RULE in PostgreSQL

drop schema if exists test_views;

create schema test_views;

drop table if exists test_views.view_table1 cascade;

create table test_views.view_table1 (
	id serial primary key,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Male', '202.48.51.58');
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Guillermo', 'Hammill', 'ghammill7@nasa.gov', 'Male', '254.255.111.71');
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Chelsey', 'Mably', 'cmably8@fc2.com', 'Female', '34.107.49.60');
insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Noak', 'Meecher', 'nmeecher9@quantcast.com', 'Male', '152.239.228.215');

drop table if exists test_views.view_table2 cascade;

create table test_views.view_table2 (
	id serial primary key,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into test_views.view_table2 (first_name, last_name, email, gender, ip_address) values ('Aloysius', 'Capnerhurst', 'acapnerhurstz@goodreads.com', 'Male', '95.114.68.42');
insert into test_views.view_table2 (first_name, last_name, email, gender, ip_address) values ('Katusha', 'Jacob', 'kjacob10@answers.com', 'Female', '76.225.177.100');
insert into test_views.view_table2 (first_name, last_name, email, gender, ip_address) values ('Clywd', 'Rahl', 'crahl11@phoca.cz', 'Male', '108.153.62.82');
insert into test_views.view_table2 (first_name, last_name, email, gender, ip_address) values ('Darnell', 'Fyfield', 'dfyfield12@ucoz.com', 'Male', '246.157.90.10');
insert into test_views.view_table2 (first_name, last_name, email, gender, ip_address) values ('Myrlene', 'Connikie', 'mconnikie13@twitpic.com', 'Female', '54.208.146.115');
insert into test_views.view_table2 (first_name, last_name, email, gender, ip_address) values ('Ettore', 'Vossgen', 'evossgen14@com.com', 'Male', '156.26.89.33');
insert into test_views.view_table2 (first_name, last_name, email, gender, ip_address) values ('Christie', 'McGrory', 'cmcgrory15@ning.com', 'Female', '198.178.94.32');
insert into test_views.view_table2 (first_name, last_name, email, gender, ip_address) values ('Agatha', 'Amey', 'aamey16@hibu.com', 'Female', '132.36.221.179');
insert into test_views.view_table2 (first_name, last_name, email, gender, ip_address) values ('Ranee', 'Hast', 'rhast17@webeden.co.uk', 'Female', '68.206.219.63');


create or replace view test_views.v1 as select first_name,last_name from test_views.view_table1 where gender='Female';

select * from test_views.v1;

create or replace view test_views.v2 as select a.first_name,b.last_name from test_views.view_table1 a,test_views.view_table2 b where a.id=b.id;

select * from test_views.v2;

create or replace view test_views.v3 as select a.first_name,b.last_name from test_views.view_table1 a inner join test_views.view_table2 b using(id);

select * from test_views.v3;

-- need to refresh on the target as it exports as "with no data"

drop materialized view if exists test_views.mv1;

create materialized view test_views.mv1 as select first_name,last_name from test_views.view_table1 where gender='Male' with data;

drop materialized view if exists test_views.xyz_mview;

create materialized view test_views.xyz_mview as select first_name,last_name from test_views.view_table1 where gender='Male' with data;

drop materialized view if exists test_views.abc_mview;
create materialized view test_views.abc_mview as select * from test_views.xyz_mview;

select * from test_views.mv1;
select * from test_views.xyz_mview;
select * from test_views.abc_mview;

insert into test_views.view_table1 (first_name, last_name, email, gender, ip_address) values ('Kah', 'Loger', 'nmeecher9@quantcast.com', 'Male', '152.239.228.215');

REFRESH MATERIALIZED VIEW test_views.mv1;
REFRESH MATERIALIZED VIEW test_views.xyz_mview;
REFRESH MATERIALIZED VIEW test_views.abc_mview;

select * from test_views.mv1;
select * from test_views.xyz_mview;
select * from test_views.abc_mview;

\d+ test_views.v1;
\d+ test_views.v2;
\d+ test_views.v3;
\d+ test_views.mv1;
\d+ test_views.xyz_mview;
\d+ test_views.abc_mview;

CREATE OR REPLACE RULE protect_test_views_view_table1 AS ON UPDATE TO test_views.view_table1 DO INSTEAD NOTHING;

CREATE OR REPLACE RULE protect_test_views_v1 AS ON UPDATE TO test_views.v1 DO INSTEAD NOTHING;

CREATE OR REPLACE RULE protect_test_views_v2 AS ON UPDATE TO test_views.v2 DO INSTEAD NOTHING;

CREATE OR REPLACE RULE protect_test_views_v3 AS ON UPDATE TO test_views.v3 DO INSTEAD NOTHING;

-- RULEs are not supported on Materialized Views in PostgreSQL