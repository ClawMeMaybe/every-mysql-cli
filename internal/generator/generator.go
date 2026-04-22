package generator

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"text/template"

	"github.com/kefan/every-mysql-cli/internal/types"
	"gopkg.in/yaml.v3"
)

func Generate(schema *types.Schema, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("creating output directory: %w", err)
	}

	funcs := TemplateFuncs()

	// Generate shared modules
	sharedFiles := map[string]string{
		"main.go":   MainTemplate,
		"db.go":     DBTemplate,
		"guard.go":  GuardTemplate,
		"output.go": OutputTemplate,
		"config.go": ConfigTemplate,
	}
	for filename, tmplStr := range sharedFiles {
		if err := renderTemplate(outputDir, filename, tmplStr, schema, funcs); err != nil {
			return err
		}
	}

	// Generate per-table command files
	for _, table := range schema.Tables {
		filename := table.Name + "_cmd.go"
		if err := renderTemplate(outputDir, filename, TableCmdTemplate, &table, funcs); err != nil {
			return err
		}

		// Note tables without primary keys
		if table.PrimaryKey == nil || len(table.PrimaryKey.Columns) == 0 {
			fmt.Printf("Note: table %s has no primary key; get/update/delete commands omitted\n", table.Name)
		}
	}

	// Write go.mod
	goModContent := fmt.Sprintf(GoModTemplate, schema.Database)
	if err := os.WriteFile(filepath.Join(outputDir, "go.mod"), []byte(goModContent), 0644); err != nil {
		return fmt.Errorf("writing go.mod: %w", err)
	}

	// Run go mod tidy
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = outputDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy failed: %w\nOutput: %s\nCheck Go toolchain installation", err, string(output))
	}

	// Run go build
	binaryName := schema.Database + "-cli"
	cmd = exec.Command("go", "build", "-o", binaryName, ".")
	cmd.Dir = outputDir
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("go build failed: %w\nOutput: %s\nCheck Go toolchain installation", err, string(output))
	}

	return nil
}

func WriteConfig(schema *types.Schema, host, port, user, password string) error {
	configDir := filepath.Join(os.Getenv("HOME"), ".every-mysql")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("creating config directory: %w", err)
	}

	config := map[string]interface{}{
		"host":     host,
		"port":     port,
		"user":     user,
		"password": password,
		"database": schema.Database,
	}

	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}

	configPath := filepath.Join(configDir, schema.Database+".yaml")
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("writing config file: %w", err)
	}

	return nil
}

func renderTemplate(dir, filename, tmplStr string, data interface{}, funcs map[string]interface{}) error {
	tmpl, err := template.New(filename).Funcs(funcs).Parse(tmplStr)
	if err != nil {
		return fmt.Errorf("parsing template %s: %w", filename, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("executing template %s: %w", filename, err)
	}

	path := filepath.Join(dir, filename)
	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filename, err)
	}
	return nil
}