# Voyager DDLs Export to Google Drive

Exports anonymized DDLs from BigQuery table `callhome_data.voyager` to Google Drive as runnable SQL files using `transform_anonymized_ddls.py`.

## How it works
- Reads new rows where `migration_phase = 'analyze-schema'` and `phase_payload` present.
- Transforms anonymized DDLs into executable SQL in dependency order.
- Uploads a file per row to Drive under a subfolder named by `uuid`.
- Persists `last_processed_at` in BigQuery state table `DELETE_ME_voyager_ddls_state`.

## Configure
Env vars (defaults in parentheses):
- `PROJECT_ID` (yugabyte-growth)
- `SOURCE_DATASET` (callhome_data)
- `SOURCE_TABLE` (voyager)
- `STATE_DATASET` (defaults to `SOURCE_DATASET`)
- `STATE_TABLE` (DELETE_ME_voyager_ddls_state)
- `DRIVE_FOLDER_ID` (REPLACE_ME_DRIVE_FOLDER_ID)
- `BATCH_LIMIT` (1000)
- `FIRST_RUN_LOOKBACK_DAYS` (30)

Grant the Cloud Run Job service account "Writer" on the destination Drive folder by sharing the folder with the SA email.

## Deploy (Cloud Run Job)
- Build an image containing this directory and repo root (so `transform_anonymized_ddls.py` is importable) or run as a source-based job.
- Configure env vars as needed.
- Schedule via Cloud Scheduler to trigger periodically.

## Local run
```bash
python3 voyager_ddls_export/main.py
```

## Notes
- State table name is obvious for easy deletion. Removing it resets the checkpoint to `now() - FIRST_RUN_LOOKBACK_DAYS`.
- Files are uploaded directly to per-UUID subfolders; no extra date folders are created.


