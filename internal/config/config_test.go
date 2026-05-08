package config

import (
	"os"
	"testing"
)

func TestLoadValidConfig(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/config.toml"
	content := `
[whatsapp]
allowlist = ["6281234567890"]

[llm]
base_url = "http://localhost:20128/v1"
api_key = "test-key"
model = "test-model"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.LLM.BaseURL != "http://localhost:20128/v1" {
		t.Errorf("base_url = %q", cfg.LLM.BaseURL)
	}
	if len(cfg.WhatsApp.Allowlist) != 1 {
		t.Errorf("allowlist len = %d", len(cfg.WhatsApp.Allowlist))
	}
}

func TestLoadMissingFile(t *testing.T) {
	_, err := Load("/nonexistent/config.toml")
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestLoadEmptyAllowlist(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/config.toml"
	content := `
[whatsapp]
allowlist = []

[llm]
base_url = "http://localhost:20128/v1"
api_key = "key"
model = "m"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for empty allowlist")
	}
}

func TestLoadMissingBaseURL(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/config.toml"
	content := `
[whatsapp]
allowlist = ["123"]

[llm]
api_key = "k"
model = "m"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	_, err := Load(path)
	if err == nil {
		t.Fatal("expected error for missing base_url")
	}
}

func TestLoadAPIKeyFromEnv(t *testing.T) {
	os.Setenv("LLM_API_KEY", "env-key")
	defer os.Unsetenv("LLM_API_KEY")

	tmp := t.TempDir()
	path := tmp + "/config.toml"
	content := `
[whatsapp]
allowlist = ["123"]

[llm]
base_url = "http://localhost:20128/v1"
api_key = ""
model = "m"
`
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	cfg, err := Load(path)
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if cfg.LLM.APIKey != "env-key" {
		t.Errorf("expected env-key, got: %q", cfg.LLM.APIKey)
	}
}

func TestLoadSystemPrompt(t *testing.T) {
	tmp := t.TempDir()
	path := tmp + "/prompt.txt"
	if err := os.WriteFile(path, []byte("Halo, aku bot."), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := LoadSystemPrompt(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != "Halo, aku bot." {
		t.Errorf("got %q", got)
	}
}
