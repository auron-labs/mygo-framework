package memory

import (
	"mygo-framework/internal/secretstest"
	"mygo-framework/secrets"
	"testing"
)

func TestStoreContract(t *testing.T) {
	t.Parallel()
	secretstest.Run(t, func(*testing.T) secrets.SecretStore { return New() })
}
