# Fleet Journey

## Problem

The goal is to enable fleet assessment. The idea is to build it on top of cli_user_journey.md
THe user's environment may have 100s of databases. 
They would want to assess all the databases, in order to figure out: which database to migrate first. 


## Proposal


### Setting up control plane
In init: 

After explaining that assess is the first step and taking source credentials, we ask a second question/prompt. 

"To assess multiple databases, set up a shared YugabyteDB instance to view all assessments together."
Give the user two options: 
1. Use a shared YugabyteDB instance
2. Use local UI 


If the user chooses option 1, ask for the control plane connection string and set that in the config file for "assessment-control-plane" under assess-migration. 
If the user chooses option 2, the default local conn string will work. No need to change anything. 



### Starting a migration that has been assessed before (and stored in a control plane)
Now, let's solve the flow where in the user has assessed a database (and used a common control plane to view the assessments.). 
Now, the user wants  to start the migration. Two possibilities here: 

1. If the user has access to the migration dir that they used to previously assess, they can just continue by running start-migration --config-file <>. 
2. If the user does not have access to the old migration dir (which is very possible), then we enhance start-migration to:
    a. take the migration-dir, assessment-control-plane, migration-uuid as CLI args. 
    b. Set up export-dir. 
    c. Retrieve assessment details from the control plane. Store the assessment JSON in the export-dir (so that it is usable by further commands)
    d. Retrivve source connection details from the control plane. If not fully available (password will never be stored so this will always be the case that we don't have full connection details), ask the user for source connection string. Store the connection details in config file. 
    e. Present asssesment summary (same as current)
    f. Ask for target DB connection string. (same as current)
    g. Ask for data migration flow (same as current)

You can't pass both config-file as well as any of the migration-dir/assessment-control-plane/migration-uuid as it doesn't make sense. 

Some of this is currently implemented as part of init, let's move that to start-migration. 


## Implementation. 
Use these for testing out the entire flow. 

Source postgres: postgresql://amakala:password@localhost:5432/db1 You can use db1, db2, db3, no other databases. 
Target YB: postgresql://yugabyte:yugabyte@localhost:5433/db2 . Create the databases on YB with the same name. Again do not touch any other databases. 
Control plane: use local control plane - postgresql://yugabyte:yugabyte@localhost:5433


postgresql://amakala:password@localhost:5432/db1
postgresql://yugabyte:yugabyte@localhost:5433/db2
postgresql://yugabyte:yugabyte@localhost:5433
yb-voyager start-migration --assessment-control-plane 'postgresql://yugabyte:yugabyte@10.9.15.135:5433' --migration-uuid 'acd1f159-2e9e-40c8-8f93-44d61bd24f54' --migration-dir ./db2-migration