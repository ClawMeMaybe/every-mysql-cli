//go:build bench

package bench

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestConvertTypeMapping(t *testing.T) {
	mappings := []struct {
		sqlite string
		mysql  string
	}{
		{"INTEGER", "INT"},
		{"INT", "INT"},
		{"TEXT", "TEXT"},
		{"REAL", "DOUBLE"},
		{"FLOAT", "DOUBLE"},
		{"NUMERIC", "DECIMAL(10,2)"},
		{"VARCHAR(255)", "VARCHAR(255)"},
		{"CHAR(10)", "CHAR(10)"},
		{"BOOLEAN", "BOOLEAN"},
		{"DATETIME", "DATETIME"},
		{"DATE", "DATE"},
		{"BLOB", "BLOB"},
	}

	for _, m := range mappings {
		result := mapSQLiteType(m.sqlite)
		if result != m.mysql {
			t.Errorf("mapSQLiteType(%s) = %s, want %s", m.sqlite, result, m.mysql)
		}
	}
}

func TestConvertCompositePK(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test_composite.sqlite")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE Affiliated_With (
			Physician INTEGER NOT NULL,
			Department INTEGER NOT NULL,
			IsAffiliated BOOLEAN NOT NULL,
			PRIMARY KEY (Physician, Department)
		);
		INSERT INTO Affiliated_With (Physician, Department, IsAffiliated) VALUES (1, 101, 1);
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	outputPath := filepath.Join(tmpDir, "test_composite.sql")
	if err := ConvertSingle(dbPath, "test_composite", outputPath); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "PRIMARY KEY (`Physician`, `Department`)") {
		t.Errorf("expected composite PRIMARY KEY, got:\n%s", content)
	}
}

func TestConvertFKConstraints(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test_fk.sqlite")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE users (id INTEGER PRIMARY KEY, name TEXT NOT NULL);
		CREATE TABLE orders (id INTEGER PRIMARY KEY, user_id INTEGER, total REAL, FOREIGN KEY (user_id) REFERENCES users(id) ON UPDATE CASCADE ON DELETE RESTRICT);
		INSERT INTO users (id, name) VALUES (1, 'Alice');
		INSERT INTO orders (id, user_id, total) VALUES (1, 1, 99.5);
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	outputPath := filepath.Join(tmpDir, "test_fk.sql")
	if err := ConvertSingle(dbPath, "test_fk", outputPath); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "FOREIGN KEY (`user_id`) REFERENCES `users` (`id`) ON UPDATE CASCADE ON DELETE RESTRICT") {
		t.Errorf("expected FK constraint with ON UPDATE/DELETE, got:\n%s", content)
	}
}

func TestConvertSkipSqliteSequence(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test_seq.sqlite")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE items (id INTEGER PRIMARY KEY AUTOINCREMENT, name TEXT);
		INSERT INTO items (name) VALUES ('item1');
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	outputPath := filepath.Join(tmpDir, "test_seq.sql")
	if err := ConvertSingle(dbPath, "test_seq", outputPath); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	if strings.Contains(string(content), "CREATE TABLE `sqlite_sequence`") {
		t.Errorf("sqlite_sequence table should be skipped, got:\n%s", content)
	}
}

func TestConvertEmptyTable(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test_empty.sqlite")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE empty_table (id INTEGER PRIMARY KEY, name TEXT);`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	outputPath := filepath.Join(tmpDir, "test_empty.sql")
	if err := ConvertSingle(dbPath, "test_empty", outputPath); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "CREATE TABLE `empty_table`") {
		t.Errorf("expected CREATE TABLE for empty_table, got:\n%s", content)
	}
	if strings.Contains(string(content), "INSERT INTO `empty_table`") {
		t.Errorf("expected no INSERT for empty table, got:\n%s", content)
	}
}

func TestConvertNullAndStringEscaping(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test_escape.sqlite")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`
		CREATE TABLE products (id INTEGER PRIMARY KEY, name TEXT, description TEXT, price REAL);
		INSERT INTO products (id, name, description, price) VALUES (1, 'O''Reilly Book', NULL, 29.99);
	`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	outputPath := filepath.Join(tmpDir, "test_escape.sql")
	if err := ConvertSingle(dbPath, "test_escape", outputPath); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(string(content), "NULL") {
		t.Errorf("expected NULL in INSERT statement, got:\n%s", content)
	}
	if !strings.Contains(string(content), "O\\'Reilly Book") {
		t.Errorf("expected escaped single quote in INSERT, got:\n%s", content)
	}
}

func TestConvertPreamble(t *testing.T) {
	tmpDir := t.TempDir()

	dbPath := filepath.Join(tmpDir, "test_preamble.sqlite")
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.Exec(`CREATE TABLE t1 (id INTEGER PRIMARY KEY);`)
	if err != nil {
		t.Fatal(err)
	}
	db.Close()

	outputPath := filepath.Join(tmpDir, "test_preamble.sql")
	if err := ConvertSingle(dbPath, "mydb", outputPath); err != nil {
		t.Fatal(err)
	}

	content, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(string(content), "CREATE DATABASE IF NOT EXISTS `mydb`; USE `mydb`;") {
		t.Errorf("expected preamble with CREATE DATABASE/USE, got:\n%s", string(content[:200]))
	}
}

func TestConvertAll(t *testing.T) {
	zipPath := filepath.Join(projectRoot(), "testdata", "archer-bench", "database.zip")
	if _, err := os.Stat(zipPath); err != nil {
		t.Skip("archer-bench database.zip not available — run EnsureDatabaseZip() first")
	}

	tmpDir := t.TempDir()
	outputDir := filepath.Join(tmpDir, "sql")

	if err := ConvertAll(zipPath, outputDir); err != nil {
		t.Fatal(err)
	}

	// Check that at least some SQL files were generated
	entries, err := os.ReadDir(outputDir)
	if err != nil {
		t.Fatalf("read output dir: %v", err)
	}

	sqlCount := 0
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		sqlCount++
		sqlPath := filepath.Join(outputDir, e.Name())
		content, err := os.ReadFile(sqlPath)
		if err != nil {
			t.Errorf("read %s: %v", e.Name(), err)
			continue
		}
		if !strings.Contains(string(content), "CREATE TABLE") {
			t.Errorf("%s has no CREATE TABLE statements", e.Name())
		}
		if !strings.Contains(string(content), "CREATE DATABASE IF NOT EXISTS") {
			t.Errorf("%s has no CREATE DATABASE preamble", e.Name())
		}
	}

	// The archer-bench dataset has 10 databases
	if sqlCount < 8 {
		t.Errorf("expected at least 8 SQL files, got %d", sqlCount)
	}
}