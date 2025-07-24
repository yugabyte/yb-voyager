package mcpservernew

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestServerCreation tests that we can create a server instance
func TestServerCreation(t *testing.T) {
	server := NewServer()
	assert.NotNil(t, server)
	assert.NotNil(t, server.server)
}
