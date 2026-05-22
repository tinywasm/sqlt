package sqlt

import (
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
)

// compiler implements orm.Compiler.
type compiler struct{}

// Compile converts an orm.Query into an engine Plan.
func (c compiler) Compile(q orm.Query, m fmt.Model) (orm.Plan, error) {
	sqlStr, args, err := translateQuery(q, m)
	if err != nil {
		return orm.Plan{}, err
	}

	return orm.Plan{
		Mode:  q.Action,
		Query: sqlStr,
		Args:  args,
	}, nil
}
