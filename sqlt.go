package sqlt

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/orm"
)

// NewCompiler returns a *compiler that implements orm.Compiler and ddl.Exporter.
func NewCompiler() *compiler {
	return &compiler{}
}

// Translate converts an orm.Query into a SQLite SQL string and positional args.
func Translate(q orm.Query, m model.Model) (string, []any, error) {
	return translateQuery(q, m)
}
