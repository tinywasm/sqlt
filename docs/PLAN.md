# PLAN — Implement Introspection, Renames, and Safe Deletes (SQLite adapter)

> The `tinywasm/orm` dev schema-sync (`db.Sync`) now supports introspective checks, renames, and safe deletes.
> This adapter must comply with the contract by implementing the `TableIntrospector` interface
> and translating `ActionRenameColumn` and `ActionDropColumn` actions.

---

## 1. Development Rules (constraints copied for execution context)

- **Pure translator + Executor extension.**
  - `translateQuery(q, m)` remains a pure query-to-SQL translator. It does **not** query the DB.
  - The `TableIntrospector` method `TableColumns` is implemented on the SQLite `Executor` (using `PRAGMA table_info`).
- **SQLite dialect.** This module owns SQLite-specific SQL. Reuse the existing `sqliteType()` map.
- **SQLite constraints.**
  - SQLite (3.35.0+) supports `ALTER TABLE ... RENAME COLUMN ...` and `ALTER TABLE ... DROP COLUMN ...`.
  - We translate columns as plain `<name> <type>` without NOT NULL / UNIQUE / PK constraints on add.
- **gotest.** Assert the generated SQL strings and mock/verify the introspector query returns correct columns.

---

## 2. Problem

The current SQLite adapter does not implement `TableIntrospector` nor translates `ActionRenameColumn` and `ActionDropColumn`, resulting in unsupported action errors.

---

## 3. Decision

1. **Implement `TableIntrospector` on the SQLite Executor:**
   ```go
   func (e *SQLiteExecutor) TableColumns(table string) ([]string, error) {
       // Query PRAGMA table_info(table_name)
       rows, err := e.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
       // Scan column names (index 1 of the returned fields)
   }
   ```
2. **Translate `ActionRenameColumn`:**
   ```sql
   ALTER TABLE <q.Table> RENAME COLUMN <q.OldName> TO <q.Column.Name>
   ```
3. **Translate `ActionDropColumn`:**
   ```sql
   ALTER TABLE <q.Table> DROP COLUMN <q.Columns[0]>
   ```

---

## 4. Implementation Steps

### Step 1 — Bump the orm dependency
`go get github.com/tinywasm/orm@vX`

### Step 2 — Implement `TableIntrospector`
**File:** [sqlt.go](../sqlt.go) (or where the Executor type is defined)

Add the method `TableColumns(table string) ([]string, error)` by executing `PRAGMA table_info(%s)` and reading column names.

### Step 3 — Add the new translation cases
**File:** [translate.go](../translate.go)

Under `switch q.Action`:
```go
case orm.ActionAddColumn:
    return buildAddColumn(q)

case orm.ActionRenameColumn:
    return buildRenameColumn(q)

case orm.ActionDropColumn:
    return buildDropColumn(q)
```

Add helper functions:
```go
func buildRenameColumn(q orm.Query) (string, []any, error) {
    if q.OldName == "" || q.Column == nil {
        return "", nil, fmt.Err("old column name and target column metadata are required for rename")
    }
    if q.Table == "" {
        return "", nil, fmt.Err("table name is required for rename")
    }
    return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s", q.Table, q.OldName, q.Column.Name), nil, nil
}

func buildDropColumn(q orm.Query) (string, []any, error) {
    if len(q.Columns) == 0 {
        return "", nil, fmt.Err("column name is required for drop")
    }
    if q.Table == "" {
        return "", nil, fmt.Err("table name is required for drop")
    }
    return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", q.Table, q.Columns[0]), nil, nil
}
```

---

## 5. Edge Cases

- Older SQLite versions lacking `DROP COLUMN` support will fail with syntax errors. We target standard modern SQLite (3.35.0+).

---

## 6. Test Strategy

`gotest` in `tinywasm/sqlt/tests/`.

| # | Case | Assert |
|---|------|--------|
| T1 | `ActionAddColumn` | `ALTER TABLE x ADD COLUMN c TEXT` |
| T2 | `ActionRenameColumn` | `ALTER TABLE x RENAME COLUMN old TO new` |
| T3 | `ActionDropColumn` | `ALTER TABLE x DROP COLUMN c` |
| T4 | Introspector check | `TableColumns("users")` parses `PRAGMA table_info` columns correctly |
