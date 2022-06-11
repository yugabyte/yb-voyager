# yb-voyager

# Sections
- [Introduction](#introduction)
- [Machine Requirements](#machine-requirements)
- [Installation](#installation)
- [Migration Steps](#migration-steps)
    - [Source DB Setup](#source-db-setup)
    - [Target DB Setup](#target-db-setup)
    - [Source DB Export](#source-db-export)
    - [Manual Review (Post-Export)](#manual-review-before-importing-schema-to-yugabytedb-cluster)
    - [Target DB Import](#target-db-import)
    - [Manual Review/Validation (Post-Import)](#import-phase-manual-review)
- [Features and Enhancements To Follow Soon](#features-and-enhancements-in-the-pipeline)

# Introduction

Yugabyte provides an open-source migration engine powered by a command line utility called *yb-voyager*. *yb-voyager* is a simple utility to migrate schema objects and data from different source database types (currently MySQL, Oracle and PostgreSQL) onto YugabyteDB. Support for more database types will be added in near future.

There are two modes of migration (offline and online):
- Offline migration - This is the default mode of migration. In this mode there are two main steps of migration. First, export all the database objects and data in files. Second, run an import phase to transfer those schema objects and data in the destination YugabyteDB cluster. Please note, if the source database continues to receive data after the migration process has started then those cannot be transferred to the destination database.  
- Online migration  - This mode addresses the shortcoming of the 'offline' mode of migration. In this mode, after the initial snapshot migration is done, the migration engine shifts into a CDC mode where it continuously transfers the delta changes from the source to the destination YugabyteDB database.

NOTE: *yb-voyager* currently only supports **offline** migration. Online is under active development.
The rest of the document is relevant for only offline migrations. 

Below are the steps to carry out a migration using *yb-voyager*.

```
                          ┌──────────────────┐
                          │                  │
                          │ Setup yb-voyager │
                          │                  │
                          └────────┬─────────┘
                                   │
                          ┌────────▼─────────┐        
                          │                  │        
                          │   Export Schema  │
                          │                  │        
                          └────────┬─────────┘        
                                   │                  
                          ┌────────▼─────────┐        
                          │     Analysis     │        
                          │                  │        
                          │                  │
                          │ ┌──────────────┐ │
┌───────────────────┐     │ │Analyse Schema│ │
│                   │     │ └───┬──────▲───┘ │
│    Export Data    ◄─────┤     │      │     │
│                   │     │     │      │     │
└────────┬──────────┘     │ ┌───▼──────┴───┐ │
         │                │ │Manual Changes│ │
         │                │ └──────────────┘ │
         │                │                  │
         │                │                  │
         │                └──────────────────┘
         │ 
         │ 
  ┌──────▼───────────┐
  │      Import      │
  │                  │
  │ ┌──────────────┐ │
  │ │Import Schema │ │      ┌─────────────────────┐
  │ └──────┬───────┘ │      │                     │
  │        │         ├──────► Manual Verification │
  │        │         │      │                     │
  │        │         │      └─────────────────────┘
  │ ┌──────▼───────┐ │  
  │ │ Import Data  │ │   
  │ └──────────────┘ │
  │                  │  
  └──────────────────┘
```

Schema objects and data objects are both migrated as per the following compatibility matrix:

|Source Database|Tables|Indexes|Constraints|Views|Procedures|Functions|Partition Tables|Sequences|Triggers|Types|Packages|Synonyms|Tablespaces|
|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
|MySQL/MariaDB|Y|Y|Y|Y|Y|Y|N(https://github.com/yugabyte/yb-db-migration/issues/55)|N/A|Y|N/A|N/A|N/A|N(https://github.com/yugabyte/yb-db-migration/issues/47)|
|PostgreSQL|Y|Y|Y|Y|Y|Y|Y|Y|Y|Y|N/A|N/A|N(https://github.com/yugabyte/yb-db-migration/issues/47)|
|Oracle|Y|Y|Y|Y|Y|Y|Y|Y|Y|Y|Y|Y|N(https://github.com/yugabyte/yb-db-migration/issues/47)|


*Note that the following versions have been tested with yb-voyager:*
- PostgreSQL 9.x - 11.x
- MySQL 8.x
- Oracle 12c - 19c

Utilize the following command for additional details:

```
yb-voyager --help
```

# Machine Requirements
yb-voyager currently supports the following OS versions:
- CentOS7
- Ubuntu 18.04 and 20.04
- MacOS (Only if source is PostgreSQL)

Disk space: It is recommended to have disk space 1.5 times the estimated size of the source DB. A fix to optimize this is being worked on.

Number of cores: Minimum 2 recommended.

# Installation
Refer the [Machine Requirements](#machine-requirements) section for supported OS versions. 

Run the `installer_scripts/install-yb-voyager` script on a machine to prepare it
for running migrations using yb-voyager.

To correctly set environment variables required for the migration process run:

```
source $HOME/.yb-voyager.rc
``` 

Optionally, the installation script sources the `.yb-voyager.rc` file from the `~/.bashrc`. In which case, restarting the bash session will be enough to set the environment variables.

# Migration Steps
Below are the steps that to carry out migrations using the *yb-voyager* utility:

## Source DB Setup
* Oracle: yb-voyager exports complete schema mentioned with `--source-db-schema` flag.
* PostgreSQL: yb-voyager exports complete database(with all schemas inside it) mentioned with `--source-db-name` flag.
* MySQL: yb-voyager exports complete database/schema(schema and database are same in MySQL) mentioned with `--source-db-name` flag.
* For each of the source database types, the database user (corresponding to the `--source-db-user` flag) must have read privileges on all database objects to be exported.

## Target DB Setup
* Create a database in the target YugabyteDB cluster which will be used during the import schema and data phases with the `--target-db-name` flag:
    ```
    CREATE DATABASE dbname;
    ```
* The target database user (corresponding to the `target-db-user` flag) should have superuser privileges; the below mentioned operations (which take place during migration) are only permitted to a superuser:
    * Setting a session variable to disable Triggers and Foreign Key violation checks during the `import data` phase (`import data` command does this internally).
    * Dropping public schema with `--start-clean` flag during the `import schema` phase. 


## Source DB Export

The export phase is carried out in two parts: `export schema` and `export data`.

For additional help use the following command:

```
yb-voyager export --help
```

### Export Schema

```
yb-voyager export schema --help
```

**Sample command:**

```
yb-voyager export schema --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username
```

The schema sql files will be found in `export-dir/schema`. A report regarding the export of schema objects can be found in `export-dir/reports`.

### Export Data

```
yb-voyager export data --help
```

**Sample command:**

```
yb-voyager export data --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username
```

The data sql files will be found in `export-dir/data`.

### SSL Connectivity

*This sub-section is useful if you wish to encrypt and secure your connection to the source database while exporting your schema and data objects using SSL encryption.*

yb-voyager supports SSL Encryption for all source database types, parallel to the configurations accepted by each database type.

yb-voyager uses the following flags to encrypt the connection to the database with SSL encryption:

- source-ssl-mode: Specify the source SSL encryption mode out of - 'disable', 'allow', 'prefer', 'require', 'verify-ca' and 'verify-full'. MySQL does not support the 'allow' sslmode, and Oracle does not use explicit sslmode paramters (Refer to the oracle-tns-alias flag below)
- source-ssl-cert: Provide the source SSL Certificate's Path (For MySQL and PostgreSQL)
- source-ssl-root-cert: Provide the source SSL Root Certificate's Path (For MySQL and PostgreSQL)
- source-ssl-key: Provide the source SSL Key's Path (For MySQL and PostgreSQL)
- source-ssl-crl: Provide the source SSL Certificate Revocation List's Path (For MySQL and PostgreSQL)
- oracle-tns-alias: Name of TNS Alias under which you wish to connect to an Oracle instance. The aliases are expected to be defined in the `$ORACLE_HOME/network/admin/tnsnames.ora` configuration file.

Sample commands for each source database type:

**MySQL:**
```
yb-voyager export --export-dir /path/to/yb/export/dir --source-db-type mysql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username --source-ssl-mode require
```

**Oracle:**
```
yb-voyager export --export-dir /path/to/yb/export/dir --source-db-type oracle --source-db-host localhost --source-db-password password --oracle-tns-alias TNS_Alias --source-db-user username --source-db-schema public
```
Note: This is the only way to export from an Oracle instance using SSL Encryption.

**PostgreSQL:**
```
yb-voyager export --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username --source-ssl-mode verify-ca --source-ssl-root-cert /path/to/root_cert.pem --source-ssl-cert /path/to/cert.pem --source-ssl-key /path/to/key.pem
```

For additional details regarding the flags used to connect to an instance using SSL connectivity, refer to the help messages:
```
yb-voyager export --help
```

## Manual Review Before Importing Schema to YugabyteDB cluster
This is a very important step in the migration process. This gives a chance to the user doing the migration to review all the schema objects which are about to be created on the YugabyteDB cluster. The export schema step dumps all the schema object definitions from the source databases. As a part of this, it also dumps those object types which are currently not supported in YugabyteDB.

The user can also choose to omit certain schema object creations which may not be useful in YugabyteDB. For example, certain indexes, constraints etc. may be removed in a distributed cluster to improve performance.

The `analyze-schema` command calls out all those incompatibilities and gives relevant GitHub issues which track these feature gaps. Note that the migration engine does not automatically make any schema corrections--it only reports them.

**Sample invocation of the analyze-schema command:**

```
yb-voyager analyze-schema --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname  --source-db-user username --output-format txt
```

The `analyze-schema` command outputs the analysis report at `export-dir/reports/report.txt`.

Go through all the issues reported in the `report.txt` and make the suggested changes in the appropriate schema files located in the `export-dir/schema/*/`. After changing any schema file, you can re-run the `analyze-schema` to get the list of remaining issues. Once all the issues from the `report.txt` are addressed, the `analyze-schema` should not report any issue. Then you're ready to proceed to the `import` phase.

### Some example scenarios for manual review

- CREATE INDEX CONCURRENTLY NOT SUPPORTED: This feature is not supported yet in YugabyteDB. It is advisable that user manually edits the DDL statement and removes the clause "CONCURRENTLY" from the definition.
- Primary Key cannot be added to Partitioned table using ALTER TABLE: It is advisable that the user adds the primary key clause in the `create table` DDL.


## Target DB Import

This command/series of commands(see below) is/are used to initiate the import of schema and data objects onto YugabyteDB. It is mandatory that the user has completed the export phase at a minimum (see above), and it is recommended that the user completes a manual review of the exported schema and data files, which will be found in the `export-dir/schema` and `export-dir/data` folders respectively. A report will be generated in the `export-dir/reports` folder to help speed up this verification process.

For additional help use the following command:

```
yb-voyager import --help
```

### Import Schema

```
yb-voyager import schema --help
```

**Sample command:**

```
yb-voyager import schema --export-dir /path/to/yb/export/dir --target-db-host localhost --target-db-password password --target-db-name dbname --target-db-schema public --target-db-user username --parallel-jobs 10 --batch-size 100000
```

The schema sql files should be located in the `export-dir/schema` folder.

### Import Data

```
yb-voyager import data --help
```

**Sample command:**

```
yb-voyager import data --export-dir /path/to/yb/export/dir --target-db-host localhost --target-db-password password --target-db-name dbname --target-db-schema public --target-db-user username --parallel-jobs 100 --batch-size 250000
```

The data sql files should be located in the `export-dir/data` folder.

### SSL Connectivity

*This sub-section is useful if you wish to encrypt and secure your connection to the target YugabyteDB instance while importing your schema and data objects using SSL encryption.*

yb-voyager allows you to configure your connection to a YugabyteDB instance with SSL encryption.

yb-voyager uses the following flags to encrypt the connection with a YugabyteDB instance with SSL encryption:

- target-ssl-mode: Specify the SSL encryption mode out of - 'disable', 'allow', 'prefer', 'require', 'verify-ca' and 'verify-full'. 
- target-ssl-cert: Provide the SSL Certificate's Path
- target-ssl-root-cert: Provide the SSL Root Certificate's Path
- target-ssl-key: Provide the SSL Key's Path
- target-ssl-crl: Provide the SSL Certificate Revocation List's Path

**Sample command:**

```
yb-voyager import --export-dir /path/to/yb/export/dir --target-db-host localhost --target-db-password password --target-db-name dbname --target-db-schema public --target-db-user username --parallel-jobs 100 --batch-size 250000 --target-ssl-mode require
```

# Features and enhancements in the pipeline

Some of the important features and enhancements to follow soon are:

- [Support for ONLINE migration from Oracle/PostgreSQL/MySQL](https://github.com/yugabyte/yb-db-migration/issues/50)
- [Support migration to YugabyteDB cluster created on Yugabyte Cloud](https://github.com/yugabyte/yb-db-migration/issues/52)
- [Reduce disk space requirements during migration process](https://github.com/yugabyte/yb-db-migration/issues/45)

You can look at all the open issues [here](https://github.com/yugabyte/yb-db-migration/issues)
