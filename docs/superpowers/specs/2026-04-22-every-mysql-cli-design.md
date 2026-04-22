# every-mysql-cli Design Spec

**Date:** 2026-04-22
**Approach:** Template-based code generation (Approach A)

## Overview

A MySQL schema scanner that connects to a database, safely scans its structure (tables, columns, keys, foreign keys, indexes), and generates a standalone Go CLI binary tailored to that specific database. The generated CLI supports CRUD operations plus relation-aware helpers per table, with dual-mode output (human-readable tables by default, `--json` for agent consumption) and destructive operation guards.

## Architecture & Components

Two phases, two binaries:

### Phase 1 — `every-mysql-cli` (the generator)

```
every-mysql-cli init \
  --host db.example.com \
  --port 3306 \
  --user root \
  --password secret \
  --database myapp \
  --output ./myapp-cli
```

1. Connects to MySQL via `go-sql-driver/mysql`
2. Safe scan: reads `information_schema` for tables, columns, types, primary keys, foreign keys, indexes
3. Generates Go source files using `text/template` — one command file per table, plus root Cobra setup, DB connection module, and safety middleware
4. Runs `go build` in the output directory → produces a standalone `myapp-cli` binary

### Phase 2 — `myapp-cli` (the generated binary)

```
myapp-cli users list
myapp-cli users list --by-order 5       # relation helper (outbound FK)
myapp-cli users get 42 --with-orders    # relation helper (inbound reference)
myapp-cli users create --name "Alice" --email alice@example.com
myapp-cli users update 42 --name "Bob"
myapp-cli users delete 42 --force       # destructive guard
myapp-cli users list --json             # agent mode
```

### Key modules in generated code

| Module | Purpose |
|--------|---------|
| `main.go` | Cobra root command, DB connection setup, subcommand registration |
| `db.go` | Connection pool, query helpers |
| `guard.go` | Destructive operation middleware (`--force` required for delete, update without WHERE) |
| `output.go` | Dual-mode rendering: table format (human) vs JSON (agent) |
| `config.go` | Connection config loading (env vars > config file > flags) |
| `<table>_cmd.go` | Command group for each table (one file per table) |

## Schema Scan & Data Model

### What gets scanned from `information_schema`

| Source | Data extracted |
|--------|---------------|
| `TABLES` | Table names, engines, row counts (approximate) |
| `COLUMNS` | Column names, types, nullable, defaults, auto_increment |
| `KEY_COLUMN_USAGE` | Primary keys, foreign key relationships |
| `STATISTICS` | Index names, columns, uniqueness |
| `REFERENTIAL_CONSTRAINTS` | FK constraint names, update/delete rules (CASCADE, RESTRICT, etc.) |

### Internal data model

```go
type Schema struct {
    Tables []Table
}

type Table struct {
    Name         string
    Columns      []Column
    PrimaryKey   *PrimaryKey
    ForeignKeys  []ForeignKey    // outbound: this table references another
    ReferencedBy []RefReference  // inbound: other tables reference this one
    Indexes      []Index
}

type Column struct {
    Name          string
    Type          string        // raw MySQL type string
    GoType        string        // mapped Go type for generated code
    Nullable      bool
    Default       string
    AutoIncrement bool
}

type ForeignKey struct {
    Name            string
    Column          string       // local column
    ReferencedTable string       // target table
    ReferencedColumn string      // target column
    OnDelete        string       // CASCADE, RESTRICT, SET NULL, NO ACTION
    OnUpdate        string
}

type RefReference struct {
    SourceTable    string       // table that references this one
    SourceColumn   string
    ForeignKeyName string
}

type PrimaryKey struct {
    Columns []string
}

type Index struct {
    Name    string
    Columns []string
    Unique  bool
}
```

### FK direction and command generation

- **Outbound FKs** (`ForeignKeys`): Generate `--by-<table>` flags on the **source table's** `list` command. If `orders.user_id` references `users.id`, then `orders list --by-user 5` translates to `WHERE orders.user_id = 5` — the flag appears on the table that holds the FK column.
- **Inbound references** (`ReferencedBy`): Generate `--with-<table>` flags on this table's `get` command. If `orders.user_id` references `users.id`, then `users get 42 --with-orders` eager-loads all orders for user 42 as a nested structure.

### MySQL-to-Go type mapping

| MySQL | Go |
|-------|-----|
| INT, BIGINT, TINYINT, SMALLINT | int64 |
| FLOAT, DOUBLE | float64 |
| DECIMAL | string (preserve precision) |
| VARCHAR, TEXT, CHAR | string |
| DATE | string (YYYY-MM-DD) |
| DATETIME, TIMESTAMP | string (ISO 8601) |
| BOOLEAN, BIT(1) | bool |
| ENUM | string |
| JSON | string (raw JSON) |
| BLOB | []byte |

## Generated Commands & Behavior

### Command structure

```
<db-cli> <table> <action> [args] [flags]
```

### Core CRUD

| Command | Description | Example |
|---------|-------------|---------|
| `list` | Paginated listing, supports filtering by column values | `users list --status active --limit 50 --offset 0` |
| `get` | Fetch single row by primary key | `users get 42` |
| `create` | Insert new row, flags for each non-auto-increment column | `users create --name "Alice" --email alice@x.com` |
| `update` | Update row by primary key, flags for updatable columns | `users update 42 --name "Bob"` |
| `delete` | Delete row by primary key (requires `--force`). `--all` variant deletes all rows, requires `--force --confirm "I understand..."` | `users delete 42 --force` |

### Relation helpers

**Outbound FKs → `list` sub-flags:**

If `orders.user_id` references `users.id`, then `orders list --by-user 5` filters by that FK.

**Inbound references → `get` sub-flags:**

If `orders.user_id` references `users.id`, then `users get 42 --with-orders` eager-loads related orders as nested data.

### Destructive guards

| Guard | Rule |
|-------|------|
| `delete` | Always requires `--force` flag. Without it, prints warning and exits. |
| `update` without PK | If called without a primary key argument, requires `--force`. Normal `update 42` is fine. |
| `delete` with `--all` | Bulk delete requires `--force --confirm "I understand this deletes all rows"` |

In agent mode (`--json`), guards still apply — the JSON output includes:
```json
{"error": "destructive operation requires --force", "hint": "re-run with --force flag"}
```

### Dual-mode output

**Human mode (default):**

Uses `tablewriter` for tabular output:
```
ID  Name   Email          Status
42  Alice  alice@x.com    active
43  Bob    bob@x.com      active
```
Single records shown as key-value pairs.

**Agent mode (`--json`):**

```json
{
  "data": [
    {"id": 42, "name": "Alice", "email": "alice@x.com", "status": "active"},
    {"id": 43, "name": "Bob", "email": "bob@x.com", "status": "active"}
  ],
  "meta": {"total": 2, "limit": 50, "offset": 0}
}
```
Includes pagination metadata.

### Pagination & filtering

- `--limit` (default: 100) and `--offset` for `list` commands
- Any column can be used as a filter flag: `users list --status active --name_like "Ali%"`
- String columns get `_like` variants for pattern matching
- `--order-by <column>` and `--order-dir asc/desc`

## Init Flow & Generated Project Structure

### Init command

```
every-mysql-cli init \
  --host <host> \
  --port <port> \
  --user <user> \
  --password <password> \
  --database <database> \
  --output <dir>        # default: ./<database>-cli
```

### Init process

1. Validate connection params and connect to MySQL
2. Run schema scan via `information_schema` queries
3. Build internal `Schema` data model
4. Generate project directory with all Go source files
5. Run `go mod init` + `go mod tidy` in the output directory
6. Run `go build` → produces standalone CLI binary
7. Write config file to `~/.every-mysql/<database>.yaml`
8. Print success message with binary path and usage example

### Generated project structure

```
myapp-cli/
  main.go                 # Cobra root, subcommand registration
  db.go                   # Connection pool init
  guard.go                # Destructive operation middleware
  output.go               # Dual-mode rendering
  config.go               # Connection config loading
  users_cmd.go            # Table commands
  orders_cmd.go           # Table commands
  products_cmd.go         # Table commands
  ...                     # One per table
  go.mod
  go.sum
```

### Connection config for the generated binary

Priority order:

1. **Environment variables:** `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`
2. **Config file:** `~/.every-mysql/<database>.yaml` (created by init)
3. **Flags on every command:** `--db-host`, `--db-port`, etc.

Agents should prefer env vars to avoid storing secrets in files. Password in config file is stored in plaintext for human convenience — agents should use `DB_PASSWORD` env var instead.

### Config file format

```yaml
host: db.example.com
port: 3306
user: root
password: secret
database: myapp
```

## Error handling

- Connection failures: clear error message with the attempted params (password masked)
- Schema scan errors: report which query failed and continue if possible
- Build failures: print `go build` output, suggest checking Go toolchain installation
- Runtime query errors: human mode shows readable message, agent mode returns `{"error": "...", "code": "QUERY_ERROR"}`
- Missing PK: tables without a primary key get `list` only (no `get`, `update`, `delete`) — print a note during init

## Testing

- Generator: unit tests for schema scanning, type mapping, template rendering
- Generator: integration test against a real MySQL instance with a known schema
- Generated CLI: each generated project includes a `--dry-run` flag that prints the SQL it would execute without running it — useful for verification
- Generated CLI: integration tests auto-generated alongside the CLI that hit a test MySQL instance

## Dependencies

### Generator (`every-mysql-cli`)

- `go-sql-driver/mysql` — MySQL connection
- `spf13/cobra` — CLI framework (used in both generator and generated code)
- `olekukonko/tablewriter` — human-mode table output
- Go standard `text/template` — code generation

### Generated CLI

- Same dependencies as above, embedded in the generated `go.mod`
- No external runtime dependencies beyond the MySQL driver