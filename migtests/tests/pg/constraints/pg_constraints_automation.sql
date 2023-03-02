drop table if exists not_null_check;

CREATE TABLE not_null_check (
id serial primary key,
	first_name VARCHAR(50) not null,
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into not_null_check (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into not_null_check (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into not_null_check (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into not_null_check (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into not_null_check (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into not_null_check (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');


drop table if exists unique_test;

CREATE TABLE unique_test (
id serial primary key,
	first_name VARCHAR(50) unique,
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into unique_test (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into unique_test (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into unique_test (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into unique_test (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into unique_test (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');



drop table if exists check_test;

CREATE TABLE check_test (
    ID serial primary key,
    first_name varchar(255) NOT NULL,
    last_name varchar(255),
    Age int,
    CHECK (Age>=18)
);
insert into check_test (first_name, last_name, age) values ('Modestine', 'MacMeeking', 20);
insert into check_test (first_name, last_name, age) values ('Genna', 'Kaysor', 50);
insert into check_test (first_name, last_name, age) values ('Tess', 'Wesker', 56);
insert into check_test (first_name, last_name, age) values ('Magnum', 'Danzelman', 89);
insert into check_test (first_name, last_name, age) values ('Mitzi', 'Pidwell', 34);
insert into check_test (first_name, last_name, age) values ('Milzie', 'Rohlfing', 70);



drop table if exists default_test;

CREATE TABLE default_test (
    ID serial primary key,
    first_name varchar(255) NOT NULL,
    last_name varchar(255),
    Age int default 18
);
insert into default_test (first_name, last_name, age) values ('Modestine', 'MacMeeking', 20);
insert into default_test (first_name, last_name, age) values ('Genna', 'Kaysor', 50);
insert into default_test (first_name, last_name, age) values ('Tess', 'Wesker', 56);
insert into default_test (first_name, last_name, age) values ('Magnum', 'Danzelman', 89);
insert into default_test (first_name, last_name, age) values ('Mitzi', 'Pidwell', 34);
insert into default_test (first_name, last_name, age) values ('Milzie', 'Rohlfing', 70);
insert into default_test (first_name, last_name) values ('Milzie', 'Rohlfing');


drop table if exists foreign_test;
drop table if exists primary_test;

CREATE TABLE primary_test (
    ID serial primary key,
    first_name varchar(255) NOT NULL,
    last_name varchar(255),
    Age int
);
insert into primary_test (first_name, last_name, age) values ('Modestine', 'MacMeeking', 20);
insert into primary_test (first_name, last_name, age) values ('Genna', 'Kaysor', 50);
insert into primary_test (first_name, last_name, age) values ('Tess', 'Wesker', 56);
insert into primary_test (first_name, last_name, age) values ('Magnum', 'Danzelman', 89);
insert into primary_test (first_name, last_name, age) values ('Mitzi', 'Pidwell', 34);
insert into primary_test (first_name, last_name, age) values ('Milzie', 'Rohlfing', 70);

CREATE TABLE foreign_test (
    ID int NOT NULL,
    ONumber int NOT NULL,
    PID int,
    PRIMARY KEY (ID),
    FOREIGN KEY (PID) REFERENCES primary_test(ID)
);

insert into foreign_test values (1,3,4);
insert into foreign_test values (2,2,3);
insert into foreign_test values (3,2,1);
insert into foreign_test values (4,1,2);

--test case for foreign key and unique index dependency

create table xyz(id int);
insert into xyz values(1),(2),(3);

create table abc(name text, xyz_id int);

create unique index idx on xyz using btree(id);

alter table abc add constraint fk FOREIGN KEY (xyz_id) REFERENCES xyz(id);
insert into abc values('fsdfs', 1),('sfdfds',2),('svss', 3); 
