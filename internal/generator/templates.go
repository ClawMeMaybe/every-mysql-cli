package generator

const GoModTemplate = `module %[1]s-cli

go 1.22

require (
	github.com/go-sql-driver/mysql v1.8.1
	github.com/olekukonko/tablewriter v0.0.5
	github.com/spf13/cobra v1.8.0
	gopkg.in/yaml.v3 v3.0.1
)
`

const MainTemplate = `package main

import (
	"os"
	"github.com/spf13/cobra"
)

func main() {
	rootCmd := &cobra.Command{
		Use:   "{{ .Database }}-cli",
		Short: "CLI for {{ .Database }} database",
	}

	db := initDB()
	defer db.Close()

{{ range .Tables }}
	rootCmd.AddCommand({{ .Name }}Cmd(db))
{{ end }}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
`

const DBTemplate = `package main

import (
	"database/sql"
	"fmt"
	"os"
	_ "github.com/go-sql-driver/mysql"
)

func initDB() *sql.DB {
	cfg := loadConfig()
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error connecting to database: %v\n", err)
		os.Exit(1)
	}
	return db
}
`

const GuardTemplate = `package main

import (
	"fmt"
	"os"
)

func requireForce(cmdPath string) {
	fmt.Fprintf(os.Stderr, "Warning: destructive operation requires --force flag.\n")
	fmt.Fprintf(os.Stderr, "Hint: re-run with --force flag: %s --force\n", cmdPath)
	os.Exit(1)
}

func requireForceWithConfirm(cmdPath, confirmMsg string) {
	fmt.Fprintf(os.Stderr, "Warning: bulk destructive operation requires --force and --confirm flags.\n")
	fmt.Fprintf(os.Stderr, "Hint: re-run with --force --confirm \"%s\"\n", confirmMsg)
	os.Exit(1)
}

func guardDestructiveJSON(errMsg, hint string) {
	fmt.Printf("{\"error\": \"%s\", \"hint\": \"%s\"}\n", errMsg, hint)
	os.Exit(1)
}
`

const OutputTemplate = `package main

import (
	"encoding/json"
	"fmt"
	"os"
	"github.com/olekukonko/tablewriter"
)

func printTable(headers []string, rows [][]string) {
	table := tablewriter.NewWriter(os.Stdout)
	table.SetHeader(headers)
	table.SetAutoWrapText(false)
	table.SetAutoFormatHeaders(true)
	table.SetHeaderAlignment(tablewriter.ALIGN_LEFT)
	table.SetAlignment(tablewriter.ALIGN_LEFT)
	table.SetCenterSeparator("")
	table.SetColumnSeparator("")
	table.SetRowSeparator("")
	table.SetHeaderLine(false)
	table.SetBorder(false)
	table.SetTablePadding("\t")
	table.SetNoWhiteSpace(true)
	for _, row := range rows {
		table.Append(row)
	}
	table.Render()
}

func printKV(keys []string, values []string) {
	for i, k := range keys {
		fmt.Printf("%s\t%s\n", k, values[i])
	}
}

func printJSONData(data interface{}) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(data)
}

func printJSONList(data []interface{}, total int, limit int, offset int) {
	result := map[string]interface{}{
		"data": data,
		"meta": map[string]interface{}{
			"total":  total,
			"limit":  limit,
			"offset": offset,
		},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	enc.Encode(result)
}

func printJSONError(code, message string) {
	fmt.Printf("{\"error\": \"%s\", \"code\": \"%s\"}\n", message, code)
	os.Exit(1)
}
`

const ConfigTemplate = `package main

import (
	"os"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

type DBConfig struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

func loadConfig() *DBConfig {
	cfg := &DBConfig{}

	// Priority 1: Environment variables
	cfg.Host = os.Getenv("DB_HOST")
	cfg.Port = os.Getenv("DB_PORT")
	cfg.User = os.Getenv("DB_USER")
	cfg.Password = os.Getenv("DB_PASSWORD")
	cfg.Database = os.Getenv("DB_NAME")

	// Priority 2: Config file
	if cfg.Host == "" || cfg.Port == "" || cfg.User == "" || cfg.Database == "" {
		configPath := os.Getenv("HOME") + "/.every-mysql/{{ .Database }}.yaml"
		data, err := os.ReadFile(configPath)
		if err == nil {
			var fileCfg DBConfig
			if yaml.Unmarshal(data, &fileCfg) == nil {
				if cfg.Host == "" { cfg.Host = fileCfg.Host }
				if cfg.Port == "" { cfg.Port = fileCfg.Port }
				if cfg.User == "" { cfg.User = fileCfg.User }
				if cfg.Password == "" { cfg.Password = fileCfg.Password }
				if cfg.Database == "" { cfg.Database = fileCfg.Database }
			}
		}
	}

	// Defaults
	if cfg.Host == "" { cfg.Host = "localhost" }
	if cfg.Port == "" { cfg.Port = "3306" }

	return cfg
}

func addDBFlags(cmd *cobra.Command) {
	cmd.Flags().String("db-host", "", "Database host")
	cmd.Flags().String("db-port", "", "Database port")
	cmd.Flags().String("db-user", "", "Database user")
	cmd.Flags().String("db-password", "", "Database password")
	cmd.Flags().String("db-name", "", "Database name")
}

func applyDBFlags(cfg *DBConfig, cmd *cobra.Command) *DBConfig {
	if v, _ := cmd.Flags().GetString("db-host"); v != "" { cfg.Host = v }
	if v, _ := cmd.Flags().GetString("db-port"); v != "" { cfg.Port = v }
	if v, _ := cmd.Flags().GetString("db-user"); v != "" { cfg.User = v }
	if v, _ := cmd.Flags().GetString("db-password"); v != "" { cfg.Password = v }
	if v, _ := cmd.Flags().GetString("db-name"); v != "" { cfg.Database = v }
	return cfg
}
`

const TableCmdTemplate = `package main

import (
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"github.com/spf13/cobra"
)

func {{ .Name }}Cmd(db *sql.DB) *cobra.Command {
	group := &cobra.Command{
		Use:   "{{ .Name }}",
		Short: "Commands for the {{ .Name }} table",
	}
	group.AddCommand({{ .Name }}ListCmd(db))
{{ if .PrimaryKey }}
	group.AddCommand({{ .Name }}GetCmd(db))
	group.AddCommand({{ .Name }}CreateCmd(db))
	group.AddCommand({{ .Name }}UpdateCmd(db))
	group.AddCommand({{ .Name }}DeleteCmd(db))
{{ else }}
	// Note: {{ .Name }} has no primary key; get/update/delete omitted
{{ end }}
	return group
}

func {{ .Name }}ListCmd(db *sql.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List rows from {{ .Name }}",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")
			limit, _ := cmd.Flags().GetInt("limit")
			offset, _ := cmd.Flags().GetInt("offset")
			orderBy, _ := cmd.Flags().GetString("order-by")
			orderDir, _ := cmd.Flags().GetString("order-dir")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if limit <= 0 { limit = 100 }

			query := "SELECT * FROM {{ .Name }}"
			var conditions []string
			var qargs []interface{}

{{ range .Columns }}
			if v, _ := cmd.Flags().GetString("{{ .Name }}"); v != "" {
				conditions = append(conditions, "{{ .Name }} = ?")
				qargs = append(qargs, v)
			}
{{ if eq .GoType "string" }}
			if v, _ := cmd.Flags().GetString("{{ .Name }}_like"); v != "" {
				conditions = append(conditions, "{{ .Name }} LIKE ?")
				qargs = append(qargs, v)
			}
{{ end }}
{{ end }}

{{ range .ForeignKeys }}
			if v, _ := cmd.Flags().GetString("by-{{ .ReferencedTable }}"); v != "" {
				conditions = append(conditions, "{{ .Column }} = ?")
				qargs = append(qargs, v)
			}
{{ end }}

			if len(conditions) > 0 {
				query += " WHERE " + strings.Join(conditions, " AND ")
			}
			if orderBy != "" {
				query += fmt.Sprintf(" ORDER BY %s %s", orderBy, orderDir)
			}
			query += fmt.Sprintf(" LIMIT %d OFFSET %d", limit, offset)

			if dryRun {
				fmt.Println("SQL:", query)
				return nil
			}

			rows, err := db.Query(query, qargs...)
			if err != nil {
				if jsonMode { printJSONError("QUERY_ERROR", err.Error()) }
				return err
			}
			defer rows.Close()

			cols, _ := rows.Columns()
{{ range .Columns }}
{{ if .Nullable }}
{{ if eq .GoType "int64" }}
			var s_{{ .Name }} sql.NullInt64
{{ else if eq .GoType "float64" }}
			var s_{{ .Name }} sql.NullFloat64
{{ else if eq .GoType "bool" }}
			var s_{{ .Name }} sql.NullBool
{{ else }}
			var s_{{ .Name }} sql.NullString
{{ end }}
{{ else }}
			var s_{{ .Name }} {{ .GoType }}
{{ end }}
{{ end }}

			var tableRows [][]string
			var jsonData []interface{}
			scanArgs := []interface{}{
{{ range .Columns }}				&s_{{ .Name }},
{{ end }}
			}

			for rows.Scan(scanArgs...) == nil {
				row := []string{
{{ range .Columns }}
					{{ valueStr . }},
{{ end }}
				}
				tableRows = append(tableRows, row)
				if jsonMode {
					entry := map[string]interface{}{
{{ range .Columns }}
						"{{ .Name }}": {{ valueJSON . }},
{{ end }}
					}
					jsonData = append(jsonData, entry)
				}
			}

			if jsonMode {
				printJSONList(jsonData, len(jsonData), limit, offset)
			} else {
				printTable(cols, tableRows)
			}
			return nil
		},
	}

	cmd.Flags().Bool("json", false, "Output in JSON format (agent mode)")
	cmd.Flags().Int("limit", 100, "Maximum rows to return")
	cmd.Flags().Int("offset", 0, "Offset for pagination")
	cmd.Flags().String("order-by", "", "Column to order by")
	cmd.Flags().String("order-dir", "asc", "Order direction (asc/desc)")
	cmd.Flags().Bool("dry-run", false, "Print SQL without executing")
{{ range .Columns }}
	cmd.Flags().String("{{ .Name }}", "", "Filter by {{ .Name }}")
{{ if eq .GoType "string" }}
	cmd.Flags().String("{{ .Name }}_like", "", "Filter {{ .Name }} with LIKE pattern")
{{ end }}
{{ end }}
{{ range .ForeignKeys }}
	cmd.Flags().String("by-{{ .ReferencedTable }}", "", "Filter by {{ .ReferencedTable }} (FK: {{ .Column }})")
{{ end }}
	return cmd
}

{{ if .PrimaryKey }}

func {{ .Name }}GetCmd(db *sql.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "get [primary-key-value]",
		Short: "Get a single {{ .Name }} row by primary key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			pkVal := args[0]
			query := "SELECT * FROM {{ .Name }} WHERE {{ index .PrimaryKey.Columns 0 }} = ?"
			if dryRun {
				fmt.Println("SQL:", query, pkVal)
				return nil
			}
{{ range .Columns }}
{{ if .Nullable }}
{{ if eq .GoType "int64" }}
			var s_{{ .Name }} sql.NullInt64
{{ else if eq .GoType "float64" }}
			var s_{{ .Name }} sql.NullFloat64
{{ else if eq .GoType "bool" }}
			var s_{{ .Name }} sql.NullBool
{{ else }}
			var s_{{ .Name }} sql.NullString
{{ end }}
{{ else }}
			var s_{{ .Name }} {{ .GoType }}
{{ end }}
{{ end }}
			err := db.QueryRow(query, pkVal).Scan(
{{ range .Columns }}				&s_{{ .Name }},
{{ end }}
			)
			if err != nil {
				if err == sql.ErrNoRows {
					if jsonMode { printJSONError("NOT_FOUND", "row not found") }
					fmt.Fprintln(os.Stderr, "Row not found")
					return nil
				}
				if jsonMode { printJSONError("QUERY_ERROR", err.Error()) }
				return err
			}
{{ range .ReferencedBy }}
			if with{{ .SourceTable }}, _ := cmd.Flags().GetBool("with-{{ .SourceTable }}"); with{{ .SourceTable }} {
				_ = db.Query("SELECT * FROM {{ .SourceTable }} WHERE {{ .SourceColumn }} = ?", pkVal)
			}
{{ end }}
			keys := []string{ {{ range .Columns }}"{{ .Name }}", {{ end }} }
			values := []string{ {{ range .Columns }}{{ valueStr . }}, {{ end }} }
			if jsonMode {
				printJSONData(map[string]interface{}{
{{ range .Columns }}					"{{ .Name }}": {{ valueJSON . }},
{{ end }}
				})
			} else {
				printKV(keys, values)
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("dry-run", false, "Print SQL without executing")
{{ range .ReferencedBy }}
	cmd.Flags().Bool("with-{{ .SourceTable }}", false, "Eager-load related {{ .SourceTable }}")
{{ end }}
	return cmd
}

func {{ .Name }}CreateCmd(db *sql.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "create",
		Short: "Create a new {{ .Name }} row",
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			var columns []string
			var placeholders []string
			var values []interface{}
{{ range .Columns }}
{{ if not .AutoIncrement }}
			if v, _ := cmd.Flags().GetString("{{ .Name }}"); v != "" {
				columns = append(columns, "{{ .Name }}")
				placeholders = append(placeholders, "?")
				values = append(values, {{ createConv . }})
			}
{{ end }}
{{ end }}
			query := fmt.Sprintf("INSERT INTO {{ .Name }} (%s) VALUES (%s)", strings.Join(columns, ", "), strings.Join(placeholders, ", "))
			if dryRun {
				fmt.Println("SQL:", query, values)
				return nil
			}
			result, err := db.Exec(query, values...)
			if err != nil {
				if jsonMode { printJSONError("QUERY_ERROR", err.Error()) }
				return err
			}
			id, _ := result.LastInsertId()
			if jsonMode {
				printJSONData(map[string]interface{}{"id": id, "affected": 1})
			} else {
				fmt.Printf("Created row with ID %d\n", id)
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("dry-run", false, "Print SQL without executing")
{{ range .Columns }}
{{ if not .AutoIncrement }}
	cmd.Flags().String("{{ .Name }}", "", "Value for {{ .Name }}")
{{ end }}
{{ end }}
	return cmd
}

func {{ .Name }}UpdateCmd(db *sql.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "update [primary-key-value]",
		Short: "Update a {{ .Name }} row by primary key",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")
			force, _ := cmd.Flags().GetBool("force")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if len(args) == 0 && !force {
				if jsonMode {
					guardDestructiveJSON("destructive operation requires --force", "re-run with --force flag")
				}
				requireForce(cmd.CommandPath())
			}
			var setClauses []string
			var values []interface{}
{{ range .Columns }}
{{ if not .AutoIncrement }}
			if v, _ := cmd.Flags().GetString("{{ .Name }}"); v != "" {
				setClauses = append(setClauses, "{{ .Name }} = ?")
				values = append(values, {{ createConv . }})
			}
{{ end }}
{{ end }}
			query := fmt.Sprintf("UPDATE {{ .Name }} SET %s", strings.Join(setClauses, ", "))
			if len(args) == 1 {
				query += " WHERE {{ index .PrimaryKey.Columns 0 }} = ?"
				values = append(values, args[0])
			}
			if dryRun {
				fmt.Println("SQL:", query, values)
				return nil
			}
			result, err := db.Exec(query, values...)
			if err != nil {
				if jsonMode { printJSONError("QUERY_ERROR", err.Error()) }
				return err
			}
			affected, _ := result.RowsAffected()
			if jsonMode {
				printJSONData(map[string]interface{}{"affected": affected})
			} else {
				fmt.Printf("Updated %d row(s)\n", affected)
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("force", false, "Required for destructive operations")
	cmd.Flags().Bool("dry-run", false, "Print SQL without executing")
{{ range .Columns }}
{{ if not .AutoIncrement }}
	cmd.Flags().String("{{ .Name }}", "", "New value for {{ .Name }}")
{{ end }}
{{ end }}
	return cmd
}

func {{ .Name }}DeleteCmd(db *sql.DB) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete [primary-key-value]",
		Short: "Delete a {{ .Name }} row by primary key",
		Args:  cobra.RangeArgs(0, 1),
		RunE: func(cmd *cobra.Command, args []string) error {
			jsonMode, _ := cmd.Flags().GetBool("json")
			force, _ := cmd.Flags().GetBool("force")
			all, _ := cmd.Flags().GetBool("all")
			dryRun, _ := cmd.Flags().GetBool("dry-run")
			if !force {
				if all {
					if jsonMode {
						guardDestructiveJSON("destructive operation requires --force", "re-run with --force --confirm \"I understand this deletes all rows\"")
					}
					requireForceWithConfirm(cmd.CommandPath(), "I understand this deletes all rows")
				}
				if jsonMode {
					guardDestructiveJSON("destructive operation requires --force", "re-run with --force flag")
				}
				requireForce(cmd.CommandPath())
			}
			if all {
				confirm, _ := cmd.Flags().GetString("confirm")
				if confirm != "I understand this deletes all rows" {
					if jsonMode {
						guardDestructiveJSON("bulk delete requires confirmation", "re-run with --force --confirm \"I understand this deletes all rows\"")
					}
					requireForceWithConfirm(cmd.CommandPath(), "I understand this deletes all rows")
				}
			}
			query := "DELETE FROM {{ .Name }}"
			var values []interface{}
			if !all && len(args) == 1 {
				query += " WHERE {{ index .PrimaryKey.Columns 0 }} = ?"
				values = append(values, args[0])
			}
			if dryRun {
				fmt.Println("SQL:", query, values)
				return nil
			}
			result, err := db.Exec(query, values...)
			if err != nil {
				if jsonMode { printJSONError("QUERY_ERROR", err.Error()) }
				return err
			}
			affected, _ := result.RowsAffected()
			if jsonMode {
				printJSONData(map[string]interface{}{"affected": affected})
			} else {
				fmt.Printf("Deleted %d row(s)\n", affected)
			}
			return nil
		},
	}
	cmd.Flags().Bool("json", false, "Output in JSON format")
	cmd.Flags().Bool("force", false, "Required for destructive operations")
	cmd.Flags().Bool("all", false, "Delete all rows (requires --force --confirm)")
	cmd.Flags().String("confirm", "", "Confirmation string for bulk delete")
	cmd.Flags().Bool("dry-run", false, "Print SQL without executing")
	return cmd
}

{{ end }}
`