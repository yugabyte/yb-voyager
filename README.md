# YugabyteDB Voyager
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Documentation Status](https://readthedocs.org/projects/ansicolortags/badge/?version=latest)](https://docs.yugabyte.com/)
[![Ask in forum](https://img.shields.io/badge/ask%20us-forum-orange.svg)](https://forum.yugabyte.com/)
[![Slack chat](https://img.shields.io/badge/Slack:-%23yugabyte_db-blueviolet.svg?logo=slack)](https://communityinviter.com/apps/yugabyte-db/register)
[![Analytics](https://yugabyte.appspot.com/UA-104956980-4/home?pixel&useReferer)](https://github.com/yugabyte/ga-beacon)

YugabyteDB Voyager is a powerful open-source data migration engine that accelerates cloud native adoption by removing barriers to moving applications to the public or private cloud. It helps you migrate databases to YugabyteDB quickly and securely.

YugabyteDB Voyager manages the entire lifecycle of a database migration, including cluster preparation for data import, schema-migration, and data-migration, using the yb-voyager command line utility.


- [Introduction](#introduction)
- [Salient Features](#salient-features)
- [Machine Requirements](#machine-requirements)
- [Installation](#installation)
- [Migration Steps](#migration-steps)
- [License](#license)

- [Features and Enhancements To Follow Soon](#features-and-enhancements-in-the-pipeline)

# Introduction

Yugabyte provides an open-source migration engine powered by a command line utility called *yb-voyager*. *yb-voyager* is a simple utility to migrate schema objects and data from different source database types (currently MySQL, Oracle and PostgreSQL) onto YugabyteDB. Support for more database types will be added in near future.

There are two modes of migration (offline and online):
- Offline migration - This is the default mode of migration. In this mode there are two main steps of migration. First, export all the database objects and data in files. Second, run an import phase to transfer those schema objects and data in the destination YugabyteDB cluster. Please note, if the source database continues to receive data after the migration process has started then those cannot be transferred to the destination database.  
- Online migration  - This mode addresses the shortcoming of the 'offline' mode of migration. In this mode, after the initial snapshot migration is done, the migration engine shifts into a CDC mode where it continuously transfers the delta changes from the source to the destination YugabyteDB database.

NOTE: *yb-voyager* currently only supports **offline** migration. Online is under active development.
The rest of the document is relevant for only offline migrations. 

# Salient Features
- Free and completely open source.
- Supports widely used databases for migration and doesn't require changes to the source databases in most cases.
- Supports all YugabyteDB products (YugabyteDB stable versions 2.14.5.0 and later, preview versions 2.17.0.0 and later) as the target database.
- Provides a unified CLI experience for all different source databases.
- Auto-tuneable based on workloads, by analyzing the target cluster capacity; runs parallel jobs by default.
- Monitor the import status, and expected time for data export and import to complete using progress bars.
- In case of failures, data import can be resumed.
- Parallelism of data across tables.
- Supports direct data import from CSV and text files.

## Compatibility Matrix
|Source Database|Tables|Indexes|Constraints|Views|Procedures|Functions|Partition Tables|Sequences|Triggers|Types|Packages|Synonyms|Tablespaces|
|-|-|-|-|-|-|-|-|-|-|-|-|-|-|
|MySQL/MariaDB|Y|Y|Y|Y|Y|Y|Y|N/A|Y|N/A|N/A|N/A|N(https://github.com/yugabyte/yb-db-migration/issues/170)|
|PostgreSQL|Y|Y|Y|Y|Y|Y|Y|Y|Y|Y|N/A|N/A|N(https://github.com/yugabyte/yb-db-migration/issues/170)|
|Oracle|Y|Y|Y|Y|Y|Y|Y|Y|Y|Y|Y|Y|N(https://github.com/yugabyte/yb-db-migration/issues/170)|

# Machine Requirements
yb-voyager currently supports the following OS versions:
- CentOS7
- CentOS8
- RHEL 7
- RHEL 8
- AlmaLinux
- Ubuntu 18.04 and 20.04
- MacOS (Only if source is PostgreSQL)

Disk space: It is recommended to have disk space 1.2 to 1.3 times the estimated size of the source DB.

Number of cores: Minimum 2 recommended.

## Supported Source Databases
*Note that the following versions have been tested with yb-voyager:*
- PostgreSQL 9.x - 11.x (On-prem, Amazon Aurora, Amazon RDS, CloudSQL, Azure)
- MySQL 8.x (On-prem, MariaDB, Amazon Aurora, Amazon RDS, CloudSQL)
- Oracle 11g - 19c (On-prem, Amazon RDS)

## Supported Target Database Versions
You can migrate data to any one of the three YugabyteDB products (Stable versions 2.14.5.0 and later, and preview versions 2.17.0.0 and later). The following cluster types are supported:
- [Local YugabyteDB clusters](https://docs.yugabyte.com/preview/quick-start/).
- [YugabyteDB Anywhere universes](https://docs.yugabyte.com/preview/yugabyte-platform/create-deployments/)
- [YugabyteDB Managed universes](https://docs.yugabyte.com/preview/yugabyte-cloud/cloud-basics/)

# Installation
Refer the [Machine Requirements](#machine-requirements) section for supported OS versions. 

Run the `installer_scripts/install-yb-voyager` script on a machine to prepare it
for running migrations using yb-voyager.

To correctly set environment variables required for the migration process run:

```
source $HOME/.yb-voyager.rc
``` 

Optionally, the installation script sources the `.yb-voyager.rc` file from the `~/.bashrc`. In which case, restarting the bash session will be enough to set the environment variables.

For additional information, refer to the [YugabyteDB Voyager Installation Docs](https://docs.yugabyte.com/preview/migrate/install-yb-voyager/).

# Migration Steps

The workflow to carry out a migration using *yb-voyager* is as follows:

```

 ┌─────────┬─────────────────────────────────────────┐
 │ Prepare │                                         │
 ├─────────┘                                         │
 │  ┌──────────┐      ┌──────────┐     ┌─────────┐   │
 │  │ Install  │      │ Prepare  │     │ Prepare │   │
 │  │yb-voyager├──────►Source DB ├─────►Target DB│   │
 │  └──────────┘      └──────────┘     └─────────┘   │
 │                                                   │
 └─────────────────────────┬─────────────────────────┘
                           │
                           │
 ┌─────────┬───────────────▼─────────────────────────┐
 │ Export  │                                         │
 ├─────────┘           ┌───────┐                     │
 │   ┌───────┐         │Analyze│         ┌──────┐    │
 │   │Export ├─────────►Schema ├─────────►Export│    │
 │   │Schema │         └─┬───▲─┘         │ Data │    │
 │   └───────┘         ┌─▼───┴─┐         └──────┘    │
 │                     │Modify │                     │
 │                     │Schema │                     │
 │                     └───────┘                     │
 └─────────────────────────┬─────────────────────────┘
                           │
                           │
 ┌─────────┬───────────────▼─────────────────────────┐
 │  Import │                                         │
 ├─────────┘                                         │
 │                        ┌─────────┐                │
 │  ┌──────┐  ┌──────┐    │Import   │    ┌──────┐    │
 │  │Import├──►Import├────►Triggers/├────►Verify│    │
 │  │Schema│  │ Data │    │Indexes  │    └──────┘    │
 │  └──────┘  └──────┘    └─────────┘                │
 │                                                   │
 └───────────────────────────────────────────────────┘
```
More details regarding the entire migration workflow, starting from setting up your source and target databases, up until verifying the migration can be found in the [Migration Steps](https://docs.yugabyte.com/preview/migrate/migrate-steps/) on the YugabyteDB Voyager Docs.

# License

Source code in this repository is variously licensed under the Apache License 2.0 and the Polyform Free Trial License 1.0.0. A copy of each license can be found in the [licenses](licenses) directory.

The build produces two sets of binaries:

* The entire database with all its features (including the enterprise ones) are licensed under the Apache License 2.0
* The  binaries that contain `-managed` in the artifact and help run a managed service are licensed under the Polyform Free Trial License 1.0.0.

> By default, the build options generate only the Apache License 2.0 binaries.
# Features and enhancements in the pipeline

Some of the important features and enhancements to follow soon are:

- [Support for ONLINE migration from Oracle/PostgreSQL/MySQL](https://github.com/yugabyte/yb-db-migration/issues/50)

You can look at all the open issues [here](https://github.com/yugabyte/yb-db-migration/issues).
