# What's new in YugabyteDB Voyager

#### New features, key enhancements, and bug fixes

Included here are the release notes for the [YugabyteDB Voyager](https://docs.yugabyte.com/preview/migrate/) v1 release series. Content will be added as new notable features and changes are available in the patch releases of the YugabyteDB v1 series.

## v1.4 - June 30, 2023

#### Key enhancements:

- `import data file` can now import multiple files to a same table. Moreover, glob expression can be provided in the `--file-table-map` to specify multiple files to be imported in a same table.

- In addition to the AWS S3, `import data file` now supports directly importing objects (CSV/TEXT files) stored in GCS and Azure Blob Store. You can specify GCS and Azure Blob Store "directories" by prefixing them with `gs://` and `https://`.

- When using the "fast data export" mode, Voyager can now connect to the source databases using SSL.

- `analyze-schema` command now reports unsupported data types.

- `--file-opts` CLI argument is now deprecated. Use the newly introduced `--escape-char` and `--quote-char` options.

#### Bug fixes:

- If a CSV file had empty lines in it, `import data status` used to continue reporting the import status as `MIGRATING` even though the import is complete and successful. This issue is now fixed.

- Explicitly close the source/target database connections when voyager is exiting.

- The `import data file` command now uses tab (`\t`) instead of comma (`,`) as the default delimiter when importing TEXT formatted files.

#### Known issues:

- Compared to earlier releases, Voyager 1.4 uses different and incompatible structure to represent import data state. And hence, Voyager 1.4 cannot "continue" a data import operation that was started by Voyager 1.3 or lower.

## v1.3 - May 30, 2023

##### Key enhancements

- Export data for MySQL and Oracle is now 2-4x faster. You can use the env var `BETA_FAST_DATA_EXPORT=1` to leverage this performance improvement. Most features such as migrating partitioned tables, sequences, etc are supported in this mode. Please refer to the [documentation](https://docs.yugabyte.com/preview/migrate/migrate-steps/#export-data) for more details. Note that for PostgreSQL, the default mode (pg_dump) is faster, and hence, this flag is not recommended to be used.

- Added support for characters such as backspace(`\b`) in quote and escape character with `--file-opts` in import data file.

- Added ability to specify null value string in `import data file` command with a `--null-string` flag.

- During `export data`, `yb-voyager` can now explicitly inform you of any unsupported datatypes, and requests for permission to ignore them.

##### Bug fixes

- [[5976]](https://github.com/yugabyte/yugabyte-db/issues/16576), `yb-voyager` can now parse `CREATE TABLE` DDL having complex check constraints.

- `import data file` command with AWS S3 now works when yb-voyager is installed via Docker.

## v1.2 - April 3, 2023

##### Key enhancements

- When using the `import data file` command with the `--data-dir` option, you can provide an AWS S3 bucket as a path to the data directory.

- Added support for rotation of log files in a new logs directory found in `export-dir/logs`.

##### Known issues

- [[16658]](https://github.com/yugabyte/yugabyte-db/issues/16658) The `import data file` command may not recognise the data directory being provided, causing the step to fail for dockerized yb-voyager.

## v1.1 - March 7, 2023

##### Key enhancements

- When using the `import data file` command with CSV files, YB Voyager now supports any character as an escape character and a quote character in the `--file-opts` flag, such as single quote (`'`) as a `quote_char` and backslash (`\`) as an `escape_char`, and so on. Previously, YB Voyager only supported double quotes (`"`) as a quote character and an escape character.

- Creating the Orafce extension on the target database for Oracle migrations is now available by default.

- User creation for Oracle no longer requires `EXECUTE` permissions on `PROCEDURE`, `FUNCTION`, `PACKAGE`, and `PACKAGE BODY` objects.

- The precision and scale of numeric data types from Oracle are migrated to the target database.

- For PostgreSQL migrations, YB Voyager no longer uses a password in the `pg_dump` command running in the background. Instead, the password is internally set as an environment variable to be used by `pg_dump`.

- For any syntax error in the data file or CSV file, complete error details such as line number, column, and data are displayed in the output of the `import data` or `import data file` commands.

- For the `export data` command, the list of table names passed in the `--table-list` and `--exclude-table-list` are, by default, case insensitive. Enclose each name in double quotes to make it case-sensitive.

- In the 1.0 release, the schema details in the report generated via `analyze-schema` are sent with diagnostics when the `--send-diagnostics` flag is on. These schema details are now removed before sending diagnostics.

- The object types which YB Voyager can't categorize are placed in a separate file as `uncategorized.sql`, and the information regarding this file is available as a note under the Notes section in the report generated via `analyze-schema`.

##### Bug fixes

- [[765]](https://github.com/yugabyte/yb-voyager/issues/765) Fixed function parsing issue when the complete body of the function is in a single line.
- [[757]](https://github.com/yugabyte/yb-voyager/issues/757) Fixed the issue of migrating tables with names as reserved keywords in the target.
