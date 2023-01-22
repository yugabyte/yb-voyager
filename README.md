# YugabyteDB Voyager
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Documentation Status](https://readthedocs.org/projects/ansicolortags/badge/?version=latest)](https://docs.yugabyte.com/)
[![Ask in forum](https://img.shields.io/badge/ask%20us-forum-orange.svg)](https://forum.yugabyte.com/)
[![Slack chat](https://img.shields.io/badge/Slack:-%23yugabyte_db-blueviolet.svg?logo=slack)](https://communityinviter.com/apps/yugabyte-db/register)
[![Analytics](https://yugabyte.appspot.com/UA-104956980-4/home?pixel&useReferer)](https://github.com/yugabyte/ga-beacon)

YugabyteDB Voyager is a powerful open-source data migration engine that accelerates cloud native adoption by removing barriers to moving applications to the public or private cloud. It helps you migrate databases to YugabyteDB quickly and securely.

YugabyteDB Voyager manages the entire lifecycle of a database migration, including cluster preparation for data import, schema-migration, and data-migration, using the yb-voyager command line utility.

<img src="docs/voyager_architecture.png" align="center" alt="YugabyteDB Voyager Architecture"/>

- [Highlights](#highlights)
- [Machine Requirements](#machine-requirements)
- [Installation](#installation)
- [Migration Steps](#migration-steps)
- [License](#license)

# Highlights
- Free and completely open source
- Identical steps and unfied experience for all sources and destinations
- Optimized for distributed databases 
  - Chunks the source files
  - Scales and auto tunes based on cluster size
  - Number of connections based on target configuration
  - Parallelism across tables and within tables
- Idempotent
- Data import is resumable in case of disruption
- Visual progress for data export and import
- Scales with nodes and cpu
- Packaged for easy installation, one-click installer
- Safe defaults
- Direct data import from CSV and text files


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
