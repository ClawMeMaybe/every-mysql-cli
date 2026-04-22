package generator

import "github.com/kefan/every-mysql-cli/internal/types"

// TableContext holds a table plus the full schema for cross-table template lookups.
type TableContext struct {
	Table   *types.Table
	Schema  *types.Schema
}

// LookupTable finds a table by name in the schema.
func (tc *TableContext) LookupTable(name string) *types.Table {
	for _, t := range tc.Schema.Tables {
		if t.Name == name {
			return &t
		}
	}
	return nil
}