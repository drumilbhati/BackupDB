# BackupDB

`BackupDB` is a powerful, production-grade, cross-platform command-line utility for backing up and restoring databases across multiple DBMS engines to local or cloud-based storage backends.

It supports PostgreSQL, MySQL, MongoDB, and SQLite, with seamless integrations for AWS S3, Google Cloud Storage (GCS), Azure Blob Storage, and the Local Filesystem.

For details on architecture and design decisions, see [IMPLEMENTATION.md](file:///Users/drumilbhati/Documents/BackupDB/IMPLEMENTATION.md).

---

## Key Features

- **Multi-DBMS Support**: Full backup and restore execution on PostgreSQL, MySQL, MongoDB, and SQLite.
- **Flexible Storage Adapters**: Direct stream uploads/downloads using official cloud SDKs (AWS, GCS, Azure) or local files.
- **On-the-Fly Compression**: Support for `gzip` and `zstd` (via high-performance `klauspost/compress/zstd`) stream compression.
- **Integrity Validation**: Computes and tracks exact SHA256 checksums and file sizes on-the-fly during pipeline streaming.
- **Credentials Security**: Automatically redacts database passwords and cloud API keys from logs and configuration dumps.
- **Slack Integrations**: Notifies a Slack channel on success/failure using secure webhook integrations.
- **Config Precedence**: Viper-based configuration loading hierarchy (CLI Flags > Environment Variables > YAML file > Defaults).
- **Automation Ready**: Output formatted as clean, structured JSON or human-readable Text with deterministic exit codes.

---

## Getting Started

### Prerequisites

To perform backups, the respective native database CLI utility must be installed and accessible in the system's `PATH`:
- `pg_dump` and `psql` (PostgreSQL)
- `mysqldump` and `mysql` (MySQL)
- `mongodump` and `mongorestore` (MongoDB)
- `sqlite3` (SQLite)

### Build the Binary

Compile the command-line utility:
```bash
go build -o bin/backupdb main.go
```

The resulting `bin/backupdb` binary is fully self-contained and cross-platform.

### Local Database Test Stack

To test the tool against live database containers, start the local stack:

```bash
docker compose -f docker-compose.test.yml up -d
```

This starts:
- PostgreSQL on `127.0.0.1:5432` with user/password `backupdb` and database `appdb`
- MySQL on `127.0.0.1:3306` with user/password `backupdb` and database `appdb`
- MongoDB on `127.0.0.1:27017` with database `appdb`

Seed data is created automatically in each container so you can validate, back up, and restore immediately.

Example commands:

```bash
bin/backupdb validate --db postgres --host 127.0.0.1 --port 5432 --user backupdb --password backupdb --database appdb
bin/backupdb backup --db mysql --host 127.0.0.1 --port 3306 --user backupdb --password backupdb --database appdb --storage local --local-path ./backups/mysql
bin/backupdb backup --db mongodb --host 127.0.0.1 --port 27017 --database appdb --storage local --local-path ./backups/mongo
```

---

## CLI Usage Guide

`backupdb` utilizes a command/subcommand pattern with the following commands:

```bash
backupdb [command] [flags]
```

### Commands

| Command | Description |
| :--- | :--- |
| `validate` | Validates DBMS credentials and verifies server connectivity. |
| `backup` | Triggers the backup pipeline (connect, dump, compress, checksum, upload, notify). |
| `restore` | Triggers the restore pipeline (download, decompress, database import). |
| `config` | Merges configurations from all sources, redacts credentials, and outputs resolved settings. |
| `version` | Prints the utility version (current version `1.0.0`). |

---

## Configuration & Flags

### Global Flags

These flags can be appended to any command:

- `--config <path>`: Path to a YAML configuration file (default is `./backupdb.yaml`).
- `--log-level <level>`: Logging level (`debug`, `info`, `warn`, `error`, default is `info`).
- `--log-file <path>`: Log file path (defaults to stdout).
- `--slack-webhook <url>`: Slack incoming webhook URL for notifications.
- `--output <format>`: Format for command results (`text` or `json`, default is `text`).

---

### Command-Specific Flags

#### 1. Database Selection & Connectivity
Applied to `validate`, `backup`, and `restore` commands:
- `--db <engine>`: Target DBMS engine (`postgres`, `mysql`, `mongodb`, `sqlite`).
- `--host <host>`: Database server host address.
- `--port <port>`: Database server port number.
- `--user <user>`: Database login username.
- `--password <pass>`: Database login password.
- `--database <db>`: Database name (or the direct file path when using `sqlite`).

#### 2. Backup Preferences
Applied to the `backup` command:
- `--mode <mode>`: Backup mode (`full`, `incremental`, `differential`, default is `full`).
- `--compress <algo>`: Compression format (`none`, `gzip`, `zstd`, default is `gzip`).
- `--compression-level <level>`: Compression tuning level.

#### 3. Restore Preferences
Applied to the `restore` command:
- `--backup-path <path>`: Path or cloud storage URI to the backup archive to restore from.
- `--tables <list>`: Comma-separated list of tables to selectively restore (Postgres/MySQL only).
- `--collections <list>`: Comma-separated list of collections to selectively restore (MongoDB only).

#### 4. Storage Adapter Options
Applied to `backup` and `restore` commands:
- `--storage <type>`: Storage backend adapter (`local`, `s3`, `gcs`, `azure`).
- `--local-path <path>`: Path to a local directory for file storage (Local adapter).
- `--bucket <bucket>`: Bucket name (AWS S3 & Google Cloud Storage adapters).
- `--prefix <prefix>`: Prefix path/key namespace (S3, GCS, Azure Blob).
- `--region <region>`: AWS Region (S3 adapter).
- `--endpoint <url>`: Custom S3-compatible API endpoint URL (e.g. MinIO).
- `--access-key <key>`: AWS access key ID (S3 adapter).
- `--secret-key <key>`: AWS secret access key (S3 adapter).
- `--container <name>`: Blob container name (Azure Blob Storage adapter).
- `--azure-account-name <name>`: Azure storage account name.
- `--azure-account-key <key>`: Azure storage account credentials key.
- `--gcs-credentials-file <path>`: Path to Google Service Account JSON credentials file.

---

## Configuration Precedence & YAML Profile

Configurations can be passed using flags, environment variables prefixed with `BACKUPDB_`, or a YAML configuration file.

### YAML Config Profile Example (`backupdb.yaml`)

```yaml
db:
  type: postgres
  host: localhost
  port: 5432
  user: postgres
  password: supersecretpassword
  database: mydb
backup:
  mode: full
  compress: zstd
storage:
  type: s3
  bucket: production-database-backups
  region: us-west-2
  access_key: AKIAIOSFODNN7EXAMPLE
  secret_key: wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY
logging:
  level: info
  file: ./backupdb.log
notifications:
  slack_webhook: https://hooks.slack.com/services/T00000000/B00000000/XXXXXXXXXXXXXXXXXXXXXXXX
output:
  format: text
```

---

## Examples

### 1. SQLite Backups

**Backup to Local Directory:**
```bash
bin/backupdb backup \
  --db sqlite \
  --database /data/production.db \
  --storage local \
  --local-path /backups/sqlite \
  --compress zstd
```

**Restore from Local File:**
```bash
bin/backupdb restore \
  --db sqlite \
  --database /data/restored_production.db \
  --storage local \
  --backup-path /backups/sqlite/sqlite/backup_20260522_120000.sql.zst
```

---

### 2. PostgreSQL Backups

**Validate Connectivity:**
```bash
bin/backupdb validate \
  --db postgres \
  --host 127.0.0.1 \
  --port 5432 \
  --user app_owner \
  --password secret \
  --database production_db
```

**Backup PostgreSQL to S3:**
```bash
bin/backupdb backup \
  --db postgres \
  --host 127.0.0.1 \
  --port 5432 \
  --user app_owner \
  --password secret \
  --database production_db \
  --storage s3 \
  --bucket prod-backups-bucket \
  --region us-east-1 \
  --compress gzip
```

**Restore PostgreSQL from S3 URI:**
```bash
bin/backupdb restore \
  --db postgres \
  --host 127.0.0.1 \
  --port 5432 \
  --user app_owner \
  --password secret \
  --database production_db \
  --storage s3 \
  --backup-path s3://prod-backups-bucket/postgres/backup_20260522_120000.sql.gz
```

---

### 3. MySQL Backups

**Backup to Azure Blob Storage:**
```bash
bin/backupdb backup \
  --db mysql \
  --host 127.0.0.1 \
  --port 3306 \
  --user root \
  --password rootpass \
  --database main_store \
  --storage azure \
  --container db-backups \
  --azure-account-name storageaccountname \
  --azure-account-key accountkeycontents \
  --compress gzip
```

**Restore from Azure Blob URI:**
```bash
bin/backupdb restore \
  --db mysql \
  --host 127.0.0.1 \
  --port 3306 \
  --user root \
  --password rootpass \
  --database main_store \
  --storage azure \
  --backup-path https://storageaccountname.blob.core.windows.net/db-backups/mysql/backup_20260522_120000.sql.gz
```

---

### 4. MongoDB Backups

**Backup to Google Cloud Storage (GCS):**
```bash
bin/backupdb backup \
  --db mongodb \
  --host 127.0.0.1 \
  --port 27017 \
  --database admin \
  --storage gcs \
  --bucket mongo-backups-gcs \
  --gcs-credentials-file /keys/gcs-service-account.json \
  --compress zstd
```

---

## Exit Codes

`backupdb` outputs deterministic exit codes to simplify pipeline orchestration:

| Exit Code | Constant Name | Cause |
| :--- | :--- | :--- |
| `0` | `ExitSuccess` | The operation completed successfully. |
| `2` | `ExitInvalidInput` | Provided parameters/configurations are incorrect or missing. |
| `3` | `ExitConnectionFailure`| Connecting to the target database server failed. |
| `4` | `ExitBackupFailure` | Database export pipeline run failed. |
| `5` | `ExitRestoreFailure` | Database import pipeline run failed. |
| `6` | `ExitStorageFailure` | Interacting with the selected storage backend failed. |
