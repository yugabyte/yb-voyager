/*
ERROR: EXCLUDE constraint not supported yet (SQLSTATE 0A000)
File :/home/centos/yb-voyager/migtests/tests/pg/sample-is/export-dir/schema/tables/table.sql
*/
ALTER TABLE ONLY public.secret_missions ADD CONSTRAINT cnt_solo_agent EXCLUDE USING gist (location WITH =, mission_timeline WITH &&);

