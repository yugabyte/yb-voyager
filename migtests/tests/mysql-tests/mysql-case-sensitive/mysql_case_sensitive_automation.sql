drop table if exists UPPER_CASE_TEST;

create table UPPER_CASE_TEST (
	id int primary key auto_increment,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Guillermo', 'Hammill', 'ghammill7@nasa.gov', 'Male', '254.255.111.71');
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Chelsey', 'Mably', 'cmably8@fc2.com', 'Female', '34.107.49.60');
insert into UPPER_CASE_TEST (first_name, last_name, email, gender, ip_address) values ('Noak', 'Meecher', 'nmeecher9@quantcast.com', 'Male', '152.239.228.215');


drop table if exists Mixed_Case_Test;

Create table Mixed_Case_Test (
	id int primary key auto_increment,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);

insert into Mixed_Case_Test (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into Mixed_Case_Test (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into Mixed_Case_Test (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into Mixed_Case_Test (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into Mixed_Case_Test (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into Mixed_Case_Test (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into Mixed_Case_Test (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');

drop table if exists lower_case_test;

create table lower_case_test (
	id int primary key auto_increment,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Guillermo', 'Hammill', 'ghammill7@nasa.gov', 'Male', '254.255.111.71');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Chelsey', 'Mably', 'cmably8@fc2.com', 'Female', '34.107.49.60');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Noak', 'Meecher', 'nmeecher9@quantcast.com', 'Male', '152.239.228.215');
insert into lower_case_test (first_name, last_name, email, gender, ip_address) values ('Charissa', 'Sturney', 'csturneya@joomla.org', 'Female', '247.146.200.196');

desc UPPER_CASE_TEST;
desc Mixed_Case_Test;
desc lower_case_test;

