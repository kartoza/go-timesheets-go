package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/kartoza/go-timesheets-go/internal/api"
)

// Config represents the application configuration
type Config struct {
	API      api.Config `json:"api"`
	UI       UIConfig   `json:"ui"`
	Storage  string     `json:"storage_path"`
	LogLevel string     `json:"log_level"`
}

// UIConfig holds UI-related configuration
type UIConfig struct {
	Theme        string `json:"theme"`
	ShowHelp     bool   `json:"show_help"`
	RefreshRate  int    `json:"refresh_rate_seconds"`
	DefaultView  string `json:"default_view"`
}

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	homeDir, _ := os.UserHomeDir()
	
	return Config{
		API: api.Config{
			BaseURL:  "http://localhost:8000",
			Username: "",
			Password: "",
			Timeout:  30,
		},
		UI: UIConfig{
			Theme:       "default",
			ShowHelp:    true,
			RefreshRate: 30,
			DefaultView: "menu",
		},
		Storage:  filepath.Join(homeDir, ".kartoza-timesheet"),
		LogLevel: "info",
	}
}

// LoadConfig loads configuration from file or creates default
func LoadConfig() (*Config, error) {
	configPath := getConfigPath()
	
	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config directory: %w", err)
	}

	// Check if config file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		// Create default config
		defaultCfg := DefaultConfig()
		if err := SaveConfig(&defaultCfg); err != nil {
			return nil, fmt.Errorf("failed to save default config: %w", err)
		}
		return &defaultCfg, nil
	}

	// Load existing config
	file, err := os.Open(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to decode config: %w", err)
	}

	// Validate and fill in any missing fields
	if config.Storage == "" {
		config.Storage = DefaultConfig().Storage
	}
	if config.LogLevel == "" {
		config.LogLevel = "info"
	}
	if config.API.Timeout == 0 {
		config.API.Timeout = 30
	}
	if config.UI.RefreshRate == 0 {
		config.UI.RefreshRate = 30
	}

	return &config, nil
}

// SaveConfig saves configuration to file
func SaveConfig(config *Config) error {
	configPath := getConfigPath()
	
	// Create config directory if it doesn't exist
	configDir := filepath.Dir(configPath)
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	file, err := os.Create(configPath)
	if err != nil {
		return fmt.Errorf("failed to create config file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(config); err != nil {
		return fmt.Errorf("failed to encode config: %w", err)
	}

	return nil
}

// getConfigPath returns the configuration file path
func getConfigPath() string {
	// Try XDG config directory first
	if configHome := os.Getenv("XDG_CONFIG_HOME"); configHome != "" {
		return filepath.Join(configHome, "kartoza-timesheets", "config.json")
	}

	// Fall back to home directory
	homeDir, err := os.UserHomeDir()
	if err != nil {
		// Last resort - current directory
		return "config.json"
	}

	return filepath.Join(homeDir, ".config", "kartoza-timesheets", "config.json")
}

// IsAPIEnabled checks if API configuration is complete
func (c *Config) IsAPIEnabled() bool {
	return c.API.BaseURL != "" && c.API.Username != "" && c.API.Password != ""
}

// GetStorageDir returns the storage directory path
func (c *Config) GetStorageDir() string {
	if c.Storage == "" {
		return DefaultConfig().Storage
	}
	return c.Storage
}

// EnsureStorageDir creates the storage directory if it doesn't exist
func (c *Config) EnsureStorageDir() error {
	storageDir := c.GetStorageDir()
	return os.MkdirAll(storageDir, 0755)
}

// Validate checks if the configuration is valid
func (c *Config) Validate() error {
	if c.API.BaseURL == "" {
		return fmt.Errorf("API base URL is required")
	}

	if c.Storage == "" {
		return fmt.Errorf("storage path is required")
	}

	if c.API.Timeout <= 0 {
		return fmt.Errorf("API timeout must be positive")
	}

	if c.UI.RefreshRate <= 0 {
		return fmt.Errorf("UI refresh rate must be positive")
	}

	return nil
}

// UpdateAPIConfig updates the API configuration
func (c *Config) UpdateAPIConfig(baseURL, username, password string) {
	c.API.BaseURL = baseURL
	c.API.Username = username
	c.API.Password = password
}

// GetConfigDir returns the configuration directory path
func GetConfigDir() string {
	configPath := getConfigPath()
	return filepath.Dir(configPath)
}

// Example configuration file content for documentation
const ExampleConfigJSON = `{
  "api": {
    "base_url": "http://localhost:8000",
    "username": "your-username",
    "password": "your-password",
    "timeout_seconds": 30
  },
  "ui": {
    "theme": "default",
    "show_help": true,
    "refresh_rate_seconds": 30,
    "default_view": "menu"
  },
  "storage_path": "/home/user/.config/kartoza-timesheets",
  "log_level": "info"
}`

// TokenStorage represents stored authentication token
type TokenStorage struct {
	Token    string `json:"token"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"` // Stored for session auth (file is mode 0600)
	BaseURL  string `json:"base_url"`
}

// GetTokenPath returns the path to the token file
func GetTokenPath() string {
	// Use the config directory + token file
	configDir := getConfigPath()
	return filepath.Join(filepath.Dir(configDir), "auth_token.json")
}

// SaveToken saves the authentication token to disk
func SaveToken(token, username, baseURL string) error {
	return SaveTokenWithPassword(token, username, "", baseURL)
}

// SaveTokenWithPassword saves the authentication token and password to disk
func SaveTokenWithPassword(token, username, password, baseURL string) error {
	tokenPath := GetTokenPath()

	// Create config directory if it doesn't exist
	tokenDir := filepath.Dir(tokenPath)
	if err := os.MkdirAll(tokenDir, 0700); err != nil {
		return fmt.Errorf("failed to create token directory: %w", err)
	}

	tokenStorage := TokenStorage{
		Token:    token,
		Username: username,
		Password: password,
		BaseURL:  baseURL,
	}

	file, err := os.OpenFile(tokenPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return fmt.Errorf("failed to create token file: %w", err)
	}
	defer file.Close()

	encoder := json.NewEncoder(file)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(tokenStorage); err != nil {
		return fmt.Errorf("failed to encode token: %w", err)
	}

	return nil
}

// LoadToken loads the authentication token from disk
func LoadToken() (*TokenStorage, error) {
	tokenPath := GetTokenPath()

	// Check if token file exists
	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		return nil, nil // No token saved
	}

	file, err := os.Open(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open token file: %w", err)
	}
	defer file.Close()

	var tokenStorage TokenStorage
	decoder := json.NewDecoder(file)
	if err := decoder.Decode(&tokenStorage); err != nil {
		return nil, fmt.Errorf("failed to decode token: %w", err)
	}

	return &tokenStorage, nil
}

// DeleteToken removes the saved authentication token
func DeleteToken() error {
	tokenPath := GetTokenPath()

	if _, err := os.Stat(tokenPath); os.IsNotExist(err) {
		return nil // Nothing to delete
	}

	if err := os.Remove(tokenPath); err != nil {
		return fmt.Errorf("failed to delete token file: %w", err)
	}

	return nil
}

// HasToken checks if a token is saved
func HasToken() bool {
	tokenPath := GetTokenPath()
	_, err := os.Stat(tokenPath)
	return err == nil
}
