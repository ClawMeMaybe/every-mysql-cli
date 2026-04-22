package mapper

import "strings"

func MySQLToGo(mysqlType string) string {
	t := strings.ToUpper(mysqlType)

	// Check BIT(1) before stripping width
	if t == "BIT(1)" {
		return "bool"
	}

	// Strip display width from integer types: INT(11) -> INT
	t = stripWidth(t)

	switch {
	case isInt(t):
		return "int64"
	case isFloat(t):
		return "float64"
	case strings.HasPrefix(t, "DECIMAL") || strings.HasPrefix(t, "NUMERIC"):
		return "string"
	case isString(t):
		return "string"
	case isDate(t):
		return "string"
	case isBool(t):
		return "bool"
	case strings.HasPrefix(t, "ENUM"):
		return "string"
	case strings.HasPrefix(t, "JSON"):
		return "string"
	case strings.HasPrefix(t, "BLOB") || strings.HasPrefix(t, "TINYBLOB") ||
		strings.HasPrefix(t, "MEDIUMBLOB") || strings.HasPrefix(t, "LONGBLOB") ||
		t == "BINARY" || strings.HasPrefix(t, "VARBINARY"):
		return "[]byte"
	default:
		return "string"
	}
}

func stripWidth(t string) string {
	if idx := strings.Index(t, "("); idx != -1 {
		return t[:idx]
	}
	return t
}

func isInt(t string) bool {
	return t == "INT" || t == "BIGINT" || t == "TINYINT" ||
		t == "SMALLINT" || t == "MEDIUMINT"
}

func isFloat(t string) bool {
	return t == "FLOAT" || t == "DOUBLE" || t == "REAL"
}

func isString(t string) bool {
	return strings.HasPrefix(t, "VARCHAR") || strings.HasPrefix(t, "CHAR") ||
		t == "TEXT" || t == "TINYTEXT" || t == "MEDIUMTEXT" || t == "LONGTEXT"
}

func isDate(t string) bool {
	return t == "DATE" || t == "DATETIME" || t == "TIMESTAMP" || t == "TIME"
}

func isBool(t string) bool {
	return t == "BOOLEAN" || t == "BOOL" || t == "BIT(1)"
}

func IsStringType(goType string) bool {
	return goType == "string"
}

func IsNullableString(goType string) bool {
	return goType == "*string"
}