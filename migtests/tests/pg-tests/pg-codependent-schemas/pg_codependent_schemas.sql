-- creating the two schemas

drop schema if exists public cascade;

create schema public;

drop schema if exists schema2 cascade;

Create schema schema2;


-- testing foreign key dependencies from different schemas


drop table if exists schema2.foreign_test cascade;
drop table if exists foreign_test cascade;
drop table if exists schema2.primary_test cascade;
drop table if exists primary_test cascade;

CREATE TABLE primary_test (
    ID serial primary key,
    first_name varchar(255) NOT NULL,
    last_name varchar(255),
    Age int
);
insert into primary_test (first_name, last_name, age) values ('Modestine', 'MacMeeking', 20);
insert into primary_test (first_name, last_name, age) values ('Genna', 'Kaysor', 50);
insert into primary_test (first_name, last_name, age) values ('Tess', 'Wesker', 56);

CREATE TABLE schema2.foreign_test (
    ID int NOT NULL,
    ONumber int NOT NULL,
    PID int unique,
    PRIMARY KEY (ID),
    FOREIGN KEY (PID) REFERENCES primary_test(ID)
);

insert into schema2.foreign_test values (1,3,1);
insert into schema2.foreign_test values (2,2,2);
insert into schema2.foreign_test values (3,2,3);

CREATE TABLE schema2.primary_test (
    ID serial primary key,
    first_name varchar(255) NOT NULL,
    last_name varchar(255),
    PID INT unique,
    FOREIGN KEY (PID) REFERENCES schema2.foreign_test(PID)
);
insert into schema2.primary_test (first_name, last_name,PID) values ('Modestine', 'MacMeeking',1);
insert into schema2.primary_test (first_name, last_name, PID) values ('Genna', 'Kaysor',2);


CREATE TABLE foreign_test (
    ID int NOT NULL,
    ONumber int NOT NULL,
    PID int,
    PRIMARY KEY (ID),
    FOREIGN KEY (PID) REFERENCES schema2.primary_test(PID)
);

insert into foreign_test values (1,3,1);
insert into foreign_test values (2,2,2);



-- creating codependent views 

drop view if exists v1 cascade;

create view v1 as select a.id, a.first_name as a_first_name,b.first_name as b_first_name from primary_test a inner join schema2.primary_test b using(id);

select * from v1;


drop view if exists schema2.v1;

create view schema2.v1 as select v1.id, a.last_name as a_last_name,b.last_name as b_last_name from primary_test a,schema2.primary_test b,v1 where v1.id=b.id and v1.id=a.id;

select * from schema2.v1;


drop materialized view if exists mv1;

create materialized view mv1 as select a.id as aid,b.id as bid from schema2.foreign_test a,foreign_test b where a.pid=b.pid with data;

select * from mv1;


-- creating partitions in different schemas

drop table if exists list_part;

CREATE TABLE list_part (id INTEGER, status TEXT, arr NUMERIC) PARTITION BY LIST(status);

CREATE TABLE list_active PARTITION OF list_part FOR VALUES IN ('ACTIVE');

CREATE TABLE list_archived PARTITION OF list_part FOR VALUES IN ('EXPIRED');

CREATE TABLE schema2.list_others PARTITION OF list_part DEFAULT;

INSERT INTO list_part VALUES (1,'ACTIVE',100), (2,'RECURRING',20), (3,'EXPIRED',38), (4,'REACTIVATED',144), (5,'ACTIVE',50);

\d+ list_part

SELECT tableoid::regclass,* FROM list_part;



/*

Describing the flow below:

schema2.tt_insert_data() PROCEDURE inserts values in public.tt TABLE  
this activates the TRIGGER public.audit_trigger which calls the PROCEDURE schema2.auditlogfunc()
PROCEDURE schema2.auditlogfunc() inserts data in schema2.audit TABLE
this activates the TRIGGER schema2.audit_trigger which calls the PROCEDURE public.auditlogfunc() 
PROCEDURE public.auditlogfunc() inserts data in public.audit2 TABLE

*/

-- creating tt table in public

drop table if exists tt;

CREATE TABLE tt (i int);

-- creating audit table in schema2

drop table if exists schema2.audit;

create table schema2.audit(id text);

-- creating audit2 table in public

drop table if exists audit2;

create table audit2(id text);

-- creating function in schema2

drop function if exists schema2.auditlogfunc() cascade;

CREATE OR REPLACE FUNCTION schema2.auditlogfunc() RETURNS TRIGGER AS $example_table$
   BEGIN
      INSERT INTO schema2.audit(id) VALUES (current_timestamp); -- random comment
      RETURN NEW;
   END;
$example_table$ LANGUAGE plpgsql;

-- creating trigger in public

drop trigger if exists audit_trigger on tt;

CREATE TRIGGER audit_trigger AFTER INSERT ON tt
FOR EACH ROW EXECUTE PROCEDURE schema2.auditlogfunc();

-- creating function in public

drop function if exists auditlogfunc();

CREATE OR REPLACE FUNCTION auditlogfunc() RETURNS TRIGGER AS $example_table$
   BEGIN
      INSERT INTO audit2(id) VALUES (current_timestamp); -- random comment
      RETURN NEW;
   END;
$example_table$ LANGUAGE plpgsql;

-- creating trigger in schema2

set search_path to schema2;

drop trigger if exists audit_trigger on schema2.audit;

CREATE TRIGGER audit_trigger AFTER INSERT ON schema2.audit
FOR EACH ROW EXECUTE PROCEDURE public.auditlogfunc();

set search_path to public;

-- creating procedure in schema2 to insert data in tt table in public

drop procedure if exists schema2.tt_insert_data;

CREATE OR REPLACE PROCEDURE schema2.tt_insert_data("i" integer)

LANGUAGE SQL

AS $$

INSERT INTO tt VALUES ("i");

$$;

-- calling tt_insert_data procedure which activates the entire flow

CALL schema2.tt_insert_data(1);
CALL schema2.tt_insert_data(2);
CALL schema2.tt_insert_data(3);
CALL schema2.tt_insert_data(4);

SELECT * from tt;
SELECT * from schema2.audit;
select * from audit2;





