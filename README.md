# yb_migrate

# Sections
- [Introduction](#introduction)
- [Machine Requirements](#machine-requirements)
- [Installation](#installation)
- [Migration Steps](#migration-steps)
    - [Source DB Setup](#source-db-setup)
    - [Target DB Setup](#target-db-setup)
    - [Report Generation](#report-generation)
    - [Source DB Export](#source-db-export)
    - [Manual Review (Export)](#export-phase-manual-review)
    - [Target DB Import](#target-db-import)
    - [Manual Review/Validation (Import Phase)](#import-phase-manual-review)
- [Limitations](#limitations)

# Introduction

Yugabyte provides an open-source migration engine powered by a command line utility called yb_migrate. yb_migrate is a simple utility to migrate databases from different source types (MySQL, Oracle and PostgreSQL) onto YugabyteDB.

Github Issue Link *TODO:link this*

*TODO:Improve the wording used for migration types*
There are two modes of migration (offline and online):
- Offline migration is disruptive to users and applications, and will impact the performance of the database instance.
- Online migration is non-disruptive to users and applications, and will have little to no effect on the performance of the database instance.

yb_migrate currently only supports offline migration, owing to which, the documentation covers only the offline aspect of migration.

The migration service requires a certain list of commands to be run in a certain sequence:
```
                          ┌──────────────────┐
                          │                  │
                          │ Setup yb_migrate ├────────┐
                          │                  │        │
                          └────────┬─────────┘        │
                                   │                  │
                          ┌────────▼─────────┐        │
                          │                  │        │
                          │ Generate Report  │        │
                          │                  │        │
                          └────────┬─────────┘        │
                                   ├◄─────────────────┘
                          ┌────────▼─────────┐        
                          │      Export      │        
                          │                  │        
                          │                  │        
                          │ ┌──────────────┐ │        
┌───────────────────┐     │ │Export Schema │ │        
│                   │     │ └──────┬───────┘ │        
│ Manual Validation ◄─────┤        │         │        
│                   │     │        │         │
└────────┬──────────┘     │  ┌─────▼──────┐  │
         │                │  │Export Data │  │
         │                │  └────────────┘  │
         │                │                  │
         │                │                  │
         │                └────────┬─────────┘
         │                         │                                                  
         │                         │
         │                ┌────────▼─────────┐
         │                │      Import      │
         │                │ ┌──────────────┐ │
         │                │ │Import Schema │ │      ┌─────────────────────┐
         │                │ └──────┬───────┘ │      │                     │
         │                │        │         ├──────► Manual Verification │
         └────────────────►  ┌─────▼──────┐  │      │                     │
                          │  │Import Data │  │      └─────────────────────┘
                          │  └────────────┘  │
                          │                  │
                          └──────────────────┘
```

Schema objects and data objects are both migrated as per the following compatibility matrix:
*TODO:Some data objects have conditions/limitations (discussed with Sanyam), should be included in limitations section*

|Source Database|Tables|Indexes|Constraints|Views|Procedures|Functions|Partition Tables|Sequences|Triggers|Types|
|-|-|-|-|-|-|-|-|-|-|-|
|MySQL/MariaDB|Y|Y|N/A|N/A|N/A|N/A|Y|N/A|N/A|N/A|
|PostgreSQL|Y|Y|N/A|N/A|N/A|Y|N/A|Y|Y|N/A|
|Oracle|Y|Y|Y|Y|Y|Y|Y|Y|Y|Y|

*TODO: Update Version numbers, Rahul has entered some placeholder values for now.*

*Note that the following versions have been tested with yb_migrate:*
- MariaDB 10.5.x
- PostgreSQL 9.x - 13.x
- MySQL 8.x
- Oracle 12.1.x - 19.3.x

Utilize the following command for additional details:

```
yb_migrate --help
```

# Machine Requirements
yb_migrate currently supports the following OS versions:
- CentOS7
- Ubuntu 18.04 and 20.04

Disk space: It is recommended to have disk space 1.5 times the estimated size of the source DB. A fix to optimize this is being worked on.

Number of cores: Minimum 2 recommended.

# Installation
We provide interactive installation scripts that the user should run on their machines. Refer to the [Machine Requirements](#machine-requirements) section for supported OS versions.
- [CentOS7](installer_scripts/install_centos7.sh)
- [Ubuntu](installer_scripts/install_ubuntu.sh)

*TODO: Review the following section regarding its correctness.*

Post-installation, the user should run

```
source $HOME/.migration_installer_bashrc
``` 

to add the dependencies required for initiating the migration process. If the user has opted for concatenating the script to `$HOME/.bashrc`, they should instead run 

```
source $HOME/.bashrc
```

or restart their terminal instance before proceeding with the rest of the migration process.

# Migration Steps
Below are the steps a user should follow to use the yb_migrate tool:

## Source DB Setup
* Oracle: yb_migrate exports complete schema mentioned with `--source-db-schema` flag.
* PostgreSQL: yb_migrate exports complete database(with all schemas inside it) mentioned with `--source-db-database` flag.
* MySQL: yb_migrate exports complete database/schema(schema and database are same in MySQL) mentioned with `--source-db-database` flag.
* For each of the source database type, the database user(used in migration) must have read privileges on all database objects to be exported.

## Target DB Setup
* Create a database in target YugabyteDB cluster required during import schema and data with `--target-db-name` flag:
    ```
    CREATE DATABASE dbname;
    ```
* The target database user(used in migration) should have a superuser privileges because below operations during migration are only allowed to superuser:
    * Setting session variable to disable Triggers and Foreign Key voilation checks during import data(import data command do this internally).
    * Dropping public schema with `--start-clean` flag during import schema. 

## Report Generation
	
Before beginning the migration cycle, the user can generate a report, which provides the details of the schema objects to be exported, along with incompatibilities, if any. The incompatibilities will be tagged and a Github issue link will be provided with it if available. If there are no solutions available, the user will have to manually review the export phase (see below).

For additional help use the following command:

```
yb_migrate generateReport --help
```

**Sample command:**

```
yb_migrate generateReport --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname  --source-db-user username --output-format html
```

The generated report will be found in `export-dir/reports/report.html`.

## Source DB Export

The export phase is carried out in two parts, export schema and export data respectively. It is recommended to start this phase after having completed the report generation phase (see above). 

For additional help use the following command:

```
yb_migrate export --help
```

### Export Schema

```
yb_migrate export schema --help
```

**Sample command:**

```
yb_migrate export schema --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username
```

The schema sql files will be found in `export-dir/schema`. A report regarding the export of schema objects can be found in `export-dir/reports`.

### Export Data

```
yb_migrate export data --help
```

**Sample command:**

```
yb_migrate export data --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username
```

The data sql files will be found in `export-dir/data`.

### SSL Connectivity

*This sub-section is useful if you wish to encrypt and secure your connection to the source database while exporting your schema and data objects using SSL encryption.*

yb_migrate supports SSL Encryption for all source database types, parallel to the configurations accepted by each database type.

yb_migrate uses the following flags to encrypt the connection to the database with SSL encryption:

- source-ssl-mode: Specify the source SSL encryption mode out of - 'disable', 'allow', 'prefer', 'require', 'verify-ca' and 'verify-full'. MySQL does not support the 'allow' sslmode, and Oracle does not use explicit sslmode paramters (Refer to the oracle-tns-alias flag below)
- source-ssl-cert: Provide the source SSL Certificate's Path (For MySQL and PostgreSQL)
- source-ssl-root-cert: Provide the source SSL Root Certificate's Path (For MySQL and PostgreSQL)
- source-ssl-key: Provide the source SSL Key's Path (For MySQL and PostgreSQL)
- source-ssl-crl: Provide the source SSL Certificate Revocation List's Path (For MySQL and PostgreSQL)
- oracle-tns-alias: Name of TNS Alias under which you wish to connect to an Oracle instance. The aliases are expected to be defined in the `$ORACLE_HOME/network/admin/tnsnames.ora` configuration file.

Sample commands for each source database type:

**MySQL:**
```
yb_migrate export --export-dir /path/to/yb/export/dir --source-db-type mysql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username --source-ssl-mode require
```

**Oracle:**
```
yb_migrate export --export-dir /path/to/yb/export/dir --source-db-type oracle --source-db-host localhost --source-db-password password --oracle-tns-alias TNS_Alias --source-db-user username --source-db-schema public
```
Note: This is the only way to export from an Oracle instance using SSL Encryption.

**PostgreSQL:**
```
yb_migrate export --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username --source-ssl-mode verify-ca --source-ssl-root-cert /path/to/root_cert.pem --source-ssl-cert /path/to/cert.pem --source-ssl-key /path/to/key.pem
```

For additional details regarding the flags used to connect to an instance using SSL connectivity, refer to the help messages:
```
yb_migrate export --help
```

## Export Phase Manual Review
*TODO:*
- *Why?*
- *What can the users do here with some examples*

## Target DB Import

This command/series of commands(see below) is/are used to initiate the import of schema and data objects onto YugabyteDB. It is mandatory that the user has completed the export phase at a minimum (see above), and it is recommended that the user completes a manual review of the exported schema and data files, which will be found in the `export-dir/schema` and `export-dir/data` folders respectively. A report will be generated in the `export-dir/reports` folder to help speed up this verification process.

For additional help use the following command:

```
yb_migrate import --help
```

### Import Schema

```
yb_migrate import schema --help
```

**Sample command:**

```
yb_migrate import schema --export-dir /path/to/yb/export/dir --target-db-host localhost --target-db-password password --target-db-name dbname --target-db-schema public --target-db-user username --parallel-jobs 10 --batch-size 100000
```

The schema sql files should be located in the `export-dir/schema` folder.

### Import Data

```
yb_migrate import data --help
```

**Sample command:**

```
yb_migrate import data --export-dir /path/to/yb/export/dir --target-db-host localhost --target-db-password password --target-db-name dbname --target-db-schema public --target-db-user username --parallel-jobs 100 --batch-size 250000
```

The data sql files should be located in the `export-dir/data` folder.

### Import Data Status
*TODO:*
- *What does this command do?*
- *How do we use it?*

### SSL Connectivity

*This sub-section is useful if you wish to encrypt and secure your connection to the target YugabyteDB instance while importing your schema and data objects using SSL encryption.*

yb_migrate allows you to configure your connection to a YugabyteDB instance with SSL encryption.

yb_migrate uses the following flags to encrypt the connection with a YugabyteDB instance with SSL encryption:

- target-ssl-mode: Specify the SSL encryption mode out of - 'disable', 'allow', 'prefer', 'require', 'verify-ca' and 'verify-full'. 
- target-ssl-cert: Provide the SSL Certificate's Path
- target-ssl-root-cert: Provide the SSL Root Certificate's Path
- target-ssl-key: Provide the SSL Key's Path
- target-ssl-crl: Provide the SSL Certificate Revocation List's Path

**Sample command:**

```
yb_migrate import --export-dir /path/to/yb/export/dir --target-db-host localhost --target-db-password password --target-db-name dbname --target-db-schema public --target-db-user username --parallel-jobs 100 --batch-size 250000 --target-ssl-mode require
```

## Import Phase Manual Review
*TODO: Fill this section*

# Limitations
