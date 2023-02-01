--view ddl having WITH CHECK OPTION
CREATE VIEW v1 AS SELECT * FROM t1 WHERE a < 2
WITH CHECK OPTION;

--dropping multiple objects
DROP VIEW IF EXISTS view1,view2,view3;

--alter view
ALTER VIEW view_name TO select * from test;