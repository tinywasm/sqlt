package tests

import (
	"testing"

	"webtyp.com/ddl"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/sqlt"
)

func TestVarchar_SQLite(t *testing.T) {
	m := &testModelVarchar{}
	sql, _, err := sqlt.TranslateDDL(ddl.Stmt{
		Op:    ddl.OpCreateTable,
		Table: "users",
	}, m)
	if err != nil {
		t.Fatal(err)
	}

	// Assert: buildCreateTable output contains "username VARCHAR(50)"
	if !fmt.Contains(sql, "username VARCHAR(50)") {
		t.Errorf("expected VARCHAR(50) for username, got %q", sql)
	}
	// Assert: field without maximum: "email TEXT"
	if !fmt.Contains(sql, "email TEXT") {
		t.Errorf("expected TEXT for email, got %q", sql)
	}
}

type testModelVarchar struct {
	dummyModel
}

func (m *testModelVarchar) ModelName() string { return "users" }
func (m *testModelVarchar) Schema() []model.Field {
	return []model.Field{
		{Name: "username", Type: model.Text(), Permitted: model.Permitted{Maximum: 50}},
		{Name: "email", Type: model.Text()},
	}
}
func (m *testModelVarchar) Pointers() []any { return nil }

func TestOnDelete_SQLite(t *testing.T) {
	tests := []struct {
		name     string
		onDelete string
		want     string
	}{
		{"default", "", "ON DELETE CASCADE"},
		{"restrict", "restrict", "ON DELETE RESTRICT"},
		{"set_null", "set_null", "ON DELETE SET NULL"},
		{"no_action", "no_action", "ON DELETE NO ACTION"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &testModelFK{onDelete: tt.onDelete}
			sql, _, err := sqlt.TranslateDDL(ddl.Stmt{
				Op:    ddl.OpCreateTable,
				Table: "sessions",
			}, m)
			if err != nil {
				t.Fatal(err)
			}
			if !fmt.Contains(sql, tt.want) {
				t.Errorf("expected %q in sql, got %q", tt.want, sql)
			}
		})
	}
}

type testModelFK struct {
	dummyModel
	onDelete string
}

func (m *testModelFK) ModelName() string { return "sessions" }
func (m *testModelFK) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}},
		{Name: "user_id", Type: model.Int()},
	}
}
func (m *testModelFK) SchemaExt() []model.FieldExt {
	return []model.FieldExt{
		{
			Field:    model.Field{Name: "user_id", Type: model.Int()},
			Ref:      "users",
			OnDelete: m.onDelete,
		},
	}
}
func (m *testModelFK) Pointers() []any { return nil }

type testModelParent struct {
	dummyModel
}

func (m *testModelParent) ModelName() string { return "users" }
func (m *testModelParent) Schema() []model.Field {
	return []model.Field{{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true}}}
}
func (m *testModelParent) Pointers() []any { return nil }
