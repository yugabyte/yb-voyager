package anon

type IdentifierAnonymizer struct {
	registry IdentifierHasher
	kind     string
}

func NewIdentifierAnonymizer(registry IdentifierHasher, kind string) Anonymizer {
	return &IdentifierAnonymizer{
		registry: registry,
		kind:     kind,
	}
}

func (s *IdentifierAnonymizer) Anonymize(origString string) (string, error) {
	// For metadata anonymization, we can use the registry to generate tokens
	// for the input object. This is a placeholder implementation.
	if origString == "" {
		return "", nil // No object to anonymize
	}

	token, err := s.registry.GetHash(s.kind, origString)
	if err != nil {
		return "", err
	}

	return token, nil // Return the anonymized token
}
