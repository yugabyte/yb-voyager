drop function if exists pure_sql_version(int) cascade;

create function pure_sql_version(max_c1 in int)
  returns table(c1 int, c2 int)
  language sql
as $body$
  with
    recursive r(c1, c2) as (

      -- Non-recursive term.
      (
        values (0, 1), (0, 2), (0, 3)
      )

      union all

      -- Recursive term.
      (
        select c1 + 1, c2 + 10
        from r
        where c1 < max_c1
      )
    )
  select c1, c2 from r order by c1, c2;
$body$;

select c1, c2 from pure_sql_version(4) order by c1, c2;

drop function if exists fibonacci_series(int) cascade;

create function fibonacci_series(max_x in int)
  returns table(x int, f int)
  language sql
as $body$
  with
    recursive r(x, prev_f, f) as (
      values (1::int, 0::int, 1::int)

      union all

      select
        r.x + 1::int,
        r.f,
        r.f + r.prev_f
      from r
      where r.x < max_x
    )
  values(0, 0)
  union all
  select x, f from r;
$body$;

-- 0, 1, 1, 2, 3, 5, 8, 13, 21, 34, 55, 89, 144
select x, f as "fib(x)"
from fibonacci_series(12)
order by x;
