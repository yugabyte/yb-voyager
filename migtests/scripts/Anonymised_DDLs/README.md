# Voyager DDLs Export to Google Drive

Exports anonymized DDLs from BigQuery table `callhome_data.voyager` to Google Drive as runnable SQL files using `transform_anonymized_ddls.py`.

## How it works
- Reads new rows where `migration_phase = 'assess-migration'`, `phase_payload` present, and `status = 'COMPLETE'`.
- Filters to only process Voyager versions >= 2025.8.2.
- Deduplicates by `db_system_identifier` (keeps latest row per database, falls back to `uuid` if identifier missing).
- Transforms anonymized DDLs into executable SQL in dependency order.
- Uploads a file per row to Drive with filename format: `anonymized_ddls_{uuid}_{start_time}.sql`.
- Persists `last_processed_at` in BigQuery state table `voyager_anonymised_ddls_gdrive_state`.

## Configure
Env vars (defaults in parentheses):
- `PROJECT_ID` (yugabyte-growth)
- `SOURCE_DATASET` (callhome_data)
- `SOURCE_TABLE` (voyager)
- `STATE_DATASET` (defaults to `SOURCE_DATASET`)
- `STATE_TABLE` (voyager_anonymised_ddls_gdrive_state)
- `DRIVE_FOLDER_ID` (DRIVE_FOLDER_ID)
- `BATCH_LIMIT` (1000)
- `FIRST_RUN_LOOKBACK_DAYS` (30)

Grant the Cloud Run Job service account "Writer" on the destination Drive folder by sharing the folder with the SA email.

## Deploy (Cloud Run Job)
- Build an image containing this directory and repo root (so `transform_anonymized_ddls.py` is importable) or run as a source-based job.
- Configure env vars as needed.
- Schedule via Cloud Scheduler to trigger periodically.

## Local run
```bash
python3 main.py
```

## Notes
- State table name is obvious for easy deletion. Removing it resets the checkpoint to `now() - FIRST_RUN_LOOKBACK_DAYS`.
- Files are uploaded directly to the Drive folder with descriptive names; no subfolders are created.
- Only processes Voyager versions >= 2025.8.2 to ensure compatibility with newer features.
- Deduplication ensures only the latest assessment per database is processed, reducing redundant uploads.


