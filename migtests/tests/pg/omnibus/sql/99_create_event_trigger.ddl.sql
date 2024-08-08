-- https://www.postgresql.org/docs/current/sql-createeventtrigger.html

CREATE OR REPLACE FUNCTION log_table_alteration()
  RETURNS event_trigger
 LANGUAGE plpgsql
  AS $$
BEGIN
  RAISE NOTICE 'command % issued', tg_tag;
END;
$$;

CREATE EVENT TRIGGER log_table_alteration ON table_rewrite
   EXECUTE FUNCTION log_table_alteration();
