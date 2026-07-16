---
PLAN: "test: sqlt prueba orm/conformance (DML) y ddl/conformance (DDL); su compilador implementa ddl.Compiler"
TAG: v0.0.8
---

# PLAN — `tinywasm/sqlt`: probar `orm/conformance` + `ddl/conformance`

Orquestado por [`DDL_DML_SPLIT_MASTER_PLAN.md`](https://github.com/tinywasm/app-releases/blob/main/docs/DDL_DML_SPLIT_MASTER_PLAN.md)
— **pieza #3, Ola A**. Autocontenido, en español. **Solo tienes este repo** (`github.com/tinywasm/sqlt`).

> **Prerequisito:** `go install github.com/tinywasm/devflow/cmd/gotest@latest`.
> Tests con `gotest`. Publica con `gopush 'mensaje'`.

## 1. Qué se hace y por qué

`sqlt` es el **compilador SQLite** de `tinywasm/orm`. Con el split DDL/DML (ver master) hay **dos**
contratos ejecutables que probar, y `sqlt` entra en **ambos** (es un backend SQL completo):

- **`orm/conformance`** (DML): que el SQL de datos que genera **ejecuta** y round-trip correcto.
- **`ddl/conformance`** (DDL, repo `tinywasm/ddl`): que el DDL que genera crea el esquema correcto.

Además, el split mueve la **mitad DDL** del compilador a una interfaz propia `ddl.Compiler`: hoy
`translate.go` compila DML y DDL en un solo `Compile`; ahora las ramas DDL (`ActionCreateTable`,
`DropTable`, `AddColumn`, `RenameColumn`, `DropColumn`, `CreateDatabase`) se exponen vía
`CompileDDL(ddl.Stmt, model.Model)` para que el runtime `tinywasm/ddl` las ejecute. La generación SQL
**no se reescribe**, solo se mueve detrás del nuevo método.

## 2. Estado verificado

- `sqlt.NewCompiler() *compiler` implementa `orm.Compiler` (`sqlt.go:9`) y `ddlc.Exporter`
  (`compiler.go:27` `ExportDDL(models []model.Model) (string, error)`).
- `translate.go` maneja `ActionCreateTable` (`CREATE TABLE IF NOT EXISTS`, línea 45), `ActionDropTable`,
  y las `AddColumn/RenameColumn/DropColumn` para `Sync`, más Create/ReadOne/ReadAll/Update/Delete.
- `sqlt` ya depende de `ddlc v0.0.2`; **no** depende de ningún driver ni de `database/sql`.
- `sqlt.Translate(q, m)` expone la traducción; `translate_test.go` compara strings (se mantiene).

## 3. Cambios

### 3.1 `go.mod`
```
go get github.com/tinywasm/orm@v0.10.0    # trae orm/conformance
go get github.com/tinywasm/ddl@v0.0.1     # runtime DDL + ddl/conformance + ddl.Compiler
go get modernc.org/sqlite@latest          # driver, SOLO para los tests de conformidad
go mod tidy
```

### 3.2 Implementar `ddl.Compiler` — mover la mitad DDL

El `*compiler` gana `CompileDDL(s ddl.Stmt, m model.Model) (string, []any, error)`. Su cuerpo es el
**mismo** SQL que hoy produce `translate.go` para las acciones DDL, mapeando `ddl.Op`→SQL:

| `ddl.Op` | SQL (ya existe en `translate.go`) |
|---|---|
| `OpCreateTable` | `CREATE TABLE IF NOT EXISTS ...` (línea 45) |
| `OpDropTable` | `DROP TABLE ...` |
| `OpAddColumn` | `ALTER TABLE ... ADD COLUMN ...` |
| `OpRenameColumn` | `ALTER TABLE ... RENAME COLUMN ...` |
| `OpDropColumn` | `ALTER TABLE ... DROP COLUMN ...` |
| `OpCreateDatabase` | (no-op/como hoy) |

Añade `var _ ddl.Compiler = sqlt.NewCompiler()`. El `orm.Compiler.Compile` **conserva solo DML** (o
delega las DDL a `CompileDDL` internamente durante la transición — lo que mantenga `translate_test.go`
verde). No dupliques la generación SQL.

### 3.3 `conformance_test.go` (`package sqlt_test`) — ambas suites

Executor mínimo `database/sql` (espejo de `sqlite/executor.go`; `sqlt` **no** importa `tinywasm/sqlite`):

```go
package sqlt_test

import (
	"database/sql"
	"testing"

	"github.com/tinywasm/ddl"
	ddlconf "github.com/tinywasm/ddl/conformance"
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
	ormconf "github.com/tinywasm/orm/conformance"
	"github.com/tinywasm/sqlt"

	_ "modernc.org/sqlite"
)

type sqlExec struct{ db *sql.DB }
func (e *sqlExec) Exec(q string, a ...any) error             { _, err := e.db.Exec(q, a...); return err }
func (e *sqlExec) QueryRow(q string, a ...any) orm.Scanner    { return &noRows{e.db.QueryRow(q, a...)} }
func (e *sqlExec) Query(q string, a ...any) (orm.Rows, error) { return e.db.Query(q, a...) }
func (e *sqlExec) Close() error                              { return e.db.Close() }
type noRows struct{ s *sql.Row }
func (r *noRows) Scan(d ...any) error {
	if err := r.s.Scan(d...); err == sql.ErrNoRows { return orm.ErrNoRows } else { return err }
}

func openMem(t *testing.T) *sql.DB {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil { t.Fatalf("sql.Open: %v", err) }
	db.SetMaxOpenConns(1); db.SetMaxIdleConns(1) // :memory: is per-connection
	return db
}

// DML: schema comes from ddlc.ExportDDL (orm is DML-only, never CreateTable here).
func TestSqlt_ORMConformance(t *testing.T) {
	ormconf.Run(t, ormconf.Factory{
		Name: "sqlt",
		New: func(t *testing.T, models ...model.Model) *orm.DB {
			raw := openMem(t)
			c := sqlt.NewCompiler()
			ddlSQL, err := c.ExportDDL(models)
			if err != nil { t.Fatalf("ExportDDL: %v", err) }
			if _, err := raw.Exec(ddlSQL); err != nil { t.Fatalf("apply DDL: %v", err) }
			return orm.New(&sqlExec{db: raw}, c)
		},
	})
}

// DDL: drive tinywasm/ddl runtime with sqlt's ddl.Compiler.
func TestSqlt_DDLConformance(t *testing.T) {
	ddlconf.Run(t, ddlconf.Factory{
		Name: "sqlt",
		New: func(t *testing.T) (*ddl.DB, orm.Executor, func(string) []string) {
			raw := openMem(t)
			exec := &sqlExec{db: raw}
			schema := ddl.New(exec, sqlt.NewCompiler())
			cols := func(table string) []string { /* PRAGMA table_info(table) → column names */ return nil }
			return schema, exec, cols
		},
	})
}
```

> Ajusta `cols` a la introspección real de SQLite (`PRAGMA table_info(<t>)`), leyendo la 2ª columna
> (name) de cada fila. La firma exacta de `ddlconf.Factory` viene de `tinywasm/ddl` — léela y adáptala si
> difiere del boceto del master.

## 4. Si alguna suite se pone en rojo → corregir `translate.go`

Nunca la suite ni el modelo `Widget`. Puntos: placeholders `?`/orden de args, DDL de tipos válidos,
`IN (?, ?)` con `[]any`, `ORDER BY/LIMIT/OFFSET`, booleanos 0/1↔bool, `ReadOne` sin match ⇒ `ErrNoRows`,
`ALTER TABLE ADD COLUMN` para `sync_adds_new_column`.

## 5. Criterios de aceptación

- `*compiler` implementa `orm.Compiler` **y** `ddl.Compiler` (`var _` de ambos).
- `TestSqlt_ORMConformance` verde: schema vía `ExportDDL`, DML round-trip.
- `TestSqlt_DDLConformance` verde: `ddl.DB`+`CompileDDL` crean/migran esquema correcto.
- `sqlt` **no** importa `tinywasm/sqlite`; `modernc.org/sqlite` solo lo usa el test.
- `translate_test.go` sigue verde; `go mod tidy` limpio; publicado con `gopush`.

## 6. Etapas

| # | Etapa | Archivo(s) | Criterio |
|---|---|---|---|
| 1 | Bump orm+ddl+driver | `go.mod` | `orm@v0.10.0`, `ddl@v0.0.1`, `modernc.org/sqlite` |
| 2 | `CompileDDL` | `compiler.go`/`translate.go` | `var _ ddl.Compiler`; DML/DDL separados |
| 3 | Test DML | `conformance_test.go` | `ormconf.Run` verde |
| 4 | Test DDL | `conformance_test.go` | `ddlconf.Run` verde |
| 5 | Correcciones (si aplica) | `translate.go` | ambas suites verdes |
| 6 | Publicar | — | `gotest` verde; `gopush 'test: orm+ddl conformance'` |

## 7. Cierre

Tras `gopush`, **borra** `docs/PLAN.md`; el diseño duradero a `README.md`.
