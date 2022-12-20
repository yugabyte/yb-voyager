-- DEMONSTRATE GOOD DATA MODELING PRACTICE

drop table if exists emps cascade;
drop domain if exists name_t cascade;
drop function if exists name_ok(i in text) cascade;

create function name_ok(i in text)
  returns boolean
  language sql
as $body$
  select
    case
      when
          (lower(i) = i and length(i) <=30) or i is null
        then true
      else
             false
    end;
$body$;

create domain name_t as text check(name_ok(value));

create table emps(
  name     name_t primary key,
  mgr_name name_t);

-- The order of insertion is arbitrary
insert into emps(name, mgr_name) values
  ('mary',   null  ),
  ('fred',   'mary'),
  ('susan',  'mary'),
  ('john',   'mary'),
  ('doris',  'fred'),
  ('alice',  'john'),
  ('bill',   'john'),
  ('joan',   'bill'),
  ('george', 'mary'),
  ('edgar',  'john'),
  ('alfie',  'fred'),
  ('dick',   'fred');

-- The ultimate manager has no manager.
-- Enforce the business rule "Maximum one ultimate manager".
-- Expression-based index.
create unique index t_mgr_name on emps((mgr_name is null)) where mgr_name is null;

-- Implement the one-to-many "pig's ear".
alter table emps
add constraint emps_mgr_name_fk
foreign key(mgr_name) references emps(name)
on delete restrict;

-- Order by... nulls first
select name, mgr_name
from emps
order by mgr_name nulls first, name;

-- Try these by hand to see the errors.
-- insert into emps(name, mgr_name) values ('second boss', null);
-- insert into emps(name, mgr_name) values ('emily', 'steve');
-- insert into emps(name, mgr_name) values ('Emily', 'dick');
