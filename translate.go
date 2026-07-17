package sqlt

import (
	"github.com/tinywasm/ddl"
	"github.com/tinywasm/ddlc"
	"github.com/tinywasm/fmt"
	"github.com/tinywasm/model"
	"github.com/tinywasm/storage"
)

// translateQuery converts a storage.Query into a SQLite SQL string and arguments.
func translateQuery(q storage.Query, m model.Model) (string, []any, error) {
	switch q.Action {
	case storage.ActionCreate:
		return buildInsert(q)
	case storage.ActionReadOne, storage.ActionReadAll:
		return buildSelect(q)
	case storage.ActionUpdate:
		return buildUpdate(q, m)
	case storage.ActionDelete:
		return buildDelete(q)
	default:
		return "", nil, fmt.Errf("unknown query action: %v", q.Action)
	}
}

// translateDDL converts a ddl.Stmt into a SQLite SQL string and arguments.
func translateDDL(s ddl.Stmt, m model.Model) (string, []any, error) {
	switch s.Op {
	case ddl.OpCreateTable:
		return buildCreateTable(s, m)
	case ddl.OpDropTable:
		return buildDropTable(s)
	case ddl.OpAddColumn:
		return buildAddColumn(s)
	case ddl.OpRenameColumn:
		return buildRenameColumn(s)
	case ddl.OpDropColumn:
		return buildDropColumn(s)
	default:
		return "", nil, fmt.Errf("unknown DDL op: %v", s.Op)
	}
}

func buildCreateTable(s ddl.Stmt, m model.Model) (string, []any, error) {
	if m == nil {
		return "", nil, fmt.Err("model is required for create table")
	}
	if s.Table == "" {
		return "", nil, fmt.Err("table name is required for create table")
	}

	var sb []string
	sb = append(sb, fmt.Sprintf("CREATE TABLE IF NOT EXISTS %s (", s.Table))

	fields := m.Schema()

	// Count composite PK fields upfront to decide between inline and table-level PK.
	var pkCols []string
	for _, f := range fields {
		if f.IsPK() {
			pkCols = append(pkCols, f.Name)
		}
	}
	compositePK := len(pkCols) > 1

	var cols []string
	for _, f := range fields {
		col := fmt.Sprintf("%s %s", f.Name, sqliteColumnType(f))
		if f.IsPK() {
			if compositePK {
				// Composite PK: columns must be NOT NULL; constraint emitted as table-level below.
				col += " NOT NULL"
			} else {
				col += " PRIMARY KEY"
				// AUTOINCREMENT is only allowed on INTEGER PRIMARY KEY in SQLite
				if f.IsAutoInc() && f.Type.Storage() == model.FieldInt {
					col += " AUTOINCREMENT"
				}
			}
		}
		if f.NotNull {
			col += " NOT NULL"
		}
		if f.IsUnique() {
			col += " UNIQUE"
		}
		cols = append(cols, col)
	}

	if compositePK {
		cols = append(cols, fmt.Sprintf("PRIMARY KEY (%s)", fmt.Convert(pkCols).Join(", ").String()))
	}

	if ext, ok := m.(interface{ SchemaExt() []ddlc.FieldExt }); ok {
		for _, f := range ext.SchemaExt() {
			if f.Ref != "" {
				refCol := f.RefColumn
				if refCol == "" {
					refCol = "id"
				}
				fkSQL := fmt.Sprintf(
					"CONSTRAINT fk_%s_%s FOREIGN KEY (%s) REFERENCES %s(%s)",
					s.Table, f.Name, f.Name, f.Ref, refCol)
				fkSQL += " ON DELETE " + onDeleteSQL(f.OnDelete)
				cols = append(cols, fkSQL)
			}
		}
	}

	sb = append(sb, fmt.Convert(cols).Join(", ").String())
	sb = append(sb, ")")

	return fmt.Convert(sb).Join("").String(), nil, nil
}

func buildDropTable(s ddl.Stmt) (string, []any, error) {
	if s.Table == "" {
		return "", nil, fmt.Err("table name is required for drop table")
	}
	return fmt.Sprintf("DROP TABLE IF EXISTS %s", s.Table), nil, nil
}

func buildAddColumn(s ddl.Stmt) (string, []any, error) {
	if s.Column == nil || s.Table == "" {
		return "", nil, fmt.Err("table and column required for add column")
	}
	return fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s",
		s.Table, s.Column.Name, sqliteType(s.Column.Type.Storage())), nil, nil
}

func buildRenameColumn(s ddl.Stmt) (string, []any, error) {
	if s.Column == nil || s.OldName == "" || s.Table == "" {
		return "", nil, fmt.Err("table, old name and column required for rename")
	}
	return fmt.Sprintf("ALTER TABLE %s RENAME COLUMN %s TO %s",
		s.Table, s.OldName, s.Column.Name), nil, nil
}

func buildDropColumn(s ddl.Stmt) (string, []any, error) {
	if s.Table == "" || s.ColumnName == "" {
		return "", nil, fmt.Err("table and column required for drop column")
	}
	return fmt.Sprintf("ALTER TABLE %s DROP COLUMN %s", s.Table, s.ColumnName), nil, nil
}

func sqliteColumnType(f model.Field) string {
	if f.Type.Storage() == model.FieldText && f.Maximum > 0 {
		return fmt.Sprintf("VARCHAR(%d)", f.Maximum)
	}
	return sqliteType(f.Type.Storage())
}

func sqliteType(t model.FieldType) string {
	switch t {
	case model.FieldInt:
		return "INTEGER"
	case model.FieldFloat:
		return "REAL"
	case model.FieldBool:
		return "INTEGER" // 0 or 1
	case model.FieldBlob:
		return "BLOB"
	default:
		return "TEXT"
	}
}

func onDeleteSQL(action string) string {
	switch action {
	case "restrict":
		return "RESTRICT"
	case "set_null":
		return "SET NULL"
	case "no_action":
		return "NO ACTION"
	default:
		return "CASCADE" // default for all ref=, including OnDelete == ""
	}
}

func buildInsert(q storage.Query) (string, []any, error) {
	if q.Table == "" {
		return "", nil, fmt.Err("table name is required for insert")
	}
	if len(q.Columns) == 0 {
		return "", nil, fmt.Err("columns are required for insert")
	}

	cols := fmt.Convert(q.Columns).Join(", ").String()
	placeholders := make([]string, len(q.Columns))
	for i := range placeholders {
		placeholders[i] = "?"
	}
	vals := fmt.Convert(placeholders).Join(", ").String()

	sql := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)", q.Table, cols, vals)
	return sql, q.Values, nil
}

func buildSelect(q storage.Query) (string, []any, error) {
	if q.Table == "" {
		return "", nil, fmt.Err("table name is required for select")
	}
	cols := "*"

	whereSQL, args, err := buildConditions(q.Conditions)
	if err != nil {
		return "", nil, err
	}

	groupBySQL := ""
	if len(q.GroupBy) > 0 {
		groupBySQL = " GROUP BY " + fmt.Convert(q.GroupBy).Join(", ").String()
	}

	orderBySQL := ""
	if len(q.OrderBy) > 0 {
		var orders []string
		for _, o := range q.OrderBy {
			orders = append(orders, fmt.Sprintf("%s %s", o.Column(), o.Dir()))
		}
		orderBySQL = " ORDER BY " + fmt.Convert(orders).Join(", ").String()
	}

	limitSQL := ""
	if q.Limit > 0 {
		limitSQL = fmt.Sprintf(" LIMIT %d", q.Limit)
	}

	offsetSQL := ""
	if q.Offset > 0 {
		offsetSQL = fmt.Sprintf(" OFFSET %d", q.Offset)
	}

	sql := fmt.Sprintf("SELECT %s FROM %s%s%s%s%s%s", cols, q.Table, whereSQL, groupBySQL, orderBySQL, limitSQL, offsetSQL)
	return sql, args, nil
}

func buildUpdate(q storage.Query, m model.Model) (string, []any, error) {
	if q.Table == "" {
		return "", nil, fmt.Err("table name is required for update")
	}
	if len(q.Columns) == 0 {
		return "", nil, fmt.Err("columns are required for update")
	}

	var pkCols map[string]bool
	if m != nil {
		pkCols = make(map[string]bool)
		for _, f := range m.Schema() {
			if f.IsPK() {
				pkCols[f.Name] = true
			}
		}
	}

	var setClauses []string
	var args []any

	for i, col := range q.Columns {
		if pkCols[col] {
			continue // skip updating primary keys
		}
		setClauses = append(setClauses, fmt.Sprintf("%s = ?", col))
		args = append(args, q.Values[i])
	}

	if len(setClauses) == 0 {
		return "", nil, fmt.Err("no non-PK columns to update")
	}

	whereSQL, condArgs, err := buildConditions(q.Conditions)
	if err != nil {
		return "", nil, err
	}
	args = append(args, condArgs...)

	sql := fmt.Sprintf("UPDATE %s SET %s%s", q.Table, fmt.Convert(setClauses).Join(", ").String(), whereSQL)
	return sql, args, nil
}

func buildDelete(q storage.Query) (string, []any, error) {
	if q.Table == "" {
		return "", nil, fmt.Err("table name is required for delete")
	}

	whereSQL, args, err := buildConditions(q.Conditions)
	if err != nil {
		return "", nil, err
	}

	sql := fmt.Sprintf("DELETE FROM %s%s", q.Table, whereSQL)
	return sql, args, nil
}

func buildConditions(conditions []storage.Condition) (string, []any, error) {
	if len(conditions) == 0 {
		return "", nil, nil
	}

	var whereClauses []string
	var args []any

	for i, c := range conditions {
		var clause string
		op := c.Operator()
		switch {
		case op == "IS NULL" || op == "IS NOT NULL":
			clause = fmt.Sprintf("%s %s", c.Field(), op)
		case op == "IN":
			slice, ok := c.Value().([]any)
			if !ok {
				return "", nil, fmt.Errf("IN operator requires []any value, got %T", c.Value())
			}
			if len(slice) == 0 {
				return "", nil, fmt.Err("IN operator slice cannot be empty")
			}
			placeholders := make([]string, len(slice))
			for i := range placeholders {
				placeholders[i] = "?"
			}
			inVals := fmt.Convert(placeholders).Join(", ").String()
			clause = fmt.Sprintf("%s IN (%s)", c.Field(), inVals)
			args = append(args, slice...)
		default:
			clause = fmt.Sprintf("%s %s ?", c.Field(), op)
			args = append(args, c.Value())
		}

		if i > 0 {
			clause = fmt.Sprintf(" %s %s", c.Logic(), clause)
		}
		whereClauses = append(whereClauses, clause)
	}

	return " WHERE " + fmt.Convert(whereClauses).Join("").String(), args, nil
}
