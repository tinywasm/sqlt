package sqlt

import (
	"webtyp.com/ddl"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/storage"
)

// compiler implements storage.Compiler (DML) and ddl.Compiler (DDL).
type compiler struct{}

// Compile converts a storage.Query into an engine Plan.
func (c compiler) Compile(q storage.Query, m model.Model) (storage.Plan, error) {
	sqlStr, args, err := translateQuery(q, m)
	if err != nil {
		return storage.Plan{}, err
	}

	return storage.Plan{
		Mode:  q.Action,
		Query: sqlStr,
		Args:  args,
	}, nil
}

// CompileDDL converts a ddl.Stmt into SQL.
func (c compiler) CompileDDL(s ddl.Stmt, m model.Model) (string, []any, error) {
	return translateDDL(s, m)
}

func (c *compiler) ExportDDL(models []model.Model) (string, error) {
	sorted, err := ddl.TopologicalSort(models)
	if err != nil {
		return "", err
	}
	var buf fmt.Builder
	buf.Write("-- dialect: sqlite\n\n")
	for _, m := range sorted {
		sql, _, err := c.CompileDDL(ddl.Stmt{Op: ddl.OpCreateTable, Table: m.ModelName()}, m)
		if err != nil {
			return "", err
		}
		buf.Write(sql)
		buf.Write(";\n\n")
		if ext, ok := m.(interface{ SchemaExt() []model.FieldExt }); ok {
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

var (
	_ storage.Compiler = (*compiler)(nil)
	_ ddl.Compiler     = (*compiler)(nil)
)
