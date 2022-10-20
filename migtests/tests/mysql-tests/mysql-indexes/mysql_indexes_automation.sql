
drop table if exists primary_index_test;
Create Table primary_index_test(id binary(16), Name VArchar(20));

INSERT INTO primary_index_test(id, Name)
VALUES(UUID_TO_BIN(UUID()),'John Doe'),
      (UUID_TO_BIN(UUID()),'Will Smith'),
      (UUID_TO_BIN(UUID()),'Mary Jane');


select * from primary_index_test;

alter table primary_index_test add constraint pkey_index primary key(id);

desc primary_index_test;
SHOW INDEXES FROM primary_index_test;



drop table if exists single_index_test;

create table single_index_test(id int primary key, first_name varchar(20),last_name varchar(20),val boolean);

insert into single_index_test values(1,'abc','def',true);
insert into single_index_test values(2,'acc','vfd',true);
insert into single_index_test values(3,'abc','vfd',true);
insert into single_index_test values(4,'ggg','def',false);

explain select * from single_index_test where first_name='abc';

CREATE INDEX single_ind ON single_index_test(first_name); 
SHOW INDEXES FROM single_index_test;
explain select * from single_index_test where first_name='abc';


drop table if exists mult_index_test;

create table mult_index_test(id int primary key, first_name varchar(20),last_name varchar(20),val boolean);

insert into mult_index_test values(1,'abc','def',true);
insert into mult_index_test values(2,'abc','vfd',true);
insert into mult_index_test values(3,'ggg','def',false);

explain select * from mult_index_test where last_name='vfd';

CREATE INDEX mult_ind ON mult_index_test(val,last_name); 
SHOW INDEXES FROM mult_index_test;
explain select * from mult_index_test where last_name='vfd';


drop table if exists inunique_index_test;

create table inunique_index_test(id int primary key, first_name varchar(20) unique,last_name varchar(20),val boolean);

insert into inunique_index_test values(1,'abc','def',true);
insert into inunique_index_test values(2,'abd','vfd',true);
insert into inunique_index_test values(3,'ggg','def',false);

explain select * from inunique_index_test where last_name='vfd';

CREATE INDEX in_uni ON inunique_index_test(val); 
SHOW INDEXES FROM inunique_index_test;
explain select * from inunique_index_test where val=true;


drop table if exists outunique_index_test;

create table outunique_index_test(id int primary key, first_name varchar(20),last_name varchar(20),val boolean);

insert into outunique_index_test values(1,'abc','def',true);
insert into outunique_index_test values(2,'abd','vfd',true);
insert into outunique_index_test values(3,'ggg','def',false);
insert into outunique_index_test values(4,'gfg','def',false);

explain select * from outunique_index_test where last_name='vfd';

CREATE unique INDEX out_uni ON outunique_index_test(first_name); 
SHOW INDEXES FROM outunique_index_test;
explain select * from outunique_index_test where val=true;


drop table if exists desc_index_test;

create table desc_index_test(id int primary key, f_name varchar(20),l_name varchar(20),val boolean);

insert into desc_index_test values(1,'abc','def',true);
insert into desc_index_test values(2,'abc','vfd',true);
insert into desc_index_test values(3,'ggg','def',false);

explain select * from desc_index_test where l_name='vfd';

CREATE INDEX desc_ind ON desc_index_test(f_name desc); 
SHOW INDEXES FROM desc_index_test;
explain select * from desc_index_test where l_name='vfd';


/*drop table if exists exp_index_test;

CREATE TABLE exp_index_test (
  emp_no int NOT NULL,
  salary int,
  from_date date NOT NULL,
  to_date date NOT NULL,
  PRIMARY KEY (emp_no)
);

insert into exp_index_test values(3,1000,'1985-07-06','1985-09-28');
insert into exp_index_test values(4,1000,'1985-07-06','1985-09-28');

CREATE INDEX exp_ind ON exp_index_test((year(to_date))); 

SELECT INDEX_NAME, EXPRESSION 
FROM INFORMATION_SCHEMA.STATISTICS 
WHERE TABLE_NAME = 'exp_index_test'
    AND INDEX_NAME='idx_year_to_date';

SHOW INDEXES FROM exp_index_test;


#SELECT INDEX_NAME, EXPRESSION 
#FROM INFORMATION_SCHEMA.STATISTICS 
#WHERE TABLE_SCHEMA='employees' 
#    AND TABLE_NAME = "exp_index_test" 
#    AND INDEX_NAME='idx_year_to_date';
    
#SHOW INDEXES FROM exp_index_test;
*/



















