# 🐘 Postgres TUI Explorer (Go + Bubble Tea)

A lightweight, interactive **terminal UI** for exploring SQL databases (currently PostgreSQL) using **Go**, **Bubble Tea**, and a pluggable DB interface.

This tool lets you:

- 🔌 Connect to a PostgreSQL database
- 📚 List all tables in the `public` schema
- 🔎 View rows from any table
- 📄 Paginate rows (next/prev page)
- 🔍 Filter rows using a SQL `WHERE` clause
- 🧭 Scroll horizontally when the table is wider than your terminal
- 🆔 Display UUID values as readable strings
- ⌨️ Navigate everything using the keyboard

The code is structured so that other databases (MySQL, SQLite, etc.) can be added later by implementing a simple `DB` interface.

---

## 🗂 Project Structure

```text
.
├── cmd/
│   └── main.go                # Entrypoint: wires DB implementation to the TUI
│
├── internal/
│   ├── app/
│   │   ├── app.go                 # New / NewProgram helpers for Bubble Tea
│   │   └── model.go               # Bubble Tea model, Update, View, key handling
│   │
│   ├── db/
│   │   ├── db.go                  # Generic DB interface + shared types
│   │   └── postgres/
│   │       └── postgres.go        # PostgreSQL implementation using pgxpool
│   │       # later you can add:
│   │       # └── mysql/mysql.go
│   │       # └── sqlite/sqlite.go
│   │
│   └── ui/
│       └── table/
│           └── render.go          # ASCII table renderer + horizontal scrolling
│
└── go.mod
```

---

## 🚀 Features

### 🔐 Connection Form

On startup, the TUI shows a form where you enter:

- Host
- Port
- User
- Password
- Database

Press **Enter** on the last field to connect.

> Note: The chosen DB implementation (currently PostgreSQL) receives these values via the `db.ConnConfig` struct.

---

### 📋 Table Browser

After a successful connection:

- Lists all tables from the `public` schema
- Navigate using **↑ / ↓**
- Select a table using **Enter**

---

### 📄 Row Viewer

For the selected table, the app:

- Fetches rows using `LIMIT` + `OFFSET` (pagination)
- Displays them in an ASCII table
- Shows page info (current page, total rows, etc.)
- Supports horizontal scrolling for wide tables

UUID-like columns are automatically detected and shown as human-readable strings instead of raw bytes.

---

### 🔍 Filtering

In the rows view, you can filter results using a SQL `WHERE` clause fragment.

- Press **`/`** to start editing the filter
- Type a condition like:
  - `id > 10`
  - `status = 'active'`
  - `name ILIKE '%john%'`
- Press **Enter** to apply the filter
- Press **Esc** while editing to cancel
- Press **`r`** to remove filter and reload all rows

Under the hood, the filter is appended to the query as:

```sql
SELECT COUNT(*) FROM <table> WHERE <your filter>;
SELECT * FROM <table> WHERE <your filter> LIMIT <pageSize> OFFSET <offset>;
```

> ⚠️ This is meant for local use and trusted environments, since the filter text is concatenated as raw SQL.

---

### 📄 Pagination

Uses a classic `LIMIT/OFFSET` approach:

- Page size: **10** rows by default
- `n` → next page
- `p` → previous page

Internally:

```sql
SELECT * FROM <table> LIMIT 10 OFFSET <offset>;
```

where `offset` is incremented/decremented based on page navigation.

---

### 🧭 Horizontal Scrolling

If the rendered ASCII table is wider than your terminal, you can scroll horizontally.

- `←` / `h` → scroll left
- `→` / `l` → scroll right
- `Shift+←` → scroll left faster
- `Shift+→` → scroll right faster

This is implemented by slicing the rendered lines based on the current `horizOffset` and terminal width.

---

## ⌨️ Keybindings

### Form Screen

| Key              | Action                     |
| ---------------- | -------------------------- |
| Tab / ↓          | Next field                 |
| Shift+Tab / ↑    | Previous field             |
| Enter (last field)| Connect to DB             |
| Esc / Ctrl+C     | Quit                       |

---

### Tables Screen

| Key          | Action                               |
| ------------ | ------------------------------------ |
| ↑ / ↓        | Move selection between tables        |
| Enter        | Load rows for selected table         |
| q / Esc      | Quit                                 |
| Ctrl+C       | Quit                                 |

---

### Rows Screen

| Key          | Action                                             |
| ------------ | -------------------------------------------------- |
| n            | Next page                                          |
| p            | Previous page                                      |
| /            | Start editing filter                               |
| Enter        | While editing filter: apply filter                 |
| Esc          | While editing filter: cancel and clear filter      |
| r            | Clear active filter and reload all rows            |
| h / ←        | Scroll left                                        |
| l / →        | Scroll right                                       |
| Shift+←      | Fast scroll left                                   |
| Shift+→      | Fast scroll right                                  |
| b            | Back to tables list                                |
| q / Esc      | Quit                                               |
| Ctrl+C       | Quit                                               |

---

## 🧠 Architecture Overview

The project is intentionally split into three main layers:

### 1. **UI Layer** (`internal/app`, `internal/ui/table`)

- `internal/app/model.go` contains:
  - Bubble Tea `Model`
  - `Update` and `View` functions
  - Input handling
  - Pagination and filtering state
- `internal/ui/table/render.go` contains:
  - `Render(columns, rows)` → ASCII table
  - `ApplyHorizontalScroll(...)` → horizontal clipping

The UI never talks directly to PostgreSQL—it only calls the **DB interface**.

---

### 2. **DB Abstraction Layer** (`internal/db/db.go`)

Defines:

```go
type ConnConfig struct {
    Host     string
    Port     string
    User     string
    Password string
    Database string
}

type QueryOptions struct {
    Limit  int
    Offset int
    Filter string
}

type RowPage struct {
    Columns   []string
    Rows      [][]string
    TotalRows int
    Offset    int
}

type DB interface {
    Connect(ctx context.Context, cfg ConnConfig) error
    Close() error
    ListTables(ctx context.Context) ([]string, error)
    FetchRows(ctx context.Context, table string, opts QueryOptions) (RowPage, error)
}
```

Any database implementation must satisfy this interface.  
The TUI doesn’t care if it’s Postgres, MySQL, or SQLite—just that it implements `DB`.

---

### 3. **Postgres Implementation** (`internal/db/postgres/postgres.go`)

Implements `db.DB` using:

- `pgx/v5/pgxpool` for connection pooling
- Type inspection & conversion to render UUIDs nicely
- `information_schema.tables` to list tables
- `SELECT * FROM <table> LIMIT/OFFSET` for pagination

You can add new database backends in the future under:

- `internal/db/mysql/`
- `internal/db/sqlite/`
- etc.

by implementing the same `db.DB` interface.

---

## 🛠 Installation & Setup

### 1. Install Go

https://go.dev/dl/

### 2. Get dependencies

From the project root:

```bash
go get github.com/charmbracelet/bubbletea
go get github.com/charmbracelet/bubbles
go get github.com/jackc/pgx/v5
go get github.com/jackc/pgx/v5/pgxpool
go get github.com/google/uuid
```

### 3. Run the TUI

```bash
go run ./cmd/main.go
```

Then enter your PostgreSQL connection details in the form.

---

## 🧩 Future Ideas

- Support for **multiple database types** (MySQL, SQLite, etc.)
- **Schema viewer** (list columns, types, indexes)
- **Inline editing** of row values
- Export results to **CSV / JSON**
- Search mode that builds filters automatically (no SQL needed)
- Configuration via **env vars / flags** instead of purely interactive form

---

## 📝 License

This is a personal / learning project structure.  
You can adapt it freely for your own tools and experiments.
