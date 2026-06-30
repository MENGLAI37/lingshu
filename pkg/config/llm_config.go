package config

import (
	"os"
	"path/filepath"
	"sync"

	"gopkg.in/yaml.v3"

	"github.com/lingshu/lingshu/pkg/llm"
)

type LLMConfig struct {
	CurrentProvider string               `yaml:"current_provider"`
	Providers       []llm.ProviderConfig `yaml:"providers"`
}

var (
	llmConfigInstance *LLMConfig
	llmConfigMu       sync.RWMutex
)

func GetLLMConfig() *LLMConfig {
	llmConfigMu.RLock()
	defer llmConfigMu.RUnlock()
	if llmConfigInstance == nil {
		return &LLMConfig{Providers: []llm.ProviderConfig{}}
	}
	return llmConfigInstance
}

func LoadLLMConfig() error {
	llmConfigMu.Lock()
	defer llmConfigMu.Unlock()

	path := getConfigPath()
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			llmConfigInstance = loadFromEnv()
			return nil
		}
		return err
	}

	var cfg LLMConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return err
	}

	llmConfigInstance = &cfg
	return nil
}

func SaveLLMConfig(cfg *LLMConfig) error {
	llmConfigMu.Lock()
	defer llmConfigMu.Unlock()

	path := getConfigPath()
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}

	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0600); err != nil {
		return err
	}

	llmConfigInstance = cfg
	return nil
}

func GetCurrentProviderConfig() *llm.ProviderConfig {
	cfg := GetLLMConfig()
	for _, p := range cfg.Providers {
		if p.Name == cfg.CurrentProvider {
			return &p
		}
	}
	if len(cfg.Providers) > 0 {
		return &cfg.Providers[0]
	}
	return nil
}

func AddProvider(cfg llm.ProviderConfig) error {
	current := GetLLMConfig()
	for i, p := range current.Providers {
		if p.Name == cfg.Name {
			current.Providers[i] = cfg
			if current.CurrentProvider == "" {
				current.CurrentProvider = cfg.Name
			}
			return SaveLLMConfig(current)
		}
	}
	current.Providers = append(current.Providers, cfg)
	if current.CurrentProvider == "" {
		current.CurrentProvider = cfg.Name
	}
	return SaveLLMConfig(current)
}

func RemoveProvider(name string) error {
	current := GetLLMConfig()
	newProviders := make([]llm.ProviderConfig, 0, len(current.Providers))
	for _, p := range current.Providers {
		if p.Name != name {
			newProviders = append(newProviders, p)
		}
	}
	current.Providers = newProviders
	if current.CurrentProvider == name {
		if len(current.Providers) > 0 {
			current.CurrentProvider = current.Providers[0].Name
		} else {
			current.CurrentProvider = ""
		}
	}
	return SaveLLMConfig(current)
}

func SetCurrentProvider(name string) error {
	current := GetLLMConfig()
	for _, p := range current.Providers {
		if p.Name == name {
			current.CurrentProvider = name
			return SaveLLMConfig(current)
		}
	}
	return nil
}

func getConfigPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".lingshu/config.yaml"
	}
	return filepath.Join(home, ".lingshu", "config.yaml")
}

func loadFromEnv() *LLMConfig {
	apiKey := os.Getenv("OPENAI_API_KEY")
	baseURL := os.Getenv("OPENAI_BASE_URL")
	model := os.Getenv("OPENAI_MODEL")

	if apiKey == "" {
		return &LLMConfig{Providers: []llm.ProviderConfig{}}
	}

	if model == "" {
		model = "gpt-4o"
	}
	if baseURL == "" {
		baseURL = "https://api.openai.com/v1"
	}

	name := "openai"
	if os.Getenv("DEEPSEEK_API_KEY") != "" || os.Getenv("DEEPSEEK_BASE_URL") != "" {
		name = "deepseek"
		if baseURL == "https://api.openai.com/v1" {
			baseURL = "https://api.deepseek.com/v1"
		}
	}

	return &LLMConfig{
		CurrentProvider: name,
		Providers: []llm.ProviderConfig{
			{
				Name:       name,
				Model:      model,
				APIKey:     apiKey,
				BaseURL:    baseURL,
				Priority:   1,
				Timeout:    30,
				IsLocal:    false,
				MaxRetries: 3,
			},
		},
	}
}