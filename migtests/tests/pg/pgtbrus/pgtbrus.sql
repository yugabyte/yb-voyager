create type myenum as enum('a', 'b');--adding some complex types
create type mycomposit as (a int, b text); --and again...
create table t(i serial not null primary key, ts timestamptz(0) default now(), j json, t text, e myenum, c mycomposit);
insert into t values(0, now(), '{"a":{"aa":[1,3,2]}}', 'foo', 'b', (3,'aloha'));
insert into t (j,e) values ('{"b":null}', 'a');
insert into t (t) select chr(g) from generate_series(100,240) g;--add some more data
delete from t where i > 3 and i < 142; --mockup activity and mix tuples to be not sequential
insert into t (t) select null;
--now what we have?..
select ctid,* from t;
update t set j = '{}' where i =3; --(0,4)
select ctid,* from t;

--now lets say we will migrate using postgres_fdw (for earlier then 9.3 releases +- same setup with dblink)
create extension postgres_fdw;
create server p10 foreign data wrapper postgres_fdw options (host 'localhost', dbname 'p10'); --I know it's the same 9.3 server - change host to other version and use other cluster. It's not important here
create user MAPPING FOR vao SERVER p10 options(user 'vao', password 'q'); --is it only for table owners or for all users?.. maybe then set role?..
--now lets create same table to migrate data to (yu can use pg_dump -s and I have DDL ready, so just:)

create type myenum as enum('a', 'b');--adding some complex types
create type mycomposit as (a int, b text); --and again...
create table t(i serial not null primary key, ts timestamptz(0) default now(), j json, t text, e myenum, c mycomposit);
--getting back to 9.3

--let's use foreign tables for data migration:
create foreign table f_t(i serial, ts timestamptz(0) default now(), j json, t text, e myenum, c mycomposit) server p10 options (TABLE_name 't');
--and create insert function and trigger:
create or replace function tgf_i() returns trigger as $$
begin
  execute format('insert into %I select ($1).*','f_'||TG_RELNAME) using NEW;
  return NEW;
end;
$$ language plpgsql;

create trigger tgi before insert on t for each row execute procedure tgf_i();
--OK - first table ready - lets try logical trigger based replication on inserts:
insert into t (t) select 'one';
--and now transactional:
begin;
  insert into t (t) select 'two';
  select ctid, * from f_t;
  select ctid, * from t;
rollback;
select ctid, * from f_t where i > 143;
select ctid, * from t where i > 143;
/*
obviously all actions triggers and initial data copy should happen in same tranaction
or, if you want to make it really complicated, but allow data insertions while copy, revoke update,delete,trucate on table from everyone, or create instead of rules with exceptions...
also pbviously if you know you have some "frozen" table part (let's say all days but current, or so) - you can copy frozen part without locking and lock only for active data part
*/
--anyway update triger:
-- much inspired by https://stackoverflow.com/questions/15343075/update-multiple-columns-in-a-trigger-function-in-plpgsql
/* ah,  before we start let us create a function returning PK name:*/

create or replace function pk(_t name) returns name as $$
  select attname
  from pg_constraint c
  join pg_attribute a on a.attrelid = conrelid and a.attnum = conkey[1]
  where conrelid = _t::regclass
  ; --of course you will need more complicated statement for PK on several attributes. and obviously we have some penalty for querying catalog on each row - this approah has a price
$$ language sql;
--and now the update trigger function:

create or replace function tgf_u() returns trigger as $$
declare
  _pk name;
  _val text;
  _cols text;
  _vals text;
  _sql text;
  _c int := 0;
begin
    _pk := pk(TG_RELNAME);

  select
    string_agg(format('%1$I', attname, atttypid::regtype),',')
  , string_agg(format('row_alias.%I', attname),',')
    into _cols, _vals
  from pg_attribute
  where attrelid = TG_RELNAME::regclass and attnum > 0 and not attisdropped
  ;

  raise info '%', NEW;
  execute format('select ($1).%I',_pk) into _val using OLD; --in case update updates the PK value we use OLD, not NEW
  execute format('update %1$I set (%2$s) = (%3$s) from (select ($1).*) row_alias where %1$I.%4$I = %5$s', 'f_'||TG_RELNAME, _cols, _vals, _pk, _val) using NEW;
  get diagnostics _c:= row_count;
  if _c < 1 then
    raise exception '%', 'This data is not replicated yet, thus can''t be deleted';
  end if;

  return NEW;
end;
$$ language plpgsql
;



create trigger tgu before update on t for each row execute procedure tgf_u();
--https://www.postgresql.org/docs/9.3/static/catalog-pg-constraint.html

begin;
	update t set j = '{"updated":true}' where i = 144;
	select * from t where i = 144;
	select * from f_t where i = 144;
rollback;
--Finally we create delete trigger:

create or replace function tgf_d() returns trigger as $$
declare
  _c int := 0;
begin
  execute format ('delete from %I where %2$I = ($1).%2$I', 'f_'||TG_RELNAME::regclass, pk(TG_RELNAME)) using OLD;
  get diagnostics _c:= row_count;
  if _c < 1 then
    raise exception '%', 'This data is not replicated yet, thus can''t be deleted';
  end if;
  return OLD;
end;
$$ language plpgsql
;



create trigger tgd before delete on t for each row execute procedure tgf_d();

begin;
	delete from t where i = 144;
        select * from t where i = 144;
        select * from f_t where i = 144;
rollback;

begin;
	select * from t where i = 3;
	delete from t where i = 3;
        select * from t where i = 3;
        select * from f_t where i = 3;
rollback;
	

--Next step - we need to watch it working with referential relation.

create table c (i serial, t int references t(i), x text);
--and accordingly a foreign table - the one on newer version...

create table c (i serial, t int references t(i), x text);

create foreign table f_c(i serial, t int, x text) server p10 options (TABLE_name 'c');
--lets pretend it had some data before we decided to migrate with triggers to a higher version
insert into c (t,x) values (1,'FK');
--- so now we add trigggers to replicate DML:

create or replace function tgf_i() returns trigger as $$
begin
  execute format('insert into %I select ($1).*','f_'||TG_RELNAME) using NEW;
  return NEW;
end;
$$ language plpgsql;


create trigger tgi before insert on c for each row execute procedure tgf_i();
create trigger tgu before update on c for each row execute procedure tgf_u();
create trigger tgd before delete on c for each row execute procedure tgf_d();
-- it is surely worth of wrapping those three up to a function with an aim to "triggerise" man tables

--now, what would happen if we tr inserting referenced FK, that does not exist on remote db?..
insert into c (t,x) values (2,'FK');
/* it fails with:
psql:blog.sql:139: ERROR:  insert or update on table "c" violates foreign key constraint "c_t_fkey"
a new row isn't inserted neither on remote, nor local db, so we have safe data consistencyy, but inserts are blocked?..
Yes untill data that existed untill trigerising gets to remote db - ou cant insert FK with before triggerising keys, yet - a new (both t and c tables) data will be accepted:
*/
insert into t(i) values(4); --I use gap we got by deleting data above, so I dont need to "returning" and know the exact id -less coding in sample script
insert into c(t) values(4);
select * from c;
select * from f_c;
--data got in...
--Now you probablyy ask yourself - how this guy is going to "sync" data that existed before "triggerising" without downtime?.. Yeah - I do myself ask this question
--Another funny question is - how do we make sure data is consistent for delete and update too. ATM we "repeat" command for remote db row, but if row with PK not found it silently performs action and exits. Aha! We can easilly fix it. With presupmtion that we can use it only on PK rows, we can check the number of affected rows and raise exception on zero! That's easy - let's do it:
--HERE GIT NEXT COMMIT FUNCTIONS...

/*
start migration for a table:
stop autovacum on table
enable trigger
copy by ctid? xmax?
send from trigger

might need to set all FK deferrable to replace UPDATE with DELETE+INSERT

*/

/*
update triggers can be done three ways:
 1. write new function for each table to name table columns explicitely
 2. write DETELE+INSERT instead UPDATE using NEW.* and OLD.*
 3. format update statement based on pg_attribute

 1 requires much coding, thus possibly various (and different per table) human errors. Also if DDL changes, you have to rewrite the function
 2 requires deferable FK?
 3 is slow, but we try it?
*/

/*
update trigger, that dos not rewrite update to insert+delete should not require updating order on dependent tables as order of updates from "master" is repeated...
*/

/*
statement level trigger for no downtime data migration - statements go to queue, while data is copied and then are aplied against remote names
this way no need to freeze dataa changes to start TBR
*/


