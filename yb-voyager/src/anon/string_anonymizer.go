package anon

type StringAnonymizer struct {
	registry TokenRegistry
	kind     string
}

func NewStringAnonymizer(registry TokenRegistry, kind string) Anonymizer {
	return &StringAnonymizer{
		registry: registry,
		kind:     kind,
	}
}

func (s *StringAnonymizer) Anonymize(origString string) (string, error) {
	// For metadata anonymization, we can use the registry to generate tokens
	// for the input object. This is a placeholder implementation.
	if origString == "" {
		return "", nil // No object to anonymize
	}

	token, err := s.registry.Token(s.kind, origString)
	if err != nil {
		return "", err
	}

	return token, nil // Return the anonymized token
}
