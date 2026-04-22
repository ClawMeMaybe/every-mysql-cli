package generator

import (
	"strings"

	"github.com/kefan/every-mysql-cli/internal/types"
)

// TemplateFuncs returns template functions for code generation.
func TemplateFuncs() map[string]interface{} {
	return map[string]interface{}{
		"lower":       strings.ToLower,
		"title":       strings.Title,
		"hasPK":       func(t *types.Table) bool { return t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > 0 },
		"pkCol0":      func(t *types.Table) string { if t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > 0 { return t.PrimaryKey.Columns[0] }; return "" },
		"scanType":    scanType,
		"valueStr":    valueStr,
		"valueJSON":   valueJSON,
		"createConv":  createConv,
		"isString":    func(c *types.Column) bool { return c.GoType == "string" },
		"isAutoInc":   func(c *types.Column) bool { return c.AutoIncrement },
		"isNullable":  func(c *types.Column) bool { return c.Nullable },
	}
}

func scanType(col *types.Column) string {
	if col.Nullable {
		switch col.GoType {
		case "int64":
			return "sql.NullInt64"
		case "float64":
			return "sql.NullFloat64"
		case "bool":
			return "sql.NullBool"
		default:
			return "sql.NullString"
		}
	}
	return col.GoType
}

func valueStr(col *types.Column) string {
	n := "s_" + col.Name
	if col.Nullable {
		switch col.GoType {
		case "int64":
			return "strconv.FormatInt(" + n + ".Int64, 10)"
		case "float64":
			return "fmt.Sprintf(\"%v\", " + n + ".Float64)"
		case "bool":
			return "fmt.Sprintf(\"%t\", " + n + ".Bool)"
		default:
			return n + ".String"
		}
	}
	switch col.GoType {
	case "int64":
		return "strconv.FormatInt(" + n + ", 10)"
	case "float64":
		return "fmt.Sprintf(\"%v\", " + n + ")"
	case "bool":
		return "fmt.Sprintf(\"%t\", " + n + ")"
	case "[]byte":
		return "string(" + n + ")"
	default:
		return n
	}
}

func valueJSON(col *types.Column) string {
	n := "s_" + col.Name
	if col.Nullable {
		switch col.GoType {
		case "int64":
			return n + ".Int64"
		case "float64":
			return n + ".Float64"
		case "bool":
			return n + ".Bool"
		default:
			return n + ".String"
		}
	}
	return n
}

func createConv(col *types.Column) string {
	switch col.GoType {
	case "int64":
		return "strconv.ParseInt(v, 10, 64)"
	case "float64":
		return "strconv.ParseFloat(v, 64)"
	case "bool":
		return "strconv.ParseBool(v)"
	default:
		return "v"
	}
}