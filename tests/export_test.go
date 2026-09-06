package tests

import (
	"os"
	"testing"

	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/sqlt"
)

func TestAutoIndex_SQLite(t *testing.T) {
	c := sqlt.NewCompiler()
	m := &testModelFK{}
	p := &testModelParent{}
	got, err := c.ExportDDL([]model.Model{m, p})
	if err != nil {
		t.Fatal(err)
	}

	want := "CREATE INDEX IF NOT EXISTS idx_sessions_user_id ON sessions(user_id)"
	if !fmt.Contains(got, want) {
		t.Errorf("expected index SQL, got %q", got)
	}
}

func TestExportDDL_EmptyInput(t *testing.T) {
	c := sqlt.NewCompiler()

	got, err := c.ExportDDL(nil)
	if err != nil {
		t.Fatal(err)
	}
	want := "-- dialect: sqlite\n\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}

	got, err = c.ExportDDL([]model.Model{})
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestExportDDL_FullSchema(t *testing.T) {
	goldenBytes, err := os.ReadFile("schema.sql")
	if err != nil {
		t.Fatal(err)
	}
	golden := string(goldenBytes)

	users := &modelFull{
		name: "users",
		fields: []model.Field{
			{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true, AutoInc: true}},
			{Name: "username", Type: model.Text(), NotNull: true, DB: &model.FieldDB{Unique: true}, Permitted: model.Permitted{Maximum: 50}},
			{Name: "email", Type: model.Text(), NotNull: true, DB: &model.FieldDB{Unique: true}},
			{Name: "score", Type: model.Float()},
			{Name: "active", Type: model.Bool()},
			{Name: "avatar", Type: model.Blob()},
		},
	}

	roles := &modelFull{
		name: "roles",
		fields: []model.Field{
			{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true, AutoInc: true}},
			{Name: "name", Type: model.Text(), NotNull: true, DB: &model.FieldDB{Unique: true}, Permitted: model.Permitted{Maximum: 100}},
		},
	}

	sessions := &modelFull{
		name: "sessions",
		fields: []model.Field{
			{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
			{Name: "user_id", Type: model.Int()},
			{Name: "metadata", Type: model.Text()},
		},
		ext: []model.FieldExt{
			{Field: model.Field{Name: "user_id", Type: model.Int()}, Ref: "users"},
		},
	}

	userRoles := &modelFull{
		name: "user_roles",
		fields: []model.Field{
			{Name: "user_id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
			{Name: "role_id", Type: model.Int(), DB: &model.FieldDB{PK: true}},
		},
		ext: []model.FieldExt{
			{Field: model.Field{Name: "user_id", Type: model.Int()}, Ref: "users"},
			{Field: model.Field{Name: "role_id", Type: model.Int()}, Ref: "roles"},
		},
	}

	c := sqlt.NewCompiler()
	got, err := c.ExportDDL([]model.Model{users, roles, sessions, userRoles})
	if err != nil {
		t.Fatal(err)
	}

	if i := fmt.Index(golden, "CREATE TABLE"); i != -1 {
		golden = "-- dialect: sqlite\n\n" + golden[i:]
	}

	if fmt.TrimSpace(got) != fmt.TrimSpace(golden) {
		t.Errorf("DDL export does not match golden file.\nGOT:\n%s\n\nWANT:\n%s", got, golden)
	}
}

type modelFull struct {
	dummyModel
	name   string
	fields []model.Field
	ext    []model.FieldExt
}

func (m *modelFull) ModelName() string           { return m.name }
func (m *modelFull) Schema() []model.Field       { return m.fields }
func (m *modelFull) SchemaExt() []model.FieldExt { return m.ext }
func (m *modelFull) Pointers() []any             { return nil }
