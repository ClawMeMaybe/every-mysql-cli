package generator

type Schema struct {
	Database string
	Tables   []Table
}

type Table struct {
	Name         string
	Columns      []Column
	PrimaryKey   *PrimaryKey
	ForeignKeys  []ForeignKey
	ReferencedBy []RefReference
	Indexes      []Index
}

type Column struct {
	Name          string
	Type          string
	GoType        string
	Nullable      bool
	Default       string
	AutoIncrement bool
}

type ForeignKey struct {
	Name             string
	Column           string
	ReferencedTable  string
	ReferencedColumn string
	OnDelete         string
	OnUpdate         string
}

type RefReference struct {
	SourceTable    string
	SourceColumn   string
	ForeignKeyName string
}

type PrimaryKey struct {
	Columns []string
}

type Index struct {
	Name    string
	Columns []string
	Unique  bool
}

func (t *Table) HasPK() bool {
	return t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > 0
}

func (t *Table) IsCompositePK() bool {
	return t.PrimaryKey != nil && len(t.PrimaryKey.Columns) > 1
}

func (t *Table) PKColumns() []Column {
	if t.PrimaryKey == nil {
		return nil
	}
	var cols []Column
	for _, pkName := range t.PrimaryKey.Columns {
		for _, c := range t.Columns {
			if c.Name == pkName {
				cols = append(cols, c)
				break
			}
		}
	}
	return cols
}

func (t *Table) NonAutoIncrementColumns() []Column {
	var cols []Column
	for _, c := range t.Columns {
		if !c.AutoIncrement {
			cols = append(cols, c)
		}
	}
	return cols
}

func MapGoType(mysqlType string) string {
	t := normalizeType(mysqlType)
	switch t {
	case "int", "bigint", "tinyint", "smallint", "mediumint":
		return "int64"
	case "float", "double":
		return "float64"
	case "decimal":
		return "string"
	case "varchar", "text", "char", "longtext", "mediumtext", "tinytext", "enum":
		return "string"
	case "date":
		return "string"
	case "datetime", "timestamp":
		return "string"
	case "boolean", "bit":
		return "bool"
	case "json":
		return "string"
	case "blob", "longblob", "mediumblob", "tinyblob":
		return "[]byte"
	default:
		return "string"
	}
}

func normalizeType(raw string) string {
	s := raw
	for i := 0; i < len(s); i++ {
		if s[i] == '(' {
			s = s[:i]
			break
		}
	}
	result := make([]byte, 0, len(s))
	for _, c := range s {
		if c >= 'A' && c <= 'Z' {
			result = append(result, byte(c+32))
		} else {
			result = append(result, byte(c))
		}
	}
	return string(result)
}