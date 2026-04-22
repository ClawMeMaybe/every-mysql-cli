package templates

import (
	"strings"
	"testing"
	"text/template"

	"github.com/kefan/every-mysql-cli/internal/model"
)

func TestAllTemplatesParse(t *testing.T) {
	tmpls := All()
	fm := FuncMap()

	for name, content := range tmpls {
		tmpl, err := template.New(name).Funcs(fm).Parse(content)
		if err != nil {
			t.Errorf("template %q failed to parse: %v", name, err)
		}
		if tmpl == nil {
			t.Errorf("template %q parsed as nil", name)
		}
	}
}

func TestFuncMapIndexPK(t *testing.T) {
	fm := FuncMap()
	indexPKFn := fm["indexPK"].(func(model.Table, int) string)

	tests := []struct {
		table model.Table
		idx   int
		want  string
	}{
		{model.Table{PrimaryKey: &model.PrimaryKey{Columns: []string{"id"}}}, 0, "id"},
		{model.Table{PrimaryKey: &model.PrimaryKey{Columns: []string{"a", "b"}}}, 1, "b"},
		{model.Table{PrimaryKey: nil}, 0, ""},
		{model.Table{PrimaryKey: &model.PrimaryKey{Columns: []string{"id"}}}, 5, ""},
	}

	for _, tt := range tests {
		got := indexPKFn(tt.table, tt.idx)
		if got != tt.want {
			t.Errorf("indexPK(%v, %d) = %q, want %q", tt.table.PrimaryKey, tt.idx, got, tt.want)
		}
	}
}

func TestFuncMapIndexColumnGoType(t *testing.T) {
	fm := FuncMap()
	indexColumnGoTypeFn := fm["indexColumnGoType"].(func(model.Table, string) string)

	table := model.Table{
		Columns: []model.Column{
			{Name: "id", GoType: "int64"},
			{Name: "name", GoType: "string"},
		},
	}

	if got := indexColumnGoTypeFn(table, "id"); got != "int64" {
		t.Errorf("indexColumnGoType(table, id) = %q, want int64", got)
	}
	if got := indexColumnGoTypeFn(table, "name"); got != "string" {
		t.Errorf("indexColumnGoType(table, name) = %q, want string", got)
	}
	if got := indexColumnGoTypeFn(table, "missing"); got != "string" {
		t.Errorf("indexColumnGoType(table, missing) = %q, want string", got)
	}
}

func TestTableCmdTemplateRenders(t *testing.T) {
	tmpls := All()
	tmpl, err := template.New("table_cmd").Funcs(FuncMap()).Parse(tmpls["table_cmd"])
	if err != nil {
		t.Fatalf("parse table_cmd: %v", err)
	}

	usersTable := model.Table{
		Name:   "users",
		Engine: "InnoDB",
		HasPK:  true,
		Columns: []model.Column{
			{Name: "id", Type: "INT(11)", GoType: "int64", AutoIncrement: true},
			{Name: "name", Type: "VARCHAR(255)", GoType: "string", Nullable: true},
			{Name: "email", Type: "VARCHAR(255)", GoType: "string"},
		},
		PrimaryKey: &model.PrimaryKey{Columns: []string{"id"}},
		ReferencedBy: []model.RefReference{
			{SourceTable: "orders", SourceColumn: "user_id", ForeignKeyName: "fk_order_user"},
		},
	}

	schema := &model.Schema{Database: "myapp", Tables: []model.Table{usersTable}}

	var buf strings.Builder
	tc := &struct {
		Schema       *model.Schema
		Table        model.Table
		ColNamesExpr string
	}{
		Schema:       schema,
		Table:        usersTable,
		ColNamesExpr: `"id", "name", "email"`,
	}

	if err := tmpl.Execute(&buf, tc); err != nil {
		t.Fatalf("execute table_cmd: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "func usersCmd()") {
		t.Error("expected usersCmd function in output")
	}
	if !strings.Contains(output, "func usersListCmd()") {
		t.Error("expected usersListCmd function in output")
	}
	if !strings.Contains(output, "func usersGetCmd()") {
		t.Error("expected usersGetCmd function in output")
	}
	if !strings.Contains(output, "func usersCreateCmd()") {
		t.Error("expected usersCreateCmd function in output")
	}
	if !strings.Contains(output, "func usersUpdateCmd()") {
		t.Error("expected usersUpdateCmd function in output")
	}
	if !strings.Contains(output, "func usersDeleteCmd()") {
		t.Error("expected usersDeleteCmd function in output")
	}
	if !strings.Contains(output, "--with-orders") {
		t.Error("expected --with-orders flag in output")
	}
	if !strings.Contains(output, "usersColNames") {
		t.Error("expected usersColNames variable in output")
	}
}

func TestTableCmdNoPK(t *testing.T) {
	tmpls := All()
	tmpl, err := template.New("table_cmd").Funcs(FuncMap()).Parse(tmpls["table_cmd"])
	if err != nil {
		t.Fatalf("parse table_cmd: %v", err)
	}

	logsTable := model.Table{
		Name:    "logs",
		HasPK:   false,
		Columns: []model.Column{
			{Name: "message", Type: "TEXT", GoType: "string"},
		},
	}

	schema := &model.Schema{Database: "myapp", Tables: []model.Table{logsTable}}

	var buf strings.Builder
	tc := &struct {
		Schema       *model.Schema
		Table        model.Table
		ColNamesExpr string
	}{
		Schema:       schema,
		Table:        logsTable,
		ColNamesExpr: `"message"`,
	}

	if err := tmpl.Execute(&buf, tc); err != nil {
		t.Fatalf("execute table_cmd no PK: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "func logsCmd()") {
		t.Error("expected logsCmd function in output")
	}
	if !strings.Contains(output, "func logsListCmd()") {
		t.Error("expected logsListCmd function in output")
	}
	if strings.Contains(output, "func logsGetCmd()") {
		t.Error("should NOT have get command for table without PK")
	}
	if strings.Contains(output, "func logsDeleteCmd()") {
		t.Error("should NOT have delete command for table without PK")
	}
}