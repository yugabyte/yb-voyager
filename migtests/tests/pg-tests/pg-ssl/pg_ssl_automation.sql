-- Minimal schema/data testing, we mainly need to validate SSL connectivity on this test case.

drop table if exists ssl_table1;

create table ssl_table1 (
    id int,
    primary key(id)
);

insert into ssl_table1 values(1);