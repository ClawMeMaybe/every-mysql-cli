package generator

import (
	"database/sql"
	"fmt"
	"log"

	"github.com/kefan/every-mysql-cli/internal/types"
	_ "github.com/go-sql-driver/mysql"
)

type Scanner struct {
	db       *sql.DB
	database string
}

func NewScanner(host, port, user, password, database string) (*Scanner, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", user, password, host, port, database)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open connection: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to connect to MySQL (host=%s port=%s user=%s password=****): %w", host, port, user, err)
	}
	return &Scanner{db: db, database: database}, nil
}

func (s *Scanner) Close() error {
	return s.db.Close()
}

func (s *Scanner) Scan() (*types.Schema, error) {
	schema := &types.Schema{Database: s.database}

	tables, err := s.scanTables()
	if err != nil {
		return nil, fmt.Errorf("scanning tables: %w", err)
	}

	for _, tbl := range tables {
		columns, err := s.scanColumns(tbl.Name)
		if err != nil {
			log.Printf("Warning: scanning columns for %s: %v", tbl.Name, err)
			continue
		}
		tbl.Columns = columns

		pk, err := s.scanPrimaryKey(tbl.Name)
		if err != nil {
			log.Printf("Warning: scanning primary key for %s: %v", tbl.Name, err)
		}
		tbl.PrimaryKey = pk

		fks, err := s.scanForeignKeys(tbl.Name)
		if err != nil {
			log.Printf("Warning: scanning foreign keys for %s: %v", tbl.Name, err)
		}
		tbl.ForeignKeys = fks

		refs, err := s.scanReferencedBy(tbl.Name)
		if err != nil {
			log.Printf("Warning: scanning inbound references for %s: %v", tbl.Name, err)
		}
		tbl.ReferencedBy = refs

		indexes, err := s.scanIndexes(tbl.Name)
		if err != nil {
			log.Printf("Warning: scanning indexes for %s: %v", tbl.Name, err)
		}
		tbl.Indexes = indexes

		// Map Go types for each column
		for i := range tbl.Columns {
			tbl.Columns[i].GoType = types.MapMySQLToGo(tbl.Columns[i].Type)
		}

		schema.Tables = append(schema.Tables, tbl)
	}

	return schema, nil
}

func (s *Scanner) scanTables() ([]types.Table, error) {
	query := `
		SELECT TABLE_NAME, ENGINE, TABLE_ROWS
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ? AND TABLE_TYPE = 'BASE TABLE'
		ORDER BY TABLE_NAME
	`
	rows, err := s.db.Query(query, s.database)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []types.Table
	for rows.Next() {
		var t types.Table
		var engine sql.NullString
		var rowCount sql.NullInt64
		if err := rows.Scan(&t.Name, &engine, &rowCount); err != nil {
			return nil, err
		}
		if engine.Valid {
			t.Engine = engine.String
		}
		if rowCount.Valid {
			t.RowCount = rowCount.Int64
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (s *Scanner) scanColumns(tableName string) ([]types.Column, error) {
	query := `
		SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION
	`
	rows, err := s.db.Query(query, s.database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []types.Column
	for rows.Next() {
		var c types.Column
		var nullable string
		var defaultVal sql.NullString
		var extra sql.NullString
		if err := rows.Scan(&c.Name, &c.Type, &nullable, &defaultVal, &extra); err != nil {
			return nil, err
		}
		c.Nullable = nullable == "YES"
		if defaultVal.Valid {
			c.Default = defaultVal.String
		}
		if extra.Valid && extra.String == "auto_increment" {
			c.AutoIncrement = true
		}
		columns = append(columns, c)
	}
	return columns, rows.Err()
}

func (s *Scanner) scanPrimaryKey(tableName string) (*types.PrimaryKey, error) {
	query := `
		SELECT COLUMN_NAME
		FROM information_schema.KEY_COLUMN_USAGE
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ? AND CONSTRAINT_NAME = 'PRIMARY'
		ORDER BY ORDINAL_POSITION
	`
	rows, err := s.db.Query(query, s.database, tableName)
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
	return &types.PrimaryKey{Columns: pkCols}, nil
}

func (s *Scanner) scanForeignKeys(tableName string) ([]types.ForeignKey, error) {
	query := `
		SELECT kcu.CONSTRAINT_NAME, kcu.COLUMN_NAME,
		       kcu.REFERENCED_TABLE_NAME, kcu.REFERENCED_COLUMN_NAME,
		       rc.DELETE_RULE, rc.UPDATE_RULE
		FROM information_schema.KEY_COLUMN_USAGE kcu
		JOIN information_schema.REFERENTIAL_CONSTRAINTS rc
		  ON kcu.CONSTRAINT_NAME = rc.CONSTRAINT_NAME
		  AND kcu.TABLE_SCHEMA = rc.CONSTRAINT_SCHEMA
		WHERE kcu.TABLE_SCHEMA = ? AND kcu.TABLE_NAME = ?
		  AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY kcu.CONSTRAINT_NAME, kcu.ORDINAL_POSITION
	`
	rows, err := s.db.Query(query, s.database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var fks []types.ForeignKey
	for rows.Next() {
		var fk types.ForeignKey
		if err := rows.Scan(&fk.Name, &fk.Column, &fk.ReferencedTable, &fk.ReferencedColumn, &fk.OnDelete, &fk.OnUpdate); err != nil {
			return nil, err
		}
		fks = append(fks, fk)
	}
	return fks, rows.Err()
}

func (s *Scanner) scanReferencedBy(tableName string) ([]types.RefReference, error) {
	query := `
		SELECT kcu.TABLE_NAME, kcu.COLUMN_NAME, kcu.CONSTRAINT_NAME
		FROM information_schema.KEY_COLUMN_USAGE kcu
		WHERE kcu.TABLE_SCHEMA = ? AND kcu.REFERENCED_TABLE_NAME = ?
		  AND kcu.REFERENCED_TABLE_NAME IS NOT NULL
		ORDER BY kcu.CONSTRAINT_NAME, kcu.ORDINAL_POSITION
	`
	rows, err := s.db.Query(query, s.database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var refs []types.RefReference
	for rows.Next() {
		var r types.RefReference
		if err := rows.Scan(&r.SourceTable, &r.SourceColumn, &r.ForeignKeyName); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

func (s *Scanner) scanIndexes(tableName string) ([]types.Index, error) {
	query := `
		SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX
	`
	rows, err := s.db.Query(query, s.database, tableName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	indexMap := make(map[string]*types.Index)
	for rows.Next() {
		var idxName string
		var colName string
		var nonUnique int
		if err := rows.Scan(&idxName, &colName, &nonUnique); err != nil {
			return nil, err
		}
		if idx, ok := indexMap[idxName]; ok {
			idx.Columns = append(idx.Columns, colName)
		} else {
			indexMap[idxName] = &types.Index{
				Name:    idxName,
				Columns: []string{colName},
				Unique:  nonUnique == 0,
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	var indexes []types.Index
	for _, idx := range indexMap {
		// Skip PRIMARY index (already captured as PrimaryKey)
		if idx.Name == "PRIMARY" {
			continue
		}
		indexes = append(indexes, *idx)
	}
	return indexes, nil
}