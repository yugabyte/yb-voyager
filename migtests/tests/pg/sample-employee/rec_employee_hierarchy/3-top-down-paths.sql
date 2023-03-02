drop view if exists top_down_paths cascade; 

create view top_down_paths(path) as          
with
  recursive paths(path) as (
    select array[name]              -- array constructor
    from emps
    where mgr_name is null
    union all
    select p.path||e.name           -- appending a new element to the existing array
    from
      emps as e
      inner join paths as p         -- "path[cardinality(path)]" gets the last element
      on e.mgr_name = p.path[cardinality(path)] 
  )
select path from paths;

-- Breadth first traversal
select cardinality(path) as depth, path
from top_down_paths
order by
  depth,
  path[2] asc nulls first,
  path[3] asc nulls first,
  path[4] asc nulls first,
  path[5] asc nulls first;

-- Depth first traversal
select cardinality(path) as depth, path
from top_down_paths
order by
  path[2] asc nulls first,
  path[3] asc nulls first,
  path[4] asc nulls first,
  path[5] asc nulls first;

-- Conventional use of indentation to improve readability
-- of the depth first traversal display.
select                             
  rpad(' ', 2*cardinality(path) - 2, ' ')||path[cardinality(path)] as "emps hierarchy"
from top_down_paths
order by
  path[2] asc nulls first,
  path[3] asc nulls first,
  path[4] asc nulls first,
  path[5] asc nulls first;

-- Approximation to Unix "tree" output
-- by using └─ and ├─ and ─
with                           
  a1 as (
    select
      cardinality(path) as depth,
      path
    from top_down_paths),
  a2 as (
    select
      depth,
      lead(depth, 1, 0) over w as next_depth,
      path
    from a1
    window w as (
      order by
        path[2] asc nulls first,
        path[3] asc nulls first,
        path[4] asc nulls first,
        path[5] asc nulls first))
select
  case depth = next_depth
    when true then
      lpad(' ├── ', (depth - 1)*5, ' ')
    else
      lpad(' └── ', (depth - 1)*5, ' ')
  end
  ||
  path[depth] as "Approx. 'Unix tree'"
from a2
order by
  path[2] asc nulls first,
  path[3] asc nulls first,
  path[4] asc nulls first,
  path[5] asc nulls first;
