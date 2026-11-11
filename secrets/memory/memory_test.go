package memory

import (
	"testing"

	"github.com/auron-labs/mygo-framework/internal/secretstest"
	"github.com/auron-labs/mygo-framework/secrets"
)

func TestStoreContract(t *testing.T) {
	t.Parallel()
	secretstest.Run(t, func(*testing.T) secrets.SecretStore { return New() })
}
