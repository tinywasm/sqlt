> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.

# Plan: sqlt — SQLite SQL Compiler for tinywasm/orm

## Context

`tinywasm/sqlt` (module `github.com/tinywasm/sqlt`) is a pure-Go SQLite SQL compiler for
`github.com/tinywasm/orm`. It translates `orm.Query` into SQL strings with `?` placeholders
compatible with the SQLite dialect (used by both `tinywasm/sqlite` and Cloudflare D1).

Module root: `tinywasm/sqlt/` (next to `go.mod`).  
No CGo. No `database/sql`. No build tags — pure Go.  
Dependencies: `github.com/tinywasm/fmt`, `github.com/tinywasm/orm`.

Tests run via `gotest` — no TinyGo installation required.

## Current State (files already present)

`translate.go` and `compiler.go` were physically moved from `tinywasm/sqlite` and the package
name was changed to `sqlt`. They are already in the repo but need fixes:

| Archivo | Estado | Acción requerida |
|---|---|---|
| `sqlt/go.mod` | sin deps | agregar `tinywasm/fmt` y `tinywasm/orm` |
| `sqlt/sqlt.go` | placeholder de gonew | reemplazar con `NewCompiler()` y `Translate()` |
| `sqlt/compiler.go` | presente — `type sqliteCompiler struct{}` | renombrar tipo a `compiler` |
| `sqlt/translate.go` | presente — lógica completa | sin cambios |
| `sqlt/sqlt_test.go` | no existe | crear |

## Goal

- `go.mod` con deps correctas.
- `sqlt.go` exporta `NewCompiler()` y `Translate()`.
- `compiler.go` renombra `sqliteCompiler` → `compiler` (el tipo es interno; solo el nombre cambia).
- `sqlt_test.go` cubre todos los `orm.Action`.

## Public API

```go
// NewCompiler returns an orm.Compiler that generates SQLite-dialect SQL.
func NewCompiler() orm.Compiler

// Translate converts an orm.Query into a SQLite SQL string and positional args.
func Translate(q orm.Query, m fmt.Model) (string, []any, error)
```

## TinyWasm Constraints (mandatory)

- No `import "errors"`, `"fmt"`, `"strings"`, `"strconv"` — use `github.com/tinywasm/fmt`.
- No `database/sql`, no CGo, no build tags.

## Code Quality (mandatory)

- SQL keywords como literales string en builders son aceptables (`"INSERT INTO"`, etc.). Solo prefijos de error repetidos van en constantes.
- `Err(...)` / `Errf(...)` de `github.com/tinywasm/fmt` para todos los errores.

## File Structure (final)

```
sqlt/
├── go.mod
├── sqlt.go        # NewCompiler(), Translate() — public entry points
├── compiler.go    # type compiler struct{} + Compile() — ya presente, renombrar tipo
├── translate.go   # translateQuery + build* helpers — ya presente, sin cambios
└── sqlt_test.go   # full coverage
```

---

## Stage 1 — go.mod

Editar `sqlt/go.mod` existente, agregar:

```
require (
    github.com/tinywasm/fmt v0.23.9
    github.com/tinywasm/orm v0.8.1
)
```

## Stage 2 — sqlt.go

Reemplazar el contenido placeholder de gonew (`type Sqlt struct{}`):

```go
package sqlt

import (
    "github.com/tinywasm/fmt"
    "github.com/tinywasm/orm"
)

// NewCompiler returns an orm.Compiler that generates SQLite-dialect SQL.
func NewCompiler() orm.Compiler {
    return compiler{}
}

// Translate converts an orm.Query into a SQLite SQL string and positional args.
func Translate(q orm.Query, m fmt.Model) (string, []any, error) {
    return translateQuery(q, m)
}
```

## Stage 3 — compiler.go

`compiler.go` ya existe con `type sqliteCompiler struct{}`. Solo renombrar el tipo:

- `sqliteCompiler` → `compiler` (en la declaración del tipo y en el receptor del método `Compile`)
- No cambiar ninguna otra línea.

## Stage 4 — sqlt_test.go

Port `TestCompilerErrors` desde `tinywasm/sqlite/sqlite_test.go` — es exactamente la cobertura
de errores de `translateQuery`. Reemplazar `sqlite.ExportTranslateQuery(...)` con `sqlt.Translate(...)`.
Agregar los tests de happy path que no existen en sqlite.

```go
package sqlt_test

import (
    "testing"
    "github.com/tinywasm/fmt"
    "github.com/tinywasm/orm"
    "github.com/tinywasm/sqlt"
)

type testModel struct{}

func (m testModel) ModelName() string { return "users" }
func (m testModel) Schema() []fmt.Field {
    return []fmt.Field{
        {Name: "id",   Type: fmt.FieldInt,  DB: &fmt.FieldDB{PK: true, AutoInc: true}},
        {Name: "name", Type: fmt.FieldText, NotNull: true},
        {Name: "age",  Type: fmt.FieldInt},
    }
}
func (m testModel) Pointers() []any { return nil }
```

### Tests de happy path (nuevos)

| Test | Action | Expected SQL |
|---|---|---|
| `TestTranslateInsert` | `ActionCreate` | `INSERT INTO users (id,name,age) VALUES (?,?,?)` |
| `TestTranslateSelect` | `ActionReadAll` | `SELECT * FROM users` |
| `TestTranslateSelectWhere` | `ActionReadOne` + Conditions | `SELECT * FROM users WHERE id = ?` |
| `TestTranslateUpdate` | `ActionUpdate` | `UPDATE users SET name = ?, age = ? WHERE id = ?` |
| `TestTranslateDelete` | `ActionDelete` | `DELETE FROM users WHERE id = ?` |
| `TestTranslateCreateTable` | `ActionCreateTable` | `CREATE TABLE IF NOT EXISTS users (...)` |
| `TestTranslateDropTable` | `ActionDropTable` | `DROP TABLE IF EXISTS users` |
| `TestCompilerImplementsInterface` | — | `var _ orm.Compiler = sqlt.NewCompiler()` |

### TestCompilerErrors — portar desde sqlite_test.go

Copiar verbatim y reemplazar `sqlite.ExportTranslateQuery(q, m)` → `sqlt.Translate(q, m)`:

```go
func TestCompilerErrors(t *testing.T) {
    // unknown action
    _, _, err := sqlt.Translate(orm.Query{Action: 99}, nil)
    if err == nil { t.Fatal("expected error for unsupported action") }

    // create table without table name
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionCreateTable}, &testModel{})
    if err == nil { t.Fatal("expected error for create table without table") }

    // create table without model
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionCreateTable, Table: "t"}, nil)
    if err == nil { t.Fatal("expected error for create table without model") }

    // drop table without table name
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionDropTable}, nil)
    if err == nil { t.Fatal("expected error for drop table without table") }

    // insert without table
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionCreate, Columns: []string{"id"}}, nil)
    if err == nil { t.Fatal("expected error for insert without table") }

    // insert without columns
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionCreate, Table: "t"}, nil)
    if err == nil { t.Fatal("expected error for insert without columns") }

    // select without table
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionReadOne}, nil)
    if err == nil { t.Fatal("expected error for select without table") }

    // update without table
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionUpdate, Columns: []string{"id"}}, nil)
    if err == nil { t.Fatal("expected error for update without table") }

    // update without columns
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionUpdate, Table: "t"}, nil)
    if err == nil { t.Fatal("expected error for update without columns") }

    // delete without table
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionDelete}, nil)
    if err == nil { t.Fatal("expected error for delete without table") }

    // IN with non-slice value
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionReadAll, Table: "t",
        Conditions: []orm.Condition{orm.In("id", 1)}}, nil)
    if err == nil { t.Fatal("expected error for non-slice IN value") }

    // IN with empty slice
    _, _, err = sqlt.Translate(orm.Query{Action: orm.ActionReadAll, Table: "t",
        Conditions: []orm.Condition{orm.In("id", []any{})}}, nil)
    if err == nil { t.Fatal("expected error for empty slice IN value") }
}
```

> Estos casos estaban en `sqlite_test.go` — son cobertura de `translateQuery`, no del adaptador.

## Stages Summary

| # | Archivo | Acción |
|---|---|---|
| 1 | `sqlt/go.mod` | Agregar `tinywasm/fmt` y `tinywasm/orm` |
| 2 | `sqlt/sqlt.go` | Reemplazar placeholder — `NewCompiler()`, `Translate()` |
| 3 | `sqlt/compiler.go` | Renombrar `sqliteCompiler` → `compiler` |
| 4 | `sqlt/sqlt_test.go` | **YA EXISTE** — fue creado y movido desde `sqlite_test.go`. No recrear. |

## Verification

```bash
gotest
```

All tests must pass. `go vet ./...` must produce no output.
