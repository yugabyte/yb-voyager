package anon

import (
	"fmt"
	"strings"
)

// VoyagerAnonymizer is a wrapper around various Anonymizer implementations,
// using a shared IdentifierHasher to generate consistent hash tokens and dispatching
// anonymization for SQL statements, schema names, table names, column names, and index names.
type VoyagerAnonymizer struct {
	identifierHashRegistry IdentifierHasher

	sqlAnonymizer Anonymizer
	// Anonymizers for different identifier kinds
	databaseNameAnonymizer   Anonymizer
	schemaNameAnonymizer     Anonymizer
	tableNameAnonymizer      Anonymizer
	columnNameAnonymizer     Anonymizer
	indexNameAnonymizer      Anonymizer
	mviewNameAnonymizer      Anonymizer
	constraintNameAnonymizer Anonymizer
}

func NewVoyagerAnonymizer(anonymizationSalt string) (*VoyagerAnonymizer, error) {
	registry, err := NewIdentifierHashRegistry(anonymizationSalt)
	if err != nil {
		return nil, fmt.Errorf("error creating schema identifier hash registry: %w", err)
	}

	return &VoyagerAnonymizer{
		identifierHashRegistry:   registry,
		sqlAnonymizer:            NewSqlAnonymizer(registry),
		databaseNameAnonymizer:   NewIdentifierAnonymizer(registry, DATABASE_KIND_PREFIX),
		schemaNameAnonymizer:     NewIdentifierAnonymizer(registry, SCHEMA_KIND_PREFIX),
		tableNameAnonymizer:      NewIdentifierAnonymizer(registry, TABLE_KIND_PREFIX),
		columnNameAnonymizer:     NewIdentifierAnonymizer(registry, COLUMN_KIND_PREFIX),
		indexNameAnonymizer:      NewIdentifierAnonymizer(registry, INDEX_KIND_PREFIX),
		mviewNameAnonymizer:      NewIdentifierAnonymizer(registry, MVIEW_KIND_PREFIX),
		constraintNameAnonymizer: NewIdentifierAnonymizer(registry, CONSTRAINT_KIND_PREFIX),
	}, nil
}

func (s *VoyagerAnonymizer) AnonymizeSql(sql string) (string, error) {
	return s.sqlAnonymizer.Anonymize(sql)
}

func (s *VoyagerAnonymizer) AnonymizeDatabaseName(databaseName string) (string, error) {
	return s.databaseNameAnonymizer.Anonymize(databaseName)
}

func (s *VoyagerAnonymizer) AnonymizeSchemaName(schemaName string) (string, error) {
	return s.schemaNameAnonymizer.Anonymize(schemaName)
}

func (s *VoyagerAnonymizer) AnonymizeTableName(tableName string) (string, error) {
	return s.tableNameAnonymizer.Anonymize(tableName)
}

func (s *VoyagerAnonymizer) AnonymizeConstraintName(constraintName string) (string, error) {
	return s.constraintNameAnonymizer.Anonymize(constraintName)
}

func (s *VoyagerAnonymizer) AnonymizeQualifiedTableName(tableName string) (string, error) {
	splits := strings.Split(tableName, ".")
	switch len(splits) {
	case 1:
		return s.tableNameAnonymizer.Anonymize(tableName)
	case 2:
		anonymizedSchemaName, err := s.AnonymizeSchemaName(splits[0])
		if err != nil {
			return "", fmt.Errorf("failed to anonymize schema name %s: %w", splits[0], err)
		}
		anonymizedTableName, err := s.AnonymizeTableName(splits[1])
		if err != nil {
			return "", fmt.Errorf("failed to anonymize table name %s: %w", splits[1], err)
		}
		return fmt.Sprintf("%s.%s", anonymizedSchemaName, anonymizedTableName), nil
	default:
		return s.tableNameAnonymizer.Anonymize(tableName)
	}
}

func (s *VoyagerAnonymizer) AnonymizeMViewName(mviewName string) (string, error) {
	return s.mviewNameAnonymizer.Anonymize(mviewName)
}

func (s *VoyagerAnonymizer) AnonymizeQualifiedMViewName(mviewName string) (string, error) {
	splits := strings.Split(mviewName, ".")
	switch len(splits) {
	case 1:
		return s.mviewNameAnonymizer.Anonymize(mviewName)
	case 2:
		anonymizedSchemaName, err := s.AnonymizeSchemaName(splits[0])
		if err != nil {
			return "", fmt.Errorf("failed to anonymize schema name %s: %w", splits[0], err)
		}
		anonymizedMviewName, err := s.AnonymizeMViewName(splits[1])
		if err != nil {
			return "", fmt.Errorf("failed to anonymize mview name %s: %w", splits[1], err)
		}
		return fmt.Sprintf("%s.%s", anonymizedSchemaName, anonymizedMviewName), nil
	default:
		return s.mviewNameAnonymizer.Anonymize(mviewName)
	}
}

func (s *VoyagerAnonymizer) AnonymizeColumnName(columnName string) (string, error) {
	return s.columnNameAnonymizer.Anonymize(columnName)
}

func (s *VoyagerAnonymizer) AnonymizeIndexName(indexName string) (string, error) {
	return s.indexNameAnonymizer.Anonymize(indexName)
}

// AnonymizeFullyQualifiedColumnName anonymizes a column name that may be fully qualified
// Input formats: "public.orders_1.customer_id", "orders_1.customer_id", "customer_id"
// Output formats: "schema_abc123.table_def456.col_ghi789", "table_def456.col_ghi789", "col_ghi789"
func (s *VoyagerAnonymizer) AnonymizeQualifiedColumnName(columnName string) (string, error) {
	if columnName == "" {
		return "", nil
	}

	// Split the column name into parts
	parts := strings.Split(columnName, ".")

	switch len(parts) {
	case 3:
		// Fully qualified: schema.table.column
		schemaName, err := s.AnonymizeSchemaName(parts[0])
		if err != nil {
			return "", fmt.Errorf("failed to anonymize schema name %s: %w", parts[0], err)
		}

		tableName, err := s.AnonymizeTableName(parts[1])
		if err != nil {
			return "", fmt.Errorf("failed to anonymize table name %s: %w", parts[1], err)
		}

		colName, err := s.AnonymizeColumnName(parts[2])
		if err != nil {
			return "", fmt.Errorf("failed to anonymize column name %s: %w", parts[2], err)
		}

		return fmt.Sprintf("%s.%s.%s", schemaName, tableName, colName), nil

	case 2:
		// Table and column: table.column
		tableName, err := s.AnonymizeTableName(parts[0])
		if err != nil {
			return "", fmt.Errorf("failed to anonymize table name %s: %w", parts[0], err)
		}

		colName, err := s.AnonymizeColumnName(parts[1])
		if err != nil {
			return "", fmt.Errorf("failed to anonymize column name %s: %w", parts[1], err)
		}

		return fmt.Sprintf("%s.%s", tableName, colName), nil

	case 1:
		// Just column name
		return s.AnonymizeColumnName(parts[0])

	default:
		// Fallback for unexpected formats - try to anonymize the whole string
		return s.AnonymizeColumnName(columnName)
	}
}
