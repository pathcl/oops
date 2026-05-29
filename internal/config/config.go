package config

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

type AzureDevOps struct {
	Org      string `yaml:"org"`
	Project  string `yaml:"project"`
	Repo     string `yaml:"repo"`
	FilePath string `yaml:"file_path"`
	Branch   string `yaml:"branch"`
}

type LLM struct {
	Provider string `yaml:"provider"` // "anthropic" (default) or "openai"
	Model    string `yaml:"model"`    // defaults per provider
	APIKey   string `yaml:"api_key"`  // falls back to ANTHROPIC_API_KEY / OPENAI_API_KEY
}

type Config struct {
	AzureDevOps AzureDevOps   `yaml:"azure_devops"`
	CacheTTL    time.Duration `yaml:"cache_ttl"`
	LLM         LLM           `yaml:"llm"`
}

func Load() (*Config, error) {
	cfg := &Config{
		AzureDevOps: AzureDevOps{Branch: "main"},
		CacheTTL:    time.Hour,
	}

	path := filepath.Join(configDir(), "oops", "config.yaml")
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if err == nil {
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config: %w", err)
		}
	}

	applyEnv(cfg)

	if cfg.AzureDevOps.Branch == "" {
		cfg.AzureDevOps.Branch = "main"
	}
	if cfg.CacheTTL == 0 {
		cfg.CacheTTL = time.Hour
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	ado := c.AzureDevOps
	if ado.Org == "" {
		return fmt.Errorf("azure_devops.org is required (or set OOPS_ADO_ORG)")
	}
	if ado.Project == "" {
		return fmt.Errorf("azure_devops.project is required (or set OOPS_ADO_PROJECT)")
	}
	if ado.Repo == "" {
		return fmt.Errorf("azure_devops.repo is required (or set OOPS_ADO_REPO)")
	}
	if ado.FilePath == "" {
		return fmt.Errorf("azure_devops.file_path is required (or set OOPS_ADO_FILE_PATH)")
	}
	return nil
}

func applyEnv(cfg *Config) {
	if v := os.Getenv("OOPS_ADO_ORG"); v != "" {
		cfg.AzureDevOps.Org = v
	}
	if v := os.Getenv("OOPS_ADO_PROJECT"); v != "" {
		cfg.AzureDevOps.Project = v
	}
	if v := os.Getenv("OOPS_ADO_REPO"); v != "" {
		cfg.AzureDevOps.Repo = v
	}
	if v := os.Getenv("OOPS_ADO_FILE_PATH"); v != "" {
		cfg.AzureDevOps.FilePath = v
	}
	if v := os.Getenv("OOPS_ADO_BRANCH"); v != "" {
		cfg.AzureDevOps.Branch = v
	}
	if v := os.Getenv("OOPS_LLM_PROVIDER"); v != "" {
		cfg.LLM.Provider = v
	}
	if v := os.Getenv("OOPS_LLM_MODEL"); v != "" {
		cfg.LLM.Model = v
	}
	if v := os.Getenv("ANTHROPIC_API_KEY"); v != "" && cfg.LLM.APIKey == "" {
		cfg.LLM.APIKey = v
	}
	if v := os.Getenv("OPENAI_API_KEY"); v != "" && cfg.LLM.APIKey == "" {
		cfg.LLM.APIKey = v
	}
}

func configDir() string {
	if d := os.Getenv("XDG_CONFIG_HOME"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config")
}
