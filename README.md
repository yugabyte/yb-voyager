# YugabyteDB Voyager

[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)
[![Documentation Status](https://readthedocs.org/projects/ansicolortags/badge/?version=latest)](https://docs.yugabyte.com/preview/yugabyte-voyager/)
[![Ask in forum](https://img.shields.io/badge/ask%20us-forum-orange.svg)](https://forum.yugabyte.com/)
[![Slack chat](https://img.shields.io/badge/Slack:-%23yugabyte_db-blueviolet.svg?logo=slack)](https://communityinviter.com/apps/yugabyte-db/register)
[![Analytics](https://yugabyte.appspot.com/UA-104956980-4/home?pixel&useReferer)](https://github.com/yugabyte/ga-beacon)

YugabyteDB Voyager is a powerful open-source data migration engine that accelerates cloud native adoption by removing barriers to moving applications to the public or private cloud. It helps you migrate databases to YugabyteDB quickly and securely.

YugabyteDB Voyager manages the entire lifecycle of a database migration, including cluster preparation for data import, schema migration, and data migration, using the `yb-voyager` command line interface (CLI).

<img src="docs/voyager_architecture.png" align="center" alt="YugabyteDB Voyager Architecture"/>

## Table of Contents

- [Features](#features)
- [Source Databases](#source-databases)
- [Target Databases](#target-databases)
- [Migration Types](#migration-types)
- [Prerequisites](#prerequisites)
- [Installation](#installation)
- [Migration Workflow](#migration-workflow)
- [Need Help?](#need-help)
- [Contribute](#contribute)
- [License](#license)

## Features

- Free and completely open source.
- Unified CLI experience for all source and target databases.
- Live migration of PostgreSQL databases with fall-forward and fall-back.
- Live migration of Oracle databases with fall-forward and fall-back.
- Direct data import from CSV or TEXT format files on local disk or cloud storage.
- Parallelism of data across tables and within tables.
- Auto-tuneable based on workloads, by analyzing the target cluster capacity.
- Data import is resumable in case of failures.
- Progress monitoring with estimated time for data export and import.
- Scales with nodes and CPU.
- Safe defaults for production use.

## Source Databases

| Source Database | Migration Type | Supported Versions and Flavors |
|---|---|---|
| **PostgreSQL** | Offline and Live | PostgreSQL 11.x - 17.x, [Amazon Aurora PostgreSQL](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.AuroraPostgreSQL.html), [Amazon RDS for PostgreSQL](https://aws.amazon.com/rds/postgresql/), [Cloud SQL for PostgreSQL](https://cloud.google.com/sql/docs/postgres), [Azure Database for PostgreSQL](https://azure.microsoft.com/en-ca/services/postgresql/) |
| **MySQL** | Offline | MySQL 8.x, MariaDB, [Amazon Aurora MySQL](https://docs.aws.amazon.com/AmazonRDS/latest/AuroraUserGuide/Aurora.AuroraMySQL.html), [Amazon RDS for MySQL](https://aws.amazon.com/rds/mysql/), [Cloud SQL for MySQL](https://cloud.google.com/sql/docs/mysql) |
| **Oracle** | Offline and Live | Oracle 11g - 19c, [Amazon RDS for Oracle](https://aws.amazon.com/rds/oracle/) |

## Target Databases

YugabyteDB Voyager supports all YugabyteDB products as the target:

- [YugabyteDB](https://docs.yugabyte.com/preview/deploy/)
- [YugabyteDB Anywhere](https://docs.yugabyte.com/preview/yugabyte-platform/create-deployments/)
- [YugabyteDB Aeon](https://docs.yugabyte.com/preview/yugabyte-cloud/cloud-basics/)

For detailed version compatibility across migration types, refer to the [target database documentation](https://docs.yugabyte.com/preview/yugabyte-voyager/introduction/#target-database).

## Migration Types

| Migration Type | Description | Supported Sources |
|---|---|---|
| [Offline migration](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/) | Take applications offline to perform the migration. | PostgreSQL, MySQL, Oracle |
| [Live migration](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-migrate/) | Migrate data while your application is running. | PostgreSQL, Oracle |
| [Live migration with fall-forward](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-fall-forward/) | Fall forward to a source-replica database for your live migration. | PostgreSQL, Oracle |
| [Live migration with fall-back](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/live-fall-back/) | Fall back to the source database for your live migration. | PostgreSQL, Oracle |
| [Bulk data load](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/bulk-data-load/) | Import data directly from CSV or TEXT files on local disk or cloud storage. | Flat files |

## Prerequisites

- **Operating systems**: macOS, Ubuntu (18.04, 20.04, 22.04), CentOS 8, RHEL (8, 9)
- **Hardware**: Minimum 2 cores (recommended); disk space at least 2x the estimated source database size
- **Software**: Java 17
- **Network**: The installation host must be able to connect to both the source and target databases

For full details, refer to the [Prerequisites documentation](https://docs.yugabyte.com/preview/yugabyte-voyager/install-yb-voyager/#prerequisites).

## Installation

Install `yb-voyager` using one of the following methods:

| Method | Command / Link |
|---|---|
| **macOS (Homebrew)** | `brew tap yugabyte/tap && brew install yb-voyager` |
| **Ubuntu (apt)** | `sudo apt-get install yb-voyager` |
| **RHEL/CentOS (yum)** | `sudo yum install yb-voyager` |
| **Docker** | `docker pull yugabytedb/yb-voyager` |
| **Source** | Clone this repo and run `installer_scripts/install-yb-voyager` |
| **Airgapped** | Available for Docker, Ubuntu, and RHEL environments |

> **Note**: On macOS, migration from MySQL/Oracle source databases requires the Docker installation method.

For detailed installation and upgrade instructions, refer to the [Install documentation](https://docs.yugabyte.com/preview/yugabyte-voyager/install-yb-voyager/).

Verify the installation:

```bash
yb-voyager version
```

## Migration Workflow

<img src="https://docs.yugabyte.com/images/migrate/migration-workflow-new.png" align="center" alt="YugabyteDB Voyager Migration Workflow"/>

A typical migration using YugabyteDB Voyager involves the following phases:

1. **[Assess migration](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/assess-migration/)** -- Generate a Migration Assessment Report to evaluate readiness and plan the migration.
2. **[Export schema](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/)** -- Export the schema from the source database.
3. **[Analyze schema](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/)** -- Analyze the exported schema for compatibility with YugabyteDB and apply suggested fixes.
4. **[Import schema](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/)** -- Import the schema into the target YugabyteDB cluster.
5. **[Export data](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/)** -- Export data from the source database.
6. **[Import data](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/)** -- Import data into YugabyteDB.
7. **[Verify migration](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/)** -- Verify the migration was successful.
8. **[End migration](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/migrate-steps/)** -- Clean up migration metadata and finalize.

For the complete migration guide, refer to the [Migration documentation](https://docs.yugabyte.com/preview/yugabyte-voyager/migrate/).

## Need Help?

- Ask questions, find answers, and help others on our community [Slack](https://communityinviter.com/apps/yugabyte-db/register), [Forum](https://forum.yugabyte.com), [Stack Overflow](https://stackoverflow.com/questions/tagged/yugabyte-db), and Twitter [@Yugabyte](https://twitter.com/yugabyte).
- Report issues or request new features on [GitHub Issues](https://github.com/yugabyte/yb-voyager/issues).
- Refer to the [full documentation](https://docs.yugabyte.com/preview/yugabyte-voyager/) for in-depth guides and reference material.

## Contribute

As an open-source project with a strong focus on the user community, we welcome contributions as GitHub pull requests. See our [Contributor Guides](https://docs.yugabyte.com/preview/contribute/) to get started. Discussions and RFCs for features happen on the design discussions section of our [Forum](https://forum.yugabyte.com).

## License

Source code in this repository is variously licensed under the Apache License 2.0 and the Polyform Free Trial License 1.0.0. A copy of each license can be found in the [licenses](licenses) directory.
