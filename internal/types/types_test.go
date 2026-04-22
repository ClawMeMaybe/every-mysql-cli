package types

import "testing"

func TestSchema_Structures(t *testing.T) {
	s := Schema{
		Database: "myapp",
		Tables: []Table{
			{
				Name:     "users",
				Engine:   "InnoDB",
				RowCount: 100,
				Columns: []Column{
					{Name: "id", Type: "INT", GoType: "int64", AutoIncrement: true},
					{Name: "name", Type: "VARCHAR(255)", GoType: "string"},
					{Name: "email", Type: "VARCHAR(255)", GoType: "string", Nullable: true},
				},
				PrimaryKey: &PrimaryKey{Columns: []string{"id"}},
				ForeignKeys: []ForeignKey{
					{
						Name:             "fk_orders_user",
						Column:           "user_id",
						ReferencedTable:  "users",
						ReferencedColumn: "id",
						OnDelete:         "CASCADE",
						OnUpdate:         "RESTRICT",
					},
				},
				ReferencedBy: []RefReference{
					{SourceTable: "orders", SourceColumn: "user_id", ForeignKeyName: "fk_orders_user"},
				},
				Indexes: []Index{
					{Name: "idx_email", Columns: []string{"email"}, Unique: true},
				},
			},
		},
	}

	if s.Database != "myapp" {
		t.Errorf("expected Database=myapp, got %s", s.Database)
	}
	if len(s.Tables) != 1 {
		t.Fatalf("expected 1 table, got %d", len(s.Tables))
	}
	tbl := s.Tables[0]
	if tbl.Name != "users" {
		t.Errorf("expected table name=users, got %s", tbl.Name)
	}
	if tbl.PrimaryKey == nil {
		t.Fatal("expected PrimaryKey to be set")
	}
	if len(tbl.PrimaryKey.Columns) != 1 || tbl.PrimaryKey.Columns[0] != "id" {
		t.Errorf("expected PK={id}, got %v", tbl.PrimaryKey.Columns)
	}
	if len(tbl.ForeignKeys) != 1 {
		t.Fatalf("expected 1 FK, got %d", len(tbl.ForeignKeys))
	}
	if tbl.ForeignKeys[0].ReferencedTable != "users" {
		t.Errorf("expected FK ref=users, got %s", tbl.ForeignKeys[0].ReferencedTable)
	}
	if len(tbl.ReferencedBy) != 1 {
		t.Fatalf("expected 1 RefReference, got %d", len(tbl.ReferencedBy))
	}
	if tbl.ReferencedBy[0].SourceTable != "orders" {
		t.Errorf("expected ref source=orders, got %s", tbl.ReferencedBy[0].SourceTable)
	}
}

func TestColumn_Fields(t *testing.T) {
	c := Column{
		Name:          "created_at",
		Type:          "DATETIME",
		GoType:        "string",
		Nullable:      true,
		Default:       "CURRENT_TIMESTAMP",
		AutoIncrement: false,
	}
	if c.GoType != "string" {
		t.Errorf("expected GoType=string, got %s", c.GoType)
	}
	if !c.Nullable {
		t.Error("expected Nullable=true")
	}
	if c.Default != "CURRENT_TIMESTAMP" {
		t.Errorf("expected Default=CURRENT_TIMESTAMP, got %s", c.Default)
	}
}

func TestIndex_Unique(t *testing.T) {
	idx := Index{Name: "idx_email", Columns: []string{"email"}, Unique: true}
	if !idx.Unique {
		t.Error("expected Unique=true")
	}
	if len(idx.Columns) != 1 || idx.Columns[0] != "email" {
		t.Errorf("expected Columns=[email], got %v", idx.Columns)
	}
}