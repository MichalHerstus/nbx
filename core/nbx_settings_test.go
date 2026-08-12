package core_test

import (
	"encoding/json"
	"os"
	"strings"
	"testing"

	"github.com/pocketbase/pocketbase/core"
	"github.com/pocketbase/pocketbase/tests"
	"github.com/pocketbase/pocketbase/tools/security"
)

func TestSettingsDefaultNbx(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	nbx := app.Settings().Nbx

	if nbx.Ai.Provider != core.AiProviderOpenRouter {
		t.Fatalf("expected default provider %q, got %q", core.AiProviderOpenRouter, nbx.Ai.Provider)
	}

	if nbx.Ai.Streaming != true {
		t.Fatal("expected default streaming true")
	}

	if nbx.Secrets == nil {
		t.Fatal("expected non-nil default secrets map")
	}
}

func TestSettingsNbxSecretsVault(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	// pin a deterministic encryption key so the at-rest assertion is stable
	// regardless of other parallel tests mutating the process env
	encryptionKey := strings.Repeat("k", 32)
	originalEnv := os.Getenv(app.EncryptionEnv())
	os.Setenv(app.EncryptionEnv(), encryptionKey)
	defer os.Setenv(app.EncryptionEnv(), originalEnv)

	settings := &core.Settings{}
	settings.Nbx.Ai.Provider = core.AiProviderOpenRouter
	settings.Nbx.Ai.ApiKey = "sk-test-123"
	settings.Nbx.Secrets = map[string]core.Credential{
		"mysql_prod": {User: "root", Password: "s3cret", URL: "http://internal"},
	}

	// DBExport persists the full blob (including the vault) encrypted at rest
	export, err := settings.DBExport(app)
	if err != nil {
		t.Fatal(err)
	}

	encrypted, ok := export["value"].(string)
	if !ok {
		t.Fatalf("expected an encrypted at-rest value, got %T", export["value"])
	}

	decrypted, err := security.Decrypt(encrypted, encryptionKey)
	if err != nil {
		t.Fatalf("failed to decrypt at-rest value: %v", err)
	}

	for _, secret := range []string{"sk-test-123", "s3cret", "mysql_prod", "root"} {
		if !strings.Contains(string(decrypted), secret) {
			t.Errorf("expected at-rest value to contain %q", secret)
		}
	}

	// Clone preserves the full vault configuration
	clone, err := settings.Clone()
	if err != nil {
		t.Fatal(err)
	}

	if clone.Nbx.Ai.ApiKey != "sk-test-123" {
		t.Fatalf("expected the clone to preserve the api key, got %q", clone.Nbx.Ai.ApiKey)
	}

	if clone.Nbx.Secrets["mysql_prod"].Password != "s3cret" {
		t.Fatalf("expected the clone to preserve the vault password")
	}
}

func TestSettingsNbxMarshalJSONMasksSecrets(t *testing.T) {
	t.Parallel()

	settings := &core.Settings{}
	settings.Nbx.Ai.Provider = core.AiProviderOpenRouter
	settings.Nbx.Ai.ApiKey = "sk-very-secret"
	settings.Nbx.Secrets = map[string]core.Credential{
		"db1": {User: "u", Password: "pw", APIKey: "ak", Token: "tk"},
	}

	raw, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	rawStr := string(raw)

	if strings.Contains(rawStr, "sk-very-secret") {
		t.Fatal("expected the AI api key to be masked")
	}

	if strings.Contains(rawStr, "pw") || strings.Contains(rawStr, "ak") || strings.Contains(rawStr, "tk") {
		t.Fatal("expected the credential secret fields to be masked")
	}

	// but public credential fields are still exposed
	if !strings.Contains(rawStr, `"user":"u"`) {
		t.Fatalf("expected the credential user to be exposed, got %s", rawStr)
	}

	// and the secrets map itself should be present as an object
	if !strings.Contains(rawStr, `"secrets":`) {
		t.Fatalf("expected a secrets key, got %s", rawStr)
	}
}

func TestSettingsNbxValidate(t *testing.T) {
	t.Parallel()

	app, _ := tests.NewTestApp()
	defer app.Cleanup()

	s := app.Settings()

	// invalid provider
	s.Nbx.Ai.Provider = "unknown_provider"
	if err := app.Validate(s); err == nil {
		t.Fatal("expected validation error for unknown AI provider")
	}

	// valid provider
	s.Nbx.Ai.Provider = core.AiProviderOllama
	if err := app.Validate(s); err != nil {
		t.Fatalf("expected no validation error, got %v", err)
	}
}
