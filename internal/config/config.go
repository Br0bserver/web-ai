package config

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const DefaultTitlePrompt = "You generate concise conversation titles. Read the first user message and first assistant reply, then output a short title in the same language as the conversation. Output title text only, with no quotes, prefixes, markdown, or punctuation unless the language naturally needs it."

type ServerConfig struct {
	Port    int    `json:"port"`
	DataDir string `json:"data_dir"`
}

type AuthConfig struct {
	AllowedUserIDs []string `json:"allowed_user_ids"`
}

type ProviderConfig struct {
	Name           string `json:"name"`
	BaseURL        string `json:"base_url"`
	APIKey         string `json:"api_key"`
	TimeoutSeconds int    `json:"timeout_seconds"`
}

type ModelConfig struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Avatar string `json:"avatar,omitempty"`
}

type TitleModelConfig struct {
	ID     string `json:"id"`
	Name   string `json:"name"`
	Prompt string `json:"prompt"`
}

type UIConfig struct {
	Title string `json:"title"`
}

type Config struct {
	Server     ServerConfig     `json:"server"`
	Auth       AuthConfig       `json:"auth"`
	Provider   ProviderConfig   `json:"provider"`
	Models     []ModelConfig    `json:"models"`
	TitleModel TitleModelConfig `json:"title_model"`
	UI         UIConfig         `json:"ui"`
}

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	applyDefaults(&cfg)
	applyEnvOverrides(&cfg)
	if err := validate(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func applyDefaults(cfg *Config) {
	if cfg.Server.Port == 0 {
		cfg.Server.Port = 8080
	}
	if cfg.Server.DataDir == "" {
		cfg.Server.DataDir = "data"
	}
	if cfg.Provider.TimeoutSeconds <= 0 {
		cfg.Provider.TimeoutSeconds = 300
	}
	if cfg.UI.Title == "" {
		cfg.UI.Title = "Web AI"
	}
	if cfg.TitleModel.Prompt == "" {
		cfg.TitleModel.Prompt = DefaultTitlePrompt
	}
	trimUsers(cfg)
	trimModels(cfg)
}

func applyEnvOverrides(cfg *Config) {
	if value := os.Getenv("PORT"); value != "" {
		if parsed, err := strconv.Atoi(value); err == nil && parsed > 0 {
			cfg.Server.Port = parsed
		}
	}
	if value := os.Getenv("DATA_DIR"); value != "" {
		cfg.Server.DataDir = value
	}
	if value := os.Getenv("BASE_URL"); value != "" {
		cfg.Provider.BaseURL = value
	}
	if value := os.Getenv("API_KEY"); value != "" {
		cfg.Provider.APIKey = value
	}
}

func validate(cfg *Config) error {
	if cfg.Server.Port <= 0 {
		return fmt.Errorf("server.port must be greater than 0")
	}
	if cfg.Server.DataDir == "" {
		return fmt.Errorf("server.data_dir is required")
	}
	if len(cfg.Auth.AllowedUserIDs) == 0 {
		return fmt.Errorf("auth.allowed_user_ids must not be empty")
	}
	if cfg.Provider.BaseURL == "" {
		return fmt.Errorf("provider.base_url is required")
	}
	if cfg.Provider.APIKey == "" {
		return fmt.Errorf("provider.api_key is required")
	}
	if len(cfg.Models) == 0 {
		return fmt.Errorf("models must not be empty")
	}
	seen := map[string]struct{}{}
	for _, model := range cfg.Models {
		if model.ID == "" || model.Name == "" {
			return fmt.Errorf("each model requires id and name")
		}
		if _, ok := seen[model.ID]; ok {
			return fmt.Errorf("duplicate model id: %s", model.ID)
		}
		seen[model.ID] = struct{}{}
	}
	if cfg.TitleModel.ID != "" {
		if _, ok := seen[cfg.TitleModel.ID]; !ok {
			return fmt.Errorf("title_model.id must exist in models")
		}
	}
	return nil
}

func trimUsers(cfg *Config) {
	users := make([]string, 0, len(cfg.Auth.AllowedUserIDs))
	for _, userID := range cfg.Auth.AllowedUserIDs {
		trimmed := strings.TrimSpace(userID)
		if trimmed != "" {
			users = append(users, trimmed)
		}
	}
	cfg.Auth.AllowedUserIDs = users
}

func trimModels(cfg *Config) {
	models := make([]ModelConfig, 0, len(cfg.Models))
	for _, model := range cfg.Models {
		model.ID = strings.TrimSpace(model.ID)
		model.Name = strings.TrimSpace(model.Name)
		model.Avatar = strings.TrimSpace(model.Avatar)
		if model.ID != "" && model.Name != "" {
			models = append(models, model)
		}
	}
	cfg.Models = models
	cfg.TitleModel.ID = strings.TrimSpace(cfg.TitleModel.ID)
	cfg.TitleModel.Name = strings.TrimSpace(cfg.TitleModel.Name)
	cfg.TitleModel.Prompt = strings.TrimSpace(cfg.TitleModel.Prompt)
}

func (c *Config) IsAllowedUser(userID string) bool {
	for _, allowed := range c.Auth.AllowedUserIDs {
		if allowed == userID {
			return true
		}
	}
	return false
}

func (c *Config) ModelByID(id string) (ModelConfig, bool) {
	for _, model := range c.Models {
		if model.ID == id {
			return model, true
		}
	}
	return ModelConfig{}, false
}
