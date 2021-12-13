# ybm
YugabyteDB's migration tool


***
### Parent Export Command
***

### Oracle
1. Example Command:
    ```
    yb_migrate export --source-db-type oracle --source-db-host ssinghal-dms-oracle.cbtcvpszcgdq.us-west-2.rds.amazonaws.com --source-db-port 1521 --export-dir /home/centos/yb_migrate_projects/ --source-db-user sakila --source-db-password password --source-db-schema sakila --source-db-name DMS
    ```

### Postgres
1. Example Command:
    ```
    yb_migrate export --source-db-type postgres --source-db-host 127.0.0.1 --source-db-port 5432 --export-dir /home/centos/yb_migrate_projects --source-db-user postgres --source-db-password postgres --source-db-name sakila
    ```

### Mysql
1. Example Command:
    ```
    yb_migrate export schema \
    --source-db-type mysql \
    --source-db-host ssinghal-dms-mysql.cbtcvpszcgdq.us-west-2.rds.amazonaws.com \
    --source-db-port 3306 --source-db-user admin \
    --source-db-password password --source-db-name sakila
    ```

***
## Export Schema
***

### Oracle
1. Example Command:
    ``` 
    yb_migrate export schema \
    --source-db-type oracle \
    --source-db-host ssinghal-dms-oracle.cbtcvpszcgdq.us-west-2.rds.amazonaws.com \
    --source-db-port 1521 --export-dir /home/centos/yb_migrate_projects \
    --source-db-user sakila --source-db-password password \
    --source-db-schema sakila --source-db-name DMS
    ```

### Postgres
1. Example Command:
    ```
    yb_migrate export schema \
    --source-db-type postgres \
    --export-dir /home/centos/yb_migrate_projects \
    --source-db-user postgres --source-db-password postgres \
    --source-db-name sakila
    ```

<br/>

### MySQL
1. Example Command:
    ```
    yb_migrate export schema \
    --source-db-type mysql \
    --source-db-host ssinghal-dms-mysql.cbtcvpszcgdq.us-west-2.rds.amazonaws.com \
    --source-db-port 3306 --source-db-user admin \
    --source-db-password password --source-db-name sakila
    ```


***
## Export Data
***

### Oracle
1. Example Command:
    ```
    yb_migrate export data \
    --source-db-type oracle \
    --source-db-host ssinghal-dms-oracle.cbtcvpszcgdq.us-west-2.rds.amazonaws.com \
    --source-db-port 1521 --export-dir /home/centos/yb_migrate_projects \
    --source-db-user sakila --source-db-password password \
    --source-db-schema sakila --source-db-name DMS
    ```

### Postgres
1. Example Command:
    ```
    yb_migrate export data \
    --source-db-type postgres --source-db-host 127.0.0.1 \
    --source-db-port 5432 --export-dir /home/centos/yb_migrate_projects \
    --source-db-user postgres --source-db-password postgres \
    --source-db-name sakila
    ```



***
## Import Schema
***

### YugabyteDB
1. Example Command:
    ```
  	 yb_migrate import schema --target-db-host localhost --target-db-port 5433 --target-db-user yugabyte --target-db-password yugabyte --target-db-name sakila --export-dir /home/centos/yb_migrate_projects/postgres-sakila-migration/
    ```

<!-- ***
## Import Data
***
 -->
