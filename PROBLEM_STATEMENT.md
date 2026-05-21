# Database Backup Utility — Problem Statement

Build a cross-platform command-line utility that can back up and restore multiple database types. The tool must support common DBMS engines (e.g., MySQL, PostgreSQL, MongoDB, SQLite) with configurable connection parameters, reliable error handling, and clear usage help.

## Core Requirements

1. **Database connectivity**
   - Support multiple DBMS engines.
   - Accept connection parameters (host, port, username, password, database name).
   - Validate credentials before running operations.
   - Handle connection failures with clear errors.

1. **Backup operations**
   - Support full, incremental, and differential backups when available for the DBMS.
   - Compress backup files.

1. **Storage options**
   - Local filesystem storage.
   - Cloud storage targets (AWS S3, Google Cloud Storage, Azure Blob Storage).

1. **Logging and notifications**
   - Log start time, end time, status, duration, and errors.
   - Optional Slack notification on completion.

1. **Restore operations**
   - Restore a database from a backup file.
   - Support selective restore (tables/collections) when the DBMS allows it.

## Constraints and Quality Goals

1. **Scalability**: Handle large databases efficiently.
1. **Reliability**: Ensure secure, reliable backup and restore workflows.
1. **Usability**: Provide clear CLI help and documentation.
1. **Performance**: Minimize impact on the database server during backups.
1. **Portability**: Compatible with Windows, Linux, and macOS.

