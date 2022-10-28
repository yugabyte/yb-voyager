set heading off
set pagesize 0
set feedback off

-- wipe out all scheduler jobs
select 'exec dbms_scheduler.drop_job(job_name => '''||j.job_creator||'.'||j.job_name||''');'
from user_scheduler_jobs j
/

-- wipe out all XML schemas
select 'exec dbms_xmlschema.deleteSchema(schemaURL => '''||s.qual_schema_url||''',delete_option => dbms_xmlschema.DELETE_CASCADE_FORCE);'
from user_xml_schemas s
/

-- wipe out all remaining objects
select 'drop '
       ||o.object_type
       ||' '||object_name
       ||case o.object_type when 'TABLE' then ' cascade constraints' when 'TYPE' then ' force' else '' end
       ||';'
from user_objects o
where o.object_type not in ('JOB','LOB','PACKAGE BODY','INDEX','TRIGGER')
and not exists (select 1
                from user_objects r
                where r.object_name = o.object_name 
                and   r.object_type = 'MATERIALIZED VIEW'
                and   o.object_type != 'MATERIALIZED VIEW'
               )
/

-- empty the recycle bin
select 'purge recyclebin;' from dual
/