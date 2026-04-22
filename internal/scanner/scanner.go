package scanner

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/kefan/every-mysql-cli/internal/mapper"
	"github.com/kefan/every-mysql-cli/internal/model"
)

func ScanSchema(db *sql.DB, dbName string) (*model.Schema, error) {
	schema := &model.Schema{Database: dbName}

	tables, err := scanTables(db, dbName)
	if err != nil {
		return nil, fmt.Errorf("scanning tables: %w", err)
	}

	for _, tbl := range tables {
		t, err := scanTable(db, dbName, tbl)
		if err != nil {
			log.Printf("warning: skipping table %s: %v", tbl, err)
			continue
		}
		schema.Tables = append(schema.Tables, *t)
	}

	return schema, nil
}

func scanTables(db *sql.DB, dbName string) ([]string, error) {
	rows, err := db.Query(`
		SELECT TABLE_NAME
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME`, dbName)
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

func scanTable(db *sql.DB, dbName, tableName string) (*model.Table, error) {
	t := &model.Table{Name: tableName}

	// Engine and row count
	var engine sql.NullString
	var rowCount sql.NullInt64
	err := db.QueryRow(`
		SELECT ENGINE, TABLE_ROWS
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?`, dbName, tableName).Scan(&engine, &rowCount)
	if err != nil {
		return nil, err
	}
	if engine.Valid {
		t.Engine = engine.String
	}
	if rowCount.Valid {
		t.RowCount = rowCount.Int64
	}

	// Columns
	t.Columns, err = scanColumns(db, dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("scanning columns: %w", err)
	}

	// Primary key
	t.PrimaryKey, err = scanPrimaryKey(db, dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("scanning primary key: %w", err)
	}
	t.HasPK = t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > 0

	// Foreign keys
	t.ForeignKeys, err = scanForeignKeys(db, dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("scanning foreign keys: %w", err)
	}

	// Referenced by
	t.ReferencedBy, err = scanReferencedBy(db, dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("scanning inbound references: %w", err)
	}

	// Indexes
	t.Indexes, err = scanIndexes(db, dbName, tableName)
	if err != nil {
		return nil, fmt.Errorf("scanning indexes: %w", err)
	}

	return t, nil
}

func scanColumns(db *sql.DB, dbName, tableName string) ([]model.Column, error) {
	rows, err := db.Query(`
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []model.Column
	for rows.Next() {
		var c model.Column
		var nullable, extra string
		var defVal sql.NullString
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &defVal, &extra); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		if defVal.Valid {
			c.Default = defVal.String
		}
		c.AutoIncrement = extra == "auto_increment"
		c.GoType = mapper.MySQLToGo(c.Type)
		cols = append(cols, c)
	}
	return cols, rows.Err()
}

func scanPrimaryKey(db *sql.DB, dbName, tableName string) (*model.PrimaryKey, error) {
	rows, err := db.Query(`
		SELECT COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION`, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pkCols []string
	for rows.Next() {
		var col string
		if err := rows.Scan(&col); err != nil {
			return nil, err
		}
		pkCols = append(pkCols, col)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(pkCols) == 0 {
		return nil, nil
	}
	return &model.PrimaryKey{Columns: pkCols}, nil
}

func scanForeignKeys(db *sql.DB, dbName, tableName string) ([]model.ForeignKey, error) {
	rows, err := db.Query(`
		SELECT kcu.CONSTRAINT_NAME, kcu.COLUMN_NAME,
		       kcu.REFERENCED_TABLE_NAME, kcu.REFERENCED_COLUMN_NAME,
		       rc.DELETE_RULE, rc.UPDATE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
		  ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
		     AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.TABLE_SCHEMA = ? AND kcu.TABLE_NAME = ?
		  AND kcu.REFERENCED_TABLE_NAME IS NOT NULL`, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []model.ForeignKey
	for rows.Next() {
		var fk model.ForeignKey
		if err := rows.Scan(&fk.Name, &fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn, &fk.OnDelete, &fk.OnUpdate); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func scanReferencedBy(db *sql.DB, dbName, tableName string) ([]model.RefReference, error) {
	rows, err := db.Query(`
		SELECT kcu.TABLE_NAME, kcu.COLUMN_NAME, kcu.CONSTRAINT_NAME
		FROM information_schema.KEY_COLUMN_USAGE kcu
		WHERE kcu.TABLE_SCHEMA = ? AND kcu.REFERENCED_TABLE_NAME = ?
		  AND kcu.REFERENCED_COLUMN_NAME IS NOT NULL`, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []model.RefReference
	for rows.Next() {
		var r model.RefReference
		if err := rows.Scan(&r.SourceTable, &r.SourceColumn, &r.ForeignKeyName); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

func scanIndexes(db *sql.DB, dbName, tableName string) ([]model.Index, error) {
	rows, err := db.Query(`
		SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`, dbName, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	idxMap := make(map[string]*model.Index)
	for rows.Next() {
		var idxName, colName string
		var nonUnique int
		if err := rows.Scan(&idxName, &colName, &nonUnique); err != nil {
			return nil, err
		}
		idx, ok := idxMap[idxName]
		if !ok {
			idx = &model.Index{Name: idxName, Unique: nonUnique == 0}
			idxMap[idxName] = idx
		}
		idx.Columns = append(idx.Columns, colName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var indexes []model.Index
	for _, idx := range idxMap {
		indexes = append(indexes, *idx)
	}
	return indexes, nil
}