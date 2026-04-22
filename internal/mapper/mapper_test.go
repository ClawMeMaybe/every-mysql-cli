package mapper

import "testing"

func TestMySQLToGo(t *testing.T) {
	tests := []struct {
		mysql string
		go_   string
	}{
		{"INT", "int64"},
		{"INT(11)", "int64"},
		{"BIGINT", "int64"},
		{"TINYINT", "int64"},
		{"SMALLINT", "int64"},
		{"MEDIUMINT", "int64"},
		{"FLOAT", "float64"},
		{"DOUBLE", "float64"},
		{"REAL", "float64"},
		{"DECIMAL(10,2)", "string"},
		{"NUMERIC(8,4)", "string"},
		{"VARCHAR(255)", "string"},
		{"CHAR(10)", "string"},
		{"TEXT", "string"},
		{"TINYTEXT", "string"},
		{"MEDIUMTEXT", "string"},
		{"LONGTEXT", "string"},
		{"DATE", "string"},
		{"DATETIME", "string"},
		{"TIMESTAMP", "string"},
		{"BOOLEAN", "bool"},
		{"BOOL", "bool"},
		{"BIT(1)", "bool"},
		{"ENUM('a','b')", "string"},
		{"JSON", "string"},
		{"BLOB", "[]byte"},
		{"TINYBLOB", "[]byte"},
		{"MEDIUMBLOB", "[]byte"},
		{"LONGBLOB", "[]byte"},
		{"BINARY(16)", "[]byte"},
		{"VARBINARY(255)", "[]byte"},
		{"UNKNOWN_TYPE", "string"},
	}

	for _, tt := range tests {
		got := MySQLToGo(tt.mysql)
		if got != tt.go_ {
			t.Errorf("MySQLToGo(%q) = %q, want %q", tt.mysql, got, tt.go_)
		}
	}
}

func TestStripWidth(t *testing.T) {
	tests := []struct {
		in  string
		out string
	}{
		{"INT(11)", "INT"},
		{"VARCHAR(255)", "VARCHAR"},
		{"TEXT", "TEXT"},
	}
	for _, tt := range tests {
		got := stripWidth(tt.in)
		if got != tt.out {
			t.Errorf("stripWidth(%q) = %q, want %q", tt.in, got, tt.out)
		}
	}
}