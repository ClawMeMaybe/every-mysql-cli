package types

import "testing"

func TestMapMySQLToGo_Integers(t *testing.T) {
	tests := []struct {
		mysql string
		want  string
	}{
		{"INT", "int64"},
		{"INT(11)", "int64"},
		{"BIGINT", "int64"},
		{"BIGINT(20)", "int64"},
		{"TINYINT", "int64"},
		{"TINYINT(1)", "int64"},
		{"SMALLINT", "int64"},
		{"SMALLINT(6)", "int64"},
		{"MEDIUMINT", "int64"},
	}
	for _, tc := range tests {
		got := MapMySQLToGo(tc.mysql)
		if got != tc.want {
			t.Errorf("MapMySQLToGo(%s) = %s, want %s", tc.mysql, got, tc.want)
		}
	}
}

func TestMapMySQLToGo_Floats(t *testing.T) {
	tests := []struct {
		mysql string
		want  string
	}{
		{"FLOAT", "float64"},
		{"DOUBLE", "float64"},
		{"REAL", "float64"},
	}
	for _, tc := range tests {
		got := MapMySQLToGo(tc.mysql)
		if got != tc.want {
			t.Errorf("MapMySQLToGo(%s) = %s, want %s", tc.mysql, got, tc.want)
		}
	}
}

func TestMapMySQLToGo_Decimal(t *testing.T) {
	tests := []struct {
		mysql string
		want  string
	}{
		{"DECIMAL(10,2)", "string"},
		{"DECIMAL", "string"},
		{"NUMERIC", "string"},
	}
	for _, tc := range tests {
		got := MapMySQLToGo(tc.mysql)
		if got != tc.want {
			t.Errorf("MapMySQLToGo(%s) = %s, want %s", tc.mysql, got, tc.want)
		}
	}
}

func TestMapMySQLToGo_Strings(t *testing.T) {
	tests := []struct {
		mysql string
		want  string
	}{
		{"VARCHAR(255)", "string"},
		{"TEXT", "string"},
		{"CHAR(10)", "string"},
		{"TINYTEXT", "string"},
		{"MEDIUMTEXT", "string"},
		{"LONGTEXT", "string"},
	}
	for _, tc := range tests {
		got := MapMySQLToGo(tc.mysql)
		if got != tc.want {
			t.Errorf("MapMySQLToGo(%s) = %s, want %s", tc.mysql, got, tc.want)
		}
	}
}

func TestMapMySQLToGo_Dates(t *testing.T) {
	tests := []struct {
		mysql string
		want  string
	}{
		{"DATE", "string"},
		{"DATETIME", "string"},
		{"TIMESTAMP", "string"},
	}
	for _, tc := range tests {
		got := MapMySQLToGo(tc.mysql)
		if got != tc.want {
			t.Errorf("MapMySQLToGo(%s) = %s, want %s", tc.mysql, got, tc.want)
		}
	}
}

func TestMapMySQLToGo_Bool(t *testing.T) {
	tests := []struct {
		mysql string
		want  string
	}{
		{"BOOLEAN", "bool"},
		{"BOOL", "bool"},
		{"BIT(1)", "bool"},
	}
	for _, tc := range tests {
		got := MapMySQLToGo(tc.mysql)
		if got != tc.want {
			t.Errorf("MapMySQLToGo(%s) = %s, want %s", tc.mysql, got, tc.want)
		}
	}
}

func TestMapMySQLToGo_BitNon1(t *testing.T) {
	got := MapMySQLToGo("BIT(8)")
	if got != "[]byte" {
		t.Errorf("MapMySQLToGo(BIT(8)) = %s, want []byte", got)
	}
}

func TestMapMySQLToGo_EnumJSON(t *testing.T) {
	tests := []struct {
		mysql string
		want  string
	}{
		{"ENUM('a','b')", "string"},
		{"JSON", "string"},
	}
	for _, tc := range tests {
		got := MapMySQLToGo(tc.mysql)
		if got != tc.want {
			t.Errorf("MapMySQLToGo(%s) = %s, want %s", tc.mysql, got, tc.want)
		}
	}
}

func TestMapMySQLToGo_Blobs(t *testing.T) {
	tests := []struct {
		mysql string
		want  string
	}{
		{"BLOB", "[]byte"},
		{"TINYBLOB", "[]byte"},
		{"MEDIUMBLOB", "[]byte"},
		{"LONGBLOB", "[]byte"},
		{"BINARY(16)", "[]byte"},
		{"VARBINARY(255)", "[]byte"},
	}
	for _, tc := range tests {
		got := MapMySQLToGo(tc.mysql)
		if got != tc.want {
			t.Errorf("MapMySQLToGo(%s) = %s, want %s", tc.mysql, got, tc.want)
		}
	}
}

func TestMapMySQLToGo_UnknownFallback(t *testing.T) {
	got := MapMySQLToGo("GEOMETRY")
	if got != "string" {
		t.Errorf("MapMySQLToGo(GEOMETRY) = %s, want string (default fallback)", got)
	}
}

func TestStripDisplayWidth(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"INT(11)", "INT"},
		{"VARCHAR(255)", "VARCHAR"},
		{"DECIMAL(10,2)", "DECIMAL"},
		{"TEXT", "TEXT"},
	}
	for _, tc := range tests {
		got := stripDisplayWidth(tc.input)
		if got != tc.want {
			t.Errorf("stripDisplayWidth(%s) = %s, want %s", tc.input, got, tc.want)
		}
	}
}