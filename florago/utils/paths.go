package utils

import (
	"fmt"
	"os"
	"path/filepath"
)

// GetFloraGoHome returns the FloraGo home directory ($HOME/.florago)
func GetFloraGoHome() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	return filepath.Join(homeDir, ".florago"), nil
}

// GetFloraGoTempDir returns the FloraGo temp directory ($HOME/.florago/tmp)
func GetFloraGoTempDir() (string, error) {
	floragoHome, err := GetFloraGoHome()
	if err != nil {
		return "", err
	}
	tmpDir := filepath.Join(floragoHome, "tmp")

	// Ensure it exists
	if err := CreateDirectory(tmpDir); err != nil {
		return "", fmt.Errorf("failed to create temp directory: %w", err)
	}

	return tmpDir, nil
}

// GetFloraGoBinDir returns the FloraGo bin directory ($HOME/.florago/bin)
func GetFloraGoBinDir() (string, error) {
	floragoHome, err := GetFloraGoHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(floragoHome, "bin"), nil
}

// GetFloraGoDataDir returns the FloraGo data directory ($HOME/.florago/data)
func GetFloraGoDataDir() (string, error) {
	floragoHome, err := GetFloraGoHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(floragoHome, "data"), nil
}

// GetFloraGoLogsDir returns the FloraGo logs directory ($HOME/.florago/logs)
func GetFloraGoLogsDir() (string, error) {
	floragoHome, err := GetFloraGoHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(floragoHome, "logs"), nil
}

// EnsureFloraGoDirectories creates all necessary FloraGo directories
func EnsureFloraGoDirectories() error {
	dirs := []string{"bin", "tmp", "data", "logs"}

	floragoHome, err := GetFloraGoHome()
	if err != nil {
		return err
	}

	for _, dir := range dirs {
		dirPath := filepath.Join(floragoHome, dir)
		if err := CreateDirectory(dirPath); err != nil {
			return fmt.Errorf("failed to create %s: %w", dirPath, err)
		}
	}

	return nil
}
