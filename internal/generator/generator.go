package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func Generate(schema *Schema, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	staticFiles := map[string]string{
		"main.go":   renderMain(schema),
		"db.go":     dbSource,
		"guard.go":  guardSource,
		"output.go": outputSource,
		"config.go": renderConfig(schema),
	}

	for name, content := range staticFiles {
		if err := os.WriteFile(filepath.Join(outputDir, name), []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", name, err)
		}
	}

	for _, table := range schema.Tables {
		content := renderTableCmd(schema, &table)
		filename := fmt.Sprintf("%s_cmd.go", table.Name)
		if err := os.WriteFile(filepath.Join(outputDir, filename), []byte(content), 0644); err != nil {
			return fmt.Errorf("write %s: %w", filename, err)
		}
	}

	if err := runBuild(outputDir, schema.Database); err != nil {
		return fmt.Errorf("build generated CLI: %w", err)
	}

	return nil
}

func runBuild(dir string, dbName string) error {
	modInit := exec.Command("go", "mod", "init", fmt.Sprintf("%s-cli", dbName))
	modInit.Dir = dir
	if out, err := modInit.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod init: %w\n%s", err, out)
	}

	modTidy := exec.Command("go", "mod", "tidy")
	modTidy.Dir = dir
	if out, err := modTidy.CombinedOutput(); err != nil {
		return fmt.Errorf("go mod tidy: %w\n%s", err, out)
	}

	goBuild := exec.Command("go", "build", "-o", fmt.Sprintf("%s-cli", dbName))
	goBuild.Dir = dir
	if out, err := goBuild.CombinedOutput(); err != nil {
		return fmt.Errorf("go build: %w\n%s", err, out)
	}

	return nil
}

func renderMain(schema *Schema) string {
	var adds []string
	for _, t := range schema.Tables {
		adds = append(adds, fmt.Sprintf("\trootCmd.AddCommand(%sCmd())", t.Name))
	}
	return fmt.Sprintf(`package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "%s-cli",
	Short: "CLI for %s database",
}

func main() {
	initDB()
%s
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
`, schema.Database, schema.Database, strings.Join(adds, "\n"))
}

func renderConfig(schema *Schema) string {
	return fmt.Sprintf(`package main

import (
	"os"
	"strconv"

	"gopkg.in/yaml.v3"
)

type dbConfig struct {
	Host     string ` + "`yaml:\"host\"`" + `
	Port     int    ` + "`yaml:\"port\"`" + `
	User     string ` + "`yaml:\"user\"`" + `
	Password string ` + "`yaml:\"password\"`" + `
	Database string ` + "`yaml:\"database\"`" + `
}

func loadConfig() *dbConfig {
	cfg := &dbConfig{
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Database: "%s",
	}

	home, _ := os.UserHomeDir()
	cfgPath := home + "/.every-mysql/%s.yaml"
	data, err := os.ReadFile(cfgPath)
	if err == nil {
		yaml.Unmarshal(data, cfg)
	}

	if v := os.Getenv("DB_HOST"); v != "" {
		cfg.Host = v
	}
	if v := os.Getenv("DB_PORT"); v != "" {
		cfg.Port, _ = strconv.Atoi(v)
	}
	if v := os.Getenv("DB_USER"); v != "" {
		cfg.User = v
	}
	if v := os.Getenv("DB_PASSWORD"); v != "" {
		cfg.Password = v
	}
	if v := os.Getenv("DB_NAME"); v != "" {
		cfg.Database = v
	}

	return cfg
}
`, schema.Database, schema.Database)
}

func renderTableCmd(schema *Schema, t *Table) string {
	colNames := columnNames(t)
	scanVars := scanVarDecls(t)
	scanList := scanRefList(t)
	jsonEntries := jsonMapEntries(t)
	fmtEntries := fmtEntries(t)

	var b strings.Builder

	// Imports
	b.WriteString("package main\n\n")
	b.WriteString("import (\n")
	b.WriteString(`	"database/sql"` + "\n")
	b.WriteString(`	"fmt"` + "\n")
	b.WriteString(`	"strconv"` + "\n")
	b.WriteString(`	"strings"` + "\n\n")
	b.WriteString(`	"github.com/spf13/cobra"` + "\n")
	b.WriteString(")\n\n")

	// Command function
	b.WriteString(fmt.Sprintf("func %sCmd() *cobra.Command {\n", t.Name))
	b.WriteString(fmt.Sprintf(`	cmd := &cobra.Command{Use: "%s", Short: "Operations on %s table"}`, t.Name, t.Name) + "\n\n")

	// List command — always present
	b.WriteString(fmt.Sprintf(`	listCmd := &cobra.Command{Use: "list", Short: "List %s rows", RunE: %sList}`, t.Name, t.Name) + "\n")
	b.WriteString(`	listCmd.Flags().Bool("json", false, "Output as JSON")` + "\n")
	b.WriteString(`	listCmd.Flags().Int("limit", 100, "Max rows")` + "\n")
	b.WriteString(`	listCmd.Flags().Int("offset", 0, "Skip rows")` + "\n")
	b.WriteString(`	listCmd.Flags().String("order-by", "", "Order by column")` + "\n")
	b.WriteString(`	listCmd.Flags().String("order-dir", "asc", "Order direction (asc/desc)")` + "\n")
	for _, c := range t.Columns {
		b.WriteString(fmt.Sprintf(`	listCmd.Flags().String("%s", "", "Filter by %s")`, snakeName(c.Name), c.Name) + "\n")
		if c.GoType == "string" {
			b.WriteString(fmt.Sprintf(`	listCmd.Flags().String("%s_like", "", "Filter %s with LIKE pattern")`, snakeName(c.Name), c.Name) + "\n")
		}
	}
	for _, fk := range t.ForeignKeys {
		b.WriteString(fmt.Sprintf(`	listCmd.Flags().String("by-%s", "", "Filter by %s (FK %s)")`, snakeName(fk.ReferencedTable), fk.ReferencedTable, fk.Column) + "\n")
	}

	// Create command — always present
	b.WriteString(fmt.Sprintf("\n\tcreateCmd := &cobra.Command{Use: \"create\", Short: \"Create a %s row\", RunE: %sCreate}\n", t.Name, t.Name))
	b.WriteString(`	createCmd.Flags().Bool("json", false, "Output as JSON")` + "\n")
	for _, c := range t.NonAutoIncrementColumns() {
		b.WriteString(fmt.Sprintf(`	createCmd.Flags().String("%s", "", "Value for %s")`, snakeName(c.Name), c.Name) + "\n")
	}

	cmdAddList := "\n\tcmd.AddCommand(listCmd, createCmd)"

	if t.HasPK() {
		// Get command
		pkArgCount := len(t.PrimaryKey.Columns)
		b.WriteString(fmt.Sprintf("\n\tgetCmd := &cobra.Command{Use: \"get [pk]\", Short: \"Get a %s row by PK\", Args: cobra.MinimumNArgs(%d), RunE: %sGet}\n", t.Name, pkArgCount, t.Name))
		b.WriteString(`	getCmd.Flags().Bool("json", false, "Output as JSON")` + "\n")
		for _, ref := range t.ReferencedBy {
			b.WriteString(fmt.Sprintf(`	getCmd.Flags().Bool("with-%s", false, "Eager-load %s")`, snakeName(ref.SourceTable), ref.SourceTable) + "\n")
		}

		// Update command
		b.WriteString(fmt.Sprintf("\n\tupdateCmd := &cobra.Command{Use: \"update [pk]\", Short: \"Update a %s row by PK\", Args: cobra.MinimumNArgs(%d), RunE: %sUpdate}\n", t.Name, pkArgCount, t.Name))
		b.WriteString(`	updateCmd.Flags().Bool("force", false, "Confirm update")` + "\n")
		for _, c := range t.NonAutoIncrementColumns() {
			b.WriteString(fmt.Sprintf(`	updateCmd.Flags().String("%s", "", "New value for %s")`, snakeName(c.Name), c.Name) + "\n")
		}

		// Delete command
		b.WriteString(fmt.Sprintf("\n\tdeleteCmd := &cobra.Command{Use: \"delete [pk]\", Short: \"Delete a %s row by PK (requires --force)", t.Name))
		b.WriteString(fmt.Sprintf(`\", Args: cobra.MinimumNArgs(%d), RunE: %sDelete}`, pkArgCount, t.Name) + "\n")
		b.WriteString(`	deleteCmd.Flags().Bool("force", false, "Confirm delete")` + "\n")
		b.WriteString(`	deleteCmd.PreRunE = requireForce` + "\n")

		cmdAddList = "\n\tcmd.AddCommand(listCmd, getCmd, createCmd, updateCmd, deleteCmd)"
	}

	b.WriteString(cmdAddList + "\n\n\treturn cmd\n}\n\n")

	// List function
	b.WriteString(renderListFunc(t, colNames, scanVars, scanList, jsonEntries, fmtEntries))

	if t.HasPK() {
		b.WriteString(renderGetFunc(t))
		b.WriteString(renderCreateFunc(t))
		b.WriteString(renderUpdateFunc(t))
		b.WriteString(renderDeleteFunc(t))
	} else {
		b.WriteString(renderCreateFuncNoPK(t))
	}

	return b.String()
}

func renderListFunc(t *Table, colNames string, scanVars string, scanList string, jsonEntries string, fmtEntries string) string {
	var filterCode strings.Builder
	for _, c := range t.Columns {
		filterCode.WriteString(fmt.Sprintf(`	if v, _ := cmd.Flags().GetString("%s"); v != "" {
		conditions = append(conditions, "%s.%s = ?")
		condArgs = append(condArgs, v)
	}
`, snakeName(c.Name), t.Name, c.Name))
		if c.GoType == "string" {
			filterCode.WriteString(fmt.Sprintf(`	if v, _ := cmd.Flags().GetString("%s_like"); v != "" {
		conditions = append(conditions, "%s.%s LIKE ?")
		condArgs = append(condArgs, v)
	}
`, snakeName(c.Name), t.Name, c.Name))
		}
	}
	for _, fk := range t.ForeignKeys {
		filterCode.WriteString(fmt.Sprintf(`	if v, _ := cmd.Flags().GetString("by-%s"); v != "" {
		conditions = append(conditions, "%s.%s = ?")
		condArgs = append(condArgs, v)
	}
`, snakeName(fk.ReferencedTable), t.Name, fk.Column))
	}

	return fmt.Sprintf(`func %sList(cmd *cobra.Command, args []string) error {
	jsonOut, _ := cmd.Flags().GetBool("json")
	limit, _ := cmd.Flags().GetInt("limit")
	offset, _ := cmd.Flags().GetInt("offset")
	orderBy, _ := cmd.Flags().GetString("order-by")
	orderDir, _ := cmd.Flags().GetString("order-dir")

	var conditions []string
	var condArgs []interface{}

%s	query := "SELECT %s FROM %s"
	if len(conditions) > 0 {
		query += " WHERE " + strings.Join(conditions, " AND ")
	}
	if orderBy != "" {
		query += " ORDER BY " + orderBy + " " + orderDir
	}
	query += " LIMIT ? OFFSET ?"
	condArgs = append(condArgs, limit, offset)

	rows, err := db.Query(query, condArgs...)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
%s		err := rows.Scan(%s)
		if err != nil {
			return err
		}
		if jsonOut {
			jsonRows = append(jsonRows, map[string]interface{}{
%s			})
		} else {
			dataRows = append(dataRows, []string{
%s			})
		}
	}

	if jsonOut {
		printJSON(listResult{Data: jsonRows, Meta: listMeta{Total: len(jsonRows), Limit: limit, Offset: offset}})
	} else {
		printTable(headers, dataRows)
	}
	return nil
}
`, t.Name, filterCode.String(), colNames, t.Name, scanVars, scanList, jsonEntries, fmtEntries)
}

func renderGetFunc(t *Table) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("func %sGet(cmd *cobra.Command, args []string) error {\n", t.Name))
	b.WriteString(`	jsonOut, _ := cmd.Flags().GetBool("json")` + "\n\n")

	if t.IsCompositePK() {
		for i, pkCol := range t.PrimaryKey.Columns {
			goType := goTypeForColumn(t, pkCol)
			varName := lowerCamelName(pkCol)
			b.WriteString(fmt.Sprintf(`	%s, err := parseArg[%s](args[%d], "%s")
	if err != nil {
		return err
	}
`, varName, goType, i, pkCol))
		}
	} else {
		pkCol := t.PrimaryKey.Columns[0]
		b.WriteString(fmt.Sprintf(`	pkVal := args[0]` + "\n"))
		b.WriteString(fmt.Sprintf(`	pkCol := "%s"` + "\n\n", pkCol))
	}

	// Query
	b.WriteString(fmt.Sprintf(`	query := "SELECT %s FROM %s WHERE `, columnNames(t), t.Name))
	if t.IsCompositePK() {
		parts := make([]string, len(t.PrimaryKey.Columns))
		for i, pkCol := range t.PrimaryKey.Columns {
			parts[i] = fmt.Sprintf("%s = ?", pkCol)
		}
		b.WriteString(strings.Join(parts, " AND "))
		b.WriteString(`"` + "\n")
		argList := make([]string, len(t.PrimaryKey.Columns))
		for _, pkCol := range t.PrimaryKey.Columns {
			argList = append(argList, lowerCamelName(pkCol))
		}
		b.WriteString(fmt.Sprintf(`	row := db.QueryRow(query, %s)` + "\n", strings.Join(argList, ", ")))
	} else {
		b.WriteString(fmt.Sprintf(`" + pkCol + " = ?"` + "\n"))
		b.WriteString(`	row := db.QueryRow(query, pkVal)` + "\n")
	}

	b.WriteString("\n")
	b.WriteString(scanVarDecls(t) + "\n")
	b.WriteString(fmt.Sprintf(`	err := row.Scan(%s)` + "\n", scanRefList(t)))
	b.WriteString(`	if err == sql.ErrNoRows {
		if jsonOut {
			printJSON(map[string]interface{}{"data": nil, "error": "row not found"})
		} else {
			fmt.Println("No row found")
		}
		return nil
	}
	if err != nil {
		return err
	}
`)

	// With-<table> eager loading
	for _, ref := range t.ReferencedBy {
		b.WriteString(fmt.Sprintf(`	with%s, _ := cmd.Flags().GetBool("with-%s")
	if with%s {
		relatedRows, err := db.Query("SELECT * FROM %s WHERE %s = ?", `, camelName(ref.SourceTable), snakeName(ref.SourceTable), camelName(ref.SourceTable), ref.SourceTable, ref.SourceColumn))
		if t.IsCompositePK() {
			b.WriteString(lowerCamelName(t.PrimaryKey.Columns[0]))
		} else {
			b.WriteString("pkVal")
		}
		b.WriteString(`)
		if err != nil {
			return err
		}
		defer relatedRows.Close()
		var ` + lowerCamelName(ref.SourceTable) + `List []map[string]interface{}
		for relatedRows.Next() {
			r := make(map[string]interface{})
			` + lowerCamelName(ref.SourceTable) + `List = append(` + lowerCamelName(ref.SourceTable) + `List, r)
		}
		if jsonOut {
			printJSON(map[string]interface{}{"` + ref.SourceTable + `": ` + lowerCamelName(ref.SourceTable) + `List})
		}
	}
`)
	}

	b.WriteString("\n\tif jsonOut {\n")
	b.WriteString(`		printJSON(map[string]interface{}{
` + jsonMapEntries(t) + `		})
	} else {
		fmt.Println("` + fmtEntriesFlat(t) + `")
	}
	return nil
}
`)

	return b.String()
}

func renderCreateFunc(t *Table) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("func %sCreate(cmd *cobra.Command, args []string) error {\n", t.Name))
	b.WriteString(`	jsonOut, _ := cmd.Flags().GetBool("json")
	var cols []string
	var vals []interface{}
	var placeholders []string
`)

	for _, c := range t.NonAutoIncrementColumns() {
		b.WriteString(fmt.Sprintf(`	if v, _ := cmd.Flags().GetString("%s"); v != "" {
		cols = append(cols, "%s")
		placeholders = append(placeholders, "?")
`, snakeName(c.Name), c.Name))
		b.WriteString(parseValueCode(c, "vals"))
		b.WriteString("	}\n")
	}

	b.WriteString(`
	if len(cols) == 0 {
		return fmt.Errorf("at least one column value required")
	}

	query := "INSERT INTO ` + t.Name + ` (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	result, err := db.Exec(query, vals...)
	if err != nil {
		return err
	}

	id, _ := result.LastInsertId()

	if jsonOut {
		printJSON(map[string]interface{}{"id": id, "affected": 1})
	} else {
		fmt.Printf("Inserted row with ID %d\\n", id)
	}
	return nil
}
`)
	return b.String()
}

func renderCreateFuncNoPK(t *Table) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("func %sCreate(cmd *cobra.Command, args []string) error {\n", t.Name))
	b.WriteString(`	jsonOut, _ := cmd.Flags().GetBool("json")
	var cols []string
	var vals []interface{}
	var placeholders []string
`)

	for _, c := range t.NonAutoIncrementColumns() {
		b.WriteString(fmt.Sprintf(`	if v, _ := cmd.Flags().GetString("%s"); v != "" {
		cols = append(cols, "%s")
		placeholders = append(placeholders, "?")
`, snakeName(c.Name), c.Name))
		b.WriteString(parseValueCode(c, "vals"))
		b.WriteString("	}\n")
	}

	b.WriteString(`
	if len(cols) == 0 {
		return fmt.Errorf("at least one column value required")
	}

	query := "INSERT INTO ` + t.Name + ` (" + strings.Join(cols, ", ") + ") VALUES (" + strings.Join(placeholders, ", ") + ")"
	result, err := db.Exec(query, vals...)
	if err != nil {
		return err
	}

	if jsonOut {
		printJSON(map[string]interface{}{"affected": 1})
	} else {
		fmt.Println("Inserted row")
	}
	return nil
}
`)
	return b.String()
}

func renderUpdateFunc(t *Table) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("func %sUpdate(cmd *cobra.Command, args []string) error {\n", t.Name))
	b.WriteString(`	var cols []string
	var vals []interface{}
`)

	for _, c := range t.NonAutoIncrementColumns() {
		b.WriteString(fmt.Sprintf(`	if v, _ := cmd.Flags().GetString("%s"); v != "" {
		cols = append(cols, "%s = ?")
`, snakeName(c.Name), c.Name))
		b.WriteString(parseValueCode(c, "vals"))
		b.WriteString("	}\n")
	}

	b.WriteString(`
	if len(cols) == 0 {
		return fmt.Errorf("at least one column value required")
	}
`)

	if t.IsCompositePK() {
		whereParts := make([]string, len(t.PrimaryKey.Columns))
		for i, pkCol := range t.PrimaryKey.Columns {
			whereParts[i] = fmt.Sprintf("%s = ?", pkCol)
			goType := goTypeForColumn(t, pkCol)
			b.WriteString(fmt.Sprintf(`	pk%d, err := parseArg[%s](args[%d], "%s")
	if err != nil {
		return err
	}
	vals = append(vals, pk%d)
`, i, goType, i, pkCol, i))
		}
		b.WriteString(fmt.Sprintf(`	query := "UPDATE %s SET " + strings.Join(cols, ", ") + " WHERE %s"
`, t.Name, strings.Join(whereParts, " AND ")))
		pkArgs := make([]string, len(t.PrimaryKey.Columns))
		for i := range t.PrimaryKey.Columns {
			pkArgs[i] = fmt.Sprintf("pk%d", i)
		}
	} else {
		pkCol := t.PrimaryKey.Columns[0]
		b.WriteString(fmt.Sprintf(`	query := "UPDATE %s SET " + strings.Join(cols, ", ") + " WHERE %s = ?"
	vals = append(vals, args[0])
`, t.Name, pkCol))
	}

	b.WriteString(`
	result, err := db.Exec(query, vals...)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	fmt.Printf("Updated %d row(s)\\n", affected)
	return nil
}
`)
	return b.String()
}

func renderDeleteFunc(t *Table) string {
	var b strings.Builder

	b.WriteString(fmt.Sprintf("func %sDelete(cmd *cobra.Command, args []string) error {\n", t.Name))

	if t.IsCompositePK() {
		whereParts := make([]string, len(t.PrimaryKey.Columns))
		for i, pkCol := range t.PrimaryKey.Columns {
			whereParts[i] = fmt.Sprintf("%s = ?", pkCol)
			goType := goTypeForColumn(t, pkCol)
			b.WriteString(fmt.Sprintf(`	pk%d, err := parseArg[%s](args[%d], "%s")
	if err != nil {
		return err
	}
`, i, goType, i, pkCol))
		}
		pkArgs := make([]string, len(t.PrimaryKey.Columns))
		for i := range t.PrimaryKey.Columns {
			pkArgs[i] = fmt.Sprintf("pk%d", i)
		}
		b.WriteString(fmt.Sprintf(`	query := "DELETE FROM %s WHERE %s"
	result, err := db.Exec(query, %s)
`, t.Name, strings.Join(whereParts, " AND "), strings.Join(pkArgs, ", ")))
	} else {
		pkCol := t.PrimaryKey.Columns[0]
		b.WriteString(fmt.Sprintf(`	query := "DELETE FROM %s WHERE %s = ?"
	result, err := db.Exec(query, args[0])
`, t.Name, pkCol))
	}

	b.WriteString(`	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	fmt.Printf("Deleted %d row(s)\\n", affected)
	return nil
}
`)
	return b.String()
}

// Helper functions for string generation

func columnNames(t *Table) string {
	names := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		names[i] = c.Name
	}
	return strings.Join(names, ", ")
}

func scanVarDecls(t *Table) string {
	var lines []string
	for _, c := range t.Columns {
		lines = append(lines, fmt.Sprintf("\tvar %s %s", lowerCamelName(c.Name), c.GoType))
	}
	return strings.Join(lines, "\n") + "\n\n\tvar jsonRows []map[string]interface{}\n\tvar headers []string\n\tvar dataRows [][]string"
}

func scanRefList(t *Table) string {
	refs := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		refs[i] = "&" + lowerCamelName(c.Name)
	}
	return strings.Join(refs, ", ")
}

func jsonMapEntries(t *Table) string {
	entries := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		entries[i] = fmt.Sprintf("\t\t\t\"%s\": %s,", c.Name, lowerCamelName(c.Name))
	}
	return strings.Join(entries, "\n")
}

func fmtEntries(t *Table) string {
	entries := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		entries[i] = fmt.Sprintf("\t\t\tfmt.Sprintf(\"%%v\", %s),", lowerCamelName(c.Name))
	}
	return strings.Join(entries, "\n")
}

func fmtEntriesFlat(t *Table) string {
	entries := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		entries[i] = fmt.Sprintf("%s: %%v", c.Name)
	}
	format := strings.Join(entries, "\\n")
	args := make([]string, len(t.Columns))
	for i, c := range t.Columns {
		args[i] = lowerCamelName(c.Name)
	}
	return fmt.Sprintf("%s\", %s)", format, strings.Join(args, ", "))
}

func parseValueCode(c Column, target string) string {
	switch c.GoType {
	case "int64":
		return fmt.Sprintf(`		n, err := strconv.ParseInt(v, 10, 64)
		if err != nil { return fmt.Errorf("invalid %s: %%w", err) }
		%s = append(%s, n)
`, c.Name, target, target)
	case "float64":
		return fmt.Sprintf(`		f, err := strconv.ParseFloat(v, 64)
		if err != nil { return fmt.Errorf("invalid %s: %%w", err) }
		%s = append(%s, f)
`, c.Name, target, target)
	case "bool":
		return fmt.Sprintf(`		b, err := strconv.ParseBool(v)
		if err != nil { return fmt.Errorf("invalid %s: %%w", err) }
		%s = append(%s, b)
`, c.Name, target, target)
	default:
		return fmt.Sprintf(`		%s = append(%s, v)
`, target, target)
	}
}

func goTypeForColumn(t *Table, colName string) string {
	for _, c := range t.Columns {
		if c.Name == colName {
			return c.GoType
		}
	}
	return "string"
}

// Name conversion helpers

func snakeName(s string) string {
	var result strings.Builder
	for i, r := range s {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				result.WriteByte('_')
			}
			result.WriteByte(byte(r + 32))
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func camelName(s string) string {
	parts := strings.Split(s, "_")
	for i := range parts {
		if len(parts[i]) > 0 {
			parts[i] = strings.ToUpper(parts[i][:1]) + parts[i][1:]
		}
	}
	return strings.Join(parts, "")
}

func lowerCamelName(s string) string {
	c := camelName(s)
	if len(c) > 0 && c[0] >= 'A' && c[0] <= 'Z' {
		return strings.ToLower(c[:1]) + c[1:]
	}
	return c
}

// Static source files for generated CLI

const dbSource = `package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"

	_ "github.com/go-sql-driver/mysql"
)

var db *sql.DB

func initDB() {
	cfg := loadConfig()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	conn, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintln(os.Stderr, "db open:", err)
		os.Exit(1)
	}
	conn.SetMaxOpenConns(10)
	conn.SetMaxIdleConns(5)
	db = conn
}

func parseArg[T any](s string, name string) (T, error) {
	var zero T
	switch any(zero) {
	case int64(0):
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil { return zero, fmt.Errorf("invalid %s: %%w", name, err) }
		return any(n).(T), nil
	case float64(0):
		f, err := strconv.ParseFloat(s, 64)
		if err != nil { return zero, fmt.Errorf("invalid %s: %%w", name, err) }
		return any(f).(T), nil
	case bool(false):
		b, err := strconv.ParseBool(s)
		if err != nil { return zero, fmt.Errorf("invalid %s: %%w", name, err) }
		return any(b).(T), nil
	default:
		return any(s).(T), nil
	}
}
`

const guardSource = `package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func requireForce(cmd *cobra.Command, args []string) error {
	force, _ := cmd.Flags().GetBool("force")
	if !force {
		fmt.Fprintln(os.Stderr, "destructive operation requires --force")
		fmt.Fprintln(os.Stderr, "re-run with --force flag")
		os.Exit(1)
	}
	return nil
}
`

const outputSource = `package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/olekukonko/tablewriter"
)

func printTable(headers []string, rows [][]string) {
	tw := tablewriter.NewWriter(os.Stdout)
	tw.SetHeader(headers)
	tw.SetAutoWrapText(false)
	tw.SetAutoFormatHeaders(true)
	tw.AppendBulk(rows)
	tw.Render()
}

func printJSON(data interface{}) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Fprintln(os.Stderr, "json marshal:", err)
		os.Exit(1)
	}
	fmt.Println(string(b))
}

type listMeta struct {
	Total  int ` + "`json:\"total\"`" + `
	Limit  int ` + "`json:\"limit\"`" + `
	Offset int ` + "`json:\"offset\"`" + `
}

type listResult struct {
	Data interface{} ` + "`json:\"data\"`" + `
	Meta listMeta    ` + "`json:\"meta\"`" + `
}
`