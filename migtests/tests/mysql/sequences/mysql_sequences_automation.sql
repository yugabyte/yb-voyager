
drop table if exists sequence_check_1;

create table sequence_check_1 (
	id int primary key auto_increment,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into sequence_check_1 (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into sequence_check_1 (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into sequence_check_1 (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into sequence_check_1 (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into sequence_check_1 (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into sequence_check_1 (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into sequence_check_1 (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');

select * from sequence_check_1;

drop table if exists sequence_check_2;

create table sequence_check_2 (
	id int primary key auto_increment,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);

ALTER TABLE sequence_check_2 AUTO_INCREMENT = 50;

insert into sequence_check_2 (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into sequence_check_2 (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into sequence_check_2 (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into sequence_check_2 (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into sequence_check_2 (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into sequence_check_2 (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into sequence_check_2 (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');

select * from sequence_check_2;

drop table if exists sequence_check_3;

create table sequence_check_3 (
	id int primary key auto_increment,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into sequence_check_3 (id,first_name, last_name, email, gender, ip_address) values (150,'Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into sequence_check_3 (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into sequence_check_3 (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into sequence_check_3 (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into sequence_check_3 (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into sequence_check_3 (id,first_name, last_name, email, gender, ip_address) values (400,'Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into sequence_check_3 (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');
insert into sequence_check_3 (first_name, last_name, email, gender, ip_address) values ('Noak', 'Meecher', 'nmeecher9@quantcast.com', 'Male', '152.239.228.215');

select * from sequence_check_3;
