# mysql-mcp

MySQL MCP server for local stdio integrations. It is read-only by default and can optionally allow writes by configuration.

`mysql-mcp` lets agents inspect schemas, describe tables, run bounded queries, check `EXPLAIN` plans, and optionally run limited write statements.

## Features

- stdio MCP server built with the official Go MCP SDK
- Read-only tools for MySQL schema inspection
- Conservative SQL guard for `SELECT`, `SHOW`, `DESCRIBE`, and `EXPLAIN`
- Optional write mode for single `INSERT`, `UPDATE`, `DELETE`, and `REPLACE` statements
- Automatic `LIMIT` enforcement for `SELECT`
- Query timeout, max rows, and cell-size truncation
- Binary release setup with GoReleaser

## Tools

| Tool | Purpose |
|---|---|
| `mysql_list_schemas` | List accessible schemas |
| `mysql_list_tables` | List tables in a schema |
| `mysql_describe_table` | Return columns and indexes for a table |
| `mysql_query` | Run one bounded SQL statement; writes require `MYSQL_MCP_ALLOW_WRITES=true` |
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
export MYSQL_MCP_ALLOW_WRITES=false
```

You can also place these values in a local `.env` file in the current working directory. Existing environment variables take precedence over `.env` values.

Optional:

```bash
export MYSQL_MCP_ALLOWED_SCHEMAS=app_db,analytics
export MYSQL_MCP_MAX_CELL_CHARS=4096
export MYSQL_MCP_LOG_LEVEL=info
```

By default, write SQL is rejected. Set `MYSQL_MCP_ALLOW_WRITES=true` to allow single `INSERT`, `UPDATE`, `DELETE`, and `REPLACE` statements through `mysql_query`. DDL, transaction control, grants, calls, `LOAD DATA`, and multiple statements remain blocked.

Use a database user with only the privileges this server should expose. The SQL guard is defense in depth, not a replacement for database privileges.

## Build

```bash
go build -o bin/mysql-mcp ./cmd/mysql-mcp
```

## Run

```bash
./bin/mysql-mcp
```

## Install with Homebrew

After a tagged release is published, install from the tap:

```bash
brew tap Showichiro/tap
brew install mysql-mcp
```

The `Showichiro/homebrew-tap` repository checks releases on a schedule and updates the formula.

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
