-- https://www.postgresql.org/docs/current/sql-createaggregate.html
-- https://github.com/postgres/postgres/blob/master/src/test/regress/sql/aggregates.sql
CREATE SCHEMA agg_ex;
create type agg_ex.avg_state as (total bigint, count bigint);

create or replace function agg_ex.avg_transfn(state agg_ex.avg_state, n int) returns agg_ex.avg_state as
$$
declare new_state agg_ex.avg_state;
begin
	raise notice 'agg_ex.avg_transfn called with %', n;
	if state is null then
		if n is not null then
			new_state.total := n;
			new_state.count := 1;
			return new_state;
		end if;
		return null;
	elsif n is not null then
		state.total := state.total + n;
		state.count := state.count + 1;
		return state;
	end if;

	return null;
end
$$ language plpgsql;

create function agg_ex.avg_finalfn(state agg_ex.avg_state) returns int4 as
$$
begin
	if state is null then
		return NULL;
	else
		return state.total / state.count;
	end if;
end
$$ language plpgsql;

create function agg_ex.sum_finalfn(state agg_ex.avg_state) returns int4 as
$$
begin
	if state is null then
		return NULL;
	else
		return state.total;
	end if;
end
$$ language plpgsql;

create aggregate agg_ex.my_avg(int4)
(
   stype = agg_ex.avg_state,
   sfunc = agg_ex.avg_transfn,
   finalfunc = agg_ex.avg_finalfn
);

create aggregate agg_ex.my_sum(int4)
(
   stype = agg_ex.avg_state,
   sfunc = agg_ex.avg_transfn,
   finalfunc = agg_ex.sum_finalfn
);

CREATE TABLE agg_ex.my_table(i int4);
CREATE VIEW agg_ex.my_view AS
  SELECT agg_ex.my_sum(i), agg_ex.my_avg(i)
  FROM agg_ex.my_table;
