> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
> Master plan: tinywasm/docs/MASTER_PLAN_SCHEMA_SQL_EXPORT.md — Fase B (parallel with postgres)

# PLAN: sqlt — Implement ddl.Exporter

## Precondition

`tinywasm/orm` Fase A must be published. The `github.com/tinywasm/orm/ddl` sub-package
must be available. Update `go.mod` to the published `orm` version before starting.

No changes needed in `tinywasm/fmt`: `fmt.Field` already embeds `Permitted.Maximum`.

## Context

`sqlt` already generates full `CREATE TABLE IF NOT EXISTS` SQL in `translate.go` via
`buildCreateTable(q Query, m fmt.Model) string` (called by `translateQuery` on `ActionCreateTable`).

This plan adds `ExportDDL` on the existing `*compiler` struct — calling `c.Compile(ActionCreateTable)`
in a loop. Zero new SQL generation.

### Interface to implement (from `orm/ddl` — already published)

```go
// From github.com/tinywasm/orm/ddl
type Exporter interface {
    ExportDDL(models []fmt.Model) (string, error)
}
```

### Compile-time check to add

```go
var _ ddl.Exporter = (*compiler)(nil)
```

---

## S0a — `VARCHAR(n)` support in `buildCreateTable` (`sqlt/translate.go`)

`fmt.Field` embeds `Permitted` which carries `Maximum int`. When `Maximum > 0` on a `FieldText`
field, emit `VARCHAR(n)` instead of `TEXT`. No new field on `fmt.Field` needed.

```go
func sqliteColumnType(f fmt.Field) string {
    if f.Type == fmt.FieldText && f.Maximum > 0 {
        return fmt.Sprintf("VARCHAR(%d)", f.Maximum)
    }
    return sqliteType(f.Type)
}
```

Replace the current call `sqliteType(f.Type)` with `sqliteColumnType(f)` in `buildCreateTable`.

Keep `sqliteType(t fmt.FieldType) string` unchanged — it's used by `buildAddColumn` and `buildRenameColumn`
which operate on `fmt.Field` values without size context.

### `TestVarchar_SQLite`
```go
// Field: username TEXT, Maximum=50
// Assert: buildCreateTable output contains "username VARCHAR(50)"
// Assert: field without maximum: "email TEXT"
```

---

## S0b — `ON DELETE` on FK constraints (`sqlt/translate.go`)

`FieldExt.OnDelete` carries the action string from `db:"on_delete=cascade"`.
Append `ON DELETE <ACTION>` to the FK constraint when non-empty.

Default: `CASCADE`. `OnDelete == ""` significa cascade — no se necesita tag.
Solo `restrict`, `set_null`, `no_action` se escriben explícitamente.

```go
func onDeleteSQL(action string) string {
    switch action {
    case "restrict":
        return "RESTRICT"
    case "set_null":
        return "SET NULL"
    case "no_action":
        return "NO ACTION"
    default:
        return "CASCADE" // default para todo ref=, incluyendo OnDelete == ""
    }
}
```

In `buildCreateTable`, where FK constraints are assembled:
```go
fkSQL := fmt.Sprintf(
    "CONSTRAINT fk_%s_%s FOREIGN KEY (%s) REFERENCES %s(%s)",
    q.Table, f.Name, f.Name, f.Ref, refCol)
fkSQL += " ON DELETE " + onDeleteSQL(f.OnDelete)
cols = append(cols, fkSQL)
```

### `TestOnDelete_SQLite`
```go
// FK with OnDelete="" (default) → "ON DELETE CASCADE"
// FK with OnDelete="restrict"   → "ON DELETE RESTRICT"
// FK with OnDelete="set_null"   → "ON DELETE SET NULL"
```

---

## S0c — Auto-index on FK columns (en `ExportDDL`, `sqlt/compiler.go`)

Every FK column must have an index. `ExportDDL` emits `CREATE INDEX` after each table's
`CREATE TABLE` for any field present in `SchemaExt()`.

**No new `Action` constant needed** — generate the index SQL as a second statement returned
alongside the `CREATE TABLE` in `ExportDDL` (not in `Compile`). The adapter assembles both.

```go
// In ExportDDL, after emitting CREATE TABLE for a model:
if ext, ok := m.(interface{ SchemaExt() []orm.FieldExt }); ok {
    for _, f := range ext.SchemaExt() {
        if f.Ref != "" {
            buf.Write(fmt.Sprintf(
                "CREATE INDEX IF NOT EXISTS idx_%s_%s ON %s(%s);\n\n",
                m.ModelName(), f.Name, m.ModelName(), f.Name))
        }
    }
}
```

> SQLite: `CREATE INDEX IF NOT EXISTS` is supported since SQLite 3.3.0 (2006).

### `TestAutoIndex_SQLite`
```go
// sessions has user_id FK → users
// Assert: output contains "CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)"
// Assert: index appears AFTER the CREATE TABLE for sessions
// tables without FK (users, roles) must NOT generate any CREATE INDEX
```

---

## S1 — `ExportDDL` on `*compiler` (`sqlt/compiler.go`)

```go
import (
    "github.com/tinywasm/fmt"
    "github.com/tinywasm/orm"
    "github.com/tinywasm/orm/ddl"
)

func (c *compiler) ExportDDL(models []fmt.Model) (string, error) {
    sorted, err := ddl.TopologicalSort(models)
    if err != nil {
        return "", err
    }
    var buf fmt.Builder
    buf.Write("-- dialect: sqlite\n\n")
    for _, m := range sorted {
        plan, err := c.Compile(orm.Query{Action: orm.ActionCreateTable, Table: m.ModelName()}, m)
        if err != nil {
            return "", err
        }
        buf.Write(plan.Query)
        buf.Write(";\n\n")
        if ext, ok := m.(interface{ SchemaExt() []orm.FieldExt }); ok {
            for _, f := range ext.SchemaExt() {
                if f.Ref != "" {
                    buf.Write(fmt.Sprintf(
                        "CREATE INDEX IF NOT EXISTS idx_%s_%s ON %s(%s);\n\n",
                        m.ModelName(), f.Name, m.ModelName(), f.Name))
                }
            }
        }
    }
    return buf.String(), nil
}

var _ ddl.Exporter = (*compiler)(nil)
```

**No new SQL.** `c.Compile(ActionCreateTable)` already emits the full statement with FKs.
`ddl.TopologicalSort` handles FK ordering — not reimplemented here.

---

## S2 — Tests (`sqlt/export_test.go` — new file)

### Fixture

The golden output lives at `sqlt/tests/schema.sql`.
Read it with `os.ReadFile("tests/schema.sql")` and compare with `strings.TrimSpace`.

The fixture covers all cases:
- `users`: int64 PK AUTOINC, string NOT NULL UNIQUE, float64, bool, []byte
- `roles`: int64 PK AUTOINC, string NOT NULL UNIQUE
- `sessions`: string PK (no autoinc/UUID), int64 FK→users.id
- `user_roles`: composite PK (int64, int64) + two FKs (→users, →roles)

Input models must be passed in order `[users, roles, sessions, user_roles]` so
`TopologicalSort` emits them in that same order (users/roles have in-degree 0).

### Stub types for tests (`sqlt/export_test.go` internal helpers)

```go
type testField struct {
    name    string
    typ     fmt.FieldType
    pk      bool
    autoInc bool
    notNull bool
    unique  bool
}
func (f testField) Name() string          { return f.name }
func (f testField) Type() fmt.FieldType   { return f.typ }
func (f testField) IsPK() bool            { return f.pk }
func (f testField) IsAutoInc() bool       { return f.autoInc }
func (f testField) IsUnique() bool        { return f.unique }
// NotNull is a struct field on fmt.Field — use fmt.Field directly:
//   fmt.Field{Name:"username", Type:fmt.FieldText, NotNull:true, ...}
```

> Use `fmt.Field` (the struct from `github.com/tinywasm/fmt`) directly for field definitions.
> Implement `SchemaExt() []orm.FieldExt` only on stubs that have FK columns.

### `TestExportDDL_ImplementsInterface`
```go
var _ ddl.Exporter = (*compiler)(nil)   // compile-time
```

### `TestExportDDL_FullSchema`
```go
// Load golden file from tests/schema.sql
// Build stubs for [users, roles, sessions, user_roles] exactly as documented in fixture header
// Call c.ExportDDL([users, roles, sessions, user_roles])
// Compare strings.TrimSpace(got) == strings.TrimSpace(golden)
```

This test exercises: int64 AUTOINC PK, TEXT PK (no autoinc), NOT NULL UNIQUE,
REAL/INTEGER(bool)/BLOB types, single FK, composite PK + dual FK.

### `TestExportDDL_EmptyInput`
```go
// Input: nil / empty slice
// Assert: output = "-- dialect: sqlite\n\n", no error
```

---

## Constraints

- RULE: `ExportDDL` calls `c.Compile(ActionCreateTable)` para cada tabla y agrega los índices FK inline — sin nueva lógica SQL.
- RULE: No stdlib. Use `github.com/tinywasm/fmt`.
- RULE: `ddl.TopologicalSort` is imported from `orm/ddl`, not reimplemented.
- RULE: `ExportDDL` is a method on `*compiler` (same receiver as `Compile`).
- RULE: Compile-time interface assertion `var _ ddl.Exporter = (*compiler)(nil)` must be present.

## Stages summary

| Stage | File | Change |
|---|---|---|
| S0a | `sqlt/translate.go` | Add `sqliteColumnType(f fmt.Field)` → VARCHAR(n) |
| S0b | `sqlt/translate.go` | Add `onDeleteSQL(action string)` + emit ON DELETE in FK constraint |
| S0c | `sqlt/compiler.go` (en `ExportDDL`) | Emit `CREATE INDEX IF NOT EXISTS` after each table with FKs |
| S0t | `sqlt/translate_test.go` | `TestVarchar_SQLite`, `TestOnDelete_SQLite` |
| S0ct | `sqlt/export_test.go` | `TestAutoIndex_SQLite` |
| S1 | `sqlt/compiler.go` | Add `ExportDDL` + interface assertion |
| S2 | `sqlt/export_test.go` (new) | `TestExportDDL_FullSchema` (golden file) + `TestExportDDL_EmptyInput` |
