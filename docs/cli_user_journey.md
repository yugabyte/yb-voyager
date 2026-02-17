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





## Config File 

When init creates a config file, it can also create a symlink in $HOME/.ybvoyager/configs/  
Thereafter, yb-voyager will automatically look at $HOME/.ybvoyager/configs. If there is only a single configuration, it will not be mandatory to pass the config file in each yb-voyager command. 

[image1]: <data:image/png;base64,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>