# Base workflow framework. 

In voyager, we seem to have various workflows that we need to model. For example, 

1. In import-data, there is the import-snapshot > import-cdc-events > cutover-processing > start-new-commands-if-necessary. 
This is essentially like a durable workflow, in that if some step fails, and we retry the command, it should resume from the last incomplete step. 

Like this various other commands have similar requirements. 
In future, we were even thinking of introducing a "schema migrate" command instead of export-schema, analyze-schema, import-schema commands currently. That leads to the requirement of that single command again having that durable workflow. 

These durable workflows for now are going to run in the same process. The state of the workflow should be persisted on sqlite/postgres. To start with, it will be sqlite; later we will use yugabytedb(postgres compatible). 

2. There is the overall migration workflow as well. this is at the command level. 
I.e. assess -> start-migration -> schema-migrate -> data->migrate -> validate -> end

Today, all of this runs on the same machine, but in future, they could ideally be executed by different workers processes (something like temporal). 

a. Here, the workflow has to be durable ofc (similar to the above case)
b. The individual steps themselves may involve workflows (like schema-migrate for example)
c. We should have a way to understand the overall status of the entire migration (including that of the sub-workflows) so that it becomes very clear to the user as to what is happening exactly. 


---

These both requirements seem to point to a generic workflow framework with state stored in sqlite/postgres. 

----

1. Does this make sense? or am i thinking in a wrong direction? Can you suggest some alternate lines of thinking for this problem statement to make sure i'm not going down the wrong path. 
2. Are there libraries out there in golang that will help me model these requirements? (go-workflows seems to be popular, but it doesn't seem to have a clear way to retrieve the workflow state/status, but i might not have checked properly.)

