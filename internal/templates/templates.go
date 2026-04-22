package templates

import (
	"embed"
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