package bench

import (
	"archive/zip"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

func ConvertAll(zipPath string, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	tmpDir, err := os.MkdirTemp("", "archer-bench-convert-*")
	if err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("open zip: %w", err)
	}
	defer r.Close()

	var sqliteFiles []string
	for _, f := range r.File {
		if strings.Contains(f.Name, ".ipynb_checkpoints") {
			continue
		}
		if !strings.HasSuffix(f.Name, ".sqlite") {
			continue
		}

		outPath := filepath.Join(tmpDir, filepath.Base(f.Name))
		if err := extractFile(f, outPath); err != nil {
			return fmt.Errorf("extract %s: %w", f.Name, err)
		}
		sqliteFiles = append(sqliteFiles, outPath)
	}

	for _, sqlitePath := range sqliteFiles {
		dbName := strings.TrimSuffix(filepath.Base(sqlitePath), ".sqlite")
		outputPath := filepath.Join(outputDir, dbName+".sql")
		if err := ConvertSingle(sqlitePath, dbName, outputPath); err != nil {
			return fmt.Errorf("convert %s: %w", dbName, err)
		}
	}

	return nil
}

func extractFile(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = out.ReadFrom(rc)
	return err
}

func ConvertSingle(sqlitePath string, dbName string, outputPath string) error {
	db, err := sql.Open("sqlite3", sqlitePath)
	if err != nil {
		return fmt.Errorf("open sqlite: %w", err)
	}
	defer db.Close()

	tableNames, err := getTableNames(db)
	if err != nil {
		return fmt.Errorf("get table names: %w", err)
	}

	var b strings.Builder
	b.WriteString(fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s`; USE `%s`;\n", dbName, dbName))

	for _, tableName := range tableNames {
		createSQL, insertSQLs, err := convertTable(db, tableName)
		if err != nil {
			return fmt.Errorf("convert table %s: %w", tableName, err)
		}
		b.WriteString(createSQL + "\n")
		for _, ins := range insertSQLs {
			b.WriteString(ins + "\n")
		}
	}

	return os.WriteFile(outputPath, []byte(b.String()), 0644)
}

func getTableNames(db *sql.DB) ([]string, error) {
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var names []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		if name == "sqlite_sequence" {
			continue
		}
		names = append(names, name)
	}
	return names, rows.Err()
}

func convertTable(db *sql.DB, tableName string) (string, []string, error) {
	columns, pkCols, err := getColumnInfo(db, tableName)
	if err != nil {
		return "", nil, fmt.Errorf("get column info: %w", err)
	}

	fks, err := getForeignKeys(db, tableName)
	if err != nil {
		return "", nil, fmt.Errorf("get foreign keys: %w", err)
	}

	createSQL := buildCreateTable(tableName, columns, pkCols, fks)

	insertSQLs, err := buildInserts(db, tableName, columns)
	if err != nil {
		return "", nil, fmt.Errorf("build inserts: %w", err)
	}

	return createSQL, insertSQLs, nil
}

type colInfo struct {
	Name          string
	MySQLType     string
	Nullable      bool
	Default       string
	AutoIncrement bool
}

func getColumnInfo(db *sql.DB, tableName string) ([]colInfo, []string, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA table_info(%s)", tableName))
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()

	var columns []colInfo
	var pkCols []string
	for rows.Next() {
		var cid int
		var name, typ string
		var notnull int
		var dfltValue sql.NullString
		var pk int
		if err := rows.Scan(&cid, &name, &typ, &notnull, &dfltValue, &pk); err != nil {
			return nil, nil, err
		}

		mysqlType := mapSQLiteType(typ)
		nullable := notnull == 0
		defaultVal := ""
		if dfltValue.Valid {
			defaultVal = dfltValue.String
		}
		autoInc := false
		if pk > 0 && mysqlType == "INT" && defaultVal == "" {
			// Check if this is an autoincrement PK
			var seqEntry int
			err := db.QueryRow("SELECT COUNT(*) FROM sqlite_sequence WHERE name = ?", tableName).Scan(&seqEntry)
			if err == nil && seqEntry > 0 {
				autoInc = true
			}
		}

		columns = append(columns, colInfo{
			Name:          name,
			MySQLType:     mysqlType,
			Nullable:      nullable,
			Default:       defaultVal,
			AutoIncrement: autoInc,
		})

		if pk > 0 {
			pkCols = append(pkCols, name)
		}
	}

	return columns, pkCols, rows.Err()
}

func mapSQLiteType(sqliteType string) string {
	t := strings.ToUpper(strings.TrimSpace(sqliteType))

	// Strip parameters for base type comparison
	base := t
	for i := 0; i < len(base); i++ {
		if base[i] == '(' {
			base = base[:i]
			break
		}
	}
	base = strings.TrimSpace(base)

	switch base {
	case "INTEGER", "INT":
		return "INT"
	case "TEXT":
		return "TEXT"
	case "REAL", "FLOAT":
		return "DOUBLE"
	case "NUMERIC":
		return "DECIMAL(10,2)"
	case "BOOLEAN":
		return "BOOLEAN"
	case "DATETIME":
		return "DATETIME"
	case "DATE":
		return "DATE"
	case "BLOB":
		return "BLOB"
	default:
		// VARCHAR(N), CHAR(N) etc — preserve parameters
		if strings.HasPrefix(base, "VARCHAR") || strings.HasPrefix(t, "VARCHAR") {
			return t
		}
		if strings.HasPrefix(base, "CHAR") || strings.HasPrefix(t, "CHAR") {
			return t
		}
		// SQLite often uses loose types like "INT(11)" or other arbitrary strings
		if strings.Contains(base, "INT") {
			return "INT"
		}
		return "TEXT"
	}
}

type fkInfo struct {
	Name             string
	Column           string
	ReferencedTable  string
	ReferencedColumn string
	OnUpdate         string
	OnDelete         string
}

func getForeignKeys(db *sql.DB, tableName string) ([]fkInfo, error) {
	rows, err := db.Query(fmt.Sprintf("PRAGMA foreign_key_list(%s)", tableName))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []fkInfo
	for rows.Next() {
		var id int
		var seq int
		var table, from, to string
		var onUpdate, onDelete string
		var match string
		if err := rows.Scan(&id, &seq, &table, &from, &to, &onUpdate, &onDelete, &match); err != nil {
			return nil, err
		}
		fks = append(fks, fkInfo{
			Name:             fmt.Sprintf("fk_%s_%s", tableName, from),
			Column:           from,
			ReferencedTable:  table,
			ReferencedColumn: to,
			OnUpdate:         onUpdate,
			OnDelete:         onDelete,
		})
	}
	return fks, rows.Err()
}

func buildCreateTable(tableName string, columns []colInfo, pkCols []string, fks []fkInfo) string {
	var b strings.Builder
	b.WriteString(fmt.Sprintf("CREATE TABLE `%s` (\n", tableName))

	for i, col := range columns {
		b.WriteString(fmt.Sprintf("  `%s` %s", col.Name, col.MySQLType))
		if col.AutoIncrement {
			b.WriteString(" AUTO_INCREMENT")
		}
		if !col.Nullable && !col.AutoIncrement {
			b.WriteString(" NOT NULL")
		}
		if col.Default != "" {
			b.WriteString(fmt.Sprintf(" DEFAULT %s", escapeDefault(col.Default, col.MySQLType)))
		}
		if i < len(columns)-1 || len(pkCols) > 0 || len(fks) > 0 {
			b.WriteString(",\n")
		} else {
			b.WriteString("\n")
		}
	}

	if len(pkCols) > 0 {
		quoted := make([]string, len(pkCols))
		for i, c := range pkCols {
			quoted[i] = fmt.Sprintf("`%s`", c)
		}
		b.WriteString(fmt.Sprintf("  PRIMARY KEY (%s)", strings.Join(quoted, ", ")))
		if len(fks) > 0 {
			b.WriteString(",\n")
		} else {
			b.WriteString("\n")
		}
	}

	for i, fk := range fks {
		b.WriteString(fmt.Sprintf("  CONSTRAINT `%s` FOREIGN KEY (`%s`) REFERENCES `%s` (`%s`) ON UPDATE %s ON DELETE %s",
			fk.Name, fk.Column, fk.ReferencedTable, fk.ReferencedColumn, fk.OnUpdate, fk.OnDelete))
		if i < len(fks)-1 {
			b.WriteString(",\n")
		} else {
			b.WriteString("\n")
		}
	}

	b.WriteString(");")
	return b.String()
}

func escapeDefault(defaultVal string, mysqlType string) string {
	if defaultVal == "NULL" {
		return "NULL"
	}
	// Numeric defaults don't need quoting
	if isNumericDefault(defaultVal) {
		return defaultVal
	}
	// String defaults need quoting
	return fmt.Sprintf("'%s'", escapeString(defaultVal))
}

func isNumericDefault(s string) bool {
	// Check if it's a number (integer or float)
	for i, c := range s {
		if c == '-' && i == 0 {
			continue
		}
		if c == '.' {
			continue
		}
		if c < '0' || c > '9' {
			return false
		}
	}
	return len(s) > 0
}

func escapeString(s string) string {
	return strings.ReplaceAll(s, "'", "\\'")
}

func buildInserts(db *sql.DB, tableName string, columns []colInfo) ([]string, error) {
	colNames := make([]string, len(columns))
	for i, c := range columns {
		colNames[i] = c.Name
	}

	query := fmt.Sprintf("SELECT %s FROM `%s`", strings.Join(colNames, ", "), tableName)
	rows, err := db.Query(query)
	if err != nil {
		// Table might have no rows — that's OK
		return nil, nil
	}
	defer rows.Close()

	var inserts []string
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range columns {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		valStrs := make([]string, len(columns))
		for i, v := range values {
			valStrs[i] = formatValue(v)
		}

		quotedCols := make([]string, len(colNames))
		for i, c := range colNames {
			quotedCols[i] = fmt.Sprintf("`%s`", c)
		}

		inserts = append(inserts, fmt.Sprintf("INSERT INTO `%s` (%s) VALUES (%s);",
		 tableName, strings.Join(quotedCols, ", "), strings.Join(valStrs, ", ")))
	}

	return inserts, rows.Err()
}

func formatValue(v interface{}) string {
	if v == nil {
		return "NULL"
	}

	switch val := v.(type) {
	case int64:
		return fmt.Sprintf("%d", val)
	case float64:
		return fmt.Sprintf("%g", val)
	case bool:
		if val {
			return "1"
		}
		return "0"
	case string:
		return fmt.Sprintf("'%s'", escapeString(val))
	case []byte:
		// BLOB values — encode as hex
		if len(val) == 0 {
			return "NULL"
		}
		return fmt.Sprintf("0x%x", val)
	default:
		return fmt.Sprintf("'%s'", escapeString(fmt.Sprintf("%v", val)))
	}
}