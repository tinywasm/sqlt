package tests

import (
	"database/sql"
	"testing"

	"webtyp.com/ddl"
	ddlconf "webtyp.com/ddl/conformance"
	"webtyp.com/model"
	"webtyp.com/sqlt"
	"webtyp.com/storage"
	dbconf "webtyp.com/storage/conformance"

	_ "modernc.org/sqlite"
)

type sqlConn struct {
	*sql.DB
	compiler storage.Compiler
}

func (c *sqlConn) Exec(q string, a ...any) error {
	_, err := c.DB.Exec(q, a...)
	return err
}

func (c *sqlConn) QueryRow(q string, a ...any) storage.Scanner {
	return &noRows{c.DB.QueryRow(q, a...)}
}

func (c *sqlConn) Query(q string, a ...any) (storage.Rows, error) {
	return c.DB.Query(q, a...)
}

func (c *sqlConn) Close() error {
	return c.DB.Close()
}

func (c *sqlConn) Compile(q storage.Query, m model.Model) (storage.Plan, error) {
	return c.compiler.Compile(q, m)
}

type noRows struct {
	s *sql.Row
}

func (r *noRows) Scan(d ...any) error {
	err := r.s.Scan(d...)
	if err == sql.ErrNoRows {
		return storage.ErrNoRows
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

func TestSqlt_DBConformance(t *testing.T) {
	dbconf.Run(t, dbconf.Factory{
		Name: "sqlt",
		New: func(t *testing.T, models ...model.Model) storage.Conn {
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

func TestSqlt_DDLConformance(t *testing.T) {
	ddlconf.Run(t, ddlconf.Factory{
		Name: "sqlt",
		New: func(t *testing.T) (schema *ddl.DB, conn storage.Conn, cols func(string) []string) {
			raw := openMem(t)
			c := sqlt.NewCompiler()
			sc := &sqlConn{DB: raw, compiler: c}
			schema = ddl.New(sc, c)
			cols = func(table string) []string {
				rows, err := raw.Query("PRAGMA table_info(" + table + ")")
				if err != nil {
					t.Fatalf("PRAGMA table_info error: %v", err)
				}
				defer rows.Close()
				var names []string
				for rows.Next() {
					var cid int
					var name string
					var typeStr string
					var notnull int
					var dfltValue any
					var pk int
					if err := rows.Scan(&cid, &name, &typeStr, &notnull, &dfltValue, &pk); err != nil {
						t.Fatalf("scan PRAGMA table_info error: %v", err)
					}
					names = append(names, name)
				}
				return names
			}
			return schema, sc, cols
		},
	})
}
