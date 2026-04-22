package generator

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/kefan/every-mysql-cli/internal/types"
)

func TestRenderMainTemplate(t *testing.T) {
	schema := &types.Schema{
		Database: "myapp",
		Tables: []types.Table{
			{Name: "users"},
			{Name: "orders"},
		},
	}

	tmpDir := t.TempDir()
	funcs := TemplateFuncs()
	err := renderTemplate(tmpDir, "main.go", MainTemplate, schema, funcs)
	if err != nil {
		t.Fatalf("renderTemplate main.go: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "main.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	if !contains(string(content), "myapp-cli") {
		t.Error("main.go should contain 'myapp-cli'")
	}
	if !contains(string(content), "usersCmd(db)") {
		t.Error("main.go should register usersCmd")
	}
	if !contains(string(content), "ordersCmd(db)") {
		t.Error("main.go should register ordersCmd")
	}
}

func TestRenderConfigTemplate(t *testing.T) {
	schema := &types.Schema{Database: "testdb"}
	tmpDir := t.TempDir()
	funcs := TemplateFuncs()

	err := renderTemplate(tmpDir, "config.go", ConfigTemplate, schema, funcs)
	if err != nil {
		t.Fatalf("renderTemplate config.go: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "config.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	if !contains(string(content), "testdb.yaml") {
		t.Error("config.go should reference testdb.yaml config path")
	}
	if !contains(string(content), "DB_HOST") {
		t.Error("config.go should read DB_HOST env var")
	}
	if !contains(string(content), "DB_PASSWORD") {
		t.Error("config.go should read DB_PASSWORD env var")
	}
}

func TestRenderGuardTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	funcs := TemplateFuncs()

	err := renderTemplate(tmpDir, "guard.go", GuardTemplate, nil, funcs)
	if err != nil {
		t.Fatalf("renderTemplate guard.go: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "guard.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	if !contains(string(content), "requireForce") {
		t.Error("guard.go should define requireForce")
	}
	if !contains(string(content), "requireForceWithConfirm") {
		t.Error("guard.go should define requireForceWithConfirm")
	}
	if !contains(string(content), "guardDestructiveJSON") {
		t.Error("guard.go should define guardDestructiveJSON")
	}
}

func TestRenderTableCmdTemplate_WithPK(t *testing.T) {
	tc := &TableContext{
		Table: &types.Table{
			Name: "users",
			Columns: []types.Column{
				{Name: "id", Type: "INT", GoType: "int64", AutoIncrement: true},
				{Name: "name", Type: "VARCHAR(255)", GoType: "string"},
				{Name: "email", Type: "VARCHAR(255)", GoType: "string", Nullable: true},
			},
			PrimaryKey: &types.PrimaryKey{Columns: []string{"id"}},
		},
		Schema: &types.Schema{Database: "testdb"},
	}

	tmpDir := t.TempDir()
	funcs := TemplateFuncs()
	err := renderTemplate(tmpDir, "users_cmd.go", TableCmdTemplate, tc, funcs)
	if err != nil {
		t.Fatalf("renderTemplate users_cmd.go: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "users_cmd.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	s := string(content)
	if !contains(s, "usersCmd") {
		t.Error("should define usersCmd")
	}
	if !contains(s, "usersListCmd") {
		t.Error("should define usersListCmd")
	}
	if !contains(s, "usersGetCmd") {
		t.Error("should define usersGetCmd (table has PK)")
	}
	if !contains(s, "usersCreateCmd") {
		t.Error("should define usersCreateCmd")
	}
	if !contains(s, "usersUpdateCmd") {
		t.Error("should define usersUpdateCmd")
	}
	if !contains(s, "usersDeleteCmd") {
		t.Error("should define usersDeleteCmd")
	}
	if !contains(s, "--force") {
		t.Error("delete command should have --force flag")
	}
	if !contains(s, "\"dry-run\"") {
		t.Error("commands should have \"dry-run\" flag")
	}
	if !contains(s, "\"json\"") {
		t.Error("commands should have \"json\" flag")
	}
}

func TestRenderTableCmdTemplate_NoPK(t *testing.T) {
	tc := &TableContext{
		Table: &types.Table{
			Name: "logs",
			Columns: []types.Column{
				{Name: "message", Type: "TEXT", GoType: "string"},
			},
			PrimaryKey: nil,
		},
		Schema: &types.Schema{Database: "testdb"},
	}

	tmpDir := t.TempDir()
	funcs := TemplateFuncs()
	err := renderTemplate(tmpDir, "logs_cmd.go", TableCmdTemplate, tc, funcs)
	if err != nil {
		t.Fatalf("renderTemplate logs_cmd.go: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "logs_cmd.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	s := string(content)
	if !contains(s, "logsCmd") {
		t.Error("should define logsCmd")
	}
	if !contains(s, "logsListCmd") {
		t.Error("should define logsListCmd")
	}
	if contains(s, "logsGetCmd") {
		t.Error("should NOT define logsGetCmd (no PK)")
	}
	if contains(s, "logsDeleteCmd") {
		t.Error("should NOT define logsDeleteCmd (no PK)")
	}
}

func TestRenderTableCmdTemplate_FKFlags(t *testing.T) {
	tc := &TableContext{
		Table: &types.Table{
			Name: "orders",
			Columns: []types.Column{
				{Name: "id", Type: "INT", GoType: "int64", AutoIncrement: true},
				{Name: "user_id", Type: "INT", GoType: "int64"},
			},
			PrimaryKey: &types.PrimaryKey{Columns: []string{"id"}},
			ForeignKeys: []types.ForeignKey{
				{Name: "fk_user", Column: "user_id", ReferencedTable: "users", ReferencedColumn: "id"},
			},
			ReferencedBy: []types.RefReference{
				{SourceTable: "products", SourceColumn: "order_id", ForeignKeyName: "fk_order"},
			},
		},
		Schema: &types.Schema{
			Database: "testdb",
			Tables: []types.Table{
				{Name: "orders"},
				{Name: "products", Columns: []types.Column{{Name: "id", GoType: "int64"}, {Name: "order_id", GoType: "int64"}}},
			},
		},
	}

	tmpDir := t.TempDir()
	funcs := TemplateFuncs()
	err := renderTemplate(tmpDir, "orders_cmd.go", TableCmdTemplate, tc, funcs)
	if err != nil {
		t.Fatalf("renderTemplate orders_cmd.go: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "orders_cmd.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	s := string(content)
	if !contains(s, "by-users") {
		t.Error("list command should have --by-users flag for outbound FK")
	}
	if !contains(s, "with-products") {
		t.Error("get command should have --with-products flag for inbound reference")
	}
}

func TestRenderOutputTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	funcs := TemplateFuncs()

	err := renderTemplate(tmpDir, "output.go", OutputTemplate, nil, funcs)
	if err != nil {
		t.Fatalf("renderTemplate output.go: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "output.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	s := string(content)
	if !contains(s, "printTable") {
		t.Error("output.go should define printTable")
	}
	if !contains(s, "printKV") {
		t.Error("output.go should define printKV")
	}
	if !contains(s, "printJSONData") {
		t.Error("output.go should define printJSONData")
	}
	if !contains(s, "printJSONList") {
		t.Error("output.go should define printJSONList")
	}
	if !contains(s, "printJSONError") {
		t.Error("output.go should define printJSONError")
	}
}

func TestRenderDBTemplate(t *testing.T) {
	tmpDir := t.TempDir()
	funcs := TemplateFuncs()

	err := renderTemplate(tmpDir, "db.go", DBTemplate, nil, funcs)
	if err != nil {
		t.Fatalf("renderTemplate db.go: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "db.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	if !contains(string(content), "initDB") {
		t.Error("db.go should define initDB")
	}
	if !contains(string(content), "go-sql-driver/mysql") {
		t.Error("db.go should import mysql driver")
	}
}

func TestTemplateFuncs_ValueStr(t *testing.T) {
	tests := []struct {
		name string
		col  types.Column
		want string
	}{
		{"int64 non-null", types.Column{Name: "id", GoType: "int64"}, "strconv.FormatInt(s_id, 10)"},
		{"string nullable", types.Column{Name: "email", GoType: "string", Nullable: true}, "s_email.String"},
		{"string non-null", types.Column{Name: "name", GoType: "string"}, "s_name"},
		{"float64 non-null", types.Column{Name: "price", GoType: "float64"}, "fmt.Sprintf(\"%v\", s_price)"},
		{"bool non-null", types.Column{Name: "active", GoType: "bool"}, "fmt.Sprintf(\"%t\", s_active)"},
		{"int64 nullable", types.Column{Name: "count", GoType: "int64", Nullable: true}, "strconv.FormatInt(s_count.Int64, 10)"},
	}
	for _, tc := range tests {
		got := valueStr(&tc.col)
		if got != tc.want {
			t.Errorf("valueStr(%s) = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestTemplateFuncs_ValueJSON(t *testing.T) {
	tests := []struct {
		name string
		col  types.Column
		want string
	}{
		{"int64 non-null", types.Column{Name: "id", GoType: "int64"}, "s_id"},
		{"string nullable", types.Column{Name: "email", GoType: "string", Nullable: true}, "s_email.String"},
		{"int64 nullable", types.Column{Name: "count", GoType: "int64", Nullable: true}, "s_count.Int64"},
	}
	for _, tc := range tests {
		got := valueJSON(&tc.col)
		if got != tc.want {
			t.Errorf("valueJSON(%s) = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestTemplateFuncs_ScanType(t *testing.T) {
	tests := []struct {
		name string
		col  types.Column
		want string
	}{
		{"int64 non-null", types.Column{GoType: "int64"}, "int64"},
		{"int64 nullable", types.Column{GoType: "int64", Nullable: true}, "sql.NullInt64"},
		{"string nullable", types.Column{GoType: "string", Nullable: true}, "sql.NullString"},
		{"float64 nullable", types.Column{GoType: "float64", Nullable: true}, "sql.NullFloat64"},
		{"bool nullable", types.Column{GoType: "bool", Nullable: true}, "sql.NullBool"},
		{"string non-null", types.Column{GoType: "string"}, "string"},
	}
	for _, tc := range tests {
		got := scanType(&tc.col)
		if got != tc.want {
			t.Errorf("scanType(%s) = %s, want %s", tc.name, got, tc.want)
		}
	}
}

func TestTemplateFuncs_PK(t *testing.T) {
	singlePK := &types.Table{
		Name: "users",
		PrimaryKey: &types.PrimaryKey{Columns: []string{"id"}},
	}
	compositePK := &types.Table{
		Name: "order_items",
		PrimaryKey: &types.PrimaryKey{Columns: []string{"order_id", "item_id"}},
	}
	noPK := &types.Table{
		Name: "logs",
		PrimaryKey: nil,
	}

	tests := []struct {
		name string
		table *types.Table
		argCount int
		whereClause string
		scanArgs string
	}{
		{"single PK", singlePK, 1, "id = ?", "args[0]"},
		{"composite PK", compositePK, 2, "order_id = ? AND item_id = ?", "args[0], args[1]"},
		{"no PK", noPK, 0, "", ""},
	}
	for _, tc := range tests {
		gotArgCount := pkArgCount(tc.table)
		if gotArgCount != tc.argCount {
			t.Errorf("pkArgCount(%s) = %d, want %d", tc.name, gotArgCount, tc.argCount)
		}
		gotWhere := pkWhereClause(tc.table)
		if gotWhere != tc.whereClause {
			t.Errorf("pkWhereClause(%s) = %s, want %s", tc.name, gotWhere, tc.whereClause)
		}
		gotScan := pkScanArgs(tc.table)
		if gotScan != tc.scanArgs {
			t.Errorf("pkScanArgs(%s) = %s, want %s", tc.name, gotScan, tc.scanArgs)
		}
	}
}

func TestRenderTableCmdTemplate_CompositePK(t *testing.T) {
	tc := &TableContext{
		Table: &types.Table{
			Name: "order_items",
			Columns: []types.Column{
				{Name: "order_id", Type: "INT", GoType: "int64"},
				{Name: "item_id", Type: "INT", GoType: "int64"},
				{Name: "quantity", Type: "INT", GoType: "int64"},
			},
			PrimaryKey: &types.PrimaryKey{Columns: []string{"order_id", "item_id"}},
		},
		Schema: &types.Schema{Database: "testdb"},
	}

	tmpDir := t.TempDir()
	funcs := TemplateFuncs()
	err := renderTemplate(tmpDir, "order_items_cmd.go", TableCmdTemplate, tc, funcs)
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "order_items_cmd.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	s := string(content)
	if !contains(s, "ExactArgs(2)") {
		t.Error("get command should require 2 args for composite PK")
	}
	if !contains(s, "order_id = ? AND item_id = ?") {
		t.Error("WHERE clause should use all composite PK columns")
	}
	if !contains(s, "RangeArgs(0, 2)") {
		t.Error("update/delete should accept up to 2 args for composite PK")
	}
	if !contains(s, "args[0], args[1]") {
		t.Error("composite PK query args should use args[0] and args[1]")
	}
}

func TestRenderTableCmdTemplate_BoolColumn(t *testing.T) {
	tc := &TableContext{
		Table: &types.Table{
			Name: "users",
			Columns: []types.Column{
				{Name: "id", Type: "INT", GoType: "int64", AutoIncrement: true},
				{Name: "active", Type: "BOOLEAN", GoType: "bool"},
			},
			PrimaryKey: &types.PrimaryKey{Columns: []string{"id"}},
		},
		Schema: &types.Schema{Database: "testdb"},
	}

	tmpDir := t.TempDir()
	funcs := TemplateFuncs()
	err := renderTemplate(tmpDir, "users_cmd.go", TableCmdTemplate, tc, funcs)
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "users_cmd.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	s := string(content)
	if !contains(s, "strconv.ParseBool") {
		t.Error("bool columns should use strconv.ParseBool in create/update")
	}
}

func TestRenderTableCmdTemplate_DeleteGuardNoAll(t *testing.T) {
	tc := &TableContext{
		Table: &types.Table{
			Name: "users",
			Columns: []types.Column{
				{Name: "id", Type: "INT", GoType: "int64", AutoIncrement: true},
				{Name: "name", Type: "VARCHAR(255)", GoType: "string"},
			},
			PrimaryKey: &types.PrimaryKey{Columns: []string{"id"}},
		},
		Schema: &types.Schema{Database: "testdb"},
	}

	tmpDir := t.TempDir()
	funcs := TemplateFuncs()
	err := renderTemplate(tmpDir, "users_cmd.go", TableCmdTemplate, tc, funcs)
	if err != nil {
		t.Fatalf("renderTemplate: %v", err)
	}

	content, err := os.ReadFile(filepath.Join(tmpDir, "users_cmd.go"))
	if err != nil {
		t.Fatalf("reading generated file: %v", err)
	}

	s := string(content)
	if !contains(s, "len(args) == 0 && !all") {
		t.Error("delete should reject zero-arg invocation without --all")
	}
	if !contains(s, "requires a primary key or --all flag") {
		t.Error("delete guard should mention --all requirement")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}