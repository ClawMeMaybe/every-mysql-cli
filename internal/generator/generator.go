package generator

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/kefan/every-mysql-cli/internal/model"
	"github.com/kefan/every-mysql-cli/internal/scanner"
	"github.com/kefan/every-mysql-cli/internal/templates"
)

type Config struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
	Output   string
}

func Generate(cfg Config) error {
	if cfg.Output == "" {
		cfg.Output = cfg.Database + "-cli"
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("connecting to MySQL: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		maskedDSN := fmt.Sprintf("%s:****@tcp(%s:%d)/%s", cfg.User, cfg.Host, cfg.Port, cfg.Database)
		return fmt.Errorf("connection failed (%s): %w", maskedDSN, err)
	}

	schema, err := scanner.ScanSchema(db, cfg.Database)
	if err != nil {
		return fmt.Errorf("scanning schema: %w", err)
	}

	for _, t := range schema.Tables {
		if !t.HasPK {
			fmt.Printf("note: table %s has no primary key — only list command will be generated\n", t.Name)
		}
	}

	if err := os.MkdirAll(cfg.Output, 0o755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	tmpls := templates.All()

	if err := renderFile(cfg.Output, "main.go", tmpls["main"], schema); err != nil {
		return fmt.Errorf("generating main.go: %w", err)
	}
	if err := renderFile(cfg.Output, "db.go", tmpls["db"], schema); err != nil {
		return fmt.Errorf("generating db.go: %w", err)
	}
	if err := renderFile(cfg.Output, "guard.go", tmpls["guard"], schema); err != nil {
		return fmt.Errorf("generating guard.go: %w", err)
	}
	if err := renderFile(cfg.Output, "output.go", tmpls["output"], schema); err != nil {
		return fmt.Errorf("generating output.go: %w", err)
	}
	if err := renderFile(cfg.Output, "config.go", tmpls["config"], schema); err != nil {
		return fmt.Errorf("generating config.go: %w", err)
	}

	for _, t := range schema.Tables {
		filename := t.Name + "_cmd.go"
		if err := renderFile(cfg.Output, filename, tmpls["table_cmd"], &tableContext{Schema: schema, Table: t}); err != nil {
			return fmt.Errorf("generating %s: %w", filename, err)
		}
	}

	if err := writeGoMod(cfg.Output, cfg.Database); err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	if err := runGoInit(cfg.Output); err != nil {
		return fmt.Errorf("building generated CLI: %w\n\nPlease ensure the Go toolchain is installed and accessible", err)
	}

	if err := writeConfigFile(cfg); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	binaryName := filepath.Join(cfg.Output, cfg.Database+"-cli")
	fmt.Printf("\nSuccess! Generated CLI at %s\n", binaryName)
	fmt.Printf("Usage example:\n  %s users list\n  %s users get 42\n  %s users list --json\n",
		binaryName, binaryName, binaryName)

	return nil
}

type tableContext struct {
	Schema *model.Schema
	Table  model.Table
}

func renderFile(dir, filename, tmplStr string, data interface{}) error {
	tmpl, err := template.New(filename).Funcs(template.FuncMap{
		"indexColumnGoType": indexColumnGoType,
		"indexPK":           indexPK,
	}).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("parsing template %s: %w", filename, err)
	}

	path := filepath.Join(dir, filename)
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating file %s: %w", path, err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, data); err != nil {
		return fmt.Errorf("rendering template %s: %w", filename, err)
	}
	return nil
}

func indexColumnGoType(t model.Table, colName string) string {
	for _, c := range t.Columns {
		if c.Name == colName {
			return c.GoType
		}
	}
	return "string"
}

func indexPK(t model.Table, idx int) string {
	if t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > idx {
		return t.PrimaryKey.Columns[idx]
	}
	return ""
}

func writeGoMod(dir, dbName string) error {
	content := fmt.Sprintf(`module %s-cli

go 1.22

require (
	github.com/go-sql-driver/mysql v1.8.1
	github.com/olekukonko/tablewriter v0.0.5
	github.com/spf13/cobra v1.8.0
	gopkg.in/yaml.v3 v3.0.1
)
`, dbName)
	return os.WriteFile(filepath.Join(dir, "go.mod"), []byte(content), 0o644)
}

func runGoInit(dir string) error {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go mod tidy: %w", err)
	}

	cmd = exec.Command("go", "build", "-o", filepath.Base(dir))
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("go build: %w", err)
	}
	return nil
}

func writeConfigFile(cfg Config) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".every-mysql")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		return err
	}

	content := fmt.Sprintf(`host: %s
port: %d
user: %s
password: %s
database: %s
`, cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database)

	return os.WriteFile(filepath.Join(configDir, cfg.Database+".yaml"), []byte(content), 0o600)
}