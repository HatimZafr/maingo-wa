package config

import (
	"fmt"
	"os"

	"github.com/BurntSushi/toml"
)

type Config struct {
	WhatsApp WhatsAppConfig `toml:"whatsapp"`
	LLM      LLMConfig      `toml:"llm"`
	Tools    ToolsConfig    `toml:"tools"`
	Session  SessionConfig  `toml:"session"`
}

type WhatsAppConfig struct {
	Allowlist []string `toml:"allowlist"`
}

type LLMConfig struct {
	BaseURL       string  `toml:"base_url"`
	APIKey        string  `toml:"api_key"`
	Model         string  `toml:"model"`
	Temperature   float64 `toml:"temperature"`
	MaxToolRounds int     `toml:"max_tool_rounds"`
}

type ToolsConfig struct {
	DefinitionsDir  string `toml:"definitions_dir"`
	CustomDir       string `toml:"custom_dir"`
	ShellTimeoutSec int    `toml:"shell_timeout_sec"`
	HTTPTimeoutSec  int    `toml:"http_timeout_sec"`
}

type SessionConfig struct {
	DBPath     string `toml:"db_path"`
	MaxHistory int    `toml:"max_history"`
}

type SystemPromptConfig struct {
	Path string `toml:"path"`
}

func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	if cfg.LLM.APIKey == "" {
		cfg.LLM.APIKey = os.Getenv("LLM_API_KEY")
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func (c *Config) validate() error {
	if c.LLM.BaseURL == "" {
		return fmt.Errorf("llm.base_url wajib diisi")
	}
	if c.LLM.APIKey == "" {
		return fmt.Errorf("llm.api_key wajib diisi (isi di config.toml atau env LLM_API_KEY)")
	}
	if c.LLM.Model == "" {
		return fmt.Errorf("llm.model wajib diisi")
	}
	if c.LLM.MaxToolRounds <= 0 {
		c.LLM.MaxToolRounds = 5
	}
	if c.LLM.Temperature == 0 {
		c.LLM.Temperature = 0.7
	}
	if c.Tools.DefinitionsDir == "" {
		c.Tools.DefinitionsDir = "tools/definitions"
	}
	if c.Tools.CustomDir == "" {
		c.Tools.CustomDir = "tools/custom"
	}
	if c.Tools.ShellTimeoutSec <= 0 {
		c.Tools.ShellTimeoutSec = 30
	}
	if c.Tools.HTTPTimeoutSec <= 0 {
		c.Tools.HTTPTimeoutSec = 15
	}
	if c.Session.DBPath == "" {
		c.Session.DBPath = "session.db"
	}
	if c.Session.MaxHistory <= 0 {
		c.Session.MaxHistory = 50
	}
	if len(c.WhatsApp.Allowlist) == 0 {
		return fmt.Errorf("whatsapp.allowlist wajib diisi minimal satu nomor")
	}
	return nil
}

func LoadSystemPrompt(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("baca system prompt: %w", err)
	}
	return string(data), nil
}
