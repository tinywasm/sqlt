package sqlt

import (
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/model"
	"github.com/tinywasm/storage"
)

// NewCompiler returns a *compiler that implements storage.Compiler and ddl.Compiler.
func NewCompiler() *compiler {
	return &compiler{}
}

// Translate converts a storage.Query into a SQLite SQL string and positional args.
func Translate(q storage.Query, m model.Model) (string, []any, error) {
	return translateQuery(q, m)
}

// TranslateDDL converts a ddl.Stmt into a SQLite SQL string and positional args.
func TranslateDDL(s ddl.Stmt, m model.Model) (string, []any, error) {
	return translateDDL(s, m)
}
