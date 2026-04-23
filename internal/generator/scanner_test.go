package generator

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestScanTables(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("creating sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"TABLE_NAME", "ENGINE", "TABLE_ROWS"}).
		AddRow("users", "InnoDB", 100).
		AddRow("orders", "InnoDB", 500)

	mock.ExpectQuery("SELECT TABLE_NAME, ENGINE, TABLE_ROWS FROM information_schema.TABLES").
		WithArgs("myapp").
		WillReturnRows(rows)

	s := &Scanner{db: db, database: "myapp"}
	tables, err := s.scanTables()
	if err != nil {
		t.Fatalf("scanTables: %v", err)
	}

	if len(tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(tables))
	}
	if tables[0].Name != "users" {
		t.Errorf("expected first table=users, got %s", tables[0].Name)
	}
	if tables[0].Engine != "InnoDB" {
		t.Errorf("expected engine=InnoDB, got %s", tables[0].Engine)
	}
	if tables[0].RowCount != 100 {
		t.Errorf("expected rowCount=100, got %d", tables[0].RowCount)
	}
	if tables[1].Name != "orders" {
		t.Errorf("expected second table=orders, got %s", tables[1].Name)
	}
}

func TestScanColumns(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("creating sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"COLUMN_NAME", "COLUMN_TYPE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA"}).
		AddRow("id", "int", "NO", nil, "auto_increment").
		AddRow("name", "varchar(255)", "NO", nil, "").
		AddRow("email", "varchar(255)", "YES", "NULL", "")

	mock.ExpectQuery("SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA FROM information_schema.COLUMNS").
		WithArgs("myapp", "users").
		WillReturnRows(rows)

	s := &Scanner{db: db, database: "myapp"}
	columns, err := s.scanColumns("users")
	if err != nil {
		t.Fatalf("scanColumns: %v", err)
	}

	if len(columns) != 3 {
		t.Fatalf("expected 3 columns, got %d", len(columns))
	}

	c0 := columns[0]
	if c0.Name != "id" {
		t.Errorf("expected col0 name=id, got %s", c0.Name)
	}
	if c0.Type != "int" {
		t.Errorf("expected col0 type=int, got %s", c0.Type)
	}
	if c0.AutoIncrement != true {
		t.Error("expected col0 AutoIncrement=true")
	}
	if c0.Nullable {
		t.Error("expected col0 Nullable=false")
	}

	c2 := columns[2]
	if c2.Name != "email" {
		t.Errorf("expected col2 name=email, got %s", c2.Name)
	}
	if !c2.Nullable {
		t.Error("expected col2 Nullable=true")
	}
}

func TestScanPrimaryKey(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("creating sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"COLUMN_NAME"}).
		AddRow("id")

	mock.ExpectQuery("SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("myapp", "users").
		WillReturnRows(rows)

	s := &Scanner{db: db, database: "myapp"}
	pk, err := s.scanPrimaryKey("users")
	if err != nil {
		t.Fatalf("scanPrimaryKey: %v", err)
	}
	if pk == nil {
		t.Fatal("expected PK to be found")
	}
	if len(pk.Columns) != 1 || pk.Columns[0] != "id" {
		t.Errorf("expected PK columns=[id], got %v", pk.Columns)
	}
}

func TestScanPrimaryKey_NoPK(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("creating sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"COLUMN_NAME"})

	mock.ExpectQuery("SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("myapp", "logs").
		WillReturnRows(rows)

	s := &Scanner{db: db, database: "myapp"}
	pk, err := s.scanPrimaryKey("logs")
	if err != nil {
		t.Fatalf("scanPrimaryKey: %v", err)
	}
	if pk != nil {
		t.Errorf("expected nil PK for table without PK, got %v", pk)
	}
}

func TestScanForeignKeys(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("creating sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME", "DELETE_RULE", "UPDATE_RULE"}).
		AddRow("fk_user", "user_id", "users", "id", "CASCADE", "RESTRICT")

	mock.ExpectQuery("SELECT kcu.CONSTRAINT_NAME, kcu.COLUMN_NAME").
		WithArgs("myapp", "orders").
		WillReturnRows(rows)

	s := &Scanner{db: db, database: "myapp"}
	fks, err := s.scanForeignKeys("orders")
	if err != nil {
		t.Fatalf("scanForeignKeys: %v", err)
	}
	if len(fks) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(fks))
	}
	fk := fks[0]
	if fk.Name != "fk_user" {
		t.Errorf("expected FK name=fk_user, got %s", fk.Name)
	}
	if fk.Column != "user_id" {
		t.Errorf("expected FK column=user_id, got %s", fk.Column)
	}
	if fk.ReferencedTable != "users" {
		t.Errorf("expected FK ref table=users, got %s", fk.ReferencedTable)
	}
	if fk.OnDelete != "CASCADE" {
		t.Errorf("expected OnDelete=CASCADE, got %s", fk.OnDelete)
	}
}

func TestScanReferencedBy(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("creating sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"TABLE_NAME", "COLUMN_NAME", "CONSTRAINT_NAME"}).
		AddRow("orders", "user_id", "fk_user")

	mock.ExpectQuery("SELECT kcu.TABLE_NAME, kcu.COLUMN_NAME, kcu.CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("myapp", "users").
		WillReturnRows(rows)

	s := &Scanner{db: db, database: "myapp"}
	refs, err := s.scanReferencedBy("users")
	if err != nil {
		t.Fatalf("scanReferencedBy: %v", err)
	}
	if len(refs) != 1 {
		t.Fatalf("expected 1 ref, got %d", len(refs))
	}
	if refs[0].SourceTable != "orders" {
		t.Errorf("expected source table=orders, got %s", refs[0].SourceTable)
	}
	if refs[0].SourceColumn != "user_id" {
		t.Errorf("expected source column=user_id, got %s", refs[0].SourceColumn)
	}
}

func TestScanIndexes(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("creating sqlmock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"INDEX_NAME", "COLUMN_NAME", "NON_UNIQUE"}).
		AddRow("PRIMARY", "id", 0).
		AddRow("idx_email", "email", 0).
		AddRow("idx_name", "name", 1)

	mock.ExpectQuery("SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS").
		WithArgs("myapp", "users").
		WillReturnRows(rows)

	s := &Scanner{db: db, database: "myapp"}
	indexes, err := s.scanIndexes("users")
	if err != nil {
		t.Fatalf("scanIndexes: %v", err)
	}

	// PRIMARY should be excluded
	if len(indexes) != 2 {
		t.Fatalf("expected 2 non-PRIMARY indexes, got %d", len(indexes))
	}

	foundEmail := false
	foundName := false
	for _, idx := range indexes {
		if idx.Name == "idx_email" {
			foundEmail = true
			if !idx.Unique {
				t.Error("idx_email should be unique")
			}
		}
		if idx.Name == "idx_name" {
			foundName = true
			if idx.Unique {
				t.Error("idx_name should NOT be unique")
			}
		}
	}
	if !foundEmail {
		t.Error("idx_email not found in indexes")
	}
	if !foundName {
		t.Error("idx_name not found in indexes")
	}
}

func TestScanFullSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("creating sqlmock: %v", err)
	}
	defer db.Close()

	// Tables query
	mock.ExpectQuery("SELECT TABLE_NAME, ENGINE, TABLE_ROWS FROM information_schema.TABLES").
		WithArgs("myapp").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "ENGINE", "TABLE_ROWS"}).
			AddRow("users", "InnoDB", 100))

	// Columns query
	mock.ExpectQuery("SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA FROM information_schema.COLUMNS").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "COLUMN_TYPE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA"}).
			AddRow("id", "int", "NO", nil, "auto_increment").
			AddRow("name", "varchar(255)", "NO", nil, ""))

	// PK query
	mock.ExpectQuery("SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME"}).
			AddRow("id"))

	// FK query
	mock.ExpectQuery("SELECT kcu.CONSTRAINT_NAME").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME", "DELETE_RULE", "UPDATE_RULE"}))

	// ReferencedBy query
	mock.ExpectQuery("SELECT kcu.TABLE_NAME, kcu.COLUMN_NAME, kcu.CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "COLUMN_NAME", "CONSTRAINT_NAME"}).
			AddRow("orders", "user_id", "fk_user"))

	// Indexes query
	mock.ExpectQuery("SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"INDEX_NAME", "COLUMN_NAME", "NON_UNIQUE"}).
			AddRow("PRIMARY", "id", 0))

	s := &Scanner{db: db, database: "myapp"}
	schema, err := s.Scan()
	if err != nil {
		t.Fatalf("Scan: %v", err)
	}

	if schema.Database != "myapp" {
		t.Errorf("expected database=myapp, got %s", schema.Database)
	}
	if len(schema.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(schema.Tables))
	}
	tbl := schema.Tables[0]
	if tbl.Name != "users" {
		t.Errorf("expected table=users, got %s", tbl.Name)
	}
	if len(tbl.Columns) != 2 {
		t.Errorf("expected 2 columns, got %d", len(tbl.Columns))
	}
	if tbl.PrimaryKey == nil {
		t.Error("expected primary key")
	}
	if tbl.Columns[0].GoType != "int64" {
		t.Errorf("expected id GoType=int64, got %s", tbl.Columns[0].GoType)
	}
	if tbl.Columns[1].GoType != "string" {
		t.Errorf("expected name GoType=string, got %s", tbl.Columns[1].GoType)
	}
	if len(tbl.ReferencedBy) != 1 {
		t.Errorf("expected 1 inbound reference, got %d", len(tbl.ReferencedBy))
	}
}