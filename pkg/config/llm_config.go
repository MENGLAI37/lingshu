package config

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

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

// readConfigFile safely reads the config file after validating permissions.
func readConfigFile(path string) ([]byte, error) {
	// Open the file first to check its properties
	// #nosec G304 -- path is validated by getConfigPath() which restricts to user home directory
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}

	// Get file info to validate permissions
	info, err := file.Stat()
	if err != nil {
		_ = file.Close()
		return nil, err
	}

	// Security check: file should not be a symlink
	if info.Mode()&os.ModeSymlink != 0 {
		_ = file.Close()
		return nil, fmt.Errorf("config file is a symlink, refusing to read: %s", path)
	}

	// Security check: validate file permissions (should be readable only by owner)
	perm := info.Mode().Perm()
	if perm&0077 != 0 {
		_ = file.Close()
		return nil, fmt.Errorf("config file has insecure permissions %o: %s", perm, path)
	}

	// Read file contents
	data, err := io.ReadAll(file)
	if closeErr := file.Close(); closeErr != nil {
		return nil, closeErr
	}
	if err != nil {
		return nil, err
	}

	return data, nil
}

func LoadLLMConfig() error {
	llmConfigMu.Lock()
	defer llmConfigMu.Unlock()

	path, err := getConfigPath()
	if err != nil {
		return err
	}

	data, err := readConfigFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			llmConfigInstance = loadFromEnv()
			if len(llmConfigInstance.Providers) > 0 {
				_ = saveLLMConfigLocked(llmConfigInstance)
			}
			return nil
		}
		llmConfigInstance = loadFromEnv()
		if len(llmConfigInstance.Providers) > 0 {
			return nil
		}
		return err
	}

	var cfg LLMConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		llmConfigInstance = loadFromEnv()
		if len(llmConfigInstance.Providers) > 0 {
			return nil
		}
		return err
	}

	for i := range cfg.Providers {
		if cfg.Providers[i].Timeout > 0 && cfg.Providers[i].Timeout < time.Second {
			cfg.Providers[i].Timeout = cfg.Providers[i].Timeout * time.Second
		}
	}

	llmConfigInstance = &cfg
	return nil
}

func saveLLMConfigLocked(cfg *LLMConfig) error {
	path, err := getConfigPath()
	if err != nil {
		return err
	}
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

	return nil
}

func SaveLLMConfig(cfg *LLMConfig) error {
	llmConfigMu.Lock()
	defer llmConfigMu.Unlock()

	if err := saveLLMConfigLocked(cfg); err != nil {
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

func getConfigPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	path := filepath.Join(home, ".lingshu", "config.yaml")

	// Security check: ensure path doesn't contain traversal sequences
	cleanPath := filepath.Clean(path)
	if cleanPath != path {
		return "", fmt.Errorf("invalid config path: potential path traversal detected")
	}

	// Verify the path is under the home directory
	if !strings.HasPrefix(cleanPath, home) {
		return "", fmt.Errorf("invalid config path: outside home directory")
	}

	return path, nil
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
				Timeout:    30 * time.Second,
				IsLocal:    false,
				MaxRetries: 3,
			},
		},
	}
}