package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// DebuggerManager handles embedded debugger functionality
type DebuggerManager struct {
	logger       *Logger
	delveVersion string
	binaryPath   string
}

// NewDebuggerManager creates a new debugger manager
func NewDebuggerManager(logger *Logger) *DebuggerManager {
	if logger == nil {
		logger = DefaultLogger
	}
	return &DebuggerManager{
		logger:       logger,
		delveVersion: "v1.21.2", // Latest stable version
	}
}

// GetDelveBinaryPath returns the path where Delve should be installed
// Always uses $HOME/.florago/bin to avoid filling up /tmp on SLURM login nodes
func (d *DebuggerManager) GetDelveBinaryPath() string {
	if d.binaryPath != "" {
		return d.binaryPath
	}

	binDir, err := GetFloraGoBinDir()
	if err != nil {
		d.logger.Warning("Could not determine bin directory: %v", err)
		// Last resort fallback
		homeDir := os.Getenv("HOME")
		if homeDir == "" {
			homeDir = "/tmp"
		}
		binDir = filepath.Join(homeDir, ".florago", "bin")
	}

	d.binaryPath = filepath.Join(binDir, "dlv")
	return d.binaryPath
}

// IsDelveInstalled checks if Delve is available
func (d *DebuggerManager) IsDelveInstalled() bool {
	// Check system PATH first
	if _, err := exec.LookPath("dlv"); err == nil {
		d.logger.Debug("Found dlv in system PATH")
		d.binaryPath = "dlv"
		return true
	}

	// Check local installation
	localPath := d.GetDelveBinaryPath()
	if _, err := os.Stat(localPath); err == nil {
		d.logger.Debug("Found dlv at: %s", localPath)
		return true
	}

	return false
}

// InstallDelve installs Delve using go install
func (d *DebuggerManager) InstallDelve() error {
	d.logger.Info("Installing Delve %s...", d.delveVersion)

	// Check if Go is available
	if _, err := exec.LookPath("go"); err != nil {
		return fmt.Errorf("Go is not installed or not in PATH. Please install Go first")
	}

	// Create directories
	binDir := filepath.Dir(d.GetDelveBinaryPath())
	if err := CreateDirectory(binDir); err != nil {
		return err
	}

	// Use go install to build and install Delve
	packagePath := fmt.Sprintf("github.com/go-delve/delve/cmd/dlv@%s", d.delveVersion)
	d.logger.Info("Building Delve from source...")
	d.logger.Debug("Running: go install %s", packagePath)

	// Install to temporary GOBIN
	cmd := exec.Command("go", "install", packagePath)
	cmd.Env = append(os.Environ(), fmt.Sprintf("GOBIN=%s", binDir))

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to install Delve: %v\nOutput: %s", err, string(output))
	}

	// Verify installation
	dlvPath := d.GetDelveBinaryPath()
	if _, err := os.Stat(dlvPath); err != nil {
		return fmt.Errorf("Delve binary not found after installation: %s", dlvPath)
	}

	d.logger.Success("Delve installed at: %s", dlvPath)
	return nil
}

// EnsureDelve ensures Delve is available, installs if needed
func (d *DebuggerManager) EnsureDelve() error {
	if d.IsDelveInstalled() {
		d.logger.Success("Delve is available")
		return nil
	}

	d.logger.Warning("Delve not found, installing...")
	return d.InstallDelve()
}

// StartDebugServer starts a Delve debug server
func (d *DebuggerManager) StartDebugServer(binaryPath string, port int, args []string) error {
	if err := d.EnsureDelve(); err != nil {
		return err
	}

	dlvPath := d.GetDelveBinaryPath()
	if d.binaryPath == "dlv" {
		dlvPath = "dlv"
	}

	cmdArgs := []string{
		"exec",
		binaryPath,
		"--headless",
		fmt.Sprintf("--listen=:%d", port),
		"--api-version=2",
		"--accept-multiclient",
	}

	if len(args) > 0 {
		cmdArgs = append(cmdArgs, "--")
		cmdArgs = append(cmdArgs, args...)
	}

	d.logger.Info("Starting debug server on port %d...", port)
	d.logger.Debug("Command: %s %v", dlvPath, cmdArgs)

	cmd := exec.Command(dlvPath, cmdArgs...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	return cmd.Run()
}

// GetDelveVersion returns the installed Delve version
func (d *DebuggerManager) GetDelveVersion() (string, error) {
	dlvPath := d.GetDelveBinaryPath()
	if d.binaryPath == "dlv" {
		dlvPath = "dlv"
	}

	cmd := exec.Command(dlvPath, "version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}

	return string(output), nil
}
