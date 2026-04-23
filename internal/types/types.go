package types

type Schema struct {
	Database string
	Tables   []Table
}

type Table struct {
	Name         string
	Engine       string
	RowCount     int64
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