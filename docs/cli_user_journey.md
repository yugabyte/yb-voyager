# Improve User Journey \- Phase 1

Current help behaves like a man page. 

- Lists commands alphabetically / structurally   
- Assumes users already know what Voyager does internally   
- Which commands are mandatory vs optional   
- Which path applies to their database   
- No notion of phases, happy path, or decision points

# Current User Journey

![][image1] 

1. Install voyager on a machine  
2. Prepare Source DB  
   1. Create a user \`ybvoyager\`   
   2. Run yb-voyager-pg-grant-migration-permissions.sql from /opt/yb-voyager/…   
3. Prepare Target DB  
   1. CREATE DATABASE target\_db\_name;  
   2. CREATE USER ybvoyager SUPERUSER PASSWORD 'password';  
4. Create directory export-dir  
   1. Migration state  
   2. Data  
   3. Reports  
   4. schema  
5. Setup config file  
   1. Copy a template from /opt/yb-voyager/…  
   2. Fill in values for source/target connection details   
   3. Configure control plane UI  
6. Assess Migration  
   1. Run assessment  
      1. With connectivity  
         1. ***yb-voyager assess-migration \--config-file \<path-to-config-file\>***  
      2. Without connectivity  
         1. Run bash/psql script present at /etc/yb-voyager/gather-assessment-metadata/ to gather assessment data  
         2. ***yb-voyager assess-migration … \--assessment-metadata-dir /path/to/assessment\_metadata\_dir***  
   2. Manually adjust target DB  
      1. Resize your target YugabyteDB cluster based on sizing recommendations in assessment report  
      2. Ensure your DB is colocated (if assessment recommended some tables to be colocated)  
7. Migrate Schema  
   1. Export Schema – optimized for YB	  
      1. ***yb-voyager export schema***  
   2. Analyze Schema  
      1. ***yb-voyager analyze-schema***  
      2. Manually Iterate on schema changes by editing the schema files  
   3. Import Schema  
      1. ***yb-voyager import schema***  
8. Migrate Data  
   1. Export Data  
      1. ***yb-voyager export data***  
   2. Import Data  
      1. ***yb-voyager import data***  
9. Cutover  
   1. fall-forward/fall-back if required  
10. Validate  
    1. Validate Data  
       1. (outside of voyager) Manually run queries to validate data  
    2. Validate performance  
       1. Run workloads, compare perf, and iterate  
          1. ***yb-voyager compare-performance***  
11. End \- cleanup  
    1. ***yb-voyager end-migration***

## Gaps in user journey:

1. The CLI does not clearly convey the current state of a migration or guide the user on what to do next. Users must rely heavily on documentation to figure out the migration flow.  
2. Assessment/TargetDB  
   1. The user may not have created a target DB at the time of assessment but we require the user to prepare the target DB **before** assessment. After assessment, the user is asked to re-size/modify target YB.  
3. lack of automation in export-dir and config file setup  
   1. manually create export-dir.  
   2. manually copy the config template from /opt to desired location  
   3. fill in the export-dir in the config-file just created.

# Proposal

## Introduce a “Guided Entry Point”

* Add a top-level yb-voyager init and start-migration commands


1. Yb-voyager Init   
2. Yb-voyager assess-migration   
3. Yb-voyager start-migration

## Init

```shell
yb-voyager init --migration-dir /path/to/migration-dir 

══════════════════════════════════════════════════════════════
                    Welcome to YB Voyager
══════════════════════════════════════════════════════════════

YB Voyager is a database migration tool that helps you migrate
to YugabyteDB.

What YB Voyager can do:
  • Assess your source database for migration complexity and sizing
  • Export and import schema with automatic YugabyteDB optimizations
  • Migrate data via offline snapshot or live change data capture (CDC)
  • Validate data consistency between source and target

Supported source databases: PostgreSQL, Oracle, MySQL
Documentation: https://docs.yugabyte.com/preview/yugabyte-voyager/

══════════════════════════════════════════════════════════════

First, let's assess your source database.

? How would you like to connect? (Enter empty string to skip and configure later)
It is recommended to run assessment against production database for an accurate assessment.
  1. Enter a connection string
  2. I don't have access to source database. Generate bash/sql scripts that can be run on source database to gather metadata for assessment.


> 





```

### Option: Connection string provided: 

```sql
> postgresql://app_user:secret@prod-db.example.com:5432/ecommerce

  ✓ Connected to prod-db.example.com:5432
  ✓ Created export directory: /path/to/migration-dir/export-dir
  ✓ Generated config: /path/to/migration-dir/config.yaml

  Next steps:

  1. Run assessment:
     yb-voyager assess-migration --config /path/to/migration-dir/config.yaml

```

1. If connection succeeds  
   1. Config file	  
      1. we fill the configuration file source connection details appropriately. (db type, host, port, dbname, ssl, schema, etc)  
2. Schemas  
   1. Not typical to have schema name in connection string.   
      1. psql "postgresql://user:pwd@127.0.0.1:6789/postgres?options=-csearch\_path%3Dabc"   
   2. If schema is present in connection string, we use that, otherwise we assume all schemas present in db (fetch that from DB)  
3. If connection does not succeed  
   1. We ask the first prompt again.  
4. Data migration flow?   
   1. The config file will be generated in a way where the data migration flow (offline/live/live-ff/live-fb) is not fixed.   
      1. Export-data: export-type is not filled. (mandatory param)  
      2. initiate-cutover-to-target	prepare-for-fall-back	is not filled (mandatory param)  
   2. Separate \`yb-voyager start-migration\` command.   
5. Permissions?  
   1. We can run guardrail checks after validating the connection (if they have enough permissions for assessment-only),   
   2. if the user does not have sufficient permissions, we add another step to the next steps \- to run the grant-permissions script (before running assess-migration). 
   we can also tell the user to create a new user if they want: 
   If you wish to create a new user, run the following: 
         1. CREATE USER ybvoyager PASSWORD 'password';
         2. Grant permissions to the user by running : 
psql -h prod-db.example.com -d ecommerce -U app_user ... \
       -f /path/to/migration-dir/export-dir/scripts/yb-voyager-pg-grant-migration-permissions.sql

### Option: Skip and configure later

```sql
> [empty input]

  ✓ Created export directory: /path/to/migration-dir/export-dir
  ✓ Generated config: /path/to/migration-dir/config.yaml

  Next steps:

  1. Add your source connection details to the config file:
     vi /path/to/migration-dir/config.yaml

  2. Run assessment:
     yb-voyager assess-migration --config /path/to/migration-dir/config.yaml

══════════════════════════════════════════════════
Tip: Run yb-voyager status -c /path/to/migration-dir/config.yaml
     anytime to check migration progress and see what to do next.
══════════════════════════════════════════════════

```

Permissions

1. When assess migration is run, we will guardrail and point the user to the grant permissions script if required. 

## 

### Option: Generate scripts 

```shell
> 2

  ✓ Created export directory: /path/to/migration-dir/export-dir
  ✓ Generated config: /path/to/migration-dir/config.yaml
  ✓ Copied metadata-gathering scripts to:
    /path/to/migration-dir/export-dir/scripts/

  Next steps:

  1. Copy the appropriate scripts directory to a machine that can reach
     your source database:

     PostgreSQL:  export-dir/scripts/postgres
     Oracle:      export-dir/scripts/oracle

     Run the yb-voyager-<db_type>-gather-assessment-metadata.sh script with --help to see usage.

  2. Copy the resulting metadata directory back to this machine.

  3. Run assessment:
     yb-voyager assess-migration --config /path/to/migration-dir/config.yaml \
       --assessment-metadata-dir /path/to/assessment-metadata

══════════════════════════════════════════════════
Tip: Run yb-voyager status -c /path/to/migration-dir/config.yaml
     anytime to check migration progress and see what to do next.
══════════════════════════════════════════════════

```

## 

## Start-migration

At the end of running assess-migration, the next step is to run \`start-migration\` which sets up for starting migration (export-schema/import-schema , export-data/import-data, etc, etc.)

```shell
yb-voyager start-migration --config /path/to/migration-dir/config.yaml

══════════════════════════════════════════════════════════════
                    Start Migration
══════════════════════════════════════════════════════════════

Source: PostgreSQL — prod-db.example.com:5432/ecommerce
Assessment: completed ✓

  Your assessment report recommended:
    • Target YB cluster size: 3 nodes, 4 vCPU / 16 GB RAM each
    • Estimated data size: 120 GB

  View full report:
    /path/to/migration-dir/export-dir/assessment/reports/migration_assessment_report.html

══════════════════════════════════════════════════════════════

Before proceeding, 
1. Ensure your target YugabyteDB cluster is created and running per the sizing recommendations above.
2. Create a database. 3. Create a superuser `CREATE USER ybvoyager SUPERUSER PASSWORD 'password';`

? Enter the connection string for your target YugabyteDB: (Format:  postgresql://user:password@host:5433)
> postgresql://yugabyte:yugabyte@yb-cluster.example.com:5433

  ✓ Connected to yb-cluster.example.com:5433 (YugabyteDB 2024.2.1.0)

? Select your data migration workflow:
  1. Offline                — One-time snapshot; requires downtime
  2. Live                   — Minimal downtime using change data capture
  3. Live with fall-back    — Can switch back to source if needed
  4. Live with fall-forward — Can switch to a source-replica if needed

> 2

  ✓ Updated config: /path/to/migration-dir/config.yaml
══════════════════════════════════════════════════════════════

  Next steps:

  1. Grant permissions on your source database for live migration:

     psql -h prod-db.example.com -d ecommerce -U app_user ... \
       -f /path/to/migration-dir/export-dir/scripts/yb-voyager-pg-grant-migration-permissions.sql

  2. Export schema:
     yb-voyager export schema --config /path/to/migration-dir/config.yaml

══════════════════════════════════════════════════
Tip: Run yb-voyager status -c /path/to/migration-dir/config.yaml
     anytime to check migration progress and see what to do next.
══════════════════════════════════════════════════

```
   

## Add a “migration status” command

```shell
$ yb-voyager status -c <config-file>

+--------------+--------------------------------------------------+
| Migration    | /path/to/export/dir                               |
|              | UUID: a1b2c3d4-e5f6-7890-abcd-ef1234567890        |
|              | Workflow: Live Migration                          |
+--------------+--------------------------------------------------+
| Source       | PostgreSQL  source-db.example.com:5432            |
|              | Database: myapp_production                        |
|              | Schema: public                                    |
+--------------+--------------------------------------------------+
| Target       | YugabyteDB  yb-cluster.example.com:5433           |
|              | Database: myapp                                   |
|              | Schema: public                                    |
+--------------+--------------------------------------------------+
| Status       | [✓] Assess     done                               |
|              | [>] Schema     in progress (step 1 of 4)          |
|              | [ ] Data       pending                            |
|              | [ ] Cutover    pending                            |
|              | [ ] End        pending                            |
+--------------+--------------------------------------------------+
| Last Command | export-schema  Completed                          |
|              | Exported: 45 tables, 12 views, 8 functions        |
+--------------+--------------------------------------------------+
| Next Step    | yb-voyager analyze-schema -c <config-file>        |
+--------------+--------------------------------------------------+
| Docs         | https://docs.yugabyte.com/...                     |
+--------------+--------------------------------------------------+


```

## Add a summary/next-steps footer to every command

```shell

Assessment footer

✓ Assessment completed

+--------------+--------------------------------------------------+
| Artifacts    | reports/migration_assessment_report.html           |
|              | reports/migration_assessment_report.json           |
+--------------+--------------------------------------------------+
| Summary      | Compatibility   94% (186/198 objects)              |
|              | Issues          12 (8 auto-fixable, 4 manual)      |
|              | Est. Effort     Low                                |
+--------------+--------------------------------------------------+
| Next Step    | yb-voyager export-schema -c <config-file>          |
+--------------+--------------------------------------------------+
| Docs         | https://docs.yugabyte.com/.../assess-migration     |
+--------------+--------------------------------------------------+
| Status       | [✓] Assess     done                                |
|              | [ ] Schema     pending                             |
|              | [ ] Data       pending                             |
|              | [ ] Validate   pending                             |
|              | [ ] End        pending                             |
+--------------+--------------------------------------------------+


Analyze-schema footer
+--------------+--------------------------------------------------+
| Artifacts    | reports/schema_analysis_report.html               |
|              | reports/schema_analysis_report.json               |
+--------------+--------------------------------------------------+
| Summary      | Objects Analyzed   65                              |
|              | Compatible         58 (89%)                        |
|              | Errors             2 (must fix)                    |
|              | Warnings           5 (review recommended)          |
+--------------+--------------------------------------------------+
| Next Step    | 1. Review/fix issues in schema/*.sql files         |
|              | 2. Re-run: yb-voyager schema analyze -e /export    |
|              | 3. Import: yb-voyager schema import -e /export     |
+--------------+--------------------------------------------------+
| Docs         | https://docs.yugabyte.com/.../analyze-schema       |
+--------------+--------------------------------------------------+
| Status       | [✓] Assess     done                                |
|              | [>] Schema     in progress (step 2 of 4)           |
|              | [ ] Data       pending                             |
|              | [ ] Validate   pending                             |
|              | [ ] End        pending                             |
+--------------+--------------------------------------------------+

export-data footer
+--------------+--------------------------------------------------+
| Artifacts    | data/public.users.csv                             |
|              | data/public.orders.csv                            |
|              | ... (42 files total)                              |
+--------------+--------------------------------------------------+
| Summary      | Tables Exported   45                              |
|              | Total Rows        1,234,567                       |
|              | Total Size        2.3 GB                          |
|              | Duration          12m 34s                         |
+--------------+--------------------------------------------------+
| Next Step    | yb-voyager import-data-to-target -e /export       |
+--------------+--------------------------------------------------+
| Docs         | https://docs.yugabyte.com/.../export-data         |
+--------------+--------------------------------------------------+
| Status       | [✓] Assess     done                               |
|              | [✓] Schema     done                               |
|              | [>] Data       in progress (step 1 of 2)          |
|              | [ ] Validate   pending                            |
|              | [ ] End        pending                            |
+--------------+--------------------------------------------------+

```

## Reframe help output around Migration Phases

Rename commands and group them by phase sub-commands.

```shell
# Global Commands
yb-voyager init (NEW) - Setup wizard
yb-voyager status (NEW) - Overall migration progress
yb-voyager version

# Assessment Phase
yb-voyager assess run - Single DB assessment
yb-voyager assess run-fleet - Fleet assessment 
yb-voyager assess status - View assessment results

# Schema Phase
yb-voyager schema export - Export DDL from source
yb-voyager schema analyze - Check YB compatibility
yb-voyager schema import - Import to target
yb-voyager schema finalize-post-data-import - Refresh m-views, create not-valid contraints
yb-voyager schema status - Schema migration progress

# Data Phase
yb-voyager data export-from-source
yb-voyager data import-to-target

yb-voyager data export-from-target
yb-voyager data import-to-source
yb-voyager data import-to-source-replica

yb-voyager data prepare-cutover-to-target    # default: to target
yb-voyager data prepare-cutover-to-source    # fall-back
yb-voyager data prepare-cutover-to-replica   # fall-forward
yb-voyager data archive-changes - Cleanup applied CDC events from local disk
yb-voyager data status - Consolidated data migration report

yb-voyager data import-file - Import from CSV/text files

# Validation Phase
yb-voyager validate compare-performance - performance comparison between source/target

# Cleanup
yb-voyager end - End migration, cleanup metadata
```

## Docs must mirror CLI phases

**CLI Phase		Docs Section**   
assess-migration	/assess-migration/   
export schema	/schema-migration/export-schema/   
analyze-schema	/schema-migration/analyze-schema/   
import data		/data-migration/import-data/   
cutover		/cutover/





Latest feedback: 

- make it simpler for basic use-cases. fleet is advanced use-case. 
    - don't ask question about assessment control plane. assume local UI by default
- init sounds like something you run once but it seems to be run once per migration. 
    - maybe begin? 
- use $HOME/.voyager
- use migration name instead of uuid/config files
    - If only one migration just assume that
- voyager status should return status of all migrations 
    - --migration-name xyz
- git style flow does not really make sense because you don't really work within that directory in the case of voyager. It's essentialyl just some data/metadata that you mostly should not have to look at. 
- migrate -> modernize
- use yugabyted UI as inspiration
- control plane
    - probably until schema import (involving manual schema edits), local control plane makes sense. after that point user to target db control plane UI? 
    - you could also update both. (local UI + target DB control plane)
- help should be more consise and the commands should be self-explanatory as possible. 