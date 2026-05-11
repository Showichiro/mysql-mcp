# mysql-mcp

Read-only MySQL MCP server for local stdio integrations.

`mysql-mcp` lets agents inspect schemas, describe tables, run bounded read-only queries, and check `EXPLAIN` plans without granting write access.

## Features

- stdio MCP server built with the official Go MCP SDK
- Read-only tools for MySQL schema inspection
- Conservative SQL guard for `SELECT`, `SHOW`, `DESCRIBE`, and `EXPLAIN`
- Automatic `LIMIT` enforcement for `SELECT`
- Query timeout, max rows, and cell-size truncation
- Binary release setup with GoReleaser

## Tools

| Tool | Purpose |
|---|---|
| `mysql_list_schemas` | List accessible schemas |
| `mysql_list_tables` | List tables in a schema |
| `mysql_describe_table` | Return columns and indexes for a table |
| `mysql_query` | Run a bounded read-only SQL statement |
| `mysql_explain` | Run `EXPLAIN` for a SELECT statement |

## Configuration

Set environment variables before launching the server.

```bash
export MYSQL_MCP_HOST=127.0.0.1
export MYSQL_MCP_PORT=3306
export MYSQL_MCP_USER=readonly
export MYSQL_MCP_PASSWORD=...
export MYSQL_MCP_DATABASE=app_db
export MYSQL_MCP_SSL=false
export MYSQL_MCP_MAX_ROWS=100
export MYSQL_MCP_TIMEOUT_MS=5000
```

Optional:

```bash
export MYSQL_MCP_ALLOWED_SCHEMAS=app_db,analytics
export MYSQL_MCP_MAX_CELL_CHARS=4096
export MYSQL_MCP_LOG_LEVEL=info
```

Use a database user that is read-only. The SQL guard is defense in depth, not a replacement for database privileges.

## Build

```bash
go build -o bin/mysql-mcp ./cmd/mysql-mcp
```

## Run

```bash
./bin/mysql-mcp
```

## Claude Code MCP config

```json
{
  "mcpServers": {
    "mysql": {
      "command": "/path/to/mysql-mcp",
      "env": {
        "MYSQL_MCP_HOST": "127.0.0.1",
        "MYSQL_MCP_PORT": "3306",
        "MYSQL_MCP_USER": "readonly",
        "MYSQL_MCP_PASSWORD": "...",
        "MYSQL_MCP_DATABASE": "app_db"
      }
    }
  }
}
```

## Release

Push a tag to publish binaries:

```bash
git tag v0.1.0
git push origin v0.1.0
```

GitHub Actions runs GoReleaser and uploads archives for macOS and Linux.

