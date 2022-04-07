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

# Introduction

Yugabyte provides an open-source migration engine powered by a command line utility called yb_migrate. yb_migrate is a simple utility to migrate databases from different source types (MySQL, Oracle, PostgreSQL) onto YugabyteDB.

Github Issue Link -yet to be linked-

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
                                   │                  │
                          ┌────────▼─────────┐        │
                          │      Export      │        │
                          │                  │        │
                          │                  │        │
                          │ ┌──────────────┐ │        │
┌───────────────────┐     │ │Export Schema │ │        │
│                   │     │ └──────┬───────┘ │        │
│ Manual Validation ◄─────┤        │         │        │
│                   │     │        │         ◄────────┘
└────────┬──────────┘     │  ┌─────▼──────┐  │
         │                │  │Export Data │  │
         │                │  └────────────┘  │
         │                │                  │
         │                │                  │
         │                └────────┬─────────┘
         │                         │                                                  ▼
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

-will insert matrix later-


Utilize the following command for additional details:

```
yb_migrate --help
```

# Machine Requirements
yb_migrate currently supports the following OS versions:
- CentOS7
- Ubuntu -exact versions are to be mentioned soon-

Disk space: It is recommended to have disk space twice the estimated size of the source DB. We are currently working on a [fix](https://yugabyte.atlassian.net/browse/DMS-18) to optimize this.

Number of cores: Minimum 2 recommended.

# Installation
We provide interactive installation scripts that the user should run on their machines. Refer to the [Machine Requirements](#machine-requirements) section for supported OS versions.
- [CentOS7](installer_scripts/install_centos7.sh)
- [Ubuntu](installer_scripts/install_ubuntu.sh)

*Note to the DMS team: I believe we had a discussion talking about cat-ing the required shell scripts into a specific bash_rc-like file meant for yb_migrate, so that the user may use* `source yb_migrate.sh`*. Please confirm this detail to complete this section.*

# Migration Steps
Below are the steps a user should follow to use the yb_migrate tool:

## Source DB Setup
*TODO (Sanyam): Mention the various permissions required by the user for each source DB type in a few lines each*

## Target DB Setup
*TODO (Sanyam): Mention the permissions required by the user for YugabyteDB in a few lines*

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

## Export Phase Manual Review
*To be included:*
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
*To be included:*
- *What does this command do?*
- *How do we use it?*

## Import Phase Manual Review
