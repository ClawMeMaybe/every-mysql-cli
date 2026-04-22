package templates

import (
	"embed"

	"github.com/kefan/every-mysql-cli/internal/model"
)

//go:embed *.tmpl
var templateFS embed.FS

func All() map[string]string {
	names := []string{"main", "db", "guard", "output", "config", "table_cmd"}
	result := make(map[string]string)
	for _, name := range names {
		data, err := templateFS.ReadFile(name + ".tmpl")
		if err != nil {
			panic("missing template: " + name + ".tmpl: " + err.Error())
		}
		result[name] = string(data)
	}
	return result
}

// FuncMap returns template helper functions used during code generation.
func FuncMap() map[string]interface{} {
	return map[string]interface{}{
		"indexPK": func(t model.Table, idx int) string {
			if t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > idx {
				return t.PrimaryKey.Columns[idx]
			}
			return ""
		},
		"indexColumnGoType": func(t model.Table, colName string) string {
			for _, c := range t.Columns {
				if c.Name == colName {
					return c.GoType
				}
			}
			return "string"
		},
	}
}