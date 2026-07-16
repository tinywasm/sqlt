---
PLAN: "refactor!: sqlt implementa db.Conn + ddl.Compiler (contrato movido de orm a tinywasm/db)"
TAG: v0.1.0
---

# PLAN — `tinywasm/sqlt`: migrar de `orm.Compiler` a `db.Compiler` + `ddl.Compiler`

Orquestado por
[`DB_PORT_MASTER_PLAN.md`](https://github.com/tinywasm/app-releases/blob/main/docs/DB_PORT_MASTER_PLAN.md)
— **pieza #4**. Autocontenido, en español. **Solo tienes este repo** (`github.com/tinywasm/sqlt`).

> **Prerequisito:** `go install github.com/tinywasm/devflow/cmd/gotest@latest`.
> Tests con `gotest`. Publica con `gopush 'mensaje'`.
> Este plan **requiere `tinywasm/db@v0.0.1` y `tinywasm/ddl@v0.0.1` ya publicados**. Si no resuelven
> en `go get`, para y repórtalo.

## 0. Qué cambió respecto a la versión anterior de este plan

Antes: `sqlt` iba a implementar `orm.Compiler` (DML) + `ddl.Compiler` (DDL), probando
`orm/conformance`+`ddl/conformance`. Eso asumía que `orm` seguía siendo dueño del contrato de
almacenamiento. Ya no lo es: el contrato completo (DML) se extrajo a `tinywasm/db`. Ahora:

- `sqlt` implementa **`db.Compiler`** (no `orm.Compiler` — `orm` ni se importa) para DML, y
  **`ddl.Compiler`** para DDL (sin cambios de intención respecto a la versión anterior de este plan).
- Se prueba contra **`db/conformance`** (no `orm/conformance`) + `ddl/conformance`.
- `sqlt` **no** necesita saber nada de `orm`. Su `go.mod` final: `db`+`ddl`+`ddlc`+`model`+`fmt` (+
  `modernc.org/sqlite` para los tests de conformidad).

## 1. Qué se hace y por qué

`sqlt` es el **compilador SQLite puro** — traduce `db.Query`/`ddl.Stmt` a SQL de sqlite. No abre
conexiones, no ejecuta nada (eso lo hace `tinywasm/sqlite`, un repo hermano que envuelve un `*sql.DB`
real en un `db.Conn` — fuera de alcance de este plan, tiene el suyo propio). `sqlt` entra en **ambos**
contratos ejecutables porque es un compilador SQL completo (DML+DDL):

- **`db/conformance`** (DML): que el SQL de datos que genera ejecuta y da round-trip correcto.
- **`ddl/conformance`** (DDL): que el DDL que genera crea el esquema correcto.

## 2. Estado verificado (código actual del repo, antes de este plan)

- `sqlt.NewCompiler() *compiler` implementa `orm.Compiler` (`compiler.go:14`, `Compile(q orm.Query, m
  model.Model) (orm.Plan, error)`) y `ddlc.Exporter` (`compiler.go:27`, `ExportDDL(models
  []model.Model) (string, error)` — internamente llama `c.Compile(orm.Query{Action:
  orm.ActionCreateTable, ...})`, mezclando DDL dentro de la ruta DML — esto cambia, ver §3.2).
- `translate.go:translateQuery(q orm.Query, m model.Model)` (línea 11) es un `switch q.Action` que
  despacha a `buildInsert`/`buildSelect`/`buildUpdate`/`buildDelete` (DML) y
  `buildCreateTable`/`buildDropTable`/`buildAddColumn`/`buildRenameColumn`/`buildDropColumn` (DDL) —
  **todos en un solo switch**, todos tipados sobre `orm.Query`/`orm.Condition`. Este plan **separa el
  switch en dos funciones** (§3.2), una por tipo de contrato — el **cuerpo SQL de cada `buildX` no
  cambia**, solo el tipo del parámetro `q` y de dónde vienen `Action`/`Condition`.
- `sqlt.go:Translate(q orm.Query, m model.Model)` es un export público de `translateQuery` — se
  divide igual (§3.3).
- `sqlt` depende hoy de `orm@v0.9.27` + `ddlc@v0.0.2`. No depende de ningún driver ni de
  `database/sql` — eso no cambia.

## 3. Cambios

### 3.1 `go.mod`

```
go get github.com/tinywasm/db@v0.0.1
go get github.com/tinywasm/ddl@v0.0.1
go get modernc.org/sqlite@latest   # driver, SOLO para los tests de conformidad
go mod tidy                         # esto debe QUITAR github.com/tinywasm/orm por completo
```

### 3.2 `compiler.go` — dos compiladores, uno por contrato

```go
package sqlt

import (
	"github.com/tinywasm/db"
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/ddlc"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
)

// compiler implements db.Compiler (DML) and ddl.Compiler (DDL) — two distinct methods, no
// shared switch. It also implements ddlc.Exporter for build-time DDL generation.
type compiler struct{}

// Compile converts a db.Query (DML only — Create/ReadOne/ReadAll/Update/Delete) into an
// engine Plan.
func (c compiler) Compile(q db.Query, m model.Model) (db.Plan, error) {
	sqlStr, args, err := translateQuery(q, m)
	if err != nil {
		return db.Plan{}, err
	}
	return db.Plan{Mode: q.Action, Query: sqlStr, Args: args}, nil
}

// CompileDDL converts a ddl.Stmt (CreateTable/DropTable/AddColumn/RenameColumn/DropColumn) into
// SQL. Distinct method, distinct switch (§3.4) — DDL never flows through Compile anymore.
func (c compiler) CompileDDL(s ddl.Stmt, m model.Model) (string, []any, error) {
	return translateDDL(s, m)
}

func (c *compiler) ExportDDL(models []model.Model) (string, error) {
	sorted, err := ddlc.TopologicalSort(models)
	if err != nil {
		return "", err
	}
	var buf fmt.Builder
	buf.Write("-- dialect: sqlite\n\n")
	for _, m := range sorted {
		sql, _, err := c.CompileDDL(ddl.Stmt{Op: ddl.OpCreateTable, Table: m.ModelName()}, m)
		if err != nil {
			return "", err
		}
		buf.Write(sql)
		buf.Write(";\n\n")
		if ext, ok := m.(interface{ SchemaExt() []ddlc.FieldExt }); ok {
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

var (
	_ db.Compiler  = (*compiler)(nil)
	_ ddl.Compiler = (*compiler)(nil)
	_ ddlc.Exporter = (*compiler)(nil)
)
```

> **Cambio de fondo respecto al código actual:** `ExportDDL` ya **no** pasa por `Compile`
> (`db.Compiler`) — pasa por `CompileDDL` (`ddl.Compiler`) directamente, con `ddl.Stmt{Op:
> ddl.OpCreateTable, ...}` en vez de `orm.Query{Action: orm.ActionCreateTable, ...}`. Antes esto
> "colaba" DDL por la ruta DML porque `orm.Compiler` hacía ambos; ahora que están separados, `ExportDDL`
> debe llamar al método que realmente le corresponde.

### 3.3 `translate.go` — separar el switch en DML y DDL

```go
package sqlt

import (
	"github.com/tinywasm/db"
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
)

// translateQuery converts a db.Query (DML only) into a SQLite SQL string and arguments.
func translateQuery(q db.Query, m model.Model) (string, []any, error) {
	switch q.Action {
	case db.ActionCreate:
		return buildInsert(q)
	case db.ActionReadOne, db.ActionReadAll:
		return buildSelect(q)
	case db.ActionUpdate:
		return buildUpdate(q)
	case db.ActionDelete:
		return buildDelete(q)
	default:
		return "", nil, fmt.Errf("sqlt: unknown DML action: %v", q.Action)
	}
}

// translateDDL converts a ddl.Stmt into a SQLite SQL string and arguments. Separate dispatch,
// separate types — DDL never reaches translateQuery and vice versa.
func translateDDL(s ddl.Stmt, m model.Model) (string, []any, error) {
	switch s.Op {
	case ddl.OpCreateTable:
		return buildCreateTable(s, m)
	case ddl.OpDropTable:
		return buildDropTable(s)
	case ddl.OpAddColumn:
		return buildAddColumn(s)
	case ddl.OpRenameColumn:
		return buildRenameColumn(s)
	case ddl.OpDropColumn:
		return buildDropColumn(s)
	default:
		return "", nil, fmt.Errf("sqlt: unknown DDL op: %v", s.Op)
	}
}
```

**El cuerpo de cada `buildX` no cambia de lógica SQL** — solo su firma y de dónde lee los campos:

| Función | Firma antes | Firma ahora | Qué cambia dentro |
|---|---|---|---|
| `buildInsert`/`buildSelect`/`buildUpdate`/`buildDelete` | `(q orm.Query) (string, []any, error)` | `(q db.Query) (string, []any, error)` | Nada más que el tipo de `q`; `q.Table`/`q.Columns`/`q.Values`/`q.Conditions`/`q.OrderBy`/`q.Limit`/`q.Offset` existen igual en `db.Query`. |
| `buildConditions` | `(conditions []orm.Condition) (string, []any, error)` | `(conditions []db.Condition) (string, []any, error)` | Solo el tipo; `Condition.Field()`/`Operator()`/`Value()`/`Logic()` son los mismos getters. |
| `buildCreateTable` | `(q orm.Query, m model.Model)` | `(s ddl.Stmt, m model.Model)` | Antes leía `q.Table`; ahora lee `s.Table`. El resto (generar columnas desde `m.Schema()`) no cambia. |
| `buildDropTable` | `(q orm.Query)` | `(s ddl.Stmt)` | `q.Table` → `s.Table`. |
| `buildAddColumn` | `(q orm.Query)` — leía `q.Column *model.Field` | `(s ddl.Stmt)` — lee `s.Column *model.Field` | Mismo campo, mismo tipo, distinto contenedor. |
| `buildRenameColumn` | `(q orm.Query)` — leía `q.Column`, `q.OldName` | `(s ddl.Stmt)` — lee `s.Column`, `s.OldName` | Igual. |
| `buildDropColumn` | `(q orm.Query)` — leía `q.Columns []string` (¡ojo, un slice!) | `(s ddl.Stmt)` — lee `s.ColumnName string` | **Cambio real de forma**: antes `DropColumn` reusaba el slice `Columns` de `orm.Query` con un solo elemento; `ddl.Stmt` tiene un campo dedicado `ColumnName string` (ver `ddl/docs/PLAN.md` §3.1). Ajusta `buildDropColumn` para leer `s.ColumnName` directo, sin slice. |

### 3.4 `sqlt.go` — el export público `Translate` se divide igual

```go
package sqlt

import (
	"github.com/tinywasm/db"
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/model"
)

func NewCompiler() *compiler { return &compiler{} }

// Translate exposes the DML translation for callers that want the SQL string without a full
// Compile round-trip (e.g. debugging, translate_test.go).
func Translate(q db.Query, m model.Model) (string, []any, error) {
	return translateQuery(q, m)
}

// TranslateDDL is the DDL counterpart — new export, mirrors Translate.
func TranslateDDL(s ddl.Stmt, m model.Model) (string, []any, error) {
	return translateDDL(s, m)
}
```

### 3.5 `conformance_test.go` (`package sqlt_test`) — ambas suites, sobre `db`/`ddl`

```go
package sqlt_test

import (
	"database/sql"
	"testing"

	"github.com/tinywasm/db"
	dbconf "github.com/tinywasm/db/conformance"
	"github.com/tinywasm/ddl"
	ddlconf "github.com/tinywasm/ddl/conformance"
	"github.com/tinywasm/model"
	"github.com/tinywasm/sqlt"

	_ "modernc.org/sqlite"
)

// sqlConn implements db.Conn (Executor+Compiler) directly over a *sql.DB — this is test-only
// wiring, the same role tinywasm/sqlite plays for real consumers (see its own PLAN.md). sqlt
// itself never imports database/sql outside tests.
type sqlConn struct {
	*sql.DB
	compiler *sqltCompilerAlias // placeholder name — use sqlt.NewCompiler()'s returned type directly
}

func (c *sqlConn) Exec(q string, a ...any) error             { _, err := c.DB.Exec(q, a...); return err }
func (c *sqlConn) QueryRow(q string, a ...any) db.Scanner     { return &noRows{c.DB.QueryRow(q, a...)} }
func (c *sqlConn) Query(q string, a ...any) (db.Rows, error)  { return c.DB.Query(q, a...) }
func (c *sqlConn) Close() error                               { return c.DB.Close() }
func (c *sqlConn) Compile(q db.Query, m model.Model) (db.Plan, error) { return c.compiler.Compile(q, m) }

type noRows struct{ s *sql.Row }
func (r *noRows) Scan(d ...any) error {
	err := r.s.Scan(d...)
	if err == sql.ErrNoRows {
		return db.ErrNoRows
	}
	return err
}

func openMem(t *testing.T) *sql.DB {
	raw, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	raw.SetMaxOpenConns(1)
	raw.SetMaxIdleConns(1) // :memory: is per-connection
	return raw
}

// DML: schema comes from ddlc.ExportDDL (db is DML-only, never CreateTable here).
func TestSqlt_DBConformance(t *testing.T) {
	dbconf.Run(t, dbconf.Factory{
		Name: "sqlt",
		New: func(t *testing.T, models ...model.Model) db.Conn {
			raw := openMem(t)
			c := sqlt.NewCompiler()
			ddlSQL, err := c.ExportDDL(models)
			if err != nil {
				t.Fatalf("ExportDDL: %v", err)
			}
			if _, err := raw.Exec(ddlSQL); err != nil {
				t.Fatalf("apply DDL: %v", err)
			}
			return &sqlConn{DB: raw, compiler: c}
		},
	})
}

// DDL: drive tinywasm/ddl runtime with sqlt's ddl.Compiler.
func TestSqlt_DDLConformance(t *testing.T) {
	ddlconf.Run(t, ddlconf.Factory{
		Name: "sqlt",
		New: func(t *testing.T) (schema *ddl.DB, conn db.Conn, cols func(string) []string) {
			raw := openMem(t)
			c := sqlt.NewCompiler()
			sc := &sqlConn{DB: raw, compiler: c}
			schema = ddl.New(sc, c)
			cols = func(table string) []string { /* PRAGMA table_info(table) → column names, 2nd col of each row */ return nil }
			return schema, sc, cols
		},
	})
}
```

> Ajusta `cols` a la introspección real de SQLite (`PRAGMA table_info(<t>)`), leyendo la 2ª columna
> (name) de cada fila. La firma exacta de `ddlconf.Factory` viene de `tinywasm/ddl/docs/PLAN.md` §3.3
> — léela y adáptala si difiere. El nombre `sqltCompilerAlias` en el boceto de `sqlConn` es un
> placeholder de forma: usa directamente el tipo que devuelve `sqlt.NewCompiler()` (no exportado hoy;
> si necesitas nombrarlo en la firma del struct, usa `*compiler` si estás en `package sqlt`, o
> reestructura `sqlConn` para no necesitar nombrar el tipo — por ejemplo guardando un `db.Compiler`
> genérico en vez del tipo concreto).

## 4. Si alguna suite se pone en rojo → corregir `translate.go`

Nunca la suite ni el modelo `Widget`. Puntos: placeholders `?`/orden de args, DDL de tipos válidos,
`IN (?, ?)` con `[]any`, `ORDER BY/LIMIT/OFFSET`, booleanos 0/1↔bool, `ReadOne` sin match ⇒
`db.ErrNoRows`, `ALTER TABLE ADD COLUMN` para `sync_adds_new_column`, `buildDropColumn` leyendo
`s.ColumnName` (no un slice) tras el cambio de §3.3.

## 5. Criterios de aceptación

- `*compiler` implementa `db.Compiler` **y** `ddl.Compiler` **y** `ddlc.Exporter` (`var _` de los
  tres). **Cero** `github.com/tinywasm/orm` en todo el repo (`grep -rn "tinywasm/orm" .` vacío).
- `TestSqlt_DBConformance` verde: schema vía `ExportDDL`, DML round-trip sobre `db/conformance`.
- `TestSqlt_DDLConformance` verde: `ddl.DB`+`CompileDDL` crean/migran esquema correcto.
- `sqlt` **no** importa `tinywasm/sqlite`; `modernc.org/sqlite` solo lo usa el test.
- `translate_test.go` adaptado a los nuevos tipos, sigue verde; `go mod tidy` limpio; publicado con
  `gopush`.

## 6. Etapas

| # | Etapa | Archivo(s) | Criterio |
|---|---|---|---|
| 1 | Bump db+ddl+driver, quitar orm | `go.mod` | `db@v0.0.1`, `ddl@v0.0.1`, `modernc.org/sqlite`; `orm` fuera |
| 2 | `Compile`+`CompileDDL` separados | `compiler.go` | `var _ db.Compiler`, `var _ ddl.Compiler` (§3.2) |
| 3 | Switch DML/DDL separados | `translate.go` | `translateQuery`/`translateDDL` (§3.3), `buildDropColumn` usa `ColumnName` |
| 4 | Export público dividido | `sqlt.go` | `Translate`/`TranslateDDL` (§3.4) |
| 5 | Test DML | `conformance_test.go` | `dbconf.Run` verde |
| 6 | Test DDL | `conformance_test.go` | `ddlconf.Run` verde |
| 7 | Correcciones (si aplica) | `translate.go` | ambas suites verdes |
| 8 | Publicar | — | `gotest` verde; `gopush 'refactor!: db+ddl conformance'` |

## 7. Cierre

Tras `gopush`, **borra** `docs/PLAN.md`; el diseño duradero a `README.md`.
