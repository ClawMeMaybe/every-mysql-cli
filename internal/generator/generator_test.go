package generator

import (
	"fmt"
	"strings"
	"testing"

	"github.com/kefan/every-mysql-cli/internal/model"
)

func TestColNamesExpr(t *testing.T) {
	table := model.Table{
		Columns: []model.Column{
			{Name: "id"},
			{Name: "name"},
			{Name: "email"},
		},
	}

	got := colNamesExpr(table)
	want := `"id", "name", "email"`
	if got != want {
		t.Errorf("colNamesExpr = %q, want %q", got, want)
	}
}

func TestColNamesExprEmpty(t *testing.T) {
	table := model.Table{}
	got := colNamesExpr(table)
	if got != "" {
		t.Errorf("colNamesExpr on empty table = %q, want empty", got)
	}
}

func TestJoinCommas(t *testing.T) {
	tests := []struct {
		parts []string
		want  string
	}{
		{[]string{"a", "b", "c"}, "a, b, c"},
		{[]string{"a"}, "a"},
		{[]string{}, ""},
	}
	for _, tt := range tests {
		got := joinCommas(tt.parts)
		if got != tt.want {
			t.Errorf("joinCommas(%v) = %q, want %q", tt.parts, got, tt.want)
		}
	}
}

func TestWriteGoModFormat(t *testing.T) {
	content := fmt.Sprintf("module myapp-cli\n\ngo 1.22\n\nrequire (\n\tgithub.com/go-sql-driver/mysql v1.8.1\n\tgithub.com/olekukonko/tablewriter v0.0.5\n\tgithub.com/spf13/cobra v1.8.0\n\tgopkg.in/yaml.v3 v3.0.1\n)\n")
	if !strings.Contains(content, "module myapp-cli") {
		t.Error("go.mod should contain module declaration")
	}
	if !strings.Contains(content, "go 1.22") {
		t.Error("go.mod should contain go version")
	}
}