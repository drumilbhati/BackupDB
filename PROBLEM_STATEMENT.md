# Database Backup Utility — Problem Statement

Build a cross-platform command-line utility that can back up and restore multiple database types. The tool must support common DBMS engines (e.g., MySQL, PostgreSQL, MongoDB, SQLite) with configurable connection parameters, reliable error handling, and clear usage help.

## Core Requirements

1. **Database connectivity** [DONE]
   - Support multiple DBMS engines (Postgres, MySQL, MongoDB, SQLite).
   - Accept connection parameters (host, port, username, password, database name).
   - Validate credentials before running operations.
   - Handle connection failures with clear errors.

1. **Backup operations** [DONE]
   - Support full, incremental, and differential backups (managed via local catalog and delta encoding).
   - Compress backup files (Gzip, Zstd).

1. **Storage options** [DONE]
   - Local filesystem storage.
   - Cloud storage targets (AWS S3, Google Cloud Storage, Azure Blob Storage).

1. **Logging and notifications** [DONE]
   - Log start time, end time, status, duration, and errors.
   - Optional Slack notification on completion.

1. **Restore operations** [DONE]
   - Restore a database from a backup file (including reconstruction of incremental chains).
   - Support selective restore (tables/collections) when the DBMS allows it.

## Constraints and Quality Goals

1. **Scalability**: Handle large databases efficiently. [DONE]
1. **Reliability**: Ensure secure, reliable backup and restore workflows. [DONE]
1. **Usability**: Provide clear CLI help and documentation. [DONE]
1. **Performance**: Minimize impact on the database server during backups. [DONE]
1. **Portability**: Compatible with Windows, Linux, and macOS. [DONE]

