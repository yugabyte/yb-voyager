-- This test contains reserved words and case sensitive tables/columns

CREATE TABLE `order` (
    id int PRIMARY KEY,
    name varchar(10)
);

CREATE TABLE `user` (
    id int PRIMARY KEY,
    name varchar(10)
);


CREATE TABLE `group` (
    id int PRIMARY KEY,
    name varchar(10)
);

CREATE TABLE reserved_column (
    `user` int,
    `case` text
);

CREATE TABLE `check` (
    `user` int,
    `case` text
);

INSERT into `order` values(1, 'abc');
INSERT into `order` values(2, 'abc');
INSERT into `order` values(3, 'abc');
INSERT into `order` values(4, 'abc');
INSERT into `order` values(5, 'abc');

INSERT into `user` values(1, 'abc');
INSERT into `user` values(2, 'abc');
INSERT into `user` values(3, 'abc');
INSERT into `user` values(4, 'abc');
INSERT into `user` values(5, 'abc');

INSERT into `group` values(1, 'abc');
INSERT into `group` values(2, 'abc');
INSERT into `group` values(3, 'abc');
INSERT into `group` values(4, 'abc');
INSERT into `group` values(5, 'abc');

INSERT into reserved_column values(1, 'abc');
INSERT into reserved_column values(2, 'abc');
INSERT into reserved_column values(3, 'abc');
INSERT into reserved_column values(4, 'abc');
INSERT into reserved_column values(5, 'abc');

INSERT into `check` values(1, 'abc');
INSERT into `check` values(2, 'abc');
INSERT into `check` values(3, 'abc');
INSERT into `check` values(4, 'abc');
INSERT into `check` values(5, 'abc');

drop table if exists `Mixed_Case_Test`;

CREATE TABLE `Mixed_Case_Test` (
	id int primary key auto_increment,
	first_name VARCHAR(50) not null,
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into `Mixed_Case_Test` (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into `Mixed_Case_Test` (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into `Mixed_Case_Test` (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into `Mixed_Case_Test` (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into `Mixed_Case_Test` (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into `Mixed_Case_Test` (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');

drop table if exists `Case_Sensitive_Columns`;

CREATE TABLE `Case_Sensitive_Columns` (
	id int primary key auto_increment,
	`First_Name` VARCHAR(50) not null,
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into `Case_Sensitive_Columns` (`First_Name`, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');

