package utils

import (
	"encoding/json"
	"fmt"
)

// VenvConfig represents virtual environment configuration
type VenvConfig struct {
	Name       string `json:"name"`
	Path       string `json:"path"`
	PythonPath string `json:"python_path"`
	Activate   string `json:"activate"`
}

// Config represents the FloraGo configuration
type Config struct {
	Version     string            `json:"version"`
	ProjectName string            `json:"project_name"`
	Settings    map[string]string `json:"settings"`
	Venv        *VenvConfig       `json:"venv,omitempty"`
}

// DefaultConfig returns a default configuration
func DefaultConfig(projectName string) *Config {
	return &Config{
		Version:     "1.0.0",
		ProjectName: projectName,
		Settings: map[string]string{
			"environment": "development",
		},
	}
}

// SetVenv sets the virtual environment configuration
func (c *Config) SetVenv(name, path, pythonPath, activate string) {
	c.Venv = &VenvConfig{
		Name:       name,
		Path:       path,
		PythonPath: pythonPath,
		Activate:   activate,
	}
}

// ToJSON converts the config to JSON string
func (c *Config) ToJSON() (string, error) {
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal config: %w", err)
	}
	return string(data), nil
}

// FromJSON loads config from JSON string
func FromJSON(jsonStr string) (*Config, error) {
	var config Config
	if err := json.Unmarshal([]byte(jsonStr), &config); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}
	return &config, nil
}
