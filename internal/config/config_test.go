package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExampleConfigurationLoadsAsDocumented(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", ".env.example"))
	if err != nil {
		t.Fatal(err)
	}
	// The documented first-run workflow fills credentials in a copy of the
	// example. Isolate its keys from the developer's existing environment.
	for _, line := range strings.Split(string(data), "\n") {
		key, _, ok := strings.Cut(line, "=")
		if !ok || strings.HasPrefix(strings.TrimSpace(line), "#") {
			continue
		}
		key = strings.TrimSpace(key)
		value, existed := os.LookupEnv(key)
		t.Cleanup(func() {
			if existed {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		})
		os.Unsetenv(key)
	}
	path := filepath.Join(t.TempDir(), "dev.env")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load()
	if err != nil {
		t.Fatalf("copied example must load: %v", err)
	}
	if cfg.Env != "development" || cfg.WebDir != "./app/build/web" || cfg.Mail.TLS != "starttls" || cfg.PublicURL != "http://localhost:8787" {
		t.Fatalf("example values contain annotations: %+v", cfg)
	}
}

func TestEnvFilePreservesLiteralSecretCharacters(t *testing.T) {
	const key = "SPACE_TEST_LITERAL_SECRET"
	t.Setenv(key, "")
	os.Unsetenv(key)
	path := filepath.Join(t.TempDir(), "secrets.env")
	if err := os.WriteFile(path, []byte(key+"='secret # literal $ and = characters'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := LoadEnvFile(path); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv(key); got != "secret # literal $ and = characters" {
		t.Fatalf("literal secret changed: %q", got)
	}
}
