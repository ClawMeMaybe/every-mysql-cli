package generator

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
)

func Scan(host string, port int, user string, password string, database string) (*Schema, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true", user, password, host, port, database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("connect to MySQL: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping MySQL: %w", err)
	}

	schema := &Schema{Database: database}

	tableNames, err := loadTableNames(db, database)
	if err != nil {
		return nil, fmt.Errorf("load table names: %w", err)
	}

	for _, tableName := range tableNames {
		table := Table{Name: tableName}

		table.Columns, err = loadColumns(db, database, tableName)
		if err != nil {
			return nil, fmt.Errorf("load columns for %s: %w", tableName, err)
		}

		table.PrimaryKey, err = loadPrimaryKey(db, database, tableName)
		if err != nil {
			return nil, fmt.Errorf("load primary key for %s: %w", tableName, err)
		}

		table.ForeignKeys, err = loadForeignKeys(db, database, tableName)
		if err != nil {
			return nil, fmt.Errorf("load foreign keys for %s: %w", tableName, err)
		}

		table.Indexes, err = loadIndexes(db, database, tableName)
		if err != nil {
			return nil, fmt.Errorf("load indexes for %s: %w", tableName, err)
		}

		schema.Tables = append(schema.Tables, table)
	}

	for i := range schema.Tables {
		schema.Tables[i].ReferencedBy = findReferencedBy(schema, schema.Tables[i].Name)
	}

	return schema, nil
}

func loadTableNames(db *sql.DB, database string) ([]string, error) {
	rows, err := db.Query(
		"SELECT TABLE_NAME FROM information_schema.TABLES WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'",
		database,
	)
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
		names = append(names, name)
	}
	return names, rows.Err()
}

func loadColumns(db *sql.DB, database string, table string) ([]Column, error) {
	rows, err := db.Query(
		"SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA FROM information_schema.COLUMNS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY ORDINAL_POSITION",
		database, table,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []Column
	for rows.Next() {
		var col Column
		var nullable string
		var defaultVal sql.NullString
		var extra string
		if err := rows.Scan(&col.Name, &col.Type, &nullable, &defaultVal, &extra); err != nil {
			return nil, err
		}
		col.Nullable = nullable == "YES"
		if defaultVal.Valid {
			col.Default = defaultVal.String
		}
		col.AutoIncrement = strings.Contains(extra, "auto_increment")
		col.GoType = MapGoType(col.Type)
		columns = append(columns, col)
	}
	return columns, rows.Err()
}

func loadPrimaryKey(db *sql.DB, database string, table string) (*PrimaryKey, error) {
	rows, err := db.Query(
		"SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY' ORDER BY ORDINAL_POSITION",
		database, table,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pkCols []string
	for rows.Next() {
		var colName string
		if err := rows.Scan(&colName); err != nil {
			return nil, err
		}
		pkCols = append(pkCols, colName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(pkCols) == 0 {
		return nil, nil
	}
	return &PrimaryKey{Columns: pkCols}, nil
}

func loadForeignKeys(db *sql.DB, database string, table string) ([]ForeignKey, error) {
	rows, err := db.Query(
		`SELECT kcu.CONSTRAINT_NAME, kcu.COLUMN_NAME, kcu.REFERENCED_TABLE_NAME, kcu.REFERENCED_COLUMN_NAME,
		        rc.UPDATE_RULE, rc.DELETE_RULE
		 FROM information_schema.KEY_COLUMN_USAGE kcu
		 JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
		   ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
		 WHERE kcu.TABLE_SCHEMA = ? AND kcu.TABLE_NAME = ? AND kcu.REFERENCED_TABLE_NAME IS NOT NULL`,
		database, table,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []ForeignKey
	for rows.Next() {
		var fk ForeignKey
		if err := rows.Scan(&fk.Name, &fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn, &fk.OnUpdate, &fk.OnDelete); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func loadIndexes(db *sql.DB, database string, table string) ([]Index, error) {
	rows, err := db.Query(
		"SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? ORDER BY INDEX_NAME, SEQ_IN_INDEX",
		database, table,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	idxMap := make(map[string]*Index)
	for rows.Next() {
		var idxName, colName string
		var nonUnique int
		if err := rows.Scan(&idxName, &colName, &nonUnique); err != nil {
			return nil, err
		}
		if idxName == "PRIMARY" {
			continue
		}
		idx, ok := idxMap[idxName]
		if !ok {
			idx = &Index{Name: idxName, Unique: nonUnique == 0}
			idxMap[idxName] = idx
		}
		idx.Columns = append(idx.Columns, colName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var indexes []Index
	for _, idx := range idxMap {
		indexes = append(indexes, *idx)
	}
	return indexes, nil
}

func findReferencedBy(schema *Schema, tableName string) []RefReference {
	var refs []RefReference
	for _, t := range schema.Tables {
		for _, fk := range t.ForeignKeys {
			if fk.ReferencedTable == tableName {
				refs = append(refs, RefReference{
					SourceTable:    t.Name,
					SourceColumn:   fk.Column,
					ForeignKeyName: fk.Name,
				})
			}
		}
	}
	return refs
}