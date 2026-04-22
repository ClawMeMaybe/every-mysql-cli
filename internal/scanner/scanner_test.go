package scanner

import (
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestScanSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("creating sqlmock: %v", err)
	}
	defer db.Close()

	// 1. scanTables query
	mock.ExpectQuery("SELECT TABLE_NAME FROM information_schema.TABLES").
		WithArgs("myapp").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME"}).
			AddRow("users").AddRow("orders"))

	// 2. scanTable for "users": engine/row count (QueryRow)
	mock.ExpectQuery("SELECT ENGINE, TABLE_ROWS FROM information_schema.TABLES").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"ENGINE", "TABLE_ROWS"}).
			AddRow("InnoDB", 100))

	// 3. scanColumns for users
	mock.ExpectQuery("SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA FROM information_schema.COLUMNS").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "COLUMN_TYPE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA"}).
			AddRow("id", "INT(11)", "NO", nil, "auto_increment").
			AddRow("name", "VARCHAR(255)", "YES", nil, "").
			AddRow("email", "VARCHAR(255)", "NO", nil, ""))

	// 4. scanPrimaryKey for users
	mock.ExpectQuery("SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME"}).AddRow("id"))

	// 5. scanForeignKeys for users (no outbound FKs)
	mock.ExpectQuery("SELECT kcu.CONSTRAINT_NAME, kcu.COLUMN_NAME").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME", "DELETE_RULE", "UPDATE_RULE"}))

	// 6. scanReferencedBy for users (orders references users)
	mock.ExpectQuery("SELECT kcu.TABLE_NAME, kcu.COLUMN_NAME, kcu.CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE kcu").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "COLUMN_NAME", "CONSTRAINT_NAME"}).
			AddRow("orders", "user_id", "fk_order_user"))

	// 7. scanIndexes for users
	mock.ExpectQuery("SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS").
		WithArgs("myapp", "users").
		WillReturnRows(sqlmock.NewRows([]string{"INDEX_NAME", "COLUMN_NAME", "NON_UNIQUE"}).
			AddRow("idx_email", "email", 0))

	// 8. scanTable for "orders": engine/row count
	mock.ExpectQuery("SELECT ENGINE, TABLE_ROWS FROM information_schema.TABLES").
		WithArgs("myapp", "orders").
		WillReturnRows(sqlmock.NewRows([]string{"ENGINE", "TABLE_ROWS"}).
			AddRow("InnoDB", 500))

	// 9. scanColumns for orders
	mock.ExpectQuery("SELECT COLUMN_NAME, COLUMN_TYPE, IS_NULLABLE, COLUMN_DEFAULT, EXTRA FROM information_schema.COLUMNS").
		WithArgs("myapp", "orders").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME", "COLUMN_TYPE", "IS_NULLABLE", "COLUMN_DEFAULT", "EXTRA"}).
			AddRow("id", "INT(11)", "NO", nil, "auto_increment").
			AddRow("user_id", "INT(11)", "NO", nil, "").
			AddRow("total", "DECIMAL(10,2)", "NO", "0.00", ""))

	// 10. scanPrimaryKey for orders
	mock.ExpectQuery("SELECT COLUMN_NAME FROM information_schema.KEY_COLUMN_USAGE").
		WithArgs("myapp", "orders").
		WillReturnRows(sqlmock.NewRows([]string{"COLUMN_NAME"}).AddRow("id"))

	// 11. scanForeignKeys for orders (user_id references users)
	mock.ExpectQuery("SELECT kcu.CONSTRAINT_NAME, kcu.COLUMN_NAME").
		WithArgs("myapp", "orders").
		WillReturnRows(sqlmock.NewRows([]string{"CONSTRAINT_NAME", "COLUMN_NAME", "REFERENCED_TABLE_NAME", "REFERENCED_COLUMN_NAME", "DELETE_RULE", "UPDATE_RULE"}).
			AddRow("fk_order_user", "user_id", "users", "id", "CASCADE", "RESTRICT"))

	// 12. scanReferencedBy for orders (none inbound)
	mock.ExpectQuery("SELECT kcu.TABLE_NAME, kcu.COLUMN_NAME, kcu.CONSTRAINT_NAME FROM information_schema.KEY_COLUMN_USAGE kcu").
		WithArgs("myapp", "orders").
		WillReturnRows(sqlmock.NewRows([]string{"TABLE_NAME", "COLUMN_NAME", "CONSTRAINT_NAME"}))

	// 13. scanIndexes for orders
	mock.ExpectQuery("SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE FROM information_schema.STATISTICS").
		WithArgs("myapp", "orders").
		WillReturnRows(sqlmock.NewRows([]string{"INDEX_NAME", "COLUMN_NAME", "NON_UNIQUE"}).
			AddRow("idx_user_id", "user_id", 1))

	schema, err := ScanSchema(db, "myapp")
	if err != nil {
		t.Fatalf("ScanSchema: %v", err)
	}

	if len(schema.Tables) != 2 {
		t.Fatalf("expected 2 tables, got %d", len(schema.Tables))
	}

	users := schema.Tables[0]
	if users.Name != "users" {
		t.Errorf("first table name: got %q, want %q", users.Name, "users")
	}
	if len(users.Columns) != 3 {
		t.Errorf("users columns: got %d, want 3", len(users.Columns))
	}
	if !users.HasPK {
		t.Error("users should have a primary key")
	}
	if users.PrimaryKey.Columns[0] != "id" {
		t.Errorf("users PK: got %q, want %q", users.PrimaryKey.Columns[0], "id")
	}
	if len(users.ReferencedBy) != 1 {
		t.Errorf("users ReferencedBy: got %d, want 1", len(users.ReferencedBy))
	}
	if users.ReferencedBy[0].SourceTable != "orders" {
		t.Errorf("users referenced by: got %q, want %q", users.ReferencedBy[0].SourceTable, "orders")
	}

	orders := schema.Tables[1]
	if len(orders.ForeignKeys) != 1 {
		t.Errorf("orders ForeignKeys: got %d, want 1", len(orders.ForeignKeys))
	}
	if orders.ForeignKeys[0].ReferencedTable != "users" {
		t.Errorf("orders FK ref table: got %q, want %q", orders.ForeignKeys[0].ReferencedTable, "users")
	}

	idCol := users.Columns[0]
	if idCol.GoType != "int64" {
		t.Errorf("id GoType: got %q, want %q", idCol.GoType, "int64")
	}
	if idCol.AutoIncrement != true {
		t.Error("id should be auto_increment")
	}

	nameCol := users.Columns[1]
	if nameCol.GoType != "string" {
		t.Errorf("name GoType: got %q, want %q", nameCol.GoType, "string")
	}
	if nameCol.Nullable != true {
		t.Error("name should be nullable")
	}

	totalCol := orders.Columns[2]
	if totalCol.GoType != "string" {
		t.Errorf("total GoType: got %q, want %q (DECIMAL->string)", totalCol.GoType, "string")
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}