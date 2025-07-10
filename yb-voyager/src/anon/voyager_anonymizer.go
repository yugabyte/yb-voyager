package anon

import "fmt"

type VoyagerAnonymizer struct {
	identifierHashRegistry IdentifierHasher

	sqlAnonymizer Anonymizer
	// Anonymizers for different identifier kinds
	schemaNameAnonymizer Anonymizer
	tableNameAnonymizer  Anonymizer
	columnNameAnonymizer Anonymizer
	indexNameAnonymizer  Anonymizer
}

func NewVoyagerAnonymizer(anonymizationSalt string) (*VoyagerAnonymizer, error) {
	registry, err := NewIdentifierHashRegistry(anonymizationSalt)
	if err != nil {
		return nil, fmt.Errorf("error creating schema identifier hash registry: %w", err)
	}

	return &VoyagerAnonymizer{
		identifierHashRegistry: registry,
		sqlAnonymizer:          NewSqlAnonymizer(registry),
		schemaNameAnonymizer:   NewIdentifierAnonymizer(registry, SCHEMA_KIND_PREFIX),
		tableNameAnonymizer:    NewIdentifierAnonymizer(registry, TABLE_KIND_PREFIX),
		columnNameAnonymizer:   NewIdentifierAnonymizer(registry, COLUMN_KIND_PREFIX),
		indexNameAnonymizer:    NewIdentifierAnonymizer(registry, INDEX_KIND_PREFIX),
	}, nil
}

func (s *VoyagerAnonymizer) AnonymizeSql(sql string) (string, error) {
	return s.sqlAnonymizer.Anonymize(sql)
}

func (s *VoyagerAnonymizer) AnonymizeSchemaName(schemaName string) (string, error) {
	return s.schemaNameAnonymizer.Anonymize(schemaName)
}

func (s *VoyagerAnonymizer) AnonymizeTableName(tableName string) (string, error) {
	return s.tableNameAnonymizer.Anonymize(tableName)
}

func (s *VoyagerAnonymizer) AnonymizeColumnName(columnName string) (string, error) {
	return s.columnNameAnonymizer.Anonymize(columnName)
}

func (s *VoyagerAnonymizer) AnonymizeIndexName(indexName string) (string, error) {
	return s.indexNameAnonymizer.Anonymize(indexName)
}
