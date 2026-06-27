package sqlt

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
)

// NewCompiler returns an orm.Compiler that generates SQLite-dialect SQL.
func NewCompiler() orm.Compiler {
	return &compiler{}
}

// Translate converts an orm.Query into a SQLite SQL string and positional args.
func Translate(q orm.Query, m fmt.Model) (string, []any, error) {
	return translateQuery(q, m)
}
