# PLAN — Schema-sync translation (SQLite compiler)

> `tinywasm/orm`'s dev schema-sync drives the engine through agnostic `Action`s. **`tinywasm/sqlt` is
> the SQLite compiler only** (`NewCompiler()` / `translateQuery` / `Translate`) — it has **no
> executor and no connection**. The executor, the engine registry (`orm.Register("sqlite", …)`),
> `orm.ErrNoRows` mapping, and `TableIntrospector` all live in **`tinywasm/sqlite`** (its own plan).
>
> This plan covers only what a pure compiler owns: translate the new DDL actions
> (`ActionAddColumn`/`RenameColumn`/`DropColumn`) and handle `IS NULL`/`IS NOT NULL` conditions.
>
> **Self-contained, single-module plan** (`tinywasm/sqlt`). Prerequisite: `orm` published with the
> three column actions + `Query.Column`/`OldName`/`Columns`. Bump the dep first.

---

## 1. Development Rules (constraints copied for execution context)

- **Pure translator.** `translateQuery(q, m)` maps an `orm.Query` → `(sql, args, error)`. It does
  **not** execute or read the DB — so it **cannot** check column existence. Keep it pure.
- **SQLite dialect.** Reuse the existing `sqliteType()` map.
- **No `IF NOT EXISTS` on `ADD COLUMN`.** SQLite has no such clause. The plain statement **errors** if
  the column exists; **idempotency is delegated to `db.Sync`'s additive log-and-continue** when no
  introspector is present, or skipped entirely when `tinywasm/sqlite` provides `TableIntrospector`
  (then `db.Sync` only adds genuinely-missing columns). This compiler emits the plain statement.
- **Additive only, nullable.** Emit only `<name> <type>` on `ADD COLUMN`. No PK/UNIQUE/AUTOINCREMENT
  (SQLite forbids them via `ALTER TABLE ADD COLUMN`).
- **`DROP COLUMN` / `RENAME COLUMN`** are supported by modern SQLite (3.25+/3.35+). Emit them plainly;
  the executor module documents the version floor.
- **`gotest` (not `go test`).** `translateQuery` is pure; assert the SQL string.
- **Documentation first.**

---

## 2. Problem

`translateQuery` ([translate.go:10](../translate.go#L10)) `default`-errors on `ActionAddColumn`,
`ActionRenameColumn`, `ActionDropColumn` (`"unknown query action"`). And `buildConditions`
([translate.go:243](../translate.go#L243)) always emits `field op ?` with a bind arg — for the
safe-drop check's `IS NOT NULL` (no value) that produces the broken `col IS NOT NULL ?`. So
`db.Sync`'s per-column steps and its reconcile check can't be compiled for SQLite.

---

## 3. Decision

### 3.1 Translate the column actions

Add cases to `translateQuery`'s `switch`, each delegating to a small builder:

```go
case orm.ActionAddColumn:
    return buildAddColumn(q)
case orm.ActionRenameColumn:
    return buildRenameColumn(q)
case orm.ActionDropColumn:
    return buildDropColumn(q)
```

```go
func buildAddColumn(q orm.Query) (string, []any, error) {
    if q.Column == nil || q.Table == "" {
        return "", nil, fmt.Err("table and column required for add column")
    }
    return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
        q.Table, q.Column.Name, sqliteType(q.Column.Type)), nil, nil
}

func buildRenameColumn(q orm.Query) (string, []any, error) {
    if q.Column == nil || q.OldName == "" || q.Table == "" {
        return "", nil, fmt.Err("table, old name and column required for rename")
    }
    return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
        q.Table, q.OldName, q.Column.Name), nil, nil
}

func buildDropColumn(q orm.Query) (string, []any, error) {
    if q.Table == "" || len(q.Columns) == 0 {
        return "", nil, fmt.Err("table and column required for drop column")
    }
    return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", q.Table, q.Columns[0]), nil, nil
}
```

### 3.2 Null-operator conditions

In `buildConditions` ([translate.go:217](../translate.go#L217)), special-case no-value operators
before the bind-arg branch:

```go
op := c.Operator()
switch {
case op == "IS NULL" || op == "IS NOT NULL":
    clause = fmt.Sprintf("%s %s", c.Field(), op) // no "?" , no arg
case op == "IN":
    // ...existing...
default:
    clause = fmt.Sprintf("%s %s ?", c.Field(), op)
    args = append(args, c.Value())
}
```
(Needed so the safe-drop `SELECT 1 FROM t WHERE col IS NOT NULL LIMIT 1` compiles.)

---

## 4. Implementation Steps

### Step 1 — Bump orm
`go get github.com/tinywasm/orm@vX` (the three column actions + `Query.Column`/`OldName`/`Columns`).

### Step 2 — Column actions
[translate.go](../translate.go): add the three `case`s + `buildAddColumn`/`buildRenameColumn`/
`buildDropColumn` (§3.1).

### Step 3 — Null operators
[translate.go](../translate.go): special-case `IS NULL`/`IS NOT NULL` in `buildConditions` (§3.2).

### Step 4 — Documentation
Note that `sqlt` is the SQLite **compiler**; registration + executor concerns live in
`tinywasm/sqlite`.

---

## 5. Edge Cases

- **`q.Column == nil` / empty table / empty `Columns`** → explicit error per builder.
- **Column already exists** (`ADD COLUMN`) → plain statement errors; absorbed by `db.Sync`
  (additive log-and-continue) or avoided entirely when the executor exposes `TableIntrospector`.
- **Column marked NOT NULL/PK/unique** → emitted plain `<name> <type>` (additive + SQLite rules).
- **`IS NOT NULL` condition** → no placeholder/arg (§3.2).

---

## 6. Test Strategy

`gotest` via the exported `Translate(q, m)` hook ([sqlt.go](../sqlt.go)).

| # | Case | Assert |
|---|------|--------|
| T1 | `ActionAddColumn`, `FieldText` | `ALTER TABLE x ADD COLUMN c TEXT` |
| T2 | `ActionAddColumn`, `FieldInt` | `... c INTEGER` (via `sqliteType`) |
| T3 | `ActionAddColumn`, col marked NOT NULL/PK | plain `<name> <type>`, no constraints |
| T4 | `ActionRenameColumn` | `ALTER TABLE x RENAME COLUMN old TO new` |
| T5 | `ActionDropColumn` | `ALTER TABLE x DROP COLUMN c` |
| T6 | guards: nil column / empty old name / empty table | error returned |
| T7 | `buildConditions` with `IsNotNull(col)` | `... col IS NOT NULL` (no `?`, no arg) |

---

## 7. Out of Scope

- Engine registration (`orm.Register("sqlite", …)`), `orm.ErrNoRows` mapping, and `TableIntrospector`
  — owned by the **executor** module `tinywasm/sqlite` (its plan).
- `db.Sync` / `db.SyncSchema` and its log-and-continue / reconcile — orm core plan.
- Destructive type-change / table rebuild — deferred.
