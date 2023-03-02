drop view if exists top_down_simple cascade;

create view top_down_simple(depth, mgr_name, name) as
with
  recursive hierarchy_of_emps(depth, mgr_name, name) as (
    -- Non-recursive term.
    -- Select the exactly one ultimate manager.
    -- Define this emp to be at depth 1.
    (
      select
        1,
        '-',
        name
      from emps             
      where mgr_name is null
    )
    union all
    -- Recursive term.
    -- Treat the emps from the previous iteration as managers.
    -- Join these with their reports, if they have any.
    -- Increase the emergent depth by 1 with each step.
    -- Stop when none of the current putative managers has a report.
    -- Each successive iteration goes one level deeper in the hierarchy.
    (
      select
        h.depth + 1,
        e.mgr_name,
        e.name
      from
      emps as e
      inner join
      hierarchy_of_emps as h on e.mgr_name = h.name 
    )
  )
select
  depth,
  mgr_name,
  name
from hierarchy_of_emps;

select                                               
  depth,
  mgr_name,                                              
  name                    
from top_down_simple
order by                                
  depth,
  mgr_name nulls first,
  name;
