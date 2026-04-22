package types

import (
	"fmt"
	"strings"
)

func MapMySQLToGo(mysqlType string) string {
	t := strings.ToUpper(mysqlType)

	// Strip display width for integer types: INT(11) -> INT, BIGINT(20) -> BIGINT
	baseType := stripDisplayWidth(t)

	switch {
	case baseType == "INT", baseType == "BIGINT", baseType == "TINYINT", baseType == "SMALLINT", baseType == "MEDIUMINT":
		return "int64"
	case baseType == "FLOAT", baseType == "DOUBLE", baseType == "REAL":
		return "float64"
	case strings.HasPrefix(baseType, "DECIMAL"), baseType == "NUMERIC":
		return "string"
	case baseType == "VARCHAR", baseType == "TEXT", baseType == "CHAR", baseType == "TINYTEXT", baseType == "MEDIUMTEXT", baseType == "LONGTEXT":
		return "string"
	case baseType == "DATE":
		return "string"
	case baseType == "DATETIME", baseType == "TIMESTAMP":
		return "string"
	case baseType == "BOOLEAN", baseType == "BOOL":
		return "bool"
	case baseType == "BIT":
		if t == "BIT(1)" {
			return "bool"
		}
		return "[]byte"
	case strings.HasPrefix(baseType, "ENUM"):
		return "string"
	case baseType == "JSON":
		return "string"
	case baseType == "BLOB", baseType == "TINYBLOB", baseType == "MEDIUMBLOB", baseType == "LONGBLOB", baseType == "BINARY", baseType == "VARBINARY":
		return "[]byte"
	default:
		return "string"
	}
}

func stripDisplayWidth(typeStr string) string {
	idx := strings.Index(typeStr, "(")
	if idx == -1 {
		return typeStr
	}
	return typeStr[:idx]
}

func ValidateMySQLType(mysqlType string) error {
	goType := MapMySQLToGo(mysqlType)
	if goType == "" {
		return fmt.Errorf("unsupported MySQL type: %s", mysqlType)
	}
	return nil
}