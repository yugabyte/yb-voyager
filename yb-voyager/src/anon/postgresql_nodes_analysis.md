# Complete PostgreSQL Node Types Analysis from pg_query_go Repository

Based on the analysis of the gitingest file containing the pg_query_go repository code, here is a comprehensive list of all PostgreSQL SQL node types identified:

## Core Expression Nodes (T_1-T_100)

### Basic Data Types and Constants
- **T_List** = 1 - Generic list container
- **T_Const** = 7 - Constant values
- **T_Param** = 8 - Query parameters
- **T_Integer** = 460 - Integer literals
- **T_Float** = 461 - Float literals  
- **T_Boolean** = 462 - Boolean literals
- **T_String** = 463 - String literals
- **T_BitString** = 464 - Bit string literals

### Expression Nodes
- **T_Var** = 6 - Variable references
- **T_Aggref** = 9 - Aggregate function references
- **T_GroupingFunc** = 10 - GROUPING function
- **T_WindowFunc** = 11 - Window functions
- **T_WindowFuncRunCondition** = 12 - Window function run conditions
- **T_MergeSupportFunc** = 13 - Merge support functions
- **T_SubscriptingRef** = 14 - Array/subscript references
- **T_FuncExpr** = 15 - Function expressions
- **T_NamedArgExpr** = 16 - Named argument expressions
- **T_OpExpr** = 17 - Operator expressions
- **T_DistinctExpr** = 18 - IS DISTINCT FROM expressions
- **T_NullIfExpr** = 19 - NULLIF expressions
- **T_ScalarArrayOpExpr** = 20 - Scalar array operator expressions
- **T_BoolExpr** = 21 - Boolean expressions (AND/OR/NOT)
- **T_SubLink** = 22 - Subquery links
- **T_SubPlan** = 23 - Subquery plans
- **T_AlternativeSubPlan** = 24 - Alternative subquery plans
- **T_FieldSelect** = 25 - Field selection from composite types
- **T_FieldStore** = 26 - Field assignment to composite types
- **T_RelabelType** = 27 - Type relabeling
- **T_CoerceViaIO** = 28 - Type coercion via I/O
- **T_ArrayCoerceExpr** = 29 - Array type coercion
- **T_ConvertRowtypeExpr** = 30 - Row type conversion
- **T_CollateExpr** = 31 - COLLATE expressions
- **T_CaseExpr** = 32 - CASE expressions
- **T_CaseWhen** = 33 - WHEN clauses in CASE
- **T_CaseTestExpr** = 34 - CASE test expressions
- **T_ArrayExpr** = 35 - Array construction expressions
- **T_RowExpr** = 36 - Row construction expressions
- **T_RowCompareExpr** = 37 - Row comparison expressions
- **T_CoalesceExpr** = 38 - COALESCE expressions
- **T_MinMaxExpr** = 39 - MIN/MAX expressions
- **T_SQLValueFunction** = 40 - SQL value functions (CURRENT_TIME, etc.)
- **T_XmlExpr** = 41 - XML expressions
- **T_NullTest** = 52 - IS NULL/IS NOT NULL tests
- **T_BooleanTest** = 53 - Boolean tests (IS TRUE/FALSE/UNKNOWN)
- **T_CoerceToDomain** = 55 - Domain type coercion
- **T_CoerceToDomainValue** = 56 - Domain value coercion
- **T_SetToDefault** = 57 - SET DEFAULT expressions
- **T_CurrentOfExpr** = 58 - CURRENT OF cursor expressions
- **T_NextValueExpr** = 59 - NEXTVAL expressions

### JSON Support Nodes
- **T_JsonFormat** = 42 - JSON format specifications
- **T_JsonReturning** = 43 - JSON RETURNING clauses
- **T_JsonValueExpr** = 44 - JSON value expressions
- **T_JsonConstructorExpr** = 45 - JSON constructor expressions
- **T_JsonIsPredicate** = 46 - JSON IS predicates
- **T_JsonBehavior** = 47 - JSON behavior specifications
- **T_JsonExpr** = 48 - JSON expressions
- **T_JsonTablePath** = 49 - JSON table path specifications
- **T_JsonTablePathScan** = 50 - JSON table path scans
- **T_JsonTableSiblingJoin** = 51 - JSON table sibling joins
- **T_JsonOutput** = 118 - JSON output specifications
- **T_JsonArgument** = 119 - JSON arguments
- **T_JsonFuncExpr** = 120 - JSON function expressions
- **T_JsonTablePathSpec** = 121 - JSON table path specifications
- **T_JsonTable** = 122 - JSON table expressions
- **T_JsonTableColumn** = 123 - JSON table columns
- **T_JsonKeyValue** = 124 - JSON key-value pairs
- **T_JsonParseExpr** = 125 - JSON parse expressions
- **T_JsonScalarExpr** = 126 - JSON scalar expressions
- **T_JsonSerializeExpr** = 127 - JSON serialize expressions
- **T_JsonObjectConstructor** = 128 - JSON object constructors
- **T_JsonArrayConstructor** = 129 - JSON array constructors
- **T_JsonArrayQueryConstructor** = 130 - JSON array query constructors
- **T_JsonAggConstructor** = 131 - JSON aggregate constructors
- **T_JsonObjectAgg** = 132 - JSON object aggregates
- **T_JsonArrayAgg** = 133 - JSON array aggregates

## Query Structure Nodes (T_60-T_133)

### Target and Reference Nodes
- **T_InferenceElem** = 60 - Inference elements for UPSERT
- **T_TargetEntry** = 61 - Target list entries
- **T_RangeTblRef** = 62 - Range table references
- **T_JoinExpr** = 63 - Join expressions
- **T_FromExpr** = 64 - FROM clause expressions
- **T_OnConflictExpr** = 65 - ON CONFLICT expressions
- **T_Query** = 66 - Query nodes

### Parse Tree Nodes
- **T_TypeName** = 67 - Type names
- **T_ColumnRef** = 68 - Column references
- **T_ParamRef** = 69 - Parameter references
- **T_A_Expr** = 70 - Raw parse tree expressions
- **T_A_Const** = 71 - Raw parse tree constants
- **T_TypeCast** = 72 - Type cast expressions
- **T_CollateClause** = 73 - COLLATE clauses
- **T_RoleSpec** = 74 - Role specifications
- **T_FuncCall** = 75 - Function calls
- **T_A_Star** = 76 - SELECT * expressions
- **T_A_Indices** = 77 - Array indices
- **T_A_Indirection** = 78 - Indirection (field access)
- **T_A_ArrayExpr** = 79 - Array expressions
- **T_ResTarget** = 80 - Result targets
- **T_MultiAssignRef** = 81 - Multi-assignment references
- **T_SortBy** = 82 - Sort specifications
- **T_WindowDef** = 83 - Window definitions

### Range and Table Nodes
- **T_RangeSubselect** = 84 - Subselect in FROM clause
- **T_RangeFunction** = 85 - Function in FROM clause
- **T_RangeTableFunc** = 86 - Table function in FROM clause
- **T_RangeTableFuncCol** = 87 - Table function columns
- **T_RangeTableSample** = 88 - TABLESAMPLE clauses
- **T_ColumnDef** = 89 - Column definitions
- **T_TableLikeClause** = 90 - LIKE table clauses
- **T_IndexElem** = 91 - Index elements
- **T_DefElem** = 92 - Definition elements
- **T_LockingClause** = 93 - FOR UPDATE/SHARE clauses
- **T_XmlSerialize** = 94 - XML serialization

### Partitioning Nodes
- **T_PartitionElem** = 95 - Partition elements
- **T_PartitionSpec** = 96 - Partition specifications
- **T_PartitionBoundSpec** = 97 - Partition bound specifications
- **T_PartitionRangeDatum** = 98 - Partition range datums
- **T_SinglePartitionSpec** = 99 - Single partition specifications
- **T_PartitionCmd** = 100 - Partition commands

### Query Planning and Execution Support
- **T_RangeTblEntry** = 101 - Range table entries
- **T_RTEPermissionInfo** = 102 - RTE permission information
- **T_RangeTblFunction** = 103 - Range table functions
- **T_TableSampleClause** = 104 - Table sample clauses
- **T_WithCheckOption** = 105 - WITH CHECK OPTION
- **T_SortGroupClause** = 106 - Sort/group clauses
- **T_GroupingSet** = 107 - Grouping sets
- **T_WindowClause** = 108 - Window clauses
- **T_RowMarkClause** = 109 - Row marking clauses
- **T_WithClause** = 110 - WITH clauses (CTEs)
- **T_InferClause** = 111 - UPSERT inference clauses
- **T_OnConflictClause** = 112 - ON CONFLICT clauses
- **T_CTESearchClause** = 113 - CTE SEARCH clauses
- **T_CTECycleClause** = 114 - CTE CYCLE clauses
- **T_CommonTableExpr** = 115 - Common table expressions
- **T_MergeWhenClause** = 116 - MERGE WHEN clauses
- **T_TriggerTransition** = 117 - Trigger transition tables

## Statement Nodes (T_134-T_262)

### Core DML Statements
- **T_RawStmt** = 134 - Raw statements
- **T_InsertStmt** = 135 - INSERT statements
- **T_DeleteStmt** = 136 - DELETE statements
- **T_UpdateStmt** = 137 - UPDATE statements
- **T_MergeStmt** = 138 - MERGE statements
- **T_SelectStmt** = 139 - SELECT statements
- **T_SetOperationStmt** = 140 - Set operations (UNION, INTERSECT, EXCEPT)
- **T_ReturnStmt** = 141 - RETURN statements
- **T_PLAssignStmt** = 142 - PL/pgSQL assignment statements

### DDL Schema Statements
- **T_CreateSchemaStmt** = 143 - CREATE SCHEMA
- **T_AlterTableStmt** = 144 - ALTER TABLE
- **T_ReplicaIdentityStmt** = 145 - ALTER TABLE REPLICA IDENTITY
- **T_AlterTableCmd** = 146 - ALTER TABLE commands
- **T_AlterCollationStmt** = 147 - ALTER COLLATION
- **T_AlterDomainStmt** = 148 - ALTER DOMAIN

### Security and Permission Statements
- **T_GrantStmt** = 149 - GRANT statements
- **T_ObjectWithArgs** = 150 - Objects with arguments
- **T_AccessPriv** = 151 - Access privileges
- **T_GrantRoleStmt** = 152 - GRANT role statements
- **T_AlterDefaultPrivilegesStmt** = 153 - ALTER DEFAULT PRIVILEGES

### Utility Statements
- **T_CopyStmt** = 154 - COPY statements
- **T_VariableSetStmt** = 155 - SET variable statements
- **T_VariableShowStmt** = 156 - SHOW variable statements
- **T_CreateStmt** = 157 - CREATE TABLE statements
- **T_Constraint** = 158 - Constraint definitions

### Tablespace Statements
- **T_CreateTableSpaceStmt** = 159 - CREATE TABLESPACE
- **T_DropTableSpaceStmt** = 160 - DROP TABLESPACE
- **T_AlterTableSpaceOptionsStmt** = 161 - ALTER TABLESPACE options
- **T_AlterTableMoveAllStmt** = 162 - ALTER TABLESPACE MOVE ALL

### Extension Statements
- **T_CreateExtensionStmt** = 163 - CREATE EXTENSION
- **T_AlterExtensionStmt** = 164 - ALTER EXTENSION
- **T_AlterExtensionContentsStmt** = 165 - ALTER EXTENSION contents

### Foreign Data Wrapper Statements
- **T_CreateFdwStmt** = 166 - CREATE FOREIGN DATA WRAPPER
- **T_AlterFdwStmt** = 167 - ALTER FOREIGN DATA WRAPPER
- **T_CreateForeignServerStmt** = 168 - CREATE SERVER
- **T_AlterForeignServerStmt** = 169 - ALTER SERVER
- **T_CreateForeignTableStmt** = 170 - CREATE FOREIGN TABLE
- **T_CreateUserMappingStmt** = 171 - CREATE USER MAPPING
- **T_AlterUserMappingStmt** = 172 - ALTER USER MAPPING
- **T_DropUserMappingStmt** = 173 - DROP USER MAPPING
- **T_ImportForeignSchemaStmt** = 174 - IMPORT FOREIGN SCHEMA

### Security Policy Statements
- **T_CreatePolicyStmt** = 175 - CREATE POLICY
- **T_AlterPolicyStmt** = 176 - ALTER POLICY
- **T_CreateAmStmt** = 177 - CREATE ACCESS METHOD

### Trigger Statements
- **T_CreateTrigStmt** = 178 - CREATE TRIGGER
- **T_CreateEventTrigStmt** = 179 - CREATE EVENT TRIGGER
- **T_AlterEventTrigStmt** = 180 - ALTER EVENT TRIGGER

### Language and Role Statements
- **T_CreatePLangStmt** = 181 - CREATE LANGUAGE
- **T_CreateRoleStmt** = 182 - CREATE ROLE/USER
- **T_AlterRoleStmt** = 183 - ALTER ROLE/USER
- **T_AlterRoleSetStmt** = 184 - ALTER ROLE SET
- **T_DropRoleStmt** = 185 - DROP ROLE/USER

### Sequence Statements
- **T_CreateSeqStmt** = 186 - CREATE SEQUENCE
- **T_AlterSeqStmt** = 187 - ALTER SEQUENCE

### Object Definition Statements
- **T_DefineStmt** = 188 - Generic DEFINE statements
- **T_CreateDomainStmt** = 189 - CREATE DOMAIN
- **T_CreateOpClassStmt** = 190 - CREATE OPERATOR CLASS
- **T_CreateOpClassItem** = 191 - Operator class items
- **T_CreateOpFamilyStmt** = 192 - CREATE OPERATOR FAMILY
- **T_AlterOpFamilyStmt** = 193 - ALTER OPERATOR FAMILY
- **T_DropStmt** = 194 - DROP statements
- **T_TruncateStmt** = 195 - TRUNCATE statements

### Documentation and Security Labels
- **T_CommentStmt** = 196 - COMMENT statements
- **T_SecLabelStmt** = 197 - SECURITY LABEL statements

### Cursor Statements
- **T_DeclareCursorStmt** = 198 - DECLARE cursor
- **T_ClosePortalStmt** = 199 - CLOSE cursor
- **T_FetchStmt** = 200 - FETCH statements

### Index and Statistics Statements
- **T_IndexStmt** = 201 - CREATE INDEX
- **T_CreateStatsStmt** = 202 - CREATE STATISTICS
- **T_StatsElem** = 203 - Statistics elements
- **T_AlterStatsStmt** = 204 - ALTER STATISTICS

### Function and Procedure Statements
- **T_CreateFunctionStmt** = 205 - CREATE FUNCTION/PROCEDURE
- **T_FunctionParameter** = 206 - Function parameters
- **T_AlterFunctionStmt** = 207 - ALTER FUNCTION/PROCEDURE
- **T_DoStmt** = 208 - DO statements
- **T_InlineCodeBlock** = 209 - Inline code blocks
- **T_CallStmt** = 210 - CALL statements
- **T_CallContext** = 211 - Call contexts

### Object Management Statements
- **T_RenameStmt** = 212 - RENAME statements
- **T_AlterObjectDependsStmt** = 213 - ALTER object DEPENDS
- **T_AlterObjectSchemaStmt** = 214 - ALTER object SET SCHEMA
- **T_AlterOwnerStmt** = 215 - ALTER object OWNER
- **T_AlterOperatorStmt** = 216 - ALTER OPERATOR
- **T_AlterTypeStmt** = 217 - ALTER TYPE

### Rule and Notification Statements
- **T_RuleStmt** = 218 - CREATE RULE
- **T_NotifyStmt** = 219 - NOTIFY statements
- **T_ListenStmt** = 220 - LISTEN statements
- **T_UnlistenStmt** = 221 - UNLISTEN statements

### Transaction Statements
- **T_TransactionStmt** = 222 - Transaction control statements

### Type Definition Statements
- **T_CompositeTypeStmt** = 223 - CREATE TYPE (composite)
- **T_CreateEnumStmt** = 224 - CREATE TYPE (enum)
- **T_CreateRangeStmt** = 225 - CREATE TYPE (range)
- **T_AlterEnumStmt** = 226 - ALTER TYPE (enum)

### View Statements
- **T_ViewStmt** = 227 - CREATE VIEW
- **T_LoadStmt** = 228 - LOAD statements

### Database Statements
- **T_CreatedbStmt** = 229 - CREATE DATABASE
- **T_AlterDatabaseStmt** = 230 - ALTER DATABASE
- **T_AlterDatabaseRefreshCollStmt** = 231 - ALTER DATABASE REFRESH COLLATION
- **T_AlterDatabaseSetStmt** = 232 - ALTER DATABASE SET
- **T_DropdbStmt** = 233 - DROP DATABASE

### System Maintenance Statements
- **T_AlterSystemStmt** = 234 - ALTER SYSTEM
- **T_ClusterStmt** = 235 - CLUSTER statements
- **T_VacuumStmt** = 236 - VACUUM statements
- **T_VacuumRelation** = 237 - VACUUM relation specifications
- **T_ExplainStmt** = 238 - EXPLAIN statements
- **T_CreateTableAsStmt** = 239 - CREATE TABLE AS
- **T_RefreshMatViewStmt** = 240 - REFRESH MATERIALIZED VIEW

### System Control Statements
- **T_CheckPointStmt** = 241 - CHECKPOINT statements
- **T_DiscardStmt** = 242 - DISCARD statements
- **T_LockStmt** = 243 - LOCK statements
- **T_ConstraintsSetStmt** = 244 - SET CONSTRAINTS
- **T_ReindexStmt** = 245 - REINDEX statements

### Conversion and Cast Statements
- **T_CreateConversionStmt** = 246 - CREATE CONVERSION
- **T_CreateCastStmt** = 247 - CREATE CAST
- **T_CreateTransformStmt** = 248 - CREATE TRANSFORM

### Prepared Statement Operations
- **T_PrepareStmt** = 249 - PREPARE statements
- **T_ExecuteStmt** = 250 - EXECUTE statements
- **T_DeallocateStmt** = 251 - DEALLOCATE statements

### Ownership Statements
- **T_DropOwnedStmt** = 252 - DROP OWNED
- **T_ReassignOwnedStmt** = 253 - REASSIGN OWNED

### Text Search Statements
- **T_AlterTSDictionaryStmt** = 254 - ALTER TEXT SEARCH DICTIONARY
- **T_AlterTSConfigurationStmt** = 255 - ALTER TEXT SEARCH CONFIGURATION

### Publication/Subscription Statements
- **T_PublicationTable** = 256 - Publication table specifications
- **T_PublicationObjSpec** = 257 - Publication object specifications
- **T_CreatePublicationStmt** = 258 - CREATE PUBLICATION
- **T_AlterPublicationStmt** = 259 - ALTER PUBLICATION
- **T_CreateSubscriptionStmt** = 260 - CREATE SUBSCRIPTION
- **T_AlterSubscriptionStmt** = 261 - ALTER SUBSCRIPTION
- **T_DropSubscriptionStmt** = 262 - DROP SUBSCRIPTION

## Planner and Executor Nodes (T_263-T_443)

### Planner Information Nodes
- **T_PlannerGlobal** = 263 - Global planner information
- **T_PlannerInfo** = 264 - Per-query planner information
- **T_RelOptInfo** = 265 - Relation optimization information
- **T_IndexOptInfo** = 266 - Index optimization information
- **T_ForeignKeyOptInfo** = 267 - Foreign key optimization information
- **T_StatisticExtInfo** = 268 - Extended statistics information
- **T_JoinDomain** = 269 - Join domain information
- **T_EquivalenceClass** = 270 - Equivalence class information
- **T_EquivalenceMember** = 271 - Equivalence class members
- **T_PathKey** = 272 - Path key information
- **T_GroupByOrdering** = 273 - GROUP BY ordering information
- **T_PathTarget** = 274 - Path target information
- **T_ParamPathInfo** = 275 - Parameterized path information

### Path Nodes
- **T_Path** = 276 - Generic path node
- **T_IndexPath** = 277 - Index scan paths
- **T_IndexClause** = 278 - Index clauses
- **T_BitmapHeapPath** = 279 - Bitmap heap scan paths
- **T_BitmapAndPath** = 280 - Bitmap AND paths
- **T_BitmapOrPath** = 281 - Bitmap OR paths
- **T_TidPath** = 282 - TID scan paths
- **T_TidRangePath** = 283 - TID range scan paths
- **T_SubqueryScanPath** = 284 - Subquery scan paths
- **T_ForeignPath** = 285 - Foreign scan paths
- **T_CustomPath** = 286 - Custom scan paths
- **T_AppendPath** = 287 - Append paths
- **T_MergeAppendPath** = 288 - Merge append paths
- **T_GroupResultPath** = 289 - Group result paths
- **T_MaterialPath** = 290 - Materialization paths
- **T_MemoizePath** = 291 - Memoize paths
- **T_UniquePath** = 292 - Unique paths
- **T_GatherPath** = 293 - Gather paths (parallel)
- **T_GatherMergePath** = 294 - Gather merge paths
- **T_NestPath** = 295 - Nested loop paths
- **T_MergePath** = 296 - Merge join paths
- **T_HashPath** = 297 - Hash join paths
- **T_ProjectionPath** = 298 - Projection paths
- **T_ProjectSetPath** = 299 - Project set paths
- **T_SortPath** = 300 - Sort paths
- **T_IncrementalSortPath** = 301 - Incremental sort paths
- **T_GroupPath** = 302 - Group paths
- **T_UpperUniquePath** = 303 - Upper unique paths
- **T_AggPath** = 304 - Aggregate paths
- **T_GroupingSetData** = 305 - Grouping set data
- **T_RollupData** = 306 - Rollup data
- **T_GroupingSetsPath** = 307 - Grouping sets paths
- **T_MinMaxAggPath** = 308 - Min/Max aggregate paths
- **T_WindowAggPath** = 309 - Window aggregate paths
- **T_SetOpPath** = 310 - Set operation paths
- **T_RecursiveUnionPath** = 311 - Recursive union paths
- **T_LockRowsPath** = 312 - Lock rows paths
- **T_ModifyTablePath** = 313 - Modify table paths
- **T_LimitPath** = 314 - Limit paths

### Planner Support Nodes
- **T_RestrictInfo** = 315 - Restriction information
- **T_PlaceHolderVar** = 316 - Placeholder variables
- **T_SpecialJoinInfo** = 317 - Special join information
- **T_OuterJoinClauseInfo** = 318 - Outer join clause information
- **T_AppendRelInfo** = 319 - Append relation information
- **T_RowIdentityVarInfo** = 320 - Row identity variable information
- **T_PlaceHolderInfo** = 321 - Placeholder information
- **T_MinMaxAggInfo** = 322 - Min/Max aggregate information
- **T_PlannerParamItem** = 323 - Planner parameter items
- **T_AggInfo** = 324 - Aggregate information
- **T_AggTransInfo** = 325 - Aggregate transition information

### Plan Nodes
- **T_PlannedStmt** = 326 - Planned statements
- **T_Result** = 327 - Result plan nodes
- **T_ProjectSet** = 328 - Project set plan nodes
- **T_ModifyTable** = 329 - Modify table plan nodes
- **T_Append** = 330 - Append plan nodes
- **T_MergeAppend** = 331 - Merge append plan nodes
- **T_RecursiveUnion** = 332 - Recursive union plan nodes
- **T_BitmapAnd** = 333 - Bitmap AND plan nodes
- **T_BitmapOr** = 334 - Bitmap OR plan nodes

### Scan Plan Nodes
- **T_SeqScan** = 335 - Sequential scan
- **T_SampleScan** = 336 - Sample scan
- **T_IndexScan** = 337 - Index scan
- **T_IndexOnlyScan** = 338 - Index-only scan
- **T_BitmapIndexScan** = 339 - Bitmap index scan
- **T_BitmapHeapScan** = 340 - Bitmap heap scan
- **T_TidScan** = 341 - TID scan
- **T_TidRangeScan** = 342 - TID range scan
- **T_SubqueryScan** = 343 - Subquery scan
- **T_FunctionScan** = 344 - Function scan
- **T_ValuesScan** = 345 - VALUES scan
- **T_TableFuncScan** = 346 - Table function scan
- **T_CteScan** = 347 - CTE scan
- **T_NamedTuplestoreScan** = 348 - Named tuplestore scan
- **T_WorkTableScan** = 349 - Work table scan
- **T_ForeignScan** = 350 - Foreign table scan
- **T_CustomScan** = 351 - Custom scan

### Join Plan Nodes
- **T_NestLoop** = 352 - Nested loop join
- **T_NestLoopParam** = 353 - Nested loop parameters
- **T_MergeJoin** = 354 - Merge join
- **T_HashJoin** = 355 - Hash join

### Other Plan Nodes
- **T_Material** = 356 - Materialization
- **T_Memoize** = 357 - Memoize
- **T_Sort** = 358 - Sort
- **T_IncrementalSort** = 359 - Incremental sort
- **T_Group** = 360 - Group
- **T_Agg** = 361 - Aggregate
- **T_WindowAgg** = 362 - Window aggregate
- **T_Unique** = 363 - Unique
- **T_Gather** = 364 - Gather (parallel)
- **T_GatherMerge** = 365 - Gather merge
- **T_Hash** = 366 - Hash
- **T_SetOp** = 367 - Set operation
- **T_LockRows** = 368 - Lock rows
- **T_Limit** = 369 - Limit

### Execution Support Nodes
- **T_PlanRowMark** = 370 - Plan row marking
- **T_PartitionPruneInfo** = 371 - Partition pruning information
- **T_PartitionedRelPruneInfo** = 372 - Partitioned relation pruning
- **T_PartitionPruneStepOp** = 373 - Partition pruning step operations
- **T_PartitionPruneStepCombine** = 374 - Partition pruning step combinations
- **T_PlanInvalItem** = 375 - Plan invalidation items

### Executor State Nodes
- **T_ExprState** = 376 - Expression state
- **T_IndexInfo** = 377 - Index information
- **T_ExprContext** = 378 - Expression context
- **T_ReturnSetInfo** = 379 - Return set information
- **T_ProjectionInfo** = 380 - Projection information
- **T_JunkFilter** = 381 - Junk filter
- **T_OnConflictSetState** = 382 - ON CONFLICT SET state
- **T_MergeActionState** = 383 - MERGE action state
- **T_ResultRelInfo** = 384 - Result relation information
- **T_EState** = 385 - Executor state
- **T_WindowFuncExprState** = 386 - Window function expression state
- **T_SetExprState** = 387 - Set expression state
- **T_SubPlanState** = 388 - Subplan state
- **T_DomainConstraintState** = 389 - Domain constraint state

### Plan State Nodes
- **T_ResultState** = 390 - Result state
- **T_ProjectSetState** = 391 - Project set state
- **T_ModifyTableState** = 392 - Modify table state
- **T_AppendState** = 393 - Append state
- **T_MergeAppendState** = 394 - Merge append state
- **T_RecursiveUnionState** = 395 - Recursive union state
- **T_BitmapAndState** = 396 - Bitmap AND state
- **T_BitmapOrState** = 397 - Bitmap OR state

### Scan State Nodes
- **T_ScanState** = 398 - Base scan state
- **T_SeqScanState** = 399 - Sequential scan state
- **T_SampleScanState** = 400 - Sample scan state
- **T_IndexScanState** = 401 - Index scan state
- **T_IndexOnlyScanState** = 402 - Index-only scan state
- **T_BitmapIndexScanState** = 403 - Bitmap index scan state
- **T_BitmapHeapScanState** = 404 - Bitmap heap scan state
- **T_TidScanState** = 405 - TID scan state
- **T_TidRangeScanState** = 406 - TID range scan state
- **T_SubqueryScanState** = 407 - Subquery scan state
- **T_FunctionScanState** = 408 - Function scan state
- **T_ValuesScanState** = 409 - VALUES scan state
- **T_TableFuncScanState** = 410 - Table function scan state
- **T_CteScanState** = 411 - CTE scan state
- **T_NamedTuplestoreScanState** = 412 - Named tuplestore scan state
- **T_WorkTableScanState** = 413 - Work table scan state
- **T_ForeignScanState** = 414 - Foreign scan state
- **T_CustomScanState** = 415 - Custom scan state

### Join State Nodes
- **T_JoinState** = 416 - Base join state
- **T_NestLoopState** = 417 - Nested loop state
- **T_MergeJoinState** = 418 - Merge join state
- **T_HashJoinState** = 419 - Hash join state

### Other State Nodes
- **T_MaterialState** = 420 - Material state
- **T_MemoizeState** = 421 - Memoize state
- **T_SortState** = 422 - Sort state
- **T_IncrementalSortState** = 423 - Incremental sort state
- **T_GroupState** = 424 - Group state
- **T_AggState** = 425 - Aggregate state
- **T_WindowAggState** = 426 - Window aggregate state
- **T_UniqueState** = 427 - Unique state
- **T_GatherState** = 428 - Gather state
- **T_GatherMergeState** = 429 - Gather merge state
- **T_HashState** = 430 - Hash state
- **T_SetOpState** = 431 - Set operation state
- **T_LockRowsState** = 432 - Lock rows state
- **T_LimitState** = 433 - Limit state

### Access Method Routines
- **T_IndexAmRoutine** = 434 - Index access method routines
- **T_TableAmRoutine** = 435 - Table access method routines
- **T_TsmRoutine** = 436 - Table sampling method routines

### Trigger and Event Data
- **T_EventTriggerData** = 437 - Event trigger data
- **T_TriggerData** = 438 - Trigger data
- **T_TupleTableSlot** = 439 - Tuple table slots
- **T_FdwRoutine** = 440 - Foreign data wrapper routines

### Utility Data Structures
- **T_Bitmapset** = 441 - Bitmap sets
- **T_ExtensibleNode** = 442 - Extensible nodes
- **T_ErrorSaveContext** = 443 - Error save contexts

## Replication Nodes (T_444-T_459)

### Replication Commands
- **T_IdentifySystemCmd** = 444 - IDENTIFY_SYSTEM replication command
- **T_BaseBackupCmd** = 445 - BASE_BACKUP replication command
- **T_CreateReplicationSlotCmd** = 446 - CREATE_REPLICATION_SLOT command
- **T_DropReplicationSlotCmd** = 447 - DROP_REPLICATION_SLOT command
- **T_AlterReplicationSlotCmd** = 448 - ALTER_REPLICATION_SLOT command
- **T_StartReplicationCmd** = 449 - START_REPLICATION command
- **T_ReadReplicationSlotCmd** = 450 - READ_REPLICATION_SLOT command
- **T_TimeLineHistoryCmd** = 451 - TIMELINE_HISTORY command
- **T_UploadManifestCmd** = 452 - UPLOAD_MANIFEST command

### Support Request Nodes
- **T_SupportRequestSimplify** = 453 - Simplification support requests
- **T_SupportRequestSelectivity** = 454 - Selectivity support requests
- **T_SupportRequestCost** = 455 - Cost support requests
- **T_SupportRequestRows** = 456 - Row count support requests
- **T_SupportRequestIndexCondition** = 457 - Index condition support requests
- **T_SupportRequestWFuncMonotonic** = 458 - Window function monotonic support
- **T_SupportRequestOptimizeWindowClause** = 459 - Window clause optimization support

## Additional Data Structure Nodes (T_465-T_474)

### Cache and List Nodes
- **T_ForeignKeyCacheInfo** = 465 - Foreign key cache information
- **T_IntList** = 466 - Integer lists
- **T_OidList** = 467 - OID lists
- **T_XidList** = 468 - Transaction ID lists

### Memory Context Nodes
- **T_AllocSetContext** = 469 - AllocSet memory contexts
- **T_GenerationContext** = 470 - Generation memory contexts
- **T_SlabContext** = 471 - Slab memory contexts
- **T_BumpContext** = 472 - Bump memory contexts

### Specialized Data Structures
- **T_TIDBitmap** = 473 - TID bitmaps
- **T_WindowObjectData** = 474 - Window object data

## Exclusions from Anonymization

### Objects Not Sent to Call Home (Skipped in Anonymizer)
The following PostgreSQL objects are not sent to call home and therefore not passed to the anonymizer:

#### **Function and Procedure Nodes (Intentionally Skipped)**
- **T_CreateFunctionStmt** = 205 - CREATE FUNCTION/PROCEDURE
- **T_FunctionParameter** = 206 - Function parameters  
- **T_AlterFunctionStmt** = 207 - ALTER FUNCTION/PROCEDURE
- **T_DoStmt** = 208 - DO statements
- **T_InlineCodeBlock** = 209 - Inline code blocks
- **T_CallStmt** = 210 - CALL statements
- **T_CallContext** = 211 - Call contexts

**Rationale**: Function and procedure definitions contain potentially sensitive business logic and are excluded from telemetry data collection.

#### **View and Materialized View Nodes (Intentionally Skipped)**
- **T_ViewStmt** = 227 - CREATE VIEW
- **T_RefreshMatViewStmt** = 240 - REFRESH MATERIALIZED VIEW

**Rationale**: View definitions may contain sensitive query logic and data access patterns, so they are excluded from call home data.

### Database-Level Nodes (Not Generated During Schema Export)
The following nodes are not generated during YB Voyager's schema export process:

#### **Database Management Statements**
- **T_CreatedbStmt** = 229 - CREATE DATABASE
- **T_AlterDatabaseStmt** = 230 - ALTER DATABASE  
- **T_AlterDatabaseRefreshCollStmt** = 231 - ALTER DATABASE REFRESH COLLATION
- **T_AlterDatabaseSetStmt** = 232 - ALTER DATABASE SET
- **T_DropdbStmt** = 233 - DROP DATABASE

**Rationale**: YB Voyager operates at the schema level within an existing database, so database-level DDL statements are not part of the exported schema.

## Summary

This analysis reveals a comprehensive taxonomy of PostgreSQL node types covering:

1. **Expression and Value Nodes** (T_1-T_100): Core data types, expressions, operations, and JSON support
2. **Query Structure Nodes** (T_60-T_133): Parse tree elements, query planning structures, and table references
3. **Statement Nodes** (T_134-T_262): All SQL statement types from DML to DDL to utility commands
4. **Planner and Executor Nodes** (T_263-T_443): Query planning, optimization, and execution infrastructure
5. **Replication Nodes** (T_444-T_459): Logical replication and streaming replication support
6. **Support Data Structures** (T_465-T_474): Memory management, caching, and utility data structures

### Anonymization Coverage Analysis

**Total Node Types**: 474 distinct PostgreSQL node types

**Excluded from Anonymization**:
- **Intentionally Skipped**: 9 nodes (functions, procedures, views) - not sent to call home
- **Not Generated**: 5 nodes (database-level statements) - not part of schema export
- **Total Excluded**: 14 nodes

**Requiring Anonymization Review**: 460 nodes across all remaining categories

## Missing Anonymization Coverage Analysis

Based on comprehensive testing of the SQL anonymizer, the following PostgreSQL node types are **missing anonymization support**:

### ✅ **Already Working** (7 node types):
- **T_DropOwnedStmt** - DROP OWNED statements (roles properly anonymized)
- **T_ReassignOwnedStmt** - REASSIGN OWNED statements (roles properly anonymized)  
- **T_LockStmt** - LOCK TABLE statements (tables/schemas properly anonymized)
- **T_ExplainStmt** - EXPLAIN statements (tables/schemas properly anonymized)
- **T_TransactionStmt** - Transaction control (no identifiers to anonymize)
- **T_DiscardStmt** - DISCARD statements (no identifiers to anonymize)
- **T_CheckPointStmt** - CHECKPOINT (no identifiers to anonymize)

### ❌ **Critical Missing Anonymization** (35+ node types):

#### **Trigger and Event Nodes**:
- **T_CreateTrigStmt** = 212 - CREATE TRIGGER (trigger names and function names not anonymized)
- **T_CreateEventTrigStmt** = 213 - CREATE EVENT TRIGGER (event trigger names and functions not anonymized)  
- **T_AlterEventTrigStmt** = 214 - ALTER EVENT TRIGGER (event trigger names not anonymized)

#### **Statistics and Maintenance Nodes**:
- **T_CreateStatsStmt** = 215 - CREATE STATISTICS (statistics names and column references not anonymized)
- **T_AlterStatsStmt** = 216 - ALTER STATISTICS (statistics names not anonymized)
- **T_VacuumStmt** = 217 - VACUUM (column names not anonymized)
- **T_ClusterStmt** = 218 - CLUSTER (index names not anonymized)
- **T_ReindexStmt** = 219 - REINDEX (index names not anonymized - wrong prefix used)

#### **Publication/Subscription Nodes**:
- **T_CreatePublicationStmt** = 220 - CREATE PUBLICATION (publication names not anonymized)
- **T_AlterPublicationStmt** = 221 - ALTER PUBLICATION (publication names not anonymized)
- **T_CreateSubscriptionStmt** = 222 - CREATE SUBSCRIPTION (subscription names not anonymized)
- **T_AlterSubscriptionStmt** = 223 - ALTER SUBSCRIPTION (subscription names not anonymized)
- **T_DropSubscriptionStmt** = 224 - DROP SUBSCRIPTION (subscription names not anonymized)

#### **Text Search Nodes**:
- **T_AlterTSDictionaryStmt** = 225 - ALTER TEXT SEARCH DICTIONARY (dictionary names not anonymized)
- **T_AlterTSConfigurationStmt** = 226 - ALTER TEXT SEARCH CONFIGURATION (configuration names not anonymized)

#### **System Object Nodes**:
- **T_CreatePLangStmt** = 227 - CREATE LANGUAGE (language names and handlers not anonymized)
- **T_CreateAmStmt** = 228 - CREATE ACCESS METHOD (access method names and handlers not anonymized)
- **T_CreateTransformStmt** = 229 - CREATE TRANSFORM (language names and functions not anonymized)
- **T_CreateCastStmt** = 230 - CREATE CAST (cast function names not anonymized)

#### **Cursor and Prepared Statement Nodes**:
- **T_DeclareCursorStmt** = 231 - DECLARE CURSOR (cursor names not anonymized)
- **T_ClosePortalStmt** = 232 - CLOSE (cursor names not anonymized)
- **T_FetchStmt** = 233 - FETCH (cursor names not anonymized)
- **T_PrepareStmt** = 234 - PREPARE (prepared statement names not anonymized)
- **T_ExecuteStmt** = 235 - EXECUTE (prepared statement names not anonymized)
- **T_DeallocateStmt** = 236 - DEALLOCATE (prepared statement names not anonymized)

#### **System Settings Nodes**:
- **T_VariableSetStmt** = 237 - SET (variable names not anonymized)
- **T_VariableShowStmt** = 238 - SHOW (variable names not anonymized)
- **T_AlterSystemStmt** = 239 - ALTER SYSTEM (parameter names not anonymized)
- **T_ConstraintsSetStmt** = 240 - SET CONSTRAINTS (constraint names not anonymized - wrong prefix used)

#### **Notification and Maintenance Nodes**:
- **T_NotifyStmt** = 241 - NOTIFY (channel names not anonymized)
- **T_ListenStmt** = 242 - LISTEN (channel names not anonymized)  
- **T_UnlistenStmt** = 243 - UNLISTEN (channel names not anonymized)
- **T_LoadStmt** = 244 - LOAD (extension names not anonymized)

#### **Tablespace and Security Nodes**:
- **T_CreateTableSpaceStmt** = 245 - CREATE TABLESPACE (tablespace names not anonymized)
- **T_DropTableSpaceStmt** = 246 - DROP TABLESPACE (tablespace names not anonymized)
- **T_AlterTableSpaceOptionsStmt** = 247 - ALTER TABLESPACE (tablespace names not anonymized)
- **T_SecLabelStmt** = 248 - SECURITY LABEL (provider names and object references not anonymized)

#### **Data Transfer Nodes**:
- **T_CopyStmt** = 249 - COPY (column names not anonymized)
- **T_CreateTableAsStmt** = 250 - CREATE TABLE AS (some column references missing anonymization)

### **Implementation Priority**:

**High Priority** (Critical for schema privacy):
1. Trigger and Event nodes (T_212-T_214) - Contains function names and trigger logic
2. Statistics nodes (T_215-T_216) - Contains column references and statistics names
3. Publication/Subscription nodes (T_220-T_224) - Contains table references and publication names
4. Copy statements (T_249) - Contains column names in data transfer operations

**Medium Priority** (Important for system object privacy):
5. System object nodes (T_227-T_230) - Contains language, access method, and function names
6. Cursor and Prepared statement nodes (T_231-T_236) - Contains statement names
7. System settings nodes (T_237-T_240) - Contains variable and parameter names

**Low Priority** (Less critical but should be addressed):
8. Text search nodes (T_225-T_226) - Contains dictionary and configuration names
9. Notification nodes (T_241-T_243) - Contains channel names
10. Tablespace nodes (T_245-T_247) - Contains tablespace names

### **Total Coverage Gap**: 
- **35+ node types** missing anonymization support
- **~7.4%** of all PostgreSQL nodes (35/474) need implementation
- **Significant privacy risk** for schema information in DDL statements

The node type system provides a complete representation of PostgreSQL's SQL parsing, planning, and execution infrastructure, supporting the full SQL standard plus PostgreSQL-specific extensions including JSON processing, partitioning, foreign data wrappers, logical replication, and parallel query execution. 