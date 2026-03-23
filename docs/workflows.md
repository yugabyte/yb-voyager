# Base workflow framework.
- In voyager, we seem to have various workflows that we need to model. For example,
- 1. In import-data, there is the import-snapshot > import-cdc-events > cutover-processing > start-new-commands-if-necessary.
- This is essentially like a durable workflow, in that if some step fails, and we retry the command, it should resume from the last incomplete step.
- Like this various other commands have similar requirements.
- In future, we were even thinking of introducing a "schema migrate" command instead of export-schema, analyze-schema, import-schema commands currently. That leads to the requirement of that single command again having that durable workflow.
- These durable workflows for now are going to run in the same process. The state of the workflow should be persisted on sqlite/postgres. To start with, it will be sqlite; later we will use yugabytedb(postgres compatible).

- 2. There is the overall migration workflow as well. this is at the command level.
- I.e. assess -> start-migration -> schema-migrate -> data->migrate -> validate -> end
- Today, all of this runs on the same machine, but in future, they could ideally be executed by different workers processes (something like temporal).
- a. Here, the workflow has to be durable ofc (similar to the above case)
- b. The individual steps themselves may involve workflows (like schema-migrate for example)
- c. We should have a way to understand the overall status of the entire migration (including that of the sub-workflows) so that it becomes very clear to the user as to what is happening exactly.

These both requirements seem to point to a generic workflow framework with state stored in sqlite/postgres.


# Proposal
- Here's how i'm thinking about it.
- - workflow definition (static set of steps)
- - workflow state (tracks where the workflow is, what steps are completed, what are not)
- - workflow executor (the engine that actually runs the workflow). Now this could be implemented in various ways, it could be deterministically done automatically like airflow style or temporal style using go language)
- By default, the workflow executor executes and updates state. To deal with the inter-command, we simply support a flow wherein the workflow state can be changed manually without the workflow executor.


Tasks: 
1. Create a new workflows pkg (under src/)
2. introduce the ability to define the workflow as a static set of steps (i.e. workflow definition). Take inspiration from https://github.com/xmonader/ewf , but also keep in mind that we will probably need parallel tasks (in other words, a DAG, at some point in the future)
3. There is the workflow definition, and a workflow instance (an instance of the workflow -- probably using uuid and startTime?)
4. We should support nested/child workflows (infinite nesting). For example, in the main migration workflow (assess -> start-migration -> schema-migrate -> data->migrate -> validate -> end), schema-mmigrate itself is a subworkflow (export -> analyze -> import) 
5. Introduce ability to update workflow state. Use metadb (do not re-use MIgrationStatusRecord, that is already very bloated). Use a separate table. (not json_objects) to store state. (later metadb could live on postgres/Yugabyte if required, so keep the types very basic anyway)

