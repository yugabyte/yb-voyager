# DMS OPEN TOOL DOCUMENTATION

## Sections
- [Introduction](#introduction)
- [Resource Requirements](#resource-requirements)
- [Installation](#installation)
- [Migration Steps](#migration-steps)
    - [Source DB Setup](#source-db-setup)
    - [Target DB Setup](#target-db-setup)
    - [Report Generation](#report-generation)
    - [Source DB Export](#source-db-export)
    - [Manual Review (Export)](#export-phase-manual-review)
    - [Target DB Import](#target-db-import)
    - [Manual Review/Validation (Import Phase)](#import-phase-manual-review)

## Introduction
`yb_migrate --help`

yb_migrate is a simple to use, open-source tool meant to migrate databases from different source types (MySQL, Oracle, PostgreSQL) onto YugabyteDB.
Github Issue Link -yet to be linked-

## Resource Requirements
(We should come back to this when we decide if yb_migrate can be given as a binary without source code, database-relevant details will be mentioned in “migration steps” section)

## Installation
(The script needs some changes, leaving this section blank for now)

## Migration Steps
Below are the steps a user may follow to use the yb_migrate tool:

### Source DB Setup

### Target DB Setup

### Report Generation

`yb_migrate generateReport --help`
	
Before beginning the migration cycle, the user can generate a report, which provides the details of the schema objects to be exported, along with incompatibilities, if any. The incompatibilities will be tagged and a Github issue link will be provided with it if available. If there are no solutions available, the user will have to manually review the export phase (see below).

**Sample command:**

```yb_migrate generateReport --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname  --source-db-user username --output-format html```

The generated report will be found in `export-dir/reports/report.html`.

*To be included:*
- *What if command fails. Possible reasons, Like insufficient privileges. Export Dir missing or non empty etc*
- *Corrective actions*

### Source DB Export

`yb_migrate export --help`

The export phase is carried out in two parts, export schema and export data respectively. It is recommended to start this phase after having completed the report generation phase (see above). 

#### Export Schema

`yb_migrate export schema --help`

**Sample command:**

```yb_migrate export schema --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username```

The schema sql files will be found in `export-dir/schema`. A report regarding the export of schema objects can be found in `export-dir/reports`.

*To be included:*
- *What if command fails*
- *Possible reasons*
- *Corrective actions*

#### Export Data

`yb_migrate export data --help`

**Sample command:**

```yb_migrate export data --export-dir /path/to/yb/export/dir --source-db-type postgresql --source-db-host localhost --source-db-password password --source-db-name dbname --source-db-user username```

The data sql files will be found in `export-dir/data`.

*To be included:*
- *What if command fails*
- *Possible reasons*
- *Corrective actions*

### Export Phase Manual Review
*To be included:*
- *Why?*
- *What can the users do here with some examples*

### Target DB Import

`yb_migrate import --help`

This command/series of commands is/are used to initiate the import of schema and data objects onto YugabyteDB. It is mandatory that the user has completed the export phase at a minimum (see above), and it is recommended that the user completes a manual review of the exported schema and data files, which will be found in the `export-dir/schema` and `export-dir/data` folders respectively. A report will be generated in the `export-dir/reports` folder to help speed up this verification process.

#### Import Schema

`yb_migrate import schema --help`

**Sample command:**

```yb_migrate import schema --export-dir /path/to/yb/export/dir --target-db-host localhost --target-db-password password --target-db-name dbname --target-db-schema public --target-db-user username --parallel-jobs 10 --batch-size 100000```

The schema sql files should be located in the `export-dir/schema` folder.

#### Import Data

`yb_migrate import data --help`

**Sample command:**

```yb_migrate import data --export-dir /path/to/yb/export/dir --target-db-host localhost --target-db-password password --target-db-name dbname --target-db-schema public --target-db-user username --parallel-jobs 100 --batch-size 250000```

The data sql files should be located in the `export-dir/data` folder.

#### Import Data Status
*To be included:*
- *What does this command do?*
- *How do we use it?*

### Import Phase Manual Review