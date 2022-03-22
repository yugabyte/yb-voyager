# ybm
YugabyteDB's migration tool

[Temporary README for fellow Developers]

Prerequites:
1. Golang installed and GOPATH set in your env
2. Tools like ora2pg, pg_dump, psql, ysql are installed and PATH variable is set for them

To generate a new cobra command files
> cobra add newCommandName

This will generate a file under cmd/newCommandName.go with some basic code stubs

## Sample Commands to try out yb_migrate tool

Steps before executing sample commands:
```
1. cd ybm/yb_migrate
2. go install
```
Now, you `yb_migrate` is installed in your 

***
### Parent Export Command
***
This command will do both - Export Schema and Export Data
#### Oracle
```
yb_migrate export --source-db-type oracle \
--source-db-host hostname \
--source-db-port 1521 \
--export-dir /path/to/create/migration/project/ \
--source-db-user sakila --source-db-password **** \
--source-db-schema sakila --source-db-name DMS
```

#### Postgres
```
 yb_migrate export --source-db-type postgres \
 --source-db-host 127.0.0.1 --source-db-port 5432 \
--export-dir /path/to/create/migration/project/ \
 --source-db-user postgres --source-db-password **** \
 --source-db-name sakila
```