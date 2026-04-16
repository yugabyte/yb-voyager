# Restructure commands structure and Help message. 

## Problems: 
Currently commands are structured/organized in a manner that is disjointed from what the high-level mental model of a migration should be. 

That model is: 
1. Assess Migration
2. Migrate Schema
3. Migrate Data
4. Validate consistency between source/target (data, performance)
5. CLean up state (end migration)


Now, let's restructure the commands to fit under this broad categorization so that the user has that consistent view. 

----
### Global Commands
yb-voyager init  - Setup wizard
yb-voyager status  - Overall migration progress
yb-voyager version

### Assessment Phase
yb-voyager assess run - Single DB assessment

### Prepare
yb-voyager prepare

### Schema Phase
yb-voyager schema export - Export DDL from source
yb-voyager schema analyze - Check YB compatibility
yb-voyager schema import - Import to target
yb-voyager schema finalize-post-data-import - Refresh m-views, create not-valid contraints

### Data Phase
yb-voyager data export
yb-voyager data import

yb-voyager data export-from-target
yb-voyager data import-to-source
yb-voyager data import-to-replica

yb-voyager data status - Consolidated data migration report
yb-voyager data archive-changes - Cleanup applied CDC events from local disk
yb-voyager data import-file - Import from CSV/text files

### Cutover
yb-voyager cutover prepare-target    # default: to target
yb-voyager cutover prepare-source    # fall-back
yb-voyager cutover prepare-replica   # fall-forward
yb-voyager cutover status

### Validation Phase
yb-voyager validate compare-performance - performance comparison between source/target

### Cleanup
yb-voyager end - End migration, cleanup metadata
----


Now all of these commands already exist. I just want a rename/restructure. No new commands. 
THis should also be reflect in the What's Next Next step footers we provide. (Refer cli_user_journey.md for high level design.)


As a result, the help command for voyager should ideally just list these top level commands, which will make it easier for the user to understand what is going on. 