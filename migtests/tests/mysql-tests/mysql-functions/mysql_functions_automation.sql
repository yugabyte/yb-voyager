drop table if exists auto_gen;

create table auto_gen (
	id int primary key auto_increment,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);

insert into auto_gen (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into auto_gen (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into auto_gen (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into auto_gen (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into auto_gen (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into auto_gen (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into auto_gen (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');

drop function f_name;

create function f_name(first_n varchar(50),last_n varchar(50))
returns varchar(100) deterministic
return concat(first_n,' ',last_n);

SELECT 
    routine_name,routine_type,data_type,routine_definition,is_deterministic
FROM
    information_schema.routines
WHERE
    routine_type = 'FUNCTION'
        AND routine_schema = 'func_check';
        
select f_name(first_name,last_name) from auto_gen;
        
drop function if exists add_two;

create function add_two(num1 int,num2 int)
returns int deterministic
return num1+num2;

select add_two(1,2);