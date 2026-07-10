package sqlt

import (
	"github.com/tinywasm/model"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/orm"
	"github.com/tinywasm/ddlc"
)

// compiler implements orm.Compiler.
type compiler struct{}

// Compile converts an orm.Query into an engine Plan.
func (c compiler) Compile(q orm.Query, m model.Model) (orm.Plan, error) {
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

func (c *compiler) ExportDDL(models []model.Model) (string, error) {
	sorted, err := ddlc.TopologicalSort(models)
	if err != nil {
		return "", err
	}
	var buf fmt.Builder
	buf.Write("-- dialect: sqlite\n\n")
	for _, m := range sorted {
		plan, err := c.Compile(orm.Query{Action: orm.ActionCreateTable, Table: m.ModelName()}, m)
		if err != nil {
			return "", err
		}
		buf.Write(plan.Query)
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

var _ ddlc.Exporter = (*compiler)(nil)
