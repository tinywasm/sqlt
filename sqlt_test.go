package sqlt_test

import (
	"testing"

	"github.com/tinywasm/ddl"
	"github.com/tinywasm/model"
	"github.com/tinywasm/sqlt"
	"github.com/tinywasm/storage"
)

type testModel struct {
	dummyModel
}

func (m testModel) ModelName() string { return "users" }
func (m testModel) Schema() []model.Field {
	return []model.Field{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true, AutoInc: true}},
		{Name: "name", Type: model.Text(), NotNull: true},
		{Name: "age", Type: model.Int()},
	}
}
func (m testModel) Pointers() []any { return nil }

var (
	_ storage.Compiler = sqlt.NewCompiler()
	_ ddl.Compiler     = sqlt.NewCompiler()
)

func TestTranslateInsert(t *testing.T) {
	sql, args, err := sqlt.Translate(storage.Query{
		Action:  storage.ActionCreate,
		Table:   "users",
		Columns: []string{"id", "name", "age"},
		Values:  []any{1, "alice", 30},
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "INSERT INTO users (id, name, age) VALUES (?, ?, ?)"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestTranslateSelect(t *testing.T) {
	sql, _, err := sqlt.Translate(storage.Query{
		Action: storage.ActionReadAll,
		Table:  "users",
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT * FROM users"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
}

func TestTranslateSelectWhere(t *testing.T) {
	sql, args, err := sqlt.Translate(storage.Query{
		Action:     storage.ActionReadOne,
		Table:      "users",
		Conditions: []storage.Condition{storage.Eq("id", 1)},
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT * FROM users WHERE id = ?"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestTranslateUpdate(t *testing.T) {
	sql, args, err := sqlt.Translate(storage.Query{
		Action:     storage.ActionUpdate,
		Table:      "users",
		Columns:    []string{"name", "age"},
		Values:     []any{"bob", 25},
		Conditions: []storage.Condition{storage.Eq("id", 1)},
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "UPDATE users SET name = ?, age = ? WHERE id = ?"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
	if len(args) != 3 {
		t.Errorf("expected 3 args, got %d", len(args))
	}
}

func TestTranslateDelete(t *testing.T) {
	sql, args, err := sqlt.Translate(storage.Query{
		Action:     storage.ActionDelete,
		Table:      "users",
		Conditions: []storage.Condition{storage.Eq("id", 1)},
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "DELETE FROM users WHERE id = ?"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
	if len(args) != 1 {
		t.Errorf("expected 1 arg, got %d", len(args))
	}
}

func TestTranslateCreateTable(t *testing.T) {
	sql, _, err := sqlt.TranslateDDL(ddl.Stmt{
		Op:    ddl.OpCreateTable,
		Table: "users",
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	if sql == "" {
		t.Error("expected non-empty CREATE TABLE sql")
	}
	if !contains(sql, "CREATE TABLE IF NOT EXISTS users") {
		t.Errorf("expected CREATE TABLE IF NOT EXISTS users in %q", sql)
	}
}

func TestTranslateDropTable(t *testing.T) {
	sql, _, err := sqlt.TranslateDDL(ddl.Stmt{
		Op:    ddl.OpDropTable,
		Table: "users",
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "DROP TABLE IF EXISTS users"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
}

func TestTranslateAddColumn(t *testing.T) {
	sql, _, err := sqlt.TranslateDDL(ddl.Stmt{
		Op:     ddl.OpAddColumn,
		Table:  "users",
		Column: &model.Field{Name: "bio", Type: model.Text()},
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "ALTER TABLE users ADD COLUMN bio TEXT"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}

	sql, _, err = sqlt.TranslateDDL(ddl.Stmt{
		Op:     ddl.OpAddColumn,
		Table:  "users",
		Column: &model.Field{Name: "score", Type: model.Int()},
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want = "ALTER TABLE users ADD COLUMN score INTEGER"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
}

func TestTranslateRenameColumn(t *testing.T) {
	sql, _, err := sqlt.TranslateDDL(ddl.Stmt{
		Op:      ddl.OpRenameColumn,
		Table:   "users",
		OldName: "age",
		Column:  &model.Field{Name: "years"},
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "ALTER TABLE users RENAME COLUMN age TO years"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
}

func TestTranslateDropColumn(t *testing.T) {
	sql, _, err := sqlt.TranslateDDL(ddl.Stmt{
		Op:         ddl.OpDropColumn,
		Table:      "users",
		ColumnName: "age",
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "ALTER TABLE users DROP COLUMN age"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
}

func TestTranslateNullConditions(t *testing.T) {
	sql, args, err := sqlt.Translate(storage.Query{
		Action:     storage.ActionReadAll,
		Table:      "users",
		Conditions: []storage.Condition{storage.IsNotNull("age")},
	}, testModel{})
	if err != nil {
		t.Fatal(err)
	}
	want := "SELECT * FROM users WHERE age IS NOT NULL"
	if sql != want {
		t.Errorf("got %q, want %q", sql, want)
	}
	if len(args) != 0 {
		t.Errorf("expected 0 args, got %d", len(args))
	}
}

func TestCompilerErrors(t *testing.T) {
	_, _, err := sqlt.Translate(storage.Query{Action: 99}, nil)
	if err == nil {
		t.Fatal("expected error for unsupported action")
	}

	_, _, err = sqlt.TranslateDDL(ddl.Stmt{Op: ddl.OpCreateTable}, &testModel{})
	if err == nil {
		t.Fatal("expected error for create table without table name")
	}

	_, _, err = sqlt.TranslateDDL(ddl.Stmt{Op: ddl.OpCreateTable, Table: "t"}, nil)
	if err == nil {
		t.Fatal("expected error for create table without model")
	}

	_, _, err = sqlt.TranslateDDL(ddl.Stmt{Op: ddl.OpDropTable}, nil)
	if err == nil {
		t.Fatal("expected error for drop table without table name")
	}

	_, _, err = sqlt.Translate(storage.Query{Action: storage.ActionCreate, Columns: []string{"id"}}, nil)
	if err == nil {
		t.Fatal("expected error for insert without table")
	}

	_, _, err = sqlt.Translate(storage.Query{Action: storage.ActionCreate, Table: "t"}, nil)
	if err == nil {
		t.Fatal("expected error for insert without columns")
	}

	_, _, err = sqlt.Translate(storage.Query{Action: storage.ActionReadOne}, nil)
	if err == nil {
		t.Fatal("expected error for select without table")
	}

	_, _, err = sqlt.Translate(storage.Query{Action: storage.ActionUpdate, Columns: []string{"id"}}, nil)
	if err == nil {
		t.Fatal("expected error for update without table")
	}

	_, _, err = sqlt.Translate(storage.Query{Action: storage.ActionUpdate, Table: "t"}, nil)
	if err == nil {
		t.Fatal("expected error for update without columns")
	}

	_, _, err = sqlt.Translate(storage.Query{Action: storage.ActionDelete}, nil)
	if err == nil {
		t.Fatal("expected error for delete without table")
	}

	_, _, err = sqlt.TranslateDDL(ddl.Stmt{Op: ddl.OpAddColumn, Table: "t"}, nil)
	if err == nil {
		t.Fatal("expected error for add column without column")
	}

	_, _, err = sqlt.TranslateDDL(ddl.Stmt{Op: ddl.OpAddColumn, Column: &model.Field{Name: "c"}}, nil)
	if err == nil {
		t.Fatal("expected error for add column without table")
	}

	_, _, err = sqlt.TranslateDDL(ddl.Stmt{Op: ddl.OpRenameColumn, Table: "t", OldName: "o"}, nil)
	if err == nil {
		t.Fatal("expected error for rename column without column")
	}

	_, _, err = sqlt.TranslateDDL(ddl.Stmt{Op: ddl.OpRenameColumn, Table: "t", Column: &model.Field{Name: "c"}}, nil)
	if err == nil {
		t.Fatal("expected error for rename column without old name")
	}

	_, _, err = sqlt.TranslateDDL(ddl.Stmt{Op: ddl.OpDropColumn, Table: "t"}, nil)
	if err == nil {
		t.Fatal("expected error for drop column without columnName")
	}

	_, _, err = sqlt.Translate(storage.Query{
		Action:     storage.ActionReadAll,
		Table:      "t",
		Conditions: []storage.Condition{storage.In("id", 1)},
	}, nil)
	if err == nil {
		t.Fatal("expected error for non-slice IN value")
	}

	_, _, err = sqlt.Translate(storage.Query{
		Action:     storage.ActionReadAll,
		Table:      "t",
		Conditions: []storage.Condition{storage.In("id", []any{})},
	}, nil)
	if err == nil {
		t.Fatal("expected error for empty slice IN value")
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsAt(s, sub))
}

func containsAt(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
