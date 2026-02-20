#!/usr/bin/env python3
"""
Voyager anonymized DDL exporter to Google Drive.

Reads new rows from BigQuery `callhome_data.voyager` (migration_phase = 'assess-migration'),
transforms the anonymized DDLs into executable SQL using `transform_anonymized_ddls`,
and uploads .sql files to Google Drive under per-UUID subfolders.

State management: stores the last processed timestamp in a BigQuery table
named `voyager_anonymised_ddls_gdrive_state` for easy cleanup.

Idempotency/deduplication:
- Within-run: SQL ROW_NUMBER over COALESCE(db_system_identifier, uuid) keeps the latest row per key.
- Across runs: BigQuery index `voyager_anonymised_ddls_index` maps db_key -> Drive file id; uploads update the same file and rename when UUID changes. Filenames always include UUID.

Environment variables:
  PROJECT_ID                          (default: 'yugabyte-growth')
  SOURCE_DATASET                      (default: 'callhome_data')
  SOURCE_TABLE                        (default: 'voyager')
  STATE_DATASET                       (default: SOURCE_DATASET)
  STATE_TABLE                         (default: 'voyager_anonymised_ddls_gdrive_state')
  INDEX_DATASET                       (default: STATE_DATASET)
  INDEX_TABLE                         (default: 'voyager_anonymised_ddls_index')
  DRIVE_FOLDER_ID                     (default: 'DRIVE_FOLDER_ID')
  BATCH_LIMIT                         (default: '1000')
  FIRST_RUN_LOOKBACK_DAYS             (default: '30')

Run locally:
  python3 voyager_ddls_export/main.py
"""

import os
import re
from datetime import datetime, timedelta, timezone
from typing import Any, Dict, List, Optional, Tuple

import pytz
from google.cloud import bigquery
from googleapiclient.discovery import build
from googleapiclient.http import MediaInMemoryUpload
import json
import tempfile
import subprocess


def get_bigquery_clients() -> Tuple[bigquery.Client, str, str, str, str]:
    project_id = os.environ.get("PROJECT_ID", "yugabyte-growth")
    source_dataset = os.environ.get("SOURCE_DATASET", "callhome_data")
    source_table = os.environ.get("SOURCE_TABLE", "voyager")
    state_dataset = os.environ.get("STATE_DATASET", source_dataset)
    state_table = os.environ.get("STATE_TABLE", "voyager_anonymised_ddls_gdrive_state")
    client = bigquery.Client(project=project_id)
    return client, source_dataset, source_table, state_dataset, state_table


def ensure_state_table(client: bigquery.Client, dataset: str, table: str) -> None:
    dataset_ref = bigquery.DatasetReference(client.project, dataset)
    table_ref = dataset_ref.table(table)
    try:
        client.get_table(table_ref)
        return
    except Exception:
        schema = [
            bigquery.SchemaField("id", "INT64", mode="REQUIRED"),
            bigquery.SchemaField("last_processed_at", "TIMESTAMP", mode="REQUIRED"),
        ]
        table_obj = bigquery.Table(table_ref, schema=schema)
        client.create_table(table_obj)


def ensure_index_table(client: bigquery.Client, dataset: str, table: str) -> None:
    """Ensure the cross-run Drive index table exists.

    Schema:
      - db_key STRING PRIMARY KEY (logical identity: db_system_identifier or uuid)
      - drive_file_id STRING (Google Drive file id)
      - current_uuid STRING (latest UUID associated with this key)
      - last_updated_at TIMESTAMP (when the Drive file was last updated)
    """
    dataset_ref = bigquery.DatasetReference(client.project, dataset)
    table_ref = dataset_ref.table(table)
    try:
        client.get_table(table_ref)
        return
    except Exception:
        schema = [
            bigquery.SchemaField("db_key", "STRING", mode="REQUIRED"),
            bigquery.SchemaField("drive_file_id", "STRING", mode="REQUIRED"),
            bigquery.SchemaField("current_uuid", "STRING", mode="REQUIRED"),
            bigquery.SchemaField("last_updated_at", "TIMESTAMP", mode="REQUIRED"),
        ]
        table_obj = bigquery.Table(table_ref, schema=schema)
        # Set a simple table id; primary key constraint is enforced logically via MERGE later
        client.create_table(table_obj)


def get_index_entry(
    client: bigquery.Client,
    dataset: str,
    table: str,
    db_key: str,
) -> Optional[Dict[str, Any]]:
    """Fetch an index entry by db_key. Returns None if not found.

    Output dict keys: db_key, drive_file_id, current_uuid, last_updated_at
    """
    sql = f"""
        SELECT db_key, drive_file_id, current_uuid, last_updated_at
        FROM `{client.project}.{dataset}.{table}`
        WHERE db_key = @db_key
        LIMIT 1
    """
    job_config = bigquery.QueryJobConfig(
        query_parameters=[bigquery.ScalarQueryParameter("db_key", "STRING", db_key)]
    )
    rows = list(client.query(sql, job_config=job_config).result())
    if not rows:
        return None
    row = rows[0]
    return {
        "db_key": row["db_key"],
        "drive_file_id": row["drive_file_id"],
        "current_uuid": row["current_uuid"],
        "last_updated_at": row["last_updated_at"],
    }


def upsert_index_entry(
    client: bigquery.Client,
    dataset: str,
    table: str,
    db_key: str,
    drive_file_id: str,
    current_uuid: str,
    ts: datetime,
) -> None:
    """Insert or update the index entry for db_key using MERGE."""
    sql = f"""
        MERGE `{client.project}.{dataset}.{table}` T
        USING (
          SELECT @db_key AS db_key,
                 @drive_file_id AS drive_file_id,
                 @current_uuid AS current_uuid,
                 TIMESTAMP(@ts) AS last_updated_at
        ) S
        ON T.db_key = S.db_key
        WHEN MATCHED THEN
          UPDATE SET drive_file_id = S.drive_file_id,
                     current_uuid = S.current_uuid,
                     last_updated_at = S.last_updated_at
        WHEN NOT MATCHED THEN
          INSERT (db_key, drive_file_id, current_uuid, last_updated_at)
          VALUES (S.db_key, S.drive_file_id, S.current_uuid, S.last_updated_at)
    """
    job_config = bigquery.QueryJobConfig(
        query_parameters=[
            bigquery.ScalarQueryParameter("db_key", "STRING", db_key),
            bigquery.ScalarQueryParameter("drive_file_id", "STRING", drive_file_id),
            bigquery.ScalarQueryParameter("current_uuid", "STRING", current_uuid),
            bigquery.ScalarQueryParameter("ts", "TIMESTAMP", ts),
        ]
    )
    client.query(sql, job_config=job_config).result()


def get_default_start_time() -> datetime:
    lookback_days = int(os.environ.get("FIRST_RUN_LOOKBACK_DAYS", "30"))
    return datetime.now(timezone.utc) - timedelta(days=lookback_days)


def get_last_processed_at(client: bigquery.Client, dataset: str, table: str) -> datetime:
    ensure_state_table(client, dataset, table)
    sql = f"""
        SELECT last_processed_at
        FROM `{client.project}.{dataset}.{table}`
        WHERE id = 1
        LIMIT 1
    """
    query_job = client.query(sql)
    rows = list(query_job.result())
    if not rows:
        return get_default_start_time()
    value = rows[0]["last_processed_at"]
    if value is None:
        return get_default_start_time()
    # Ensure timezone-aware UTC
    if value.tzinfo is None:
        value = value.replace(tzinfo=pytz.UTC)
    return value


def set_last_processed_at(client: bigquery.Client, dataset: str, table: str, ts: datetime) -> None:
    ensure_state_table(client, dataset, table)
    # Use MERGE to upsert a single-row state keyed by id=1
    sql = f"""
        MERGE `{client.project}.{dataset}.{table}` T
        USING (SELECT 1 AS id, TIMESTAMP(@ts) AS last_processed_at) S
        ON T.id = S.id
        WHEN MATCHED THEN
          UPDATE SET last_processed_at = S.last_processed_at
        WHEN NOT MATCHED THEN
          INSERT (id, last_processed_at) VALUES(S.id, S.last_processed_at)
    """
    job_config = bigquery.QueryJobConfig(
        query_parameters=[bigquery.ScalarQueryParameter("ts", "TIMESTAMP", ts)]
    )
    client.query(sql, job_config=job_config).result()


def fetch_voyager_rows(
    client: bigquery.Client,
    source_dataset: str,
    source_table: str,
    since: datetime,
    limit: int,
) -> List[bigquery.table.Row]:
    sql = f"""
        WITH filtered AS (
          SELECT
            uuid,
            start_time,
            last_updated_time,
            phase_payload,
            SAFE_CAST(SPLIT(yb_voyager_version, '.')[OFFSET(0)] AS INT64) AS version_year,
            SAFE_CAST(SPLIT(yb_voyager_version, '.')[SAFE_OFFSET(1)] AS INT64) AS version_month,
            SAFE_CAST(SPLIT(yb_voyager_version, '.')[SAFE_OFFSET(2)] AS INT64) AS version_release,
            JSON_VALUE(source_db_details, '$.db_system_identifier') AS db_system_identifier
          FROM `{client.project}.{source_dataset}.{source_table}`
          WHERE migration_phase = 'assess-migration'
            AND phase_payload IS NOT NULL
            AND status = 'COMPLETE'
            AND last_updated_time > @since
        ),
        ranked AS (
          SELECT
            *,
            COALESCE(db_system_identifier, uuid) AS db_key,
            ROW_NUMBER() OVER (
              PARTITION BY COALESCE(db_system_identifier, uuid)
              ORDER BY last_updated_time DESC
            ) AS rn
          FROM filtered
        )
        SELECT uuid, start_time, last_updated_time, phase_payload, db_system_identifier
        FROM ranked
        WHERE rn = 1
          AND (
            version_year > 2025 OR
            (version_year = 2025 AND version_month > 8) OR
            (version_year = 2025 AND version_month = 8 AND version_release >= 2)
          )
        ORDER BY last_updated_time ASC
        LIMIT @limit
    """
    job_config = bigquery.QueryJobConfig(
        query_parameters=[
            bigquery.ScalarQueryParameter("since", "TIMESTAMP", since),
            bigquery.ScalarQueryParameter("limit", "INT64", limit),
        ]
    )
    query_job = client.query(sql, job_config=job_config)
    return list(query_job.result())


## Removed: Python-side dedupe is redundant with SQL ROW_NUMBER rn=1


def _cleanup_temp_files(*file_paths):
    """Safely remove temporary files, ignoring errors."""
    for path in file_paths:
        try:
            if path and os.path.exists(path):
                os.remove(path)
        except Exception:
            pass


def build_sql_from_payload(phase_payload: Any) -> str:
    """Invoke the transformer script as a subprocess and return the generated SQL text."""
    # Prepare temp files
    input_path = None
    output_path = None
    try:
        # Write minimal payload expected by transformer
        input_fd, input_path = tempfile.mkstemp(suffix='.json')
        with os.fdopen(input_fd, 'w', encoding='utf-8') as f:
            json.dump({"phase_payload": phase_payload}, f)

        output_fd, output_path = tempfile.mkstemp(suffix='.sql')
        os.close(output_fd)

        script_path = os.path.join(os.path.dirname(__file__), 'transform_anonymized_ddls.py')
        cmd = [
            'python3',
            script_path,
            input_path,
            output_path,
        ]
        result = subprocess.run(cmd, capture_output=True, text=True)
        if result.returncode != 0:
            raise RuntimeError(f"transformer failed rc={result.returncode}, stderr={result.stderr}")

        with open(output_path, 'r', encoding='utf-8') as f:
            return f.read()
    finally:
        # Cleanup temp files
        _cleanup_temp_files(input_path, output_path)


def get_drive_service():
    # Auth is derived from the Cloud Run Job's service account
    return build('drive', 'v3')


def upload_sql_to_drive(drive, parent_id: str, file_name: str, content: str) -> str:
    file_metadata = {
        'name': file_name,
        'parents': [parent_id],
    }
    media = MediaInMemoryUpload(content.encode('utf-8'), mimetype='application/sql')
    created = drive.files().create(
        body=file_metadata,
        media_body=media,
        fields='id',
        supportsAllDrives=True,
    ).execute()
    print(f"Drive: uploaded file '{file_name}' (id={created['id']})")
    return created['id']


def update_or_create_drive_file(
    drive,
    parent_id: str,
    file_name: str,
    content: str,
    existing_file_id: Optional[str] = None,
) -> str:
    """Update file content (and name) if file exists; otherwise create a new file.

    Returns the Drive file id.
    """
    media = MediaInMemoryUpload(content.encode('utf-8'), mimetype='application/sql')
    if existing_file_id:
        updated = drive.files().update(
            fileId=existing_file_id,
            body={'name': file_name},
            media_body=media,
            fields='id',
            supportsAllDrives=True,
        ).execute()
        print(f"Drive: updated file '{file_name}' (id={updated['id']})")
        return updated['id']
    else:
        return upload_sql_to_drive(drive, parent_id, file_name, content)


def sanitize_filename_component(value: Any) -> str:
    s = str(value)
    s = s.replace(':', '_').replace(' ', '_').replace('/', '_')
    s = re.sub(r"[^A-Za-z0-9_\-\.]+", "_", s)
    return s


def run_export_once() -> None:
    client, source_dataset, source_table, state_dataset, state_table = get_bigquery_clients()
    drive_folder_id = os.environ.get('DRIVE_FOLDER_ID', 'DRIVE_FOLDER_ID')
    batch_limit = int(os.environ.get('BATCH_LIMIT', '1000'))

    # Ensure cross-run index table exists (used in later tasks for idempotent updates)
    index_dataset = os.environ.get('INDEX_DATASET', state_dataset)
    index_table = os.environ.get('INDEX_TABLE', 'voyager_anonymised_ddls_index')
    ensure_index_table(client, index_dataset, index_table)

    last_processed_at = get_last_processed_at(client, state_dataset, state_table)
    print(
        f"Start: project={client.project}, source={source_dataset}.{source_table}, "
        f"state={state_dataset}.{state_table}, since={last_processed_at.isoformat()}, "
        f"batch_limit={batch_limit}"
    )
    rows = fetch_voyager_rows(client, source_dataset, source_table, last_processed_at, batch_limit)
    print(f"Query: fetched {len(rows)} rows")
    if not rows:
        print("No new rows to process.")
        return

    # SQL already dedupes to one row per logical db_key (COALESCE(db_system_identifier, uuid))
    print(f"Dedupe: SQL applied; {len(rows)} rows remain (one per logical db)")

    drive = get_drive_service()

    max_seen_ts: Optional[datetime] = last_processed_at
    uploaded = 0
    updated = 0
    renamed = 0
    skipped_transform = 0
    upload_errors = 0
    for row in rows:
        uuid_val = row["uuid"]
        start_time = row["start_time"]
        last_updated_time = row["last_updated_time"]
        phase_payload = row["phase_payload"]
        db_id = row["db_system_identifier"] if "db_system_identifier" in row.keys() else None

        # Build SQL; if transform fails for this row, skip and continue.
        try:
            sql_text = build_sql_from_payload(phase_payload)
        except SystemExit as e:
            print(f"Skipping uuid={uuid_val}: transformer exited with error: {e}")
            skipped_transform += 1
            continue
        except Exception as e:
            print(f"Skipping uuid={uuid_val}: transformer error: {e}")
            skipped_transform += 1
            continue

        print(f"Process: uuid={uuid_val}, last_updated_time={last_updated_time}")
        parent_folder_id = drive_folder_id

        safe_start = sanitize_filename_component(start_time)
        file_name = f"anonymized_ddls_{uuid_val}_{safe_start}.sql"

        # Compute logical db key (db_system_identifier preferred, else uuid)
        db_key = db_id if db_id not in (None, "") else uuid_val

        # Look up existing Drive file id from index
        try:
            idx = get_index_entry(client, index_dataset, index_table, db_key)
            existing_file_id = idx["drive_file_id"] if idx else None
            existing_uuid = idx["current_uuid"] if idx else None
        except Exception as e:
            print(f"Index lookup error for db_key={db_key}: {e}")
            existing_file_id = None
            existing_uuid = None

        try:
            file_id = update_or_create_drive_file(
                drive,
                parent_folder_id,
                file_name,
                sql_text,
                existing_file_id=existing_file_id,
            )
            if existing_file_id:
                updated += 1
                was_renamed = existing_uuid != uuid_val
                if was_renamed:
                    renamed += 1
                print(
                    f"Drive: action=update, renamed={was_renamed}, db_key={db_key}, file_id={file_id}, name='{file_name}'"
                )
            else:
                uploaded += 1
                print(
                    f"Drive: action=create, db_key={db_key}, file_id={file_id}, name='{file_name}'"
                )
            # Upsert index entry with latest info
            upsert_index_entry(
                client,
                index_dataset,
                index_table,
                db_key,
                file_id,
                uuid_val,
                last_updated_time if isinstance(last_updated_time, datetime) else datetime.now(timezone.utc),
            )
        except Exception as e:
            print(f"Upload/update error for uuid={uuid_val}, file='{file_name}': {e}")
            upload_errors += 1
            # Continue to next row

        if max_seen_ts is None or last_updated_time > max_seen_ts:
            max_seen_ts = last_updated_time

    # Persist state for the next run
    if max_seen_ts is None:
        max_seen_ts = last_processed_at
    # Normalize to UTC aware
    if max_seen_ts.tzinfo is None:
        max_seen_ts = max_seen_ts.replace(tzinfo=pytz.UTC)
    set_last_processed_at(client, state_dataset, state_table, max_seen_ts)
    print(
        f"Done: uploaded={uploaded}, updated={updated}, renamed={renamed}, "
        f"skipped_transform={skipped_transform}, upload_errors={upload_errors}, "
        f"new_since={max_seen_ts.isoformat()}"
    )

def main() -> None:
    run_export_once()

if __name__ == "__main__":
    main()



