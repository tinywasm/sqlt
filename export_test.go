package sqlt_test

import (
	"os"
	"strings"
	"testing"

	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/sqlt"
)

func TestAutoIndex_SQLite(t *testing.T) {
	c := sqlt.NewCompiler()
	m := &testModelFK{}     // from translate_test.go
	p := &testModelParent{} // from translate_test.go
	got, err := c.(interface {
		ExportDDL([]fmt.Model) (string, error)
	}).ExportDDL([]fmt.Model{m, p})
	if err != nil {
		t.Fatal(err)
	}

	// Assert: output contains "CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)"
	want := "CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)"
	if !strings.Contains(got, want) {
		t.Errorf("expected index SQL, got %q", got)
	}
}

func TestExportDDL_EmptyInput(t *testing.T) {
	c := sqlt.NewCompiler()
	exporter := c.(interface {
		ExportDDL([]fmt.Model) (string, error)
	})

	got, err := exporter.ExportDDL(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "-- dialect: sqlite\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	got, err = exporter.ExportDDL([]fmt.Model{})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExportDDL_FullSchema(t *testing.T) {
	goldenBytes, err := os.ReadFile("tests/schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	golden := string(goldenBytes)

	users := &modelFull{
		name: "users",
		fields: []fmt.Field{
			{Name: "id", Type: fmt.FieldInt, DB: &fmt.FieldDB{PK: true, AutoInc: true}},
			{Name: "username", Type: fmt.FieldText, NotNull: true, DB: &fmt.FieldDB{Unique: true}, Permitted: fmt.Permitted{Maximum: 50}},
			{Name: "email", Type: fmt.FieldText, NotNull: true, DB: &fmt.FieldDB{Unique: true}},
			{Name: "score", Type: fmt.FieldFloat},
			{Name: "active", Type: fmt.FieldBool},
			{Name: "avatar", Type: fmt.FieldBlob},
		},
	}

	roles := &modelFull{
		name: "roles",
		fields: []fmt.Field{
			{Name: "id", Type: fmt.FieldInt, DB: &fmt.FieldDB{PK: true, AutoInc: true}},
			{Name: "name", Type: fmt.FieldText, NotNull: true, DB: &fmt.FieldDB{Unique: true}, Permitted: fmt.Permitted{Maximum: 100}},
		},
	}

	sessions := &modelFull{
		name: "sessions",
		fields: []fmt.Field{
			{Name: "id", Type: fmt.FieldText, DB: &fmt.FieldDB{PK: true}},
			{Name: "user_id", Type: fmt.FieldInt},
			{Name: "metadata", Type: fmt.FieldText}, // RawJSON -> FieldRaw -> FieldText (in stub)
		},
		ext: []orm.FieldExt{
			{
				Field: fmt.Field{Name: "user_id", Type: fmt.FieldInt},
				Ref:   "users",
			},
		},
	}

	userRoles := &modelFull{
		name: "user_roles",
		fields: []fmt.Field{
			{Name: "user_id", Type: fmt.FieldInt, DB: &fmt.FieldDB{PK: true}},
			{Name: "role_id", Type: fmt.FieldInt, DB: &fmt.FieldDB{PK: true}},
		},
		ext: []orm.FieldExt{
			{
				Field: fmt.Field{Name: "user_id", Type: fmt.FieldInt},
				Ref:   "users",
			},
			{
				Field: fmt.Field{Name: "role_id", Type: fmt.FieldInt},
				Ref:   "roles",
			},
		},
	}

	c := sqlt.NewCompiler()
	exporter := c.(interface {
		ExportDDL([]fmt.Model) (string, error)
	})

	got, err := exporter.ExportDDL([]fmt.Model{users, roles, sessions, userRoles})
	if err != nil {
		t.Fatal(err)
	}

	// The golden file has a header and trailing newlines. ExportDDL has its own header.
	// We want to compare the SQL content, ignoring the fixture header in golden.
	sqlPart := got
	if i := strings.Index(golden, "CREATE TABLE"); i != -1 {
		golden = "-- dialect: sqlite\n\n" + golden[i:]
	}

	if strings.TrimSpace(sqlPart) != strings.TrimSpace(golden) {
		t.Errorf("DDL export does not match golden file.\nGOT:\n%s\n\nWANT:\n%s", sqlPart, golden)
	}
}

type modelFull struct {
	name   string
	fields []fmt.Field
	ext    []orm.FieldExt
}

func (m *modelFull) ModelName() string { return m.name }
func (m *modelFull) Schema() []fmt.Field { return m.fields }
func (m *modelFull) SchemaExt() []orm.FieldExt { return m.ext }
func (m *modelFull) Pointers() []any { return nil }
