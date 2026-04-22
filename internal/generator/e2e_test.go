package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/kefan/every-mysql-cli/internal/types"
)

func TestGenerateFullProject(t *testing.T) {
	schema := &types.Schema{
		Database: "testdb",
		Tables: []types.Table{
			{
				Name: "users",
				Columns: []types.Column{
					{Name: "id", Type: "INT", GoType: "int64", AutoIncrement: true},
					{Name: "name", Type: "VARCHAR(255)", GoType: "string"},
					{Name: "email", Type: "VARCHAR(255)", GoType: "string", Nullable: true},
					{Name: "age", Type: "INT", GoType: "int64", Nullable: true},
					{Name: "active", Type: "BOOLEAN", GoType: "bool"},
				},
				PrimaryKey: &types.PrimaryKey{Columns: []string{"id"}},
				ForeignKeys: []types.ForeignKey{
					{Name: "fk_user", Column: "user_id", ReferencedTable: "users", ReferencedColumn: "id"},
				},
				ReferencedBy: []types.RefReference{
					{SourceTable: "orders", SourceColumn: "user_id", ForeignKeyName: "fk_user"},
				},
				Indexes: []types.Index{
					{Name: "idx_email", Columns: []string{"email"}, Unique: true},
				},
			},
			{
				Name: "orders",
				Columns: []types.Column{
					{Name: "id", Type: "INT", GoType: "int64", AutoIncrement: true},
					{Name: "user_id", Type: "INT", GoType: "int64"},
					{Name: "total", Type: "DECIMAL(10,2)", GoType: "string"},
					{Name: "created_at", Type: "DATETIME", GoType: "string", Nullable: true},
				},
				PrimaryKey: &types.PrimaryKey{Columns: []string{"id"}},
				ForeignKeys: []types.ForeignKey{
					{Name: "fk_user", Column: "user_id", ReferencedTable: "users", ReferencedColumn: "id", OnDelete: "CASCADE", OnUpdate: "RESTRICT"},
				},
				Indexes: []types.Index{},
			},
			{
				Name: "logs",
				Columns: []types.Column{
					{Name: "message", Type: "TEXT", GoType: "string"},
					{Name: "level", Type: "VARCHAR(20)", GoType: "string"},
				},
				PrimaryKey: nil,
				Indexes:    []types.Index{},
			},
		},
	}

	outputDir := filepath.Join(t.TempDir(), "testdb-cli")
	if err := Generate(schema, outputDir); err != nil {
		t.Fatalf("Generate: %v", err)
	}

	// Verify files exist
	expectedFiles := []string{
		"main.go", "db.go", "guard.go", "output.go", "config.go",
		"users_cmd.go", "orders_cmd.go", "logs_cmd.go",
		"go.mod",
	}
	for _, f := range expectedFiles {
		path := filepath.Join(outputDir, f)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("expected file %s to exist", f)
		}
	}

	// Verify binary was built
	binaryPath := filepath.Join(outputDir, "testdb-cli")
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		t.Errorf("expected binary testdb-cli to exist")
	}

	// Verify the generated binary runs
	cmd := exec.Command(binaryPath, "--help")
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated binary --help: %v\nOutput: %s", err, string(output))
	}
	helpStr := string(output)
	if !containsStr(helpStr, "testdb-cli") {
		t.Errorf("help output should contain 'testdb-cli', got: %s", helpStr)
	}
	if !containsStr(helpStr, "users") {
		t.Errorf("help output should list 'users' command, got: %s", helpStr)
	}
	if !containsStr(helpStr, "orders") {
		t.Errorf("help output should list 'orders' command, got: %s", helpStr)
	}
	if !containsStr(helpStr, "logs") {
		t.Errorf("help output should list 'logs' command, got: %s", helpStr)
	}

	// Verify subcommands
	cmd = exec.Command(binaryPath, "users", "--help")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("users --help: %v\nOutput: %s", err, string(output))
	}
	usersHelp := string(output)
	if !containsStr(usersHelp, "list") {
		t.Error("users should have 'list' subcommand")
	}
	if !containsStr(usersHelp, "get") {
		t.Error("users should have 'get' subcommand (has PK)")
	}
	if !containsStr(usersHelp, "create") {
		t.Error("users should have 'create' subcommand")
	}
	if !containsStr(usersHelp, "update") {
		t.Error("users should have 'update' subcommand")
	}
	if !containsStr(usersHelp, "delete") {
		t.Error("users should have 'delete' subcommand")
	}

	// Verify logs only has list (no PK)
	cmd = exec.Command(binaryPath, "logs", "--help")
	output, err = cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("logs --help: %v\nOutput: %s", err, string(output))
	}
	logsHelp := string(output)
	if !containsStr(logsHelp, "list") {
		t.Error("logs should have 'list' subcommand")
	}
	if containsStr(logsHelp, "get") {
		t.Error("logs should NOT have 'get' subcommand (no PK)")
	}
}

func TestWriteConfig(t *testing.T) {
	schema := &types.Schema{Database: "testcfg"}

	tmpHome := t.TempDir()
	t.Setenv("HOME", tmpHome)

	err := WriteConfig(schema, "db.example.com", "3306", "admin", "secret123")
	if err != nil {
		t.Fatalf("WriteConfig: %v", err)
	}

	configPath := filepath.Join(tmpHome, ".every-mysql", "testcfg.yaml")
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatalf("reading config: %v", err)
	}

	content := string(data)
	if !containsStr(content, "db.example.com") {
		t.Error("config should contain host")
	}
	if !containsStr(content, "secret123") {
		t.Error("config should contain password")
	}
	if !containsStr(content, "testcfg") {
		t.Error("config should contain database name")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}