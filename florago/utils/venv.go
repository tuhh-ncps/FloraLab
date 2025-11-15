package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// VenvManager handles Python virtual environment operations
type VenvManager struct {
	logger     *Logger
	pythonPath string
	venvPath   string
}

// NewVenvManager creates a new virtual environment manager
func NewVenvManager(pythonPath string, logger *Logger) *VenvManager {
	return &VenvManager{
		logger:     logger,
		pythonPath: pythonPath,
	}
}

// CreateVenv creates a virtual environment at the specified path
func (v *VenvManager) CreateVenv(venvPath string) error {
	v.venvPath = venvPath

	// Check if venv already exists
	if _, err := os.Stat(venvPath); err == nil {
		v.logger.Info("Virtual environment already exists at: %s", venvPath)
		return nil
	}

	v.logger.Info("Creating virtual environment at: %s", venvPath)

	// Ensure parent directory exists
	parentDir := filepath.Dir(venvPath)
	if err := CreateDirectory(parentDir); err != nil {
		return fmt.Errorf("failed to create parent directory: %w", err)
	}

	// Create virtual environment using python -m venv
	cmd := exec.Command(v.pythonPath, "-m", "venv", venvPath)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to create virtual environment: %w\nOutput: %s", err, string(output))
	}

	v.logger.Success("Virtual environment created successfully")
	return nil
}

// GetVenvPythonPath returns the path to the Python executable in the venv
func (v *VenvManager) GetVenvPythonPath() string {
	if v.venvPath == "" {
		return ""
	}
	return filepath.Join(v.venvPath, "bin", "python")
}

// GetVenvActivateScript returns the path to the activation script
func (v *VenvManager) GetVenvActivateScript() string {
	if v.venvPath == "" {
		return ""
	}
	return filepath.Join(v.venvPath, "bin", "activate")
}

// SetVenvPath sets the virtual environment path
func (v *VenvManager) SetVenvPath(venvPath string) {
	v.venvPath = venvPath
}

// InstallPackage installs a Python package into the virtual environment
func (v *VenvManager) InstallPackage(packageName string) error {
	if v.venvPath == "" {
		return fmt.Errorf("virtual environment path not set")
	}

	pipPath := filepath.Join(v.venvPath, "bin", "pip")

	v.logger.Info("Installing %s...", packageName)

	cmd := exec.Command(pipPath, "install", packageName)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install %s: %w\nOutput: %s", packageName, err, string(output))
	}

	v.logger.Success("Installed %s", packageName)
	return nil
}

// InstallPackages installs multiple Python packages into the virtual environment
func (v *VenvManager) InstallPackages(packages []string) error {
	for _, pkg := range packages {
		if err := v.InstallPackage(pkg); err != nil {
			return err
		}
	}
	return nil
}

// UpgradePip upgrades pip in the virtual environment
func (v *VenvManager) UpgradePip() error {
	if v.venvPath == "" {
		return fmt.Errorf("virtual environment path not set")
	}

	pipPath := filepath.Join(v.venvPath, "bin", "pip")

	v.logger.Info("Upgrading pip...")

	cmd := exec.Command(pipPath, "install", "--upgrade", "pip")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to upgrade pip: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// VerifyVenv checks if the virtual environment is valid
func (v *VenvManager) VerifyVenv(venvPath string) bool {
	pythonPath := filepath.Join(venvPath, "bin", "python")
	if _, err := os.Stat(pythonPath); err != nil {
		return false
	}

	activateScript := filepath.Join(venvPath, "bin", "activate")
	if _, err := os.Stat(activateScript); err != nil {
		return false
	}

	return true
}

// GetFloraGoVenvDir returns the FloraGo venv directory ($HOME/.florago/venv)
func GetFloraGoVenvDir() (string, error) {
	floragoHome, err := GetFloraGoHome()
	if err != nil {
		return "", err
	}
	return filepath.Join(floragoHome, "venv"), nil
}

// GetFlowerAIVenvPath returns the path to the flowerai-env virtual environment
func GetFlowerAIVenvPath() (string, error) {
	venvDir, err := GetFloraGoVenvDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(venvDir, "flowerai-env"), nil
}
