drop table if exists single_index_test;

create table single_index_test(id int primary key, first_name varchar(20),last_name varchar(20),val boolean);

insert into single_index_test values(1,'abc','def',true);
insert into single_index_test values(2,'acc','vfd',true);
insert into single_index_test values(3,'abc','vfd',true);
insert into single_index_test values(4,'ggg','def',false);

explain select * from single_index_test where first_name='abc';

CREATE INDEX single_ind ON single_index_test(first_name); 
\d single_index_test
explain select * from single_index_test where first_name='abc';



drop table if exists mult_index_test;

CREATE TABLE mult_index_test(
    id INT ,
    first_name VARCHAR(50) NOT NULL,
    last_name VARCHAR(50) NOT NULL
);

INSERT INTO mult_index_test
SELECT
  id,
  md5(random()::text),
  md5(random()::text)
from (
  SELECT * FROM generate_series(1,100000) AS id
) AS x;

-- explain select * from mult_index_test where first_name like '%fd%' and last_name like '%fg%';

CREATE INDEX mult_ind ON mult_index_test(first_name,last_name); 

\d mult_index_test

-- explain select * from mult_index_test where first_name like '%fd%' and last_name like '%fg%';



drop table if exists outunique_index_test;

create table outunique_index_test(id int primary key, first_name varchar(20),last_name varchar(20),val boolean);

insert into outunique_index_test values(1,'abc','ef',true);
insert into outunique_index_test values(2,'abd','vfd',true);
insert into outunique_index_test values(3,'ggg','dgef',false);
insert into outunique_index_test values(4,'gfg','deaf',false);

explain select * from outunique_index_test where last_name='vfd';

CREATE unique INDEX out_uni ON outunique_index_test(last_name); 
\d outunique_index_test
explain select * from outunique_index_test where last_name='vfd';


drop table if exists desc_index_test;

create table desc_index_test(id int primary key, f_name varchar(20),l_name varchar(20),val boolean);

insert into desc_index_test values(1,'abc','def',true);
insert into desc_index_test values(2,'abc','vfd',true);
insert into desc_index_test values(3,'ggg','def',false);

explain select * from desc_index_test where l_name='vfd';

CREATE INDEX desc_ind ON desc_index_test(l_name desc); 
\d desc_index_test
explain select * from desc_index_test where l_name='vfd';

drop table if exists partial_index_test;

create table partial_index_test (
	id serial primary key ,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Guillermo', 'Hammill', 'ghammill7@nasa.gov', 'Male', '254.255.111.71');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Chelsey', 'Mably', 'cmably8@fc2.com', 'Female', '34.107.49.60');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Noak', 'Meecher', 'nmeecher9@quantcast.com', 'Male', '152.239.228.215');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Charissa', 'Sturney', 'csturneya@joomla.org', 'Female', '247.146.200.196');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Abie', 'Steventon', 'asteventonb@bigcartel.com', 'Male', '126.156.123.200');
insert into partial_index_test (first_name, last_name, email, gender, ip_address) values ('Augustus', 'Scarbarrow', 'ascarbarrowc@over-blog.com', 'Male', '4.125.129.15');

explain select id,first_name from partial_index_test where gender='Female';

CREATE INDEX partial_ind ON partial_index_test(gender) where gender='Female';

\d partial_index_test

explain select id,first_name from partial_index_test where gender='Female';


drop table if exists exp_index_test;

create table exp_index_test (
	id serial primary key ,
	first_name VARCHAR(50),
	last_name VARCHAR(50),
	email VARCHAR(50),
	gender VARCHAR(50),
	ip_address VARCHAR(20)
);
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Modestine', 'MacMeeking', 'mmacmeeking0@zimbio.com', 'Female', '208.44.58.185');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Genna', 'Kaysor', 'gkaysor1@hibu.com', 'Female', '202.48.51.58');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Tess', 'Wesker', 'twesker2@scientificamerican.com', 'Female', '177.153.32.186');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Magnum', 'Danzelman', 'mdanzelman3@storify.com', 'Bigender', '192.200.33.56');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Mitzi', 'Pidwell', 'mpidwell4@shutterfly.com', 'Female', '216.4.250.71');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Milzie', 'Rohlfing', 'mrohlfing5@java.com', 'Female', '230.101.87.42');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Gena', 'Varga', 'gvarga6@mapquest.com', 'Female', '170.240.242.112');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Guillermo', 'Hammill', 'ghammill7@nasa.gov', 'Male', '254.255.111.71');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Chelsey', 'Mably', 'cmably8@fc2.com', 'Female', '34.107.49.60');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Noak', 'Meecher', 'nmeecher9@quantcast.com', 'Male', '152.239.228.215');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Charissa', 'Sturney', 'csturneya@joomla.org', 'Female', '247.146.200.196');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Abie', 'Steventon', 'asteventonb@bigcartel.com', 'Male', '126.156.123.200');
insert into exp_index_test (first_name, last_name, email, gender, ip_address) values ('Augustus', 'Scarbarrow', 'ascarbarrowc@over-blog.com', 'Male', '4.125.129.15');

explain select id,first_name from exp_index_test where lower(gender)='female';

CREATE INDEX exp_ind ON exp_index_test(lower(gender));

\d exp_index_test

explain select id,first_name from exp_index_test where lower(gender)='female';



drop table if exists hash_index_test;

create table hash_index_test(id int primary key, first_name varchar(20),last_name varchar(20),val boolean);

insert into hash_index_test values(1,'abc','def',true);
insert into hash_index_test values(2,'acc','vfd',true);
insert into hash_index_test values(3,'abc','vfd',true);
insert into hash_index_test values(4,'ggg','def',false);

explain select * from hash_index_test where first_name='abc';

CREATE INDEX hash_ind ON hash_index_test using hash(first_name); 
\d hash_index_test
explain select * from hash_index_test where first_name='abc';


drop table if exists covering_index_test;

CREATE TABLE IF NOT EXISTS covering_index_test (id bigint, username text);

INSERT INTO covering_index_test SELECT n,'Number'||to_hex(n) from generate_series(1,1000) n;

EXPLAIN SELECT * FROM covering_index_test WHERE upper(username)='NUMBER42';

CREATE INDEX covering_index_test_upper_covering_index_test ON covering_index_test( (upper(username))) INCLUDE (username);

EXPLAIN SELECT * FROM covering_index_test WHERE upper(username)='NUMBER42';


CREATE EXTENSION pg_trgm;

drop table if exists gin_index_test;

CREATE TABLE gin_index_test (
  body text,
  body_indexed text
);

INSERT INTO gin_index_test
SELECT
  md5(random()::text),
  md5(random()::text)
from (
  SELECT * FROM generate_series(1,100000) AS id
) AS x;

explain select * from gin_index_test where body_indexed like '%d%';


CREATE INDEX gin_search_idx ON gin_index_test USING gin (body_indexed gin_trgm_ops);

\d gin_index_test

explain select * from gin_index_test where body_indexed like '%d%';

/*
-- Not supported by yb

drop table if exists brin_index_test;

CREATE TABLE brin_index_test (
  body text,
  body_indexed text
);

INSERT INTO brin_index_test
SELECT
  md5(random()::text),
  md5(random()::text)
from (
  SELECT * FROM generate_series(1,100) AS id
) AS x;

explain select * from brin_index_test where body_indexed like '%vfd%';


CREATE INDEX brin_search_idx ON brin_index_test USING brin (body_indexed);

\d brin_index_test

explain select * from brin_index_test where body_indexed like '%vfd%';

-- Not supported by yb

drop table if exists gist_index_test;

create table gist_index_test(p point);

insert into gist_index_test(p) values
  (point '(1,1)'), (point '(3,2)'), (point '(6,3)'),
  (point '(5,5)'), (point '(7,8)'), (point '(8,6)');
  
explain select * from gist_index_test where p <@ box '(2,1),(7,4)';  
  
create index on gist_index_test using gist(p);

\d gist_index_test

explain select * from gist_index_test where p <@ box '(2,1),(7,4)';  
*/
